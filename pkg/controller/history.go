package controller

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	_ "github.com/lib/pq"

	"fixora/pkg/config"
)

type Incident struct {
	Timestamp  time.Time `json:"timestamp"`
	Reason     string    `json:"reason"`
	RootCause  string    `json:"root_cause"`
	AppliedFix string    `json:"applied_fix,omitempty"` // The patch we generated
}

type PodHistory struct {
	Incidents []Incident `json:"incidents"`
}

type PredictionState struct {
	LastAlertTime  time.Time
	LastGrowthRate float64
}

type historyCache struct {
	config *config.Config
	db     *sql.DB
}

func newHistoryCache(cfg *config.Config) *historyCache {
	hc := &historyCache{
		config: cfg,
	}

	if cfg.DBHost != "" {
		connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
			cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName)
		db, err := sql.Open("postgres", connStr)
		if err != nil {
			slog.Error("Failed to open DB connection", "error", err)
		} else {
			if err := db.Ping(); err != nil {
				slog.Error("Failed to ping DB", "error", err)
			} else {
				hc.db = db
				hc.initDB()
			}
		}
	} else {
		slog.Warn("DBHost is empty, history cache will be disabled")
	}

	return hc
}

func (h *historyCache) initDB() {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS incident_history (
			id SERIAL PRIMARY KEY,
			namespace VARCHAR(255) NOT NULL,
			pod_name VARCHAR(255) NOT NULL,
			timestamp TIMESTAMP NOT NULL,
			reason TEXT NOT NULL,
			root_cause TEXT NOT NULL,
			applied_fix TEXT
		);`,
		`CREATE INDEX IF NOT EXISTS idx_incident_pod ON incident_history (namespace, pod_name);`,
		`CREATE TABLE IF NOT EXISTS predictions (
			id SERIAL PRIMARY KEY,
			namespace VARCHAR(255) NOT NULL,
			pod_name VARCHAR(255) NOT NULL,
			last_alert_time TIMESTAMP NOT NULL,
			last_growth_rate DOUBLE PRECISION NOT NULL,
			UNIQUE(namespace, pod_name)
		);`,
	}
	for _, q := range queries {
		_, err := h.db.Exec(q)
		if err != nil {
			slog.Error("Failed to initialize database schema", "query", q, "error", err)
		}
	}
}

func (h *historyCache) getFromDB(ctx context.Context, namespace, podName string) (*PodHistory, bool) {
	if h.db == nil {
		return nil, false
	}

	query := `SELECT timestamp, reason, root_cause, COALESCE(applied_fix, '') FROM incident_history WHERE namespace = $1 AND pod_name = $2 ORDER BY timestamp ASC`
	rows, err := h.db.QueryContext(ctx, query, namespace, podName)
	if err != nil {
		slog.Error("Failed to query DB for incidents", "error", err)
		return nil, false
	}
	defer rows.Close()

	var incidents []Incident
	for rows.Next() {
		var inc Incident
		if err := rows.Scan(&inc.Timestamp, &inc.Reason, &inc.RootCause, &inc.AppliedFix); err == nil {
			incidents = append(incidents, inc)
		}
	}

	if len(incidents) > 0 {
		return &PodHistory{Incidents: incidents}, true
	}
	return nil, false
}

func (h *historyCache) saveToDB(ctx context.Context, namespace, podName string, inc Incident) {
	if h.db == nil {
		return
	}

	query := `INSERT INTO incident_history (namespace, pod_name, timestamp, reason, root_cause, applied_fix) VALUES ($1, $2, $3, $4, $5, $6)`
	_, err := h.db.ExecContext(ctx, query, namespace, podName, inc.Timestamp, inc.Reason, inc.RootCause, inc.AppliedFix)
	if err != nil {
		slog.Error("Failed to insert incident to DB", "error", err)
	}
}

func (h *historyCache) updatePatchDB(ctx context.Context, namespace, podName, patch string) {
	if h.db == nil {
		return
	}

	query := `
		UPDATE incident_history 
		SET applied_fix = $1 
		WHERE id = (
			SELECT id FROM incident_history 
			WHERE namespace = $2 AND pod_name = $3 
			ORDER BY timestamp DESC LIMIT 1
		)
	`
	_, err := h.db.ExecContext(ctx, query, patch, namespace, podName)
	if err != nil {
		slog.Error("Failed to update patch in DB", "error", err)
	}
}

func (h *historyCache) Get(ctx context.Context, namespace, podName string) (*PodHistory, bool) {
	if h.db != nil {
		return h.getFromDB(ctx, namespace, podName)
	}
	return nil, false
}

func (h *historyCache) Update(ctx context.Context, namespace, podName, reason, rootCause string) {
	inc := Incident{
		Timestamp: time.Now(),
		Reason:    reason,
		RootCause: rootCause,
	}

	if h.db != nil {
		h.saveToDB(ctx, namespace, podName, inc)
	}
}

func (h *historyCache) UpdatePatch(ctx context.Context, namespace, podName, patch string) {
	if h.db != nil {
		h.updatePatchDB(ctx, namespace, podName, patch)
	}
}

func (h *historyCache) GetPredictionState(ctx context.Context, namespace, podName string) (*PredictionState, bool) {
	if h.db == nil {
		return nil, false
	}

	query := `SELECT last_alert_time, last_growth_rate FROM predictions WHERE namespace = $1 AND pod_name = $2`
	var state PredictionState
	err := h.db.QueryRowContext(ctx, query, namespace, podName).Scan(&state.LastAlertTime, &state.LastGrowthRate)
	if err == sql.ErrNoRows {
		return nil, false
	} else if err != nil {
		slog.Error("Failed to query prediction state", "error", err)
		return nil, false
	}

	return &state, true
}

func (h *historyCache) UpdatePredictionState(ctx context.Context, namespace, podName string, alertTime time.Time, growthRate float64) {
	if h.db == nil {
		return
	}

	query := `
		INSERT INTO predictions (namespace, pod_name, last_alert_time, last_growth_rate)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (namespace, pod_name)
		DO UPDATE SET last_alert_time = EXCLUDED.last_alert_time, last_growth_rate = EXCLUDED.last_growth_rate
	`
	_, err := h.db.ExecContext(ctx, query, namespace, podName, alertTime, growthRate)
	if err != nil {
		slog.Error("Failed to update prediction state", "error", err)
	}
}

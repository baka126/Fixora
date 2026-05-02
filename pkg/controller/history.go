package controller

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sync"
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
	
	// In-memory fallback for alert tracking when DB is not configured
	recentAlerts map[string]time.Time
	alertMu      sync.RWMutex
}

func newHistoryCache(cfg *config.Config) *historyCache {
	hc := &historyCache{
		config:       cfg,
		recentAlerts: make(map[string]time.Time),
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
		slog.Warn("DBHost is empty, history cache will be limited to in-memory for some features")
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
		`CREATE TABLE IF NOT EXISTS pending_fixes (
			callback_id VARCHAR(255) PRIMARY KEY,
			created_at TIMESTAMP NOT NULL,
			vcs_type TEXT NOT NULL,
			vcs_token TEXT,
			pod_namespace TEXT NOT NULL,
			pod_name TEXT NOT NULL,
			title TEXT NOT NULL,
			body TEXT NOT NULL,
			head TEXT NOT NULL,
			base TEXT NOT NULL,
			repo_owner TEXT NOT NULL,
			repo_name TEXT NOT NULL,
			file_path TEXT NOT NULL,
			new_content BYTEA NOT NULL,
			commit_message TEXT NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_pending_fixes_created_at ON pending_fixes (created_at);`,
		`CREATE TABLE IF NOT EXISTS autofix_events (
			id SERIAL PRIMARY KEY,
			created_at TIMESTAMP NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_autofix_events_created_at ON autofix_events (created_at);`,
		`CREATE TABLE IF NOT EXISTS leader_checkpoints (
			id SERIAL PRIMARY KEY,
			leader_identity VARCHAR(255) NOT NULL,
			action_type VARCHAR(255) NOT NULL,
			details TEXT,
			timestamp TIMESTAMP NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_leader_checkpoints_timestamp ON leader_checkpoints (timestamp);`,
		`CREATE TABLE IF NOT EXISTS processed_alerts (
			alert_key VARCHAR(512) PRIMARY KEY,
			last_processed_at TIMESTAMP NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_processed_alerts_timestamp ON processed_alerts (last_processed_at);`,
	}
	for _, q := range queries {
		_, err := h.db.Exec(q)
		if err != nil {
			slog.Error("Failed to initialize database schema", "query", q, "error", err)
		}
	}
}

func (h *historyCache) HasDB() bool {
	return h.db != nil
}

func (h *historyCache) DB() *sql.DB {
	return h.db
}

func (h *historyCache) RecordActionCheckpoint(ctx context.Context, identity string, actionType string, details string) {
	if h.db == nil {
		return
	}
	query := `INSERT INTO leader_checkpoints (leader_identity, action_type, details, timestamp) VALUES ($1, $2, $3, $4)`
	_, err := h.db.ExecContext(ctx, query, identity, actionType, details, time.Now())
	if err != nil {
		slog.Error("Failed to record leader checkpoint to DB", "error", err)
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

func (h *historyCache) SavePendingFix(ctx context.Context, callbackID string, fix PendingFix) error {
	if h.db == nil {
		return fmt.Errorf("database not configured")
	}
	query := `
		INSERT INTO pending_fixes (
			callback_id, created_at, vcs_type, vcs_token, pod_namespace, pod_name,
			title, body, head, base, repo_owner, repo_name, file_path, new_content, commit_message
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11, $12, $13, $14, $15
		)
	`
	_, err := h.db.ExecContext(ctx, query,
		callbackID, fix.CreatedAt, fix.VCSType, fix.VCSToken, fix.PodNamespace, fix.PodName,
		fix.Options.Title, fix.Options.Body, fix.Options.Head, fix.Options.Base,
		fix.Options.RepoOwner, fix.Options.RepoName, fix.Options.FilePath, fix.Options.NewContent, fix.Options.CommitMessage,
	)
	return err
}

func (h *historyCache) TakePendingFix(ctx context.Context, callbackID string) (PendingFix, bool, error) {
	if h.db == nil {
		return PendingFix{}, false, fmt.Errorf("database not configured")
	}
	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		return PendingFix{}, false, err
	}
	defer tx.Rollback()

	query := `
		SELECT created_at, vcs_type, COALESCE(vcs_token, ''), pod_namespace, pod_name,
		       title, body, head, base, repo_owner, repo_name, file_path, new_content, commit_message
		FROM pending_fixes
		WHERE callback_id = $1
		FOR UPDATE
	`
	var fix PendingFix
	var newContent []byte
	err = tx.QueryRowContext(ctx, query, callbackID).Scan(
		&fix.CreatedAt, &fix.VCSType, &fix.VCSToken, &fix.PodNamespace, &fix.PodName,
		&fix.Options.Title, &fix.Options.Body, &fix.Options.Head, &fix.Options.Base,
		&fix.Options.RepoOwner, &fix.Options.RepoName, &fix.Options.FilePath, &newContent, &fix.Options.CommitMessage,
	)
	if err == sql.ErrNoRows {
		return PendingFix{}, false, nil
	}
	if err != nil {
		return PendingFix{}, false, err
	}
	fix.Options.NewContent = newContent

	if _, err := tx.ExecContext(ctx, `DELETE FROM pending_fixes WHERE callback_id = $1`, callbackID); err != nil {
		return PendingFix{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return PendingFix{}, false, err
	}
	return fix, true, nil
}

func (h *historyCache) CleanupExpiredPendingFixes(ctx context.Context, ttl time.Duration) error {
	if h.db == nil {
		return fmt.Errorf("database not configured")
	}
	if ttl <= 0 {
		return nil
	}
	_, err := h.db.ExecContext(ctx, `DELETE FROM pending_fixes WHERE created_at < $1`, time.Now().Add(-ttl))
	return err
}

func (h *historyCache) AllowAutoFixPR(ctx context.Context, maxPerHour int) (bool, error) {
	if h.db == nil {
		return false, fmt.Errorf("database not configured")
	}
	if maxPerHour <= 0 {
		return true, nil
	}
	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM autofix_events WHERE created_at < $1`, time.Now().Add(-1*time.Hour)); err != nil {
		return false, err
	}

	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM autofix_events`).Scan(&count); err != nil {
		return false, err
	}
	if count >= maxPerHour {
		if err := tx.Commit(); err != nil {
			return false, err
		}
		return false, nil
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO autofix_events (created_at) VALUES ($1)`, time.Now()); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func (h *historyCache) IsAlertRecentlyProcessed(ctx context.Context, ns, pod, alertname string, window time.Duration) bool {
	key := fmt.Sprintf("%s/%s/%s", ns, pod, alertname)
	if h.db != nil {
		var lastProcessed time.Time
		err := h.db.QueryRowContext(ctx, `SELECT last_processed_at FROM processed_alerts WHERE alert_key = $1`, key).Scan(&lastProcessed)
		if err == sql.ErrNoRows {
			return false
		} else if err != nil {
			slog.Error("Failed to query processed alerts", "error", err)
			return false
		}
		return time.Since(lastProcessed) < window
	}

	h.alertMu.RLock()
	defer h.alertMu.RUnlock()
	last, exists := h.recentAlerts[key]
	if !exists {
		return false
	}
	return time.Since(last) < window
}

func (h *historyCache) MarkAlertProcessed(ctx context.Context, ns, pod, alertname string) {
	key := fmt.Sprintf("%s/%s/%s", ns, pod, alertname)
	if h.db != nil {
		query := `
			INSERT INTO processed_alerts (alert_key, last_processed_at)
			VALUES ($1, $2)
			ON CONFLICT (alert_key)
			DO UPDATE SET last_processed_at = EXCLUDED.last_processed_at
		`
		_, err := h.db.ExecContext(ctx, query, key, time.Now())
		if err != nil {
			slog.Error("Failed to mark alert as processed in DB", "error", err)
		}
		return
	}

	h.alertMu.Lock()
	defer h.alertMu.Unlock()
	h.recentAlerts[key] = time.Now()
}

package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"fixora/pkg/models"
)

func (s *Server) handleGetActiveAlerts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if _, ok := requireBearerAuth(w, r); !ok {
		return
	}

	if !s.config.AlertmanagerEnabled || s.config.AlertmanagerURL == "" {
		http.Error(w, "alertmanager is not configured", http.StatusServiceUnavailable)
		return
	}

	alerts, err := s.controller.ActiveAlertDecisions(r.Context())
	if err != nil {
		slog.Error("Failed to fetch active Alertmanager alerts", "error", err)
		http.Error(w, "failed to fetch alerts", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	if err := json.NewEncoder(w).Encode(alerts); err != nil {
		slog.Error("Failed to encode alerts response", "error", err)
	}
}

func (s *Server) handleActiveAlertAction(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireBearerAuth(w, r)
	if !ok {
		return
	}
	if claims.Role != models.RoleAdmin && claims.Role != models.RoleOperator {
		http.Error(w, "forbidden: operator role required", http.StatusForbidden)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/alerts/active/")
	alertID, action, ok := strings.Cut(path, "/")
	if !ok || alertID == "" || action != "include" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	alert, err := s.controller.IncludeActiveAlert(r.Context(), alertID)
	if err != nil {
		slog.Warn("Failed to include active alert", "alert_id", alertID, "error", err)
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(alert); err != nil {
		slog.Error("Failed to encode included alert response", "error", err)
	}
}

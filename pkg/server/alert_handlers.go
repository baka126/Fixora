package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
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

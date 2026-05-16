package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"fixora/pkg/auth"
)

func (s *Server) handleInvestigationDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if _, ok := requireBearerAuth(w, r); !ok {
		return
	}

	// Expecting /api/v1/audit/investigations/{id}
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 5 {
		http.Error(w, "invalid investigation ID", http.StatusBadRequest)
		return
	}

	idStr := pathParts[4]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid investigation ID format", http.StatusBadRequest)
		return
	}

	inv, err := s.controller.GetInvestigation(r.Context(), id)
	if err != nil {
		if err.Error() == "investigation not found" {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		slog.Error("Failed to fetch investigation detail", "id", id, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(inv)
}

func requireBearerAuth(w http.ResponseWriter, r *http.Request) (*auth.Claims, bool) {
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return nil, false
	}
	tokenStr := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
	claims, err := auth.ValidateToken(tokenStr)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return nil, false
	}
	return claims, true
}

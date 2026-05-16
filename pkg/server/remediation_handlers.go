package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"fixora/pkg/models"
)

func (s *Server) handleRemediations(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireBearerAuth(w, r)
	if !ok {
		return
	}

	// Expecting /api/v1/remediations/{id}/diff or /api/v1/remediations/{id}/commit
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 5 {
		http.Error(w, "invalid remediation endpoint", http.StatusBadRequest)
		return
	}

	idStr := pathParts[3]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid remediation ID format", http.StatusBadRequest)
		return
	}

	action := pathParts[4]

	switch action {
	case "diff":
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleRemediationDiff(w, r, id)
	case "commit":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if claims.Role != models.RoleAdmin && claims.Role != models.RoleOperator {
			http.Error(w, "forbidden: operator role required", http.StatusForbidden)
			return
		}
		s.handleRemediationCommit(w, r, id)
	default:
		http.Error(w, "not found", http.StatusNotFound)
	}
}

func (s *Server) handleRemediationDiff(w http.ResponseWriter, r *http.Request, id int64) {
	diffs, err := s.controller.GetRemediationDiff(r.Context(), id)
	if err != nil {
		if err.Error() == "remediation not found" {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		if strings.Contains(err.Error(), "no editable changed files") {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		slog.Error("Failed to fetch remediation diff", "id", id, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(diffs)
}

type CommitRequest struct {
	FilePath string `json:"filePath"`
	Content  string `json:"content"`
	Message  string `json:"message"`
}

func (s *Server) handleRemediationCommit(w http.ResponseWriter, r *http.Request, id int64) {
	var req CommitRequest
	body := http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	if err := json.NewDecoder(body).Decode(&req); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	req.FilePath = strings.TrimSpace(req.FilePath)
	req.Message = strings.TrimSpace(req.Message)
	if req.FilePath == "" || req.Message == "" {
		http.Error(w, "filePath and message are required", http.StatusBadRequest)
		return
	}
	if len(req.Message) > 500 {
		http.Error(w, "commit message is too long", http.StatusBadRequest)
		return
	}

	err := s.controller.AppendCommitToRemediation(r.Context(), id, req.FilePath, req.Content, req.Message)
	if err != nil {
		if err.Error() == "remediation not found" {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		if strings.Contains(err.Error(), "not part of the remediation") || strings.Contains(err.Error(), "no editable PR branch") {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if strings.Contains(err.Error(), "not editable") {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		slog.Error("Failed to append commit to remediation", "id", id, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

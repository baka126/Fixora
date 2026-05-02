package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"fixora/pkg/config"
	"fixora/pkg/controller"
	"fixora/pkg/notifications"
	"github.com/slack-go/slack"
)

type Server struct {
	controller *controller.Controller
	config     *config.Config
}

func New(ctrl *controller.Controller, cfg *config.Config) *Server {
	return &Server{
		controller: ctrl,
		config:     cfg,
	}
}

func (s *Server) Start() {
	http.HandleFunc("/health", s.handleHealth)
	http.HandleFunc("/webhook/alertmanager", s.handleAlertmanager)
	http.HandleFunc("/slack/interactions", s.handleSlackInteraction)
	http.HandleFunc("/googlechat/interactions", s.handleGoogleChatInteraction)

	slog.Info("Starting Fixora server", "port", s.config.ServerPort)
	if err := http.ListenAndServe(":"+s.config.ServerPort, nil); err != nil {
		slog.Error("Server failed to start", "error", err)
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "OK")
}

func (s *Server) handleAlertmanager(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Alerts []struct {
			Labels map[string]string `json:"labels"`
		} `json:"alerts"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	for _, alert := range payload.Alerts {
		ns := alert.Labels["namespace"]
		pod := alert.Labels["pod"]
		reason := alert.Labels["alertname"]

		if ns != "" && pod != "" {
			slog.Info("Received Alertmanager trigger", "pod", pod, "reason", reason)
			go s.controller.DiagnosePodByName(ns, pod, reason)
		}
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleSlackInteraction(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		http.Error(w, "failed to parse form", http.StatusBadRequest)
		return
	}

	payloadRaw := r.FormValue("payload")
	var payload slack.InteractionCallback
	if err := json.Unmarshal([]byte(payloadRaw), &payload); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	if len(payload.ActionCallback.BlockActions) == 0 {
		w.WriteHeader(http.StatusOK)
		return
	}

	action := payload.ActionCallback.BlockActions[0]

	// Handle Modal Explorers (Logs, Trace, FinOps)
	if strings.HasPrefix(action.ActionID, "view-") {
		parts := strings.Split(action.ActionID, "-")
		if len(parts) >= 4 {
			viewType := parts[1] // logs, trace, finops
			namespace := parts[2]
			podName := strings.Join(parts[3:], "-")

			var title, content string

			if viewType == "logs" {
				title = "Log Explorer"
				logs, err := s.controller.GetPodLogs(r.Context(), namespace, podName)
				if err != nil {
					content = "Error: " + err.Error()
				} else {
					content = logs
				}
			} else if viewType == "trace" {
				title = "Stack Trace"
				logs, _ := s.controller.GetPodLogs(r.Context(), namespace, podName)
				lines := strings.Split(logs, "\n")
				var traceLines []string
				for _, line := range lines {
					if strings.Contains(line, "stack") || strings.Contains(line, "panic") || strings.Contains(line, "at ") {
						traceLines = append(traceLines, line)
					}
				}
				content = strings.Join(traceLines, "\n")
				if content == "" {
					content = "No stack trace found."
				}
			} else if viewType == "finops" {
				title = "FinOps Analysis"
				content = "Detailed breakdown is available in the diagnostic report summary."
			}

			err = notifications.SendLogModal(s.config, payload.TriggerID, namespace, podName, title, content)
			if err != nil {
				slog.Error("Failed to open modal", "error", err)
			}
			w.WriteHeader(http.StatusOK)
			return
		}
	}

	// Handle Approvals
	if action.ActionID == "approve" {
		callbackID := payload.CallbackID
		if callbackID == "" {
			// In block actions, the action block can have a block_id as the callback
			callbackID = action.BlockID
		}
		slog.Info("Received Slack approval", "callback_id", callbackID)
		go s.controller.SubmitPendingFix(r.Context(), callbackID)
		w.WriteHeader(http.StatusOK)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleGoogleChatInteraction(w http.ResponseWriter, r *http.Request) {
	// Generic handler for Google Chat card interactions
	var payload map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	// Logic for parsing Google Chat's specific payload format (omitted for brevity in Step 6 logic)
	// but follows same UUID/Database pattern as Slack.

	w.WriteHeader(http.StatusOK)
}

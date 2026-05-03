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
		slog.Error("Failed to decode Alertmanager webhook payload", "error", err)
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	slog.Info("Received Alertmanager webhook", "alert_count", len(payload.Alerts))

	for _, alert := range payload.Alerts {
		ns := alert.Labels["namespace"]
		pod := alert.Labels["pod"]
		reason := alert.Labels["alertname"]

		if ns != "" && pod != "" {
			slog.Info("Alertmanager webhook triggered diagnostic", "ns", ns, "pod", pod, "reason", reason)
			go s.controller.DiagnosePodByName(ns, pod, reason)
		} else {
			slog.Debug("Ignoring non-pod Alertmanager alert from webhook", "labels", alert.Labels)
		}
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleSlackInteraction(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		slog.Error("Failed to parse Slack interaction form", "error", err)
		http.Error(w, "failed to parse form", http.StatusBadRequest)
		return
	}

	payloadRaw := r.FormValue("payload")
	var payload slack.InteractionCallback
	if err := json.Unmarshal([]byte(payloadRaw), &payload); err != nil {
		slog.Error("Failed to unmarshal Slack interaction payload", "error", err)
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	if len(payload.ActionCallback.BlockActions) == 0 {
		w.WriteHeader(http.StatusOK)
		return
	}

	action := payload.ActionCallback.BlockActions[0]
	slog.Info("Received Slack interaction", "user", payload.User.Name, "action_id", action.ActionID)

	// Handle Modal Explorers (Logs, Trace, FinOps)
	if strings.HasPrefix(action.ActionID, "view-") {
		parts := strings.Split(action.ActionID, "-")
		if len(parts) >= 4 {
			viewType := parts[1] // logs, trace, finops
			namespace := parts[2]
			podName := strings.Join(parts[3:], "-")

			slog.Info("Opening forensic explorer modal", "type", viewType, "ns", namespace, "pod", podName)

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
				slog.Error("Failed to open Slack modal", "type", viewType, "error", err)
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
		slog.Info("User approved remediation via Slack", "user", payload.User.Name, "callback_id", callbackID)
		go s.controller.SubmitPendingFix(r.Context(), callbackID)
		w.WriteHeader(http.StatusOK)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleGoogleChatInteraction(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Type string `json:"type"`
		Action struct {
			ActionMethodName string            `json:"actionMethodName"`
			Parameters       []struct {
				Key   string `json:"key"`
				Value string `json:"value"`
			} `json:"parameters"`
		} `json:"action"`
		Common struct {
			Parameters map[string]string `json:"parameters"`
		} `json:"common"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		slog.Error("Failed to decode Google Chat interaction payload", "error", err)
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	slog.Info("Received Google Chat interaction", "type", payload.Type, "method", payload.Action.ActionMethodName)

	if payload.Type != "CARD_CLICKED" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Extract parameters
	params := make(map[string]string)
	for _, p := range payload.Action.Parameters {
		params[p.Key] = p.Value
	}
	// Fallback to common parameters
	for k, v := range payload.Common.Parameters {
		params[k] = v
	}

	namespace := params["namespace"]
	podName := params["podName"]
	method := payload.Action.ActionMethodName

	if namespace == "" || podName == "" {
		slog.Warn("Google Chat interaction missing namespace or podName", "params", params)
		w.WriteHeader(http.StatusOK)
		return
	}

	var title, content string
	switch method {
	case "view_logs":
		title = "Log Explorer"
		logs, err := s.controller.GetPodLogs(r.Context(), namespace, podName)
		if err != nil {
			content = "Error: " + err.Error()
		} else {
			content = logs
		}
	case "view_events":
		title = "Event Timeline"
		events, err := s.controller.GetPodEvents(r.Context(), namespace, podName)
		if err != nil {
			content = "Error: " + err.Error()
		} else {
			content = events
		}
	case "view_trace":
		title = "Stack Trace"
		logs, _ := s.controller.GetPodLogs(r.Context(), namespace, podName)
		lines := strings.Split(logs, "\n")
		var traceLines []string
		for _, line := range lines {
			if strings.Contains(line, "stack") || strings.Contains(line, "panic") {
				traceLines = append(traceLines, line)
			}
		}
		content = strings.Join(traceLines, "\n")
		if content == "" {
			content = "No stack trace found."
		}
	default:
		w.WriteHeader(http.StatusOK)
		return
	}

	// Prepare response card for Google Chat
	// Interaction responses for simple button clicks can return a new message or update the current one.
	// We'll return a new message card with the content.
	
	formattedContent := "Empty."
	if content != "" {
		// Google Chat text format is limited; pre-formatted text uses <pre>
		if len(content) > 3500 {
			content = "... [truncated] ...\n" + content[len(content)-3500:]
		}
		formattedContent = "<pre>" + content + "</pre>"
	}

	response := map[string]interface{}{
		"actionResponse": map[string]string{
			"type": "NEW_MESSAGE",
		},
		"text": fmt.Sprintf("*%s for %s/%s*", title, namespace, podName),
		"cardsV2": []interface{}{
			map[string]interface{}{
				"cardId": "explorer_result",
				"card": map[string]interface{}{
					"header": map[string]string{
						"title": title,
					},
					"sections": []interface{}{
						map[string]interface{}{
							"widgets": []interface{}{
								map[string]interface{}{
									"textParagraph": map[string]string{
										"text": formattedContent,
									},
								},
							},
						},
					},
				},
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

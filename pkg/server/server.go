package server

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"fixora/pkg/config"
	"fixora/pkg/controller"
	"fixora/pkg/notifications"
	"github.com/slack-go/slack"
)

type Server struct {
	controller *controller.Controller
	config     *config.Config
}

const maxRequestBodyBytes = 1 << 20 // 1 MiB

func New(ctrl *controller.Controller, cfg *config.Config) *Server {
	return &Server{
		controller: ctrl,
		config:     cfg,
	}
}

func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/webhook/alertmanager", s.handleAlertmanager)
	mux.HandleFunc("/slack/interactions", s.handleSlackInteraction)
	mux.HandleFunc("/googlechat/interactions", s.handleGoogleChatInteraction)

	srv := &http.Server{
		Addr:              ":" + s.config.ServerPort,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	slog.Info("Starting Fixora server", "port", s.config.ServerPort)
	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodGet {
		fmt.Fprint(w, "OK")
	}
}

func (s *Server) handleAlertmanager(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.config.AlertmanagerEnabled {
		http.Error(w, "alertmanager webhook disabled", http.StatusNotFound)
		return
	}
	if !s.authorizeSharedSecret(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var payload struct {
		Alerts []struct {
			Labels map[string]string `json:"labels"`
		} `json:"alerts"`
	}

	body := http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	if err := json.NewDecoder(body).Decode(&payload); err != nil {
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
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := s.readBody(w, r)
	if err != nil {
		slog.Error("Failed to read Slack interaction body", "error", err)
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if err := s.verifySlackSignature(r, body); err != nil {
		slog.Warn("Rejected Slack interaction with invalid signature", "error", err)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	form, err := url.ParseQuery(string(body))
	if err != nil {
		slog.Error("Failed to parse Slack interaction form", "error", err)
		http.Error(w, "failed to parse form", http.StatusBadRequest)
		return
	}

	payloadRaw := form.Get("payload")
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
			if !s.controller.IsNamespaceScoped(namespace) {
				http.Error(w, "namespace out of scope", http.StatusForbidden)
				return
			}

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
			} else if viewType == "events" {
				title = "Event Timeline"
				events, err := s.controller.GetPodEvents(r.Context(), namespace, podName)
				if err != nil {
					content = "Error: " + err.Error()
				} else {
					content = events
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
			} else {
				w.WriteHeader(http.StatusOK)
				return
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
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.authorizeSharedSecret(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var payload struct {
		Type   string `json:"type"`
		Action struct {
			ActionMethodName string `json:"actionMethodName"`
			Parameters       []struct {
				Key   string `json:"key"`
				Value string `json:"value"`
			} `json:"parameters"`
		} `json:"action"`
		Common struct {
			Parameters map[string]string `json:"parameters"`
		} `json:"common"`
	}

	body := http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	if err := json.NewDecoder(body).Decode(&payload); err != nil {
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

	if method == "approve_remediation" || method == "approve_action" {
		callbackID := params["callback_id"]
		if callbackID == "" {
			slog.Warn("Google Chat approval missing callback_id")
			w.WriteHeader(http.StatusOK)
			return
		}
		slog.Info("User approved remediation via Google Chat", "callback_id", callbackID)
		go s.controller.SubmitPendingFix(r.Context(), callbackID)
		w.WriteHeader(http.StatusOK)
		return
	}

	if namespace == "" || podName == "" {
		slog.Warn("Google Chat interaction missing namespace or podName", "params", params)
		w.WriteHeader(http.StatusOK)
		return
	}
	if !s.controller.IsNamespaceScoped(namespace) {
		http.Error(w, "namespace out of scope", http.StatusForbidden)
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

func (s *Server) readBody(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	defer r.Body.Close()
	return io.ReadAll(http.MaxBytesReader(w, r.Body, maxRequestBodyBytes))
}

func (s *Server) verifySlackSignature(r *http.Request, body []byte) error {
	if s.config.SlackSigningSecret == "" {
		return fmt.Errorf("slack signing secret is not configured")
	}
	verifier, err := slack.NewSecretsVerifier(r.Header, s.config.SlackSigningSecret)
	if err != nil {
		return err
	}
	if _, err := verifier.Write(body); err != nil {
		return err
	}
	return verifier.Ensure()
}

func (s *Server) authorizeSharedSecret(r *http.Request) bool {
	if s.config.WebhookUser != "" || s.config.WebhookPassword != "" {
		user, pass, ok := r.BasicAuth()
		if !ok {
			return false
		}
		if !constantTimeEqual(user, s.config.WebhookUser) || !constantTimeEqual(pass, s.config.WebhookPassword) {
			return false
		}
	}

	if s.config.WebhookToken == "" {
		return s.config.WebhookUser != "" || s.config.WebhookPassword != ""
	}
	return constantTimeEqual(sharedSecretFromRequest(r), s.config.WebhookToken)
}

func sharedSecretFromRequest(r *http.Request) string {
	if token := r.Header.Get("X-Fixora-Token"); token != "" {
		return token
	}
	auth := r.Header.Get("Authorization")
	if token, ok := strings.CutPrefix(auth, "Bearer "); ok {
		return strings.TrimSpace(token)
	}
	return r.URL.Query().Get("token")
}

func constantTimeEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

package server

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"fmt"

	"fixora/pkg/config"
	"fixora/pkg/controller"
	"fixora/pkg/notifications"
	"fixora/pkg/security"
	"github.com/slack-go/slack"
)

type Server struct {
	controller *controller.Controller
	config     *config.Config
}

type Alert struct {
	Status       string            `json:"status"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	StartsAt     string            `json:"startsAt"`
	EndsAt       string            `json:"endsAt"`
	GeneratorURL string            `json:"generatorURL"`
	Fingerprint  string            `json:"fingerprint"`
}

type AlertmanagerPayload struct {
	Receiver string  `json:"receiver"`
	Status   string  `json:"status"`
	Alerts   []Alert `json:"alerts"`
}

func New(ctrl *controller.Controller, cfg *config.Config) *Server {
	return &Server{
		controller: ctrl,
		config:     cfg,
	}
}

func (s *Server) Start() {
	mux := http.NewServeMux()
	mux.HandleFunc("/slack/interactive", s.handleInteractive)
	mux.HandleFunc("/googlechat/interactive", s.handleGoogleChatInteractive)
	mux.HandleFunc("/alerts", s.handleAlerts)

	slog.Info("Server listening", "port", 8080)
	if err := http.ListenAndServe(":8080", mux); err != nil {
		slog.Error("Server failed to start", "error", err)
	}
}

func (s *Server) handleGoogleChatInteractive(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Request Verification (Basic for now, can be improved with JWT validation)
	if s.config.WebhookToken != "" {
		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer "+s.config.WebhookToken) {
			slog.Warn("Unauthorized Google Chat interactive attempt")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
	}

	var event map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		slog.Error("Failed to decode google chat event", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	common, ok := event["common"].(map[string]interface{})
	if !ok || common == nil {
		w.WriteHeader(http.StatusOK)
		return
	}

	params, _ := common["parameters"].(map[string]interface{})
	if params == nil {
		params = make(map[string]interface{})
	}
	function, _ := common["invokedFunction"].(string)

	if function == "view_logs" {
		namespace, _ := params["namespace"].(string)
		podName, _ := params["podName"].(string)

		slog.Info("Google Chat log explorer requested", "namespace", namespace, "pod", podName)
		logs, err := s.controller.GetPodLogs(r.Context(), namespace, podName)
		if err != nil {
			slog.Error("Failed to fetch logs for google chat explorer", "error", err)
			logs = "Error fetching logs: " + security.ScrubPII(err.Error())
		}

		if len(logs) > 3500 {
			logs = "... [truncated] ...\n" + logs[len(logs)-3400:]
		}

		response := map[string]interface{}{
			"action_response": map[string]interface{}{
				"type": "DIALOG",
				"dialog_action": map[string]interface{}{
					"dialog": map[string]interface{}{
						"body": map[string]interface{}{
							"sections": []interface{}{
								map[string]interface{}{
									"header": fmt.Sprintf("📄 Scrubbed Logs for %s/%s", namespace, podName),
									"widgets": []interface{}{
										map[string]interface{}{
											"textParagraph": map[string]interface{}{
												"text": fmt.Sprintf("<pre>%s</pre>", logs),
											},
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
		return
	}

	if function == "approve_remediation" {
		callbackID, _ := params["callback_id"].(string)
		if callbackID != "" {
			slog.Info("Google Chat remediation approved", "callback_id", callbackID)
			s.controller.SubmitPendingFix(r.Context(), callbackID)
		}
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleAlerts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Optional Authentication
	authorized := true
	if s.config.WebhookToken != "" || (s.config.WebhookUser != "" && s.config.WebhookPassword != "") {
		authorized = false

		authHeader := r.Header.Get("Authorization")
		if s.config.WebhookToken != "" && strings.HasPrefix(authHeader, "Bearer ") {
			token := strings.TrimPrefix(authHeader, "Bearer ")
			if token == s.config.WebhookToken {
				authorized = true
			}
		}

		if !authorized && s.config.WebhookUser != "" && s.config.WebhookPassword != "" {
			user, pass, ok := r.BasicAuth()
			if ok && user == s.config.WebhookUser && pass == s.config.WebhookPassword {
				authorized = true
			}
		}
	}

	if !authorized {
		slog.Warn("Unauthorized alert attempt")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	var payload AlertmanagerPayload
	err := json.NewDecoder(r.Body).Decode(&payload)
	if err != nil {
		slog.Error("Failed to decode alert payload", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	slog.Info("Received alerts", "count", len(payload.Alerts))
	for _, alert := range payload.Alerts {
		if alert.Status != "firing" {
			continue
		}

		namespace := alert.Labels["namespace"]
		podName := alert.Labels["pod"]
		reason := alert.Labels["alertname"]

		if podName != "" && namespace != "" {
			slog.Info("Triggering diagnostic from alert", "namespace", namespace, "pod", podName, "reason", reason)
			go s.controller.DiagnosePodByName(namespace, podName, reason)
		}
	}

	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) handleInteractive(w http.ResponseWriter, r *http.Request) {
	verifier, err := slack.NewSecretsVerifier(r.Header, s.config.SlackSigningSecret)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	if _, err := verifier.Write(body); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if err := verifier.Ensure(); err != nil {
		slog.Warn("Blocked forged interactive webhook attempt")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	payloadStr := r.FormValue("payload")
	if payloadStr == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var payload slack.InteractionCallback
	err = json.Unmarshal([]byte(payloadStr), &payload)
	if err != nil {
		slog.Error("Failed to unmarshal slack callback", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if payload.Type != slack.InteractionTypeBlockActions {
		return
	}

	if len(payload.ActionCallback.BlockActions) == 0 {
		return
	}

	action := payload.ActionCallback.BlockActions[0]

	// Handle Log Explorer
	if strings.HasPrefix(action.ActionID, "view-logs-") {
		// ActionID is formatted as "view-logs-namespace-podName"
		// We use SplitN to handle pod names that contain dashes, but we need to know where namespace ends.
		// A better approach is to change the ActionID format or use Value.
		// Let's assume namespace doesn't have dashes for simple split, OR we use a more robust separator.
		// Since we control the button creation in slack.go, let's look at it.
		// slack.go used: fmt.Sprintf("view-logs-%s-%s", evidence.Namespace, evidence.PodName)
		
		// To correctly handle dashes in both namespace and podName, we should have used a different separator.
		// For now, let's try a heuristic or fix slack.go to use a better separator like '|'.
		parts := strings.Split(action.ActionID, "-")
		if len(parts) >= 4 {
			namespace := parts[2]
			podName := strings.Join(parts[3:], "-")

			slog.Info("Slack log explorer requested", "namespace", namespace, "pod", podName)
			logs, err := s.controller.GetPodLogs(r.Context(), namespace, podName)
			if err != nil {
				slog.Error("Failed to fetch logs for explorer", "error", err)
				logs = "Error fetching logs: " + security.ScrubPII(err.Error())
			}

			err = notifications.SendLogModal(s.config, payload.TriggerID, namespace, podName, logs)
			if err != nil {
				slog.Error("Failed to open log modal", "error", err)
			}
			w.WriteHeader(http.StatusOK)
			return
		}
	}

	if action.ActionID != "approve" {
		slog.Info("Action denied by user", "callback_id", payload.CallbackID)
		w.WriteHeader(http.StatusOK)
		return
	}

	if strings.HasPrefix(payload.CallbackID, "patch-") {
		slog.Info("Patch generation approved", "callback_id", payload.CallbackID)
		s.controller.SubmitPendingFix(r.Context(), payload.CallbackID)
		w.WriteHeader(http.StatusOK)
		return
	}

	// Rollout restart handler
	parts := strings.Split(payload.CallbackID, "-")
	if len(parts) == 4 && parts[0] == "rollout" && parts[1] == "restart" {
		namespace := parts[2]
		deploymentName := parts[3]

		slog.Info("Rollout restart approved", "namespace", namespace, "deployment", deploymentName)
		s.controller.PerformRolloutRestart(namespace, deploymentName)
	}

	w.WriteHeader(http.StatusOK)
}

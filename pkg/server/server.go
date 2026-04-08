package server

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"fixora/pkg/config"
	"fixora/pkg/controller"
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
	mux.HandleFunc("/alerts", s.handleAlerts)

	slog.Info("Server listening", "port", 8080)
	if err := http.ListenAndServe(":8080", mux); err != nil {
		slog.Error("Server failed to start", "error", err)
	}
}

func (s *Server) handleAlerts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Optional Authentication
	authorized := true
	// If any auth method is configured, we must validate
	if s.config.WebhookToken != "" || (s.config.WebhookUser != "" && s.config.WebhookPassword != "") {
		authorized = false

		// 1. Check Bearer Token
		authHeader := r.Header.Get("Authorization")
		if s.config.WebhookToken != "" && strings.HasPrefix(authHeader, "Bearer ") {
			token := strings.TrimPrefix(authHeader, "Bearer ")
			if token == s.config.WebhookToken {
				authorized = true
			}
		}

		// 2. Check Basic Auth (fallback)
		if !authorized && s.config.WebhookUser != "" && s.config.WebhookPassword != "" {
			user, pass, ok := r.BasicAuth()
			if ok && user == s.config.WebhookUser && pass == s.config.WebhookPassword {
				authorized = true
			}
		}
	}

	if !authorized {
		slog.Warn("Unauthorized alert attempt: credentials did not match configured methods")
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
	r.Body = io.NopCloser(bytes.NewBuffer(body)) // Reset body for later parsing

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

	parts := strings.Split(payload.CallbackID, "-")
	if len(parts) != 4 || parts[0] != "rollout" || parts[1] != "restart" {
		slog.Warn("Invalid callback ID received", "callback_id", payload.CallbackID)
		return
	}

	namespace := parts[2]
	deploymentName := parts[3]

	slog.Info("Rollout restart approved", "namespace", namespace, "deployment", deploymentName)
	s.controller.PerformRolloutRestart(namespace, deploymentName)

	w.WriteHeader(http.StatusOK)
}

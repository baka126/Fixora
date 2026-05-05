package notifications

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"fixora/pkg/config"
)

type AuditPayload struct {
	Timestamp         string        `json:"timestamp"`
	Namespace         string        `json:"namespace"`
	PodName           string        `json:"podName"`
	Evidence          EvidenceChain `json:"evidence"`
	ActionType        string        `json:"actionType"`
	ActionStatus      string        `json:"actionStatus"`
	ActionDetails     string        `json:"actionDetails,omitempty"`
}

func sendAuditWebhook(cfg *config.Config, payload AuditPayload) error {
	if cfg.AuditWebhookURL == "" {
		return nil
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal audit payload: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, cfg.AuditWebhookURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create audit webhook request: %v", err)
	}
	
	req.Header.Set("Content-Type", "application/json")
	if cfg.AuditWebhookToken != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.AuditWebhookToken)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send audit webhook: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("audit webhook returned non-2xx status: %d", resp.StatusCode)
	}

	return nil
}

// SendAuditLog builds and sends the structured audit payload to a configured SIEM or logging endpoint.
func SendAuditLog(cfg *config.Config, evidence EvidenceChain, actionType, actionStatus, actionDetails string) {
	if cfg.AuditWebhookURL == "" {
		return
	}

	payload := AuditPayload{
		Timestamp:    time.Now().Format(time.RFC3339),
		Namespace:    evidence.Namespace,
		PodName:      evidence.PodName,
		Evidence:     evidence,
		ActionType:   actionType,
		ActionStatus: actionStatus,
		ActionDetails: actionDetails,
	}

	if err := sendAuditWebhook(cfg, payload); err != nil {
		slog.Error("Failed to send structured audit log", "url", cfg.AuditWebhookURL, "error", err)
	} else {
		slog.Debug("Successfully sent structured audit log", "ns", evidence.Namespace, "pod", evidence.PodName)
	}
}

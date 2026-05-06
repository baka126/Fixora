package notifications

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"time"

	"fixora/pkg/config"
	"fixora/pkg/security"
)

type AuditPayload struct {
	Timestamp     string        `json:"timestamp"`
	Namespace     string        `json:"namespace"`
	PodName       string        `json:"podName"`
	Evidence      EvidenceChain `json:"evidence"`
	ActionType    string        `json:"actionType"`
	ActionStatus  string        `json:"actionStatus"`
	ActionDetails string        `json:"actionDetails,omitempty"`
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
	payload := buildAuditPayload(cfg, evidence, actionType, actionStatus, actionDetails)

	if err := sendAuditWebhook(cfg, payload); err != nil {
		slog.Error("Failed to send structured audit log", "url", cfg.AuditWebhookURL, "error", err)
	} else {
		slog.Debug("Successfully sent structured audit log", "ns", payload.Namespace, "pod", payload.PodName)
	}
}

func buildAuditPayload(cfg *config.Config, evidence EvidenceChain, actionType, actionStatus, actionDetails string) AuditPayload {
	scrubbers := auditScrubbers(cfg)
	evidence = scrubAuditEvidence(evidence, scrubbers)

	return AuditPayload{
		Timestamp:     time.Now().Format(time.RFC3339),
		Namespace:     security.ScrubPII(evidence.Namespace, scrubbers...),
		PodName:       security.ScrubPII(evidence.PodName, scrubbers...),
		Evidence:      evidence,
		ActionType:    actionType,
		ActionStatus:  actionStatus,
		ActionDetails: security.ScrubPII(actionDetails, scrubbers...),
	}
}

func scrubAuditEvidence(evidence EvidenceChain, scrubbers []*regexp.Regexp) EvidenceChain {
	evidence.Namespace = security.ScrubPII(evidence.Namespace, scrubbers...)
	evidence.PodName = security.ScrubPII(evidence.PodName, scrubbers...)
	evidence.MetricProof = security.ScrubPII(evidence.MetricProof, scrubbers...)
	evidence.ClusterContext = security.ScrubPII(evidence.ClusterContext, scrubbers...)
	evidence.HistoricalPattern = security.ScrubPII(evidence.HistoricalPattern, scrubbers...)
	evidence.EventTimeline = security.ScrubPII(evidence.EventTimeline, scrubbers...)
	evidence.RootCause = security.ScrubPII(evidence.RootCause, scrubbers...)
	evidence.FinOpsImpact = security.ScrubPII(evidence.FinOpsImpact, scrubbers...)
	evidence.StackTrace = security.ScrubPII(evidence.StackTrace, scrubbers...)
	evidence.FinOpsDetails = security.ScrubPII(evidence.FinOpsDetails, scrubbers...)
	return evidence
}

func auditScrubbers(cfg *config.Config) []*regexp.Regexp {
	if cfg == nil {
		return nil
	}
	scrubbers := make([]*regexp.Regexp, 0, len(cfg.CustomLogScrubbingPatterns))
	for _, pattern := range cfg.CustomLogScrubbingPatterns {
		compiled, err := regexp.Compile(pattern)
		if err != nil {
			slog.Warn("Failed to compile audit scrubbing pattern, skipping", "pattern", pattern, "error", err)
			continue
		}
		scrubbers = append(scrubbers, compiled)
	}
	return scrubbers
}

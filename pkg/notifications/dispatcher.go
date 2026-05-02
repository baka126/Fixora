package notifications

import (
	"fmt"
	"log/slog"
	"strings"

	"fixora/pkg/config"
	"fixora/pkg/security"
)

type EvidenceChain struct {
	Namespace              string
	PodName                string
	MetricProof            string
	ClusterContext         string
	HistoricalPattern      string
	EventTimeline          string
	RootCause              string
	FinOpsImpact           string
	PredictiveWarning      bool
	EstimatedHoursToOOM    float64
	AIConfidence           int

	// New fields for interactive triage
	StackTrace    string
	FinOpsDetails string
	ShowFixButton bool
	ShowPRButton  bool
}

func SendEvidenceChain(cfg *config.Config, evidence EvidenceChain) error {
	slog.Info("Dispatching forensic report to configured notification channels", "ns", evidence.Namespace, "pod", evidence.PodName, "predictive", evidence.PredictiveWarning)

	// Defensive Scrubbing for all fields to prevent leakage of AI-repeated PII or metrics data
	evidence.MetricProof = security.ScrubPII(evidence.MetricProof)
	evidence.ClusterContext = security.ScrubPII(evidence.ClusterContext)
	evidence.HistoricalPattern = security.ScrubPII(evidence.HistoricalPattern)
	evidence.EventTimeline = security.ScrubPII(evidence.EventTimeline)
	evidence.RootCause = security.ScrubPII(evidence.RootCause)
	evidence.FinOpsImpact = security.ScrubPII(evidence.FinOpsImpact)
	evidence.StackTrace = security.ScrubPII(evidence.StackTrace)
	evidence.FinOpsDetails = security.ScrubPII(evidence.FinOpsDetails)

	var errs []string

	if cfg.SlackToken != "" {
		if err := sendSlackEvidenceChain(cfg, evidence); err != nil {
			slog.Error("Failed to send report to Slack", "ns", evidence.Namespace, "pod", evidence.PodName, "error", err)
			errs = append(errs, fmt.Sprintf("slack: %v", err))
		} else {
			slog.Debug("Successfully sent report to Slack", "ns", evidence.Namespace, "pod", evidence.PodName)
		}
	}

	if cfg.GoogleChatWebhookURL != "" {
		if err := sendGoogleChatEvidenceChain(cfg, evidence); err != nil {
			slog.Error("Failed to send report to Google Chat", "ns", evidence.Namespace, "pod", evidence.PodName, "error", err)
			errs = append(errs, fmt.Sprintf("googlechat: %v", err))
		} else {
			slog.Debug("Successfully sent report to Google Chat", "ns", evidence.Namespace, "pod", evidence.PodName)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to send evidence chain: %s", strings.Join(errs, ", "))
	}
	return nil
}

func SendNotification(cfg *config.Config, message string) error {
	slog.Info("Dispatching system notification", "msg_preview", truncateForLog(message, 50))
	message = security.ScrubPII(message)
	var errs []string

	if cfg.SlackToken != "" {
		if err := sendSlackNotification(cfg, message); err != nil {
			errs = append(errs, fmt.Sprintf("slack: %v", err))
		}
	}

	if cfg.GoogleChatWebhookURL != "" {
		if err := sendGoogleChatNotification(cfg, message); err != nil {
			errs = append(errs, fmt.Sprintf("googlechat: %v", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to send notification: %s", strings.Join(errs, ", "))
	}
	return nil
}

func SendInteractiveNotification(cfg *config.Config, message, callbackID string) error {
	slog.Info("Dispatching interactive notification", "callback_id", callbackID)
	message = security.ScrubPII(message)
	var errs []string

	if cfg.SlackToken != "" {
		if err := sendSlackInteractiveNotification(cfg, message, callbackID); err != nil {
			errs = append(errs, fmt.Sprintf("slack: %v", err))
		}
	}

	if cfg.GoogleChatWebhookURL != "" {
		if err := sendGoogleChatInteractiveNotification(cfg, message, callbackID); err != nil {
			errs = append(errs, fmt.Sprintf("googlechat: %v", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to send interactive notification: %s", strings.Join(errs, ", "))
	}
	return nil
}

func SendRemediationApproval(cfg *config.Config, namespace, pod, patch, callbackID string) error {
	slog.Info("Dispatching remediation approval request", "ns", namespace, "pod", pod, "callback_id", callbackID)
	// Patch is already scrubbed by AI generation process in controller, but defensive scrub here too
	patch = security.ScrubPII(patch)
	var errs []string

	if cfg.SlackToken != "" {
		if err := sendSlackRemediationApproval(cfg, namespace, pod, patch, callbackID); err != nil {
			errs = append(errs, fmt.Sprintf("slack: %v", err))
		}
	}

	if cfg.GoogleChatWebhookURL != "" {
		if err := sendGoogleChatRemediationApproval(cfg, namespace, pod, patch, callbackID); err != nil {
			errs = append(errs, fmt.Sprintf("googlechat: %v", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to send remediation approval: %s", strings.Join(errs, ", "))
	}
	return nil
}

func truncateForLog(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

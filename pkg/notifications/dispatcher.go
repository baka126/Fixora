package notifications

import (
	"fmt"
	"strings"

	"fixora/pkg/config"
)

type EvidenceChain struct {
	MetricProof            string
	ClusterContext         string
	HistoricalPattern      string
	EventTimeline          string
	RootCause              string
	FinOpsImpact           string
	PredictiveWarning      bool
	EstimatedHoursToOOM    float64
}

func SendEvidenceChain(cfg *config.Config, evidence EvidenceChain) error {
	var errs []string

	if err := sendSlackEvidenceChain(cfg, evidence); err != nil {
		errs = append(errs, fmt.Sprintf("slack: %v", err))
	}

	if err := sendGoogleChatEvidenceChain(cfg, evidence); err != nil {
		errs = append(errs, fmt.Sprintf("googlechat: %v", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to send evidence chain: %s", strings.Join(errs, ", "))
	}
	return nil
}

func SendNotification(cfg *config.Config, message string) error {
	var errs []string

	if err := sendSlackNotification(cfg, message); err != nil {
		errs = append(errs, fmt.Sprintf("slack: %v", err))
	}

	if err := sendGoogleChatNotification(cfg, message); err != nil {
		errs = append(errs, fmt.Sprintf("googlechat: %v", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to send notification: %s", strings.Join(errs, ", "))
	}
	return nil
}

func SendInteractiveNotification(cfg *config.Config, message, callbackID string) error {
	var errs []string

	if err := sendSlackInteractiveNotification(cfg, message, callbackID); err != nil {
		errs = append(errs, fmt.Sprintf("slack: %v", err))
	}

	if err := sendGoogleChatInteractiveNotification(cfg, message, callbackID); err != nil {
		errs = append(errs, fmt.Sprintf("googlechat: %v", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to send interactive notification: %s", strings.Join(errs, ", "))
	}
	return nil
}

func SendRemediationApproval(cfg *config.Config, namespace, pod, patch, callbackID string) error {
	var errs []string

	if err := sendSlackRemediationApproval(cfg, namespace, pod, patch, callbackID); err != nil {
		errs = append(errs, fmt.Sprintf("slack: %v", err))
	}

	if err := sendGoogleChatRemediationApproval(cfg, namespace, pod, patch, callbackID); err != nil {
		errs = append(errs, fmt.Sprintf("googlechat: %v", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to send remediation approval: %s", strings.Join(errs, ", "))
	}
	return nil
}

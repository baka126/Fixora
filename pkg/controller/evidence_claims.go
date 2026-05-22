package controller

import (
	"fmt"
	"strings"

	"fixora/pkg/notifications"
)

func populateEvidenceClaims(evidence *notifications.EvidenceChain, diagnosis Diagnosis, corr ResourceCorrelation) {
	if evidence == nil {
		return
	}
	var validated []string
	var unvalidated []string

	if strings.TrimSpace(diagnosis.Symptom) != "" {
		validated = append(validated, "Kubernetes reported symptom: "+diagnosis.Symptom)
	}
	if strings.TrimSpace(evidence.MetricProof) != "" {
		validated = append(validated, "Metric proof was collected from the configured metrics provider.")
	}
	if strings.TrimSpace(evidence.EventTimeline) != "" {
		validated = append(validated, fmt.Sprintf("Kubernetes events were collected (%d lines).", countNonEmptyLines(evidence.EventTimeline)))
	}
	if strings.TrimSpace(evidence.StackTrace) != "" {
		validated = append(validated, "Relevant logs or stack trace were collected for the failing workload.")
	}
	for _, score := range corr.TopCorrelations(3) {
		validated = append(validated, fmt.Sprintf("%s correlated at %d%% via %s.", score.Ref, score.Score, strings.Join(score.Reasons, "; ")))
	}

	if strings.TrimSpace(evidence.RootCause) != "" {
		unvalidated = append(unvalidated, "Root-cause narrative is AI-assisted and should be reviewed against the deterministic evidence.")
	}
	if diagnosis.PatchStrategy != PatchNone {
		unvalidated = append(unvalidated, "Recommended patch strategy is a remediation hypothesis until render, semantic, and policy guardrails pass.")
	}

	evidence.ValidatedClaims = uniqueClaimLines(validated)
	evidence.UnvalidatedClaims = uniqueClaimLines(unvalidated)
}

func uniqueClaimLines(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

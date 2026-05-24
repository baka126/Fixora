package notifications

import (
	"fmt"
	"strings"
)

type evidenceMessageView struct {
	Title       string
	Subtitle    string
	Workload    string
	Namespace   string
	Status      string
	Finding     string
	Category    string
	Confidence  string
	Signal      string
	Summary     string
	RootCause   string
	Remediation string
	Signals     string
	History     string
	FinOps      string
}

func buildEvidenceMessageView(evidence EvidenceChain) evidenceMessageView {
	podName := firstNonEmpty(evidence.PodName, contextValue(evidence.ClusterContext, "Pod"))
	namespace := firstNonEmpty(evidence.Namespace, contextValue(evidence.ClusterContext, "Namespace"))
	workloadKind, workloadName := ownerWorkloadFromNotificationContext(evidence.ClusterContext)
	workloadRef := compactJoin("/", workloadKind, workloadName)
	if workloadRef == "" && podName != "" {
		workloadRef = "Pod/" + podName
	}
	category := contextValue(evidence.ClusterContext, "Category")
	reason := contextValue(evidence.ClusterContext, "Reason")
	diagnosis := contextValue(evidence.ClusterContext, "Diagnosis")
	confidence := contextValue(evidence.ClusterContext, "Confidence")
	if evidence.AIConfidence > 0 {
		confidence = fmt.Sprintf("%d%%", evidence.AIConfidence)
	}
	patchStrategy := contextValue(evidence.ClusterContext, "Recommended Patch Strategy")

	title := "Fixora Incident"
	if workloadRef != "" {
		title += ": " + workloadRef
	}
	subtitle := compactJoin(" · ", namespace, reason, titleCase(category), confidence)
	if evidence.PredictiveWarning {
		title = "Fixora Predictive Risk"
		if workloadRef != "" {
			title += ": " + workloadRef
		}
		subtitle = compactJoin(" · ", namespace, "Memory growth risk", confidence)
	}

	summaryLines := []string{}
	if workloadRef != "" {
		summaryLines = append(summaryLines, fmt.Sprintf("Workload: %s", workloadRef))
	}
	if namespace != "" {
		summaryLines = append(summaryLines, fmt.Sprintf("Namespace: %s", namespace))
	}
	if reason != "" {
		summaryLines = append(summaryLines, fmt.Sprintf("Status: %s", reason))
	}
	if diagnosis != "" {
		summaryLines = append(summaryLines, fmt.Sprintf("Finding: %s", diagnosis))
	}
	if confidence != "" {
		summaryLines = append(summaryLines, fmt.Sprintf("Confidence: %s", confidence))
	}
	if evidence.PredictiveWarning && evidence.EstimatedHoursToOOM > 0 {
		summaryLines = append(summaryLines, fmt.Sprintf("Estimated time to OOM: %.1f hours", evidence.EstimatedHoursToOOM))
	}

	remediation := remediationSummary(patchStrategy)
	return evidenceMessageView{
		Title:       title,
		Subtitle:    subtitle,
		Workload:    firstNonEmpty(workloadRef, podName),
		Namespace:   namespace,
		Status:      reason,
		Finding:     diagnosis,
		Category:    titleCase(category),
		Confidence:  confidence,
		Signal:      notificationSignal(evidence),
		Summary:     strings.Join(summaryLines, "\n"),
		RootCause:   truncateText(firstSentence(firstNonEmpty(evidence.RootCause, contextValue(evidence.ClusterContext, "Likely Cause"), diagnosis, "No root cause summary available.")), 260),
		Remediation: remediation,
		Signals:     truncateText(evidence.MetricProof, 500),
		History:     truncateText(evidence.HistoricalPattern, 500),
		FinOps:      truncateText(firstNonEmpty(evidence.FinOpsImpact, "Not estimated."), 500),
	}
}

func notificationSignal(evidence EvidenceChain) string {
	hasMetrics := strings.TrimSpace(evidence.MetricProof) != ""
	hasEvents := strings.TrimSpace(evidence.EventTimeline) != "" || contextValue(evidence.ClusterContext, "Evidence") != ""
	switch {
	case hasMetrics && hasEvents:
		return "Metrics + Events"
	case hasMetrics:
		return "Metrics"
	case hasEvents:
		return "Events"
	default:
		return "Cluster"
	}
}

func RemediationBlockedMessage(repoKey, reason string) string {
	return fmt.Sprintf("Remediation paused for %s\nFixora blocked the generated patch during validation.\nReason: %s\nAction: No PR was opened. Fixora will retry after the patch generator produces a valid manifest.", repoKey, professionalPolicyReason(reason))
}

func RemediationPROpenedMessage(namespace, podName string, urls []string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Remediation PR opened for %s/%s\n", namespace, podName))
	sb.WriteString("Fixora created a targeted GitOps pull request for review.\n")
	sb.WriteString("Pull request")
	if len(urls) != 1 {
		sb.WriteString("s")
	}
	sb.WriteString(":\n")
	for _, url := range urls {
		sb.WriteString("- ")
		sb.WriteString(strings.TrimSpace(url))
		sb.WriteString("\n")
	}
	sb.WriteString("Next step: review the proposed manifest change before merging.")
	return strings.TrimSpace(sb.String())
}

func professionalPolicyReason(reason string) string {
	switch {
	case strings.Contains(reason, "nested Kubernetes object"):
		location := ""
		if idx := strings.LastIndex(reason, " at "); idx >= 0 {
			location = strings.TrimSpace(reason[idx+4:])
		}
		file := ""
		if idx := strings.Index(reason, " in "); idx >= 0 {
			rest := strings.TrimSpace(reason[idx+4:])
			file = strings.Fields(rest)[0]
		}
		target := compactJoin(" at ", file, location)
		if target != "" {
			return "Generated manifest structure is invalid (" + target + ")."
		}
		return "Generated manifest structure is invalid."
	case strings.Contains(reason, "does not target the incident workload"):
		return "Generated manifest does not target the workload that triggered the incident."
	case strings.Contains(reason, "identity mismatch"):
		return reason + ". Verify the repo-path annotation or point Fixora at the manifest for the workload that triggered the incident."
	case strings.Contains(reason, "namespace mismatch"):
		return reason + ". Verify the repo-path annotation or GitOps source mapping before retrying."
	case strings.Contains(reason, "replacement image"):
		return reason + ". Configure ALLOWED_REPLACEMENT_IMAGES with a pinned, verified image before retrying."
	default:
		return reason
	}
}

func remediationSummary(strategy string) string {
	strategy = strings.TrimSpace(strings.ToLower(strategy))
	switch strategy {
	case "", "none":
		return "Evaluation in progress. Fixora is checking whether a safe GitOps patch can be generated. A follow-up message will report whether a PR was opened, paused, or requires approval."
	default:
		return fmt.Sprintf("Evaluation in progress. Candidate patch strategy: %s. A follow-up message will report whether a PR was opened, paused, or requires approval.", strategy)
	}
}

func contextValue(context, label string) string {
	for _, line := range strings.Split(context, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(line, "- "))
		if line == "" {
			continue
		}
		for _, part := range strings.Split(line, ",") {
			part = strings.TrimSpace(part)
			prefix := label + ":"
			if strings.HasPrefix(strings.ToLower(part), strings.ToLower(prefix)) {
				return strings.TrimSpace(part[len(prefix):])
			}
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func compactJoin(sep string, values ...string) string {
	parts := []string{}
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			parts = append(parts, strings.TrimSpace(value))
		}
	}
	return strings.Join(parts, sep)
}

func titleCase(value string) string {
	if value == "" {
		return ""
	}
	return strings.ToUpper(value[:1]) + value[1:]
}

func firstSentence(value string) string {
	value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	for _, marker := range []string{". ", "; "} {
		if idx := strings.Index(value, marker); idx > 0 {
			return strings.TrimSpace(value[:idx+1])
		}
	}
	return value
}

func truncateText(value string, max int) string {
	value = strings.TrimSpace(value)
	if len(value) <= max {
		return value
	}
	return strings.TrimSpace(value[:max]) + "..."
}

type notificationResourceRef struct {
	Kind string
	Name string
}

func ownerWorkloadFromNotificationContext(clusterContext string) (string, string) {
	preferred := map[string]int{
		"Deployment":  1,
		"StatefulSet": 1,
		"DaemonSet":   1,
		"Job":         2,
		"ReplicaSet":  3,
	}
	bestRank := 100
	var best notificationResourceRef
	for _, ref := range notificationResourceRefsFromContext(clusterContext) {
		if rank, ok := preferred[ref.Kind]; ok && rank < bestRank {
			bestRank = rank
			best = ref
		}
	}
	return best.Kind, best.Name
}

func notificationResourceRefsFromContext(clusterContext string) []notificationResourceRef {
	var refs []notificationResourceRef
	seen := map[string]bool{}
	add := func(kind, name string) {
		key := strings.ToLower(kind + "/" + name)
		if seen[key] || kind == "" || name == "" {
			return
		}
		seen[key] = true
		refs = append(refs, notificationResourceRef{Kind: kind, Name: name})
	}
	for _, line := range strings.Split(clusterContext, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "- "))
		if strings.HasPrefix(line, "Owner chain:") {
			for _, item := range strings.Split(strings.TrimPrefix(line, "Owner chain:"), "->") {
				if kind, name, ok := splitNotificationKindName(item); ok {
					add(kind, name)
				}
			}
			continue
		}
		if kind, name, ok := splitNotificationKindName(line); ok {
			add(kind, name)
		}
	}
	return refs
}

func splitNotificationKindName(value string) (string, string, bool) {
	value = strings.TrimSpace(value)
	parts := strings.Split(value, "/")
	if len(parts) < 2 {
		return "", "", false
	}
	kind := strings.TrimSpace(parts[0])
	name := strings.TrimSpace(parts[1])
	if kind == "" || name == "" || strings.Contains(name, " ") {
		return "", "", false
	}
	return kind, name, true
}

func Haystack(evidence EvidenceChain, analyzerEvidence []string, likelyCause string) string {
	return strings.ToLower(strings.Join([]string{
		evidence.RootCause,
		evidence.ClusterContext,
		evidence.StackTrace,
		strings.Join(analyzerEvidence, " "),
		likelyCause,
	}, "\n"))
}

func ContainsAny(s string, needles ...string) bool {
	s = strings.ToLower(s)
	for _, needle := range needles {
		if strings.Contains(s, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

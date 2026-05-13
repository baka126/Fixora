package notifications

import (
	"strings"
	"testing"
)

func TestBuildEvidenceMessageViewCompactsClusterContext(t *testing.T) {
	view := buildEvidenceMessageView(EvidenceChain{
		Namespace: "default",
		PodName:   "oom-test-3",
		ClusterContext: strings.Join([]string{
			"Namespace: default, Pod: oom-test-3, Reason: ExitCode:1",
			"Diagnosis: Container is repeatedly crashing",
			"Category: runtime",
			"Likely Cause: The container process exits unsuccessfully; logs and events should identify the application or runtime failure.",
			"Confidence: 70%",
			"Recommended Patch Strategy: none",
			"Evidence:",
			"- stress terminated: Error exit=1",
			"Resource Correlation:",
			"- Owner chain: Pod/oom-test-3",
		}, "\n"),
		MetricProof:       "Metric Source: Prometheus\nMemory Usage: 0.00 MiB",
		HistoricalPattern: "This is the first time we've diagnosed this pod.",
		RootCause:         "The root cause is a binary architecture mismatch.",
		FinOpsImpact:      "Medium AWS cost impact.",
		AIConfidence:      85,
	})

	if view.Title != "Fixora Incident Report" {
		t.Fatalf("unexpected title: %s", view.Title)
	}
	if !strings.Contains(view.Summary, "default/oom-test-3") {
		t.Fatalf("expected workload summary, got: %s", view.Summary)
	}
	if strings.Contains(view.Summary, "Evidence:") || strings.Contains(view.Summary, "Owner chain") {
		t.Fatalf("expected raw context dump to be omitted, got: %s", view.Summary)
	}
	if strings.Contains(view.Remediation, "No automated pull request was opened") {
		t.Fatalf("expected initial report not to claim PR outcome, got: %s", view.Remediation)
	}
	if !strings.Contains(view.Remediation, "Evaluation in progress") {
		t.Fatalf("expected initial report to describe remediation evaluation, got: %s", view.Remediation)
	}
	if !strings.Contains(view.Summary, "Confidence: 85%") {
		t.Fatalf("expected AI confidence to be used in summary, got: %s", view.Summary)
	}
}

func TestRemediationBlockedMessageIsProfessional(t *testing.T) {
	message := RemediationBlockedMessage("baka126/fixora-demo", "nested Kubernetes object is not allowed in pod.yaml at $.spec.containers[0]")

	for _, want := range []string{
		"Remediation paused for baka126/fixora-demo",
		"Fixora blocked the generated patch during validation.",
		"Generated manifest structure is invalid (pod.yaml at $.spec.containers[0]).",
		"No PR was opened.",
	} {
		if !strings.Contains(message, want) {
			t.Fatalf("expected %q in message:\n%s", want, message)
		}
	}
	if strings.Contains(message, "❌") {
		t.Fatalf("expected no emoji in professional message:\n%s", message)
	}
}

func TestRemediationPROpenedMessageIsProfessional(t *testing.T) {
	message := RemediationPROpenedMessage("default", "oom-test-4", []string{"https://github.com/baka126/fixora-demo/pull/7"})

	for _, want := range []string{
		"Remediation PR opened for default/oom-test-4",
		"Fixora created a targeted GitOps pull request for review.",
		"- https://github.com/baka126/fixora-demo/pull/7",
		"Next step: review the proposed manifest change before merging.",
	} {
		if !strings.Contains(message, want) {
			t.Fatalf("expected %q in message:\n%s", want, message)
		}
	}
	if strings.Contains(message, "🚀") {
		t.Fatalf("expected no emoji in professional message:\n%s", message)
	}
}

func TestRemediationBlockedMessageExplainsIdentityMismatch(t *testing.T) {
	message := RemediationBlockedMessage("baka126/fixora-demo", "raw Pod manifest identity mismatch: source declares Pod/oom-test-4 but incident is Pod/oom-test-5")

	for _, want := range []string{
		"Remediation paused for baka126/fixora-demo",
		"raw Pod manifest identity mismatch",
		"Verify the repo-path annotation",
	} {
		if !strings.Contains(message, want) {
			t.Fatalf("expected %q in message:\n%s", want, message)
		}
	}
}

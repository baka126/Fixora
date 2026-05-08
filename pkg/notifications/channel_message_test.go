package notifications

import (
	"encoding/json"
	"strings"
	"testing"

	"fixora/pkg/config"
)

func TestSlackEvidenceMessageOmitsInlineEventTimeline(t *testing.T) {
	evidence := noisyTimelineEvidence()
	blocks := buildSlackEvidenceBlocks(&config.Config{}, evidence)

	body := marshalForTest(t, blocks)
	assertNoInlineTimeline(t, body)
	assertProfessionalEvidenceMessage(t, body)
}

func TestSlackEvidenceMessageKeepsEventActionWithoutTimelineBody(t *testing.T) {
	evidence := noisyTimelineEvidence()
	evidence.ShowEventButton = true
	blocks := buildSlackEvidenceBlocks(&config.Config{SlackAppMode: true}, evidence)

	body := marshalForTest(t, blocks)
	assertNoTimelineBody(t, body)
	if !strings.Contains(body, "view_events") {
		t.Fatalf("expected app-mode Slack message to keep event action button: %s", body)
	}
}

func TestGoogleChatEvidenceMessageOmitsInlineEventTimeline(t *testing.T) {
	evidence := noisyTimelineEvidence()
	payload := buildGoogleChatEvidencePayload(&config.Config{}, evidence)

	body := marshalForTest(t, payload)
	assertNoInlineTimeline(t, body)
	assertProfessionalEvidenceMessage(t, body)
}

func TestGoogleChatEvidenceMessageKeepsEventActionWithoutTimelineBody(t *testing.T) {
	evidence := noisyTimelineEvidence()
	evidence.ShowEventButton = true
	payload := buildGoogleChatEvidencePayload(&config.Config{GoogleChatAppMode: true}, evidence)

	body := marshalForTest(t, payload)
	assertNoTimelineBody(t, body)
	if !strings.Contains(body, "view_events") {
		t.Fatalf("expected app-mode Google Chat message to keep event action button: %s", body)
	}
}

func noisyTimelineEvidence() EvidenceChain {
	return EvidenceChain{
		Namespace:         "default",
		PodName:           "api",
		MetricProof:       "memory usage is high",
		ClusterContext:    "Namespace: default, Pod: api, Reason: CrashLoopBackOff\nDiagnosis: pod is restarting\nCategory: runtime\nConfidence: 82%\nRecommended Patch Strategy: image\nEvidence:\n- noisy low-level diagnostic",
		HistoricalPattern: "first occurrence",
		EventTimeline:     "NOISY_EVENT_TIMELINE_BODY should stay out of channel alerts",
		RootCause:         "container crashed",
		FinOpsImpact:      "low",
	}
}

func assertProfessionalEvidenceMessage(t *testing.T, body string) {
	t.Helper()
	for _, unwanted := range []string{
		"Cluster Context",
		"Metric Proof",
		"Historical Pattern",
		"FinOps Impact",
		"noisy low-level diagnostic",
	} {
		if strings.Contains(body, unwanted) {
			t.Fatalf("expected professional compact message to omit %q: %s", unwanted, body)
		}
	}
	for _, wanted := range []string{"Fixora Incident Report", "Summary", "Root", "Remediation", "Signals"} {
		if !strings.Contains(body, wanted) {
			t.Fatalf("expected professional message to include %q: %s", wanted, body)
		}
	}
}

func marshalForTest(t *testing.T, value interface{}) string {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal value: %v", err)
	}
	return string(body)
}

func assertNoInlineTimeline(t *testing.T, body string) {
	t.Helper()
	assertNoTimelineBody(t, body)
	if strings.Contains(body, "Event Timeline") {
		t.Fatalf("expected channel message to omit event timeline section: %s", body)
	}
}

func assertNoTimelineBody(t *testing.T, body string) {
	t.Helper()
	if strings.Contains(body, "NOISY_EVENT_TIMELINE_BODY") {
		t.Fatalf("expected channel message to omit raw event timeline body: %s", body)
	}
}

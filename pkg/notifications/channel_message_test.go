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
		ClusterContext:    "pod is restarting",
		HistoricalPattern: "first occurrence",
		EventTimeline:     "NOISY_EVENT_TIMELINE_BODY should stay out of channel alerts",
		RootCause:         "container crashed",
		FinOpsImpact:      "low",
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

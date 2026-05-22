package controller

import (
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestResolveIncidentWindowUsesContainerStateAnchor(t *testing.T) {
	now := time.Date(2026, 5, 21, 12, 0, 0, 0, time.UTC)
	finished := metav1.NewTime(now.Add(-25 * time.Minute))
	started := metav1.NewTime(now.Add(-30 * time.Minute))
	pod := &v1.Pod{Status: v1.PodStatus{ContainerStatuses: []v1.ContainerStatus{{
		Name: "api",
		LastTerminationState: v1.ContainerState{Terminated: &v1.ContainerStateTerminated{
			StartedAt:  started,
			FinishedAt: finished,
		}},
	}}}}

	got := ResolveIncidentWindow(pod, now, time.Hour)
	if got.Source != incidentWindowSourceContainer {
		t.Fatalf("source = %q, want %q", got.Source, incidentWindowSourceContainer)
	}
	if !got.Since.Equal(finished.Time.Add(-time.Hour)) {
		t.Fatalf("since = %s, want %s", got.Since, finished.Time.Add(-time.Hour))
	}
	if !got.Until.Equal(now.Add(defaultIncidentForwardBuffer)) {
		t.Fatalf("until = %s, want %s", got.Until, now.Add(defaultIncidentForwardBuffer))
	}
	if got.Confidence != 1 {
		t.Fatalf("confidence = %f, want 1", got.Confidence)
	}
}

func TestResolveIncidentWindowFallsBackToCreationTimestamp(t *testing.T) {
	now := time.Date(2026, 5, 21, 12, 0, 0, 0, time.UTC)
	created := metav1.NewTime(now.Add(-45 * time.Minute))
	pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{CreationTimestamp: created}}

	got := ResolveIncidentWindow(pod, now, 30*time.Minute)
	if got.Source != incidentWindowSourcePodCreated {
		t.Fatalf("source = %q, want %q", got.Source, incidentWindowSourcePodCreated)
	}
	if !got.Since.Equal(created.Time.Add(-30 * time.Minute)) {
		t.Fatalf("since = %s, want %s", got.Since, created.Time.Add(-30*time.Minute))
	}
}

func TestIncidentWindowLookbackBounds(t *testing.T) {
	now := time.Date(2026, 5, 21, 12, 0, 0, 0, time.UTC)
	window := IncidentWindow{Since: now.Add(-9 * 24 * time.Hour), Until: now, Source: incidentWindowSourceDefault}

	if got := window.Lookback(now); got != maxIncidentWindowLookback {
		t.Fatalf("lookback = %s, want %s", got, maxIncidentWindowLookback)
	}
}

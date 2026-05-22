package controller

import (
	"fmt"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
)

const (
	incidentWindowSourceDefault    = "default"
	incidentWindowSourcePodStart   = "pod.startTime"
	incidentWindowSourcePodCreated = "pod.creationTimestamp"
	incidentWindowSourceContainer  = "container.state"
	defaultIncidentLookback        = 2 * time.Hour
	defaultIncidentForwardBuffer   = 10 * time.Minute
	maxIncidentWindowLookback      = 7 * 24 * time.Hour
)

type IncidentWindow struct {
	Since      time.Time
	Until      time.Time
	Source     string
	Confidence float64
}

func ResolveIncidentWindow(pod *v1.Pod, now time.Time, lookback time.Duration) IncidentWindow {
	if now.IsZero() {
		now = time.Now()
	}
	now = now.UTC()
	if lookback <= 0 {
		lookback = defaultIncidentLookback
	}
	if lookback > maxIncidentWindowLookback {
		lookback = maxIncidentWindowLookback
	}

	anchor, source, confidence := incidentAnchorFromPod(pod)
	if anchor.IsZero() {
		anchor = now
		source = incidentWindowSourceDefault
		confidence = 0
	}
	anchor = anchor.UTC()
	until := now.Add(defaultIncidentForwardBuffer)
	if anchor.After(until) {
		until = anchor.Add(defaultIncidentForwardBuffer)
	}
	since := anchor.Add(-lookback)
	if !since.Before(until) {
		since = until.Add(-lookback)
	}
	return IncidentWindow{
		Since:      since,
		Until:      until,
		Source:     source,
		Confidence: confidence,
	}
}

func (w IncidentWindow) Lookback(now time.Time) time.Duration {
	if now.IsZero() {
		now = time.Now()
	}
	if w.Since.IsZero() {
		return defaultIncidentLookback
	}
	d := now.UTC().Sub(w.Since.UTC())
	if d <= 0 {
		return defaultIncidentLookback
	}
	if d > maxIncidentWindowLookback {
		return maxIncidentWindowLookback
	}
	return d
}

func (w IncidentWindow) Summary() string {
	if w.Since.IsZero() || w.Until.IsZero() {
		return ""
	}
	source := strings.TrimSpace(w.Source)
	if source == "" {
		source = incidentWindowSourceDefault
	}
	return fmt.Sprintf(
		"Incident Window: %s to %s (source: %s, confidence: %.0f%%)",
		w.Since.UTC().Format(time.RFC3339),
		w.Until.UTC().Format(time.RFC3339),
		source,
		w.Confidence*100,
	)
}

func incidentAnchorFromPod(pod *v1.Pod) (time.Time, string, float64) {
	if pod == nil {
		return time.Time{}, "", 0
	}
	var latest time.Time
	for _, status := range pod.Status.InitContainerStatuses {
		latest = laterTime(latest, containerStateTime(status.State))
		latest = laterTime(latest, containerStateTime(status.LastTerminationState))
	}
	for _, status := range pod.Status.ContainerStatuses {
		latest = laterTime(latest, containerStateTime(status.State))
		latest = laterTime(latest, containerStateTime(status.LastTerminationState))
	}
	if !latest.IsZero() {
		return latest, incidentWindowSourceContainer, 1
	}
	if pod.Status.StartTime != nil && !pod.Status.StartTime.IsZero() {
		return pod.Status.StartTime.Time, incidentWindowSourcePodStart, 0.85
	}
	if !pod.CreationTimestamp.IsZero() {
		return pod.CreationTimestamp.Time, incidentWindowSourcePodCreated, 0.7
	}
	return time.Time{}, "", 0
}

func containerStateTime(state v1.ContainerState) time.Time {
	switch {
	case state.Waiting != nil:
		return time.Time{}
	case state.Running != nil:
		return state.Running.StartedAt.Time
	case state.Terminated != nil:
		return laterTime(state.Terminated.StartedAt.Time, state.Terminated.FinishedAt.Time)
	default:
		return time.Time{}
	}
}

func laterTime(a, b time.Time) time.Time {
	if a.IsZero() {
		return b
	}
	if b.IsZero() || a.After(b) {
		return a
	}
	return b
}

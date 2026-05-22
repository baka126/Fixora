package controller

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"fixora/pkg/helm"
	v1 "k8s.io/api/core/v1"
)

type EvidenceType string
type EvidenceSideEffect string

const (
	EvidenceTypeMetrics EvidenceType = "metrics"
	EvidenceTypeEvents  EvidenceType = "events"
	EvidenceTypeLogs    EvidenceType = "logs"

	EvidenceSideEffectReadOnly EvidenceSideEffect = "read_only"
)

type EvidenceCollector struct {
	Name            string
	Source          string
	Type            EvidenceType
	SideEffectLevel EvidenceSideEffect
	Cost            string
	Available       func(*Controller) bool
	Collect         func(context.Context, *Controller, *v1.Pod, IncidentWindow) (CollectedEvidence, error)
}

type CollectedEvidence struct {
	MetricProof            string
	EventTimeline          string
	Logs                   string
	StackTrace             string
	ClusterContextAppend   []string
	ExecutedCollectorNames []string
}

func (c *Controller) collectIncidentEvidence(ctx context.Context, pod *v1.Pod, window IncidentWindow) CollectedEvidence {
	var out CollectedEvidence
	for _, collector := range defaultEvidenceCollectors() {
		if collector.Available != nil && !collector.Available(c) {
			continue
		}
		collected, err := collector.Collect(ctx, c, pod, window)
		if err != nil {
			slog.Debug("Evidence collector failed", "collector", collector.Name, "source", collector.Source, "ns", pod.Namespace, "pod", pod.Name, "error", err)
			continue
		}
		out.ExecutedCollectorNames = append(out.ExecutedCollectorNames, collector.Name)
		if collected.MetricProof != "" {
			out.MetricProof = collected.MetricProof
		}
		if collected.EventTimeline != "" {
			out.EventTimeline = collected.EventTimeline
		}
		if collected.Logs != "" {
			out.Logs = collected.Logs
			if patterns := ClusterLogPatterns(collected.Logs, 5); len(patterns) > 0 {
				out.ClusterContextAppend = append(out.ClusterContextAppend, formatLogPatterns(patterns))
			}
		}
		if collected.StackTrace != "" {
			out.StackTrace = collected.StackTrace
		}
		out.ClusterContextAppend = append(out.ClusterContextAppend, collected.ClusterContextAppend...)
	}
	return out
}

func defaultEvidenceCollectors() []EvidenceCollector {
	return []EvidenceCollector{
		{
			Name:            "pod_metric_proof",
			Source:          "prometheus_or_kubernetes_metrics",
			Type:            EvidenceTypeMetrics,
			SideEffectLevel: EvidenceSideEffectReadOnly,
			Cost:            "cheap",
			Available:       func(c *Controller) bool { return c != nil && c.promClient != nil },
			Collect: func(ctx context.Context, c *Controller, pod *v1.Pod, window IncidentWindow) (CollectedEvidence, error) {
				_ = ctx
				usage, _ := c.promClient.GetPodUsage(pod.Namespace, pod.Name)
				request, limit, _ := c.promClient.GetPodLimits(pod.Namespace, pod.Name)
				rss, cache := c.getGranularMetrics(pod.Namespace, pod.Name)
				metricSource := "Prometheus"
				if _, err := c.promClient.GetHistory(pod.Namespace, pod.Name, window.Lookback(timeNowUTC())); err != nil {
					metricSource = "K8s API (Historical trend unavailable)"
				}
				return CollectedEvidence{MetricProof: fmt.Sprintf("Metric Source: %s\nMemory Usage: %.2f MiB (RSS: %.2f, Cache: %.2f)\nLimit: %.2f MiB, Request: %.2f MiB",
					metricSource, usage/1024/1024, rss/1024/1024, cache/1024/1024, limit/1024/1024, request/1024/1024)}, nil
			},
		},
		{
			Name:            "pod_events",
			Source:          "kubernetes_events",
			Type:            EvidenceTypeEvents,
			SideEffectLevel: EvidenceSideEffectReadOnly,
			Cost:            "cheap",
			Available:       func(c *Controller) bool { return c != nil && c.clientset != nil },
			Collect: func(ctx context.Context, c *Controller, pod *v1.Pod, _ IncidentWindow) (CollectedEvidence, error) {
				eventsTimeline, err := c.getPodEvents(ctx, pod)
				if err != nil {
					return CollectedEvidence{}, err
				}
				return CollectedEvidence{EventTimeline: eventsTimeline}, nil
			},
		},
		{
			Name:            "pod_logs",
			Source:          "kubernetes_logs",
			Type:            EvidenceTypeLogs,
			SideEffectLevel: EvidenceSideEffectReadOnly,
			Cost:            "moderate",
			Available:       func(c *Controller) bool { return c != nil && c.clientset != nil },
			Collect: func(ctx context.Context, c *Controller, pod *v1.Pod, _ IncidentWindow) (CollectedEvidence, error) {
				logs, err := c.getPodLogs(ctx, pod.Namespace, pod.Name)
				if err != nil {
					return CollectedEvidence{}, err
				}
				return CollectedEvidence{Logs: logs, StackTrace: extractStackTrace(logs)}, nil
			},
		},
		{
			Name:            "helm_runtime_metadata",
			Source:          "helm_cli",
			Type:            EvidenceTypeEvents,
			SideEffectLevel: EvidenceSideEffectReadOnly,
			Cost:            "moderate",
			Available:       func(c *Controller) bool { return c != nil && c.clientset != nil },
			Collect: func(ctx context.Context, c *Controller, pod *v1.Pod, _ IncidentWindow) (CollectedEvidence, error) {
				releaseName, releaseNamespace := helmReleaseForPod(pod)
				if releaseName == "" {
					return CollectedEvidence{}, nil
				}
				inspection, err := helm.NewRuntimeInspector().InspectRelease(ctx, releaseName, releaseNamespace)
				if err != nil {
					return CollectedEvidence{}, err
				}
				parts := []string{fmt.Sprintf("release=%s/%s", inspection.Namespace, inspection.ReleaseName)}
				if inspection.Chart != "" {
					parts = append(parts, "chart="+inspection.Chart)
				}
				if inspection.AppVersion != "" {
					parts = append(parts, "appVersion="+inspection.AppVersion)
				}
				if inspection.Status != "" {
					parts = append(parts, "status="+inspection.Status)
				}
				if inspection.Revision > 0 {
					parts = append(parts, fmt.Sprintf("revision=%d", inspection.Revision))
				}
				return CollectedEvidence{ClusterContextAppend: []string{"Helm Runtime: " + strings.Join(parts, ", ")}}, nil
			},
		},
	}
}

func helmReleaseForPod(pod *v1.Pod) (string, string) {
	if pod == nil {
		return "", ""
	}
	releaseName := strings.TrimSpace(pod.Annotations["meta.helm.sh/release-name"])
	releaseNamespace := strings.TrimSpace(pod.Annotations["meta.helm.sh/release-namespace"])
	if releaseName == "" {
		releaseName = strings.TrimSpace(pod.Labels["app.kubernetes.io/instance"])
	}
	if releaseNamespace == "" {
		releaseNamespace = pod.Namespace
	}
	return releaseName, releaseNamespace
}

func extractStackTrace(logs string) string {
	lines := strings.Split(logs, "\n")
	var traceLines []string
	inTrace := false
	for _, line := range lines {
		if strings.Contains(line, "stack trace:") || strings.Contains(line, "panic:") || strings.Contains(line, "goroutine") {
			inTrace = true
		}
		if inTrace {
			traceLines = append(traceLines, line)
		}
		if len(traceLines) > 50 {
			break
		}
	}
	return strings.Join(traceLines, "\n")
}

func timeNowUTC() time.Time {
	return time.Now().UTC()
}

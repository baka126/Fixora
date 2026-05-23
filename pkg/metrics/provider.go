package metrics

import (
	"time"

	"github.com/prometheus/common/model"
)

type PodMetricResult struct {
	Namespace string
	PodName   string
	Value     float64
}

type SLOBurnRateResult struct {
	Namespace     string
	PodName       string
	ShortBurnRate float64
	LongBurnRate  float64
	ErrorBudget   float64
}

type TrafficEdge struct {
	SourceNamespace      string
	SourceWorkload       string
	DestinationNamespace string
	DestinationWorkload  string
	RequestsPerSecond    float64
}

// MetricsProvider is the unified interface for gathering K8s resource metrics.
type MetricsProvider interface {
	// GetPodUsage returns the current memory usage for a pod.
	GetPodUsage(ns, pod string) (float64, error)

	// GetPodLimits returns the memory requests and limits for a pod.
	GetPodLimits(ns, pod string) (float64, float64, error) // Requests, Limits

	// GetPodCPULimits returns the CPU requests and limits for a pod (in cores).
	GetPodCPULimits(ns, pod string) (float64, float64, error) // Requests, Limits

	// GetPodMemoryRSS returns the RSS memory for a pod.
	GetPodMemoryRSS(ns, pod string) (float64, error)

	// GetPodMemoryCache returns the cache memory for a pod.
	GetPodMemoryCache(ns, pod string) (float64, error)

	// GetHistory returns a historical matrix of memory usage (if supported by the provider).
	GetHistory(ns, pod string, d time.Duration) (model.Matrix, error)

	// GetHTTPErrorRate returns the 5xx error rate for a pod.
	GetHTTPErrorRate(ns, pod string) (float64, error)

	// GetP99Latency returns the 99th percentile latency for a pod.
	GetP99Latency(ns, pod string) (float64, error)

	// GetHTTPRequestsPerSecond returns the current RPS for a pod.
	GetHTTPRequestsPerSecond(ns, pod string) (float64, error)
}

// BulkMetricsProvider extends MetricsProvider with methods to find problematic pods across the cluster in one query.
type BulkMetricsProvider interface {
	MetricsProvider
	// GetHighErrorRatePods finds all pods exceeding the error rate threshold.
	GetHighErrorRatePods(threshold float64) ([]PodMetricResult, error)
	// GetHighLatencyPods finds all pods exceeding the latency threshold.
	GetHighLatencyPods(threshold float64) ([]PodMetricResult, error)
}

type SLOBurnRateProvider interface {
	GetHighSLOBurnRatePods(objective float64, shortWindow, longWindow time.Duration, threshold float64) ([]SLOBurnRateResult, error)
}

type TrafficGraphProvider interface {
	GetTrafficEdges(window time.Duration, minRPS float64) ([]TrafficEdge, error)
}

type RightSizingProvider interface {
	GetPodMemoryStats(ns, pod string, d time.Duration) (avgBytes, peakBytes float64, err error)
}

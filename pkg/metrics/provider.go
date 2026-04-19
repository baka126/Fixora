package metrics

import (
	"time"

	"github.com/prometheus/common/model"
)

// MetricsProvider is the unified interface for gathering K8s resource metrics.
type MetricsProvider interface {
	// GetPodUsage returns the current memory usage for a pod.
	GetPodUsage(ns, pod string) (float64, error)

	// GetPodLimits returns the memory requests and limits for a pod.
	GetPodLimits(ns, pod string) (float64, float64, error) // Requests, Limits

	// GetHistory returns a historical matrix of memory usage (if supported by the provider).
	GetHistory(ns, pod string, d time.Duration) (model.Matrix, error)
}

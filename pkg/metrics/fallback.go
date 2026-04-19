package metrics

import (
	"time"

	"github.com/prometheus/common/model"
)

// FallbackProvider wraps two MetricsProviders and provides fallback logic.
// It prioritizes the Primary provider and falls back to Secondary on failure.
type FallbackProvider struct {
	Primary   MetricsProvider
	Secondary MetricsProvider
}

// NewFallbackProvider creates a new FallbackProvider.
func NewFallbackProvider(primary, secondary MetricsProvider) *FallbackProvider {
	return &FallbackProvider{
		Primary:   primary,
		Secondary: secondary,
	}
}

// GetPodUsage attempts to get usage from Primary, falling back to Secondary.
func (f *FallbackProvider) GetPodUsage(ns, pod string) (float64, error) {
	val, err := f.Primary.GetPodUsage(ns, pod)
	if err != nil {
		return f.Secondary.GetPodUsage(ns, pod)
	}
	return val, nil
}

// GetPodLimits attempts to get limits from Primary, falling back to Secondary.
func (f *FallbackProvider) GetPodLimits(ns, pod string) (float64, float64, error) {
	req, lim, err := f.Primary.GetPodLimits(ns, pod)
	if err != nil {
		return f.Secondary.GetPodLimits(ns, pod)
	}
	return req, lim, nil
}

// GetPodCPULimits attempts to get CPU limits from Primary, falling back to Secondary.
func (f *FallbackProvider) GetPodCPULimits(ns, pod string) (float64, float64, error) {
	req, lim, err := f.Primary.GetPodCPULimits(ns, pod)
	if err != nil {
		return f.Secondary.GetPodCPULimits(ns, pod)
	}
	return req, lim, nil
}

// GetHistory attempts to get history from Primary, falling back to Secondary.
func (f *FallbackProvider) GetHistory(ns, pod string, d time.Duration) (model.Matrix, error) {
	matrix, err := f.Primary.GetHistory(ns, pod, d)
	if err != nil {
		return f.Secondary.GetHistory(ns, pod, d)
	}
	return matrix, nil
}

// Ensure FallbackProvider implements MetricsProvider
var _ MetricsProvider = (*FallbackProvider)(nil)

package metrics

import (
	"fmt"
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
	if f.Primary == nil && f.Secondary == nil {
		return 0, fmt.Errorf("no metrics providers configured")
	}

	if f.Primary != nil {
		val, err := f.Primary.GetPodUsage(ns, pod)
		if err == nil {
			return val, nil
		}
		if f.Secondary == nil {
			return 0, err
		}
	}

	return f.Secondary.GetPodUsage(ns, pod)
}

// GetPodLimits attempts to get limits from Primary, falling back to Secondary.
func (f *FallbackProvider) GetPodLimits(ns, pod string) (float64, float64, error) {
	if f.Primary == nil && f.Secondary == nil {
		return 0, 0, fmt.Errorf("no metrics providers configured")
	}

	if f.Primary != nil {
		req, lim, err := f.Primary.GetPodLimits(ns, pod)
		if err == nil {
			return req, lim, nil
		}
		if f.Secondary == nil {
			return 0, 0, err
		}
	}

	return f.Secondary.GetPodLimits(ns, pod)
}

// GetPodCPULimits attempts to get CPU limits from Primary, falling back to Secondary.
func (f *FallbackProvider) GetPodCPULimits(ns, pod string) (float64, float64, error) {
	if f.Primary == nil && f.Secondary == nil {
		return 0, 0, fmt.Errorf("no metrics providers configured")
	}

	if f.Primary != nil {
		req, lim, err := f.Primary.GetPodCPULimits(ns, pod)
		if err == nil {
			return req, lim, nil
		}
		if f.Secondary == nil {
			return 0, 0, err
		}
	}

	return f.Secondary.GetPodCPULimits(ns, pod)
}

// GetHistory attempts to get history from Primary, falling back to Secondary.
func (f *FallbackProvider) GetHistory(ns, pod string, d time.Duration) (model.Matrix, error) {
	if f.Primary == nil && f.Secondary == nil {
		return nil, fmt.Errorf("no metrics providers configured")
	}

	if f.Primary != nil {
		matrix, err := f.Primary.GetHistory(ns, pod, d)
		if err == nil {
			return matrix, nil
		}
		if f.Secondary == nil {
			return nil, err
		}
	}

	return f.Secondary.GetHistory(ns, pod, d)
}

// Ensure FallbackProvider implements MetricsProvider
var _ MetricsProvider = (*FallbackProvider)(nil)

package finops

import (
	"fmt"
	"strings"
)

// PricingProvider is the unified interface for fetching cloud resource costs.
type PricingProvider interface {
	// GetProfileForInstance fetches the pricing profile (CPU/Mem rates) for a specific instance.
	GetProfileForInstance(vendor, region, instanceType string) (*PricingProfile, error)
}

// MultiPricingProvider coordinates multiple pricing strategies (e.g., Infracost vs. Direct Vendor API).
type MultiPricingProvider struct {
	providers []PricingProvider
}

// NewMultiPricingProvider creates a provider that tries multiple strategies in order.
func NewMultiPricingProvider(providers ...PricingProvider) *MultiPricingProvider {
	return &MultiPricingProvider{providers: providers}
}

func (m *MultiPricingProvider) GetProfileForInstance(vendor, region, instanceType string) (*PricingProfile, error) {
	var lastErr error
	for _, p := range m.providers {
		profile, err := p.GetProfileForInstance(vendor, region, instanceType)
		if err == nil && profile != nil {
			return profile, nil
		}
		if err != nil {
			lastErr = err
		}
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("no pricing profile found for %s %s in %s", vendor, instanceType, region)
}

// DetectVendor uses heuristics to identify the cloud provider from instance/region metadata.
func DetectVendor(instanceType, region string) string {
	if strings.HasPrefix(instanceType, "Standard_") || strings.HasPrefix(instanceType, "Basic_") || !strings.Contains(region, "-") {
		return "azure"
	}
	// Simple heuristic for GCP: regions like us-central1, nodes with machine type like n1-standard-1
	if strings.Contains(instanceType, "-") && (strings.HasPrefix(instanceType, "n1-") || strings.HasPrefix(instanceType, "e2-") || strings.HasPrefix(instanceType, "c2-")) {
		return "gcp"
	}
	return "aws"
}

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

// ParseNodeLabels extracts cloud provider details from standard Kubernetes node labels.
func ParseNodeLabels(labels map[string]string) (vendor, region, instanceType string) {
	return ParseNodeMetadata(labels, "")
}

// ParseNodeMetadata extracts cloud provider details from standard Kubernetes node labels and providerID.
func ParseNodeMetadata(labels map[string]string, providerID string) (vendor, region, instanceType string) {
	// Standard topology labels
	region = labels["topology.kubernetes.io/region"]
	if region == "" {
		region = labels["failure-domain.beta.kubernetes.io/region"]
	}

	instanceType = labels["node.kubernetes.io/instance-type"]
	if instanceType == "" {
		instanceType = labels["beta.kubernetes.io/instance-type"]
	}

	// Try to determine vendor explicitly
	if providerID != "" {
		if strings.HasPrefix(providerID, "aws://") {
			vendor = "aws"
		} else if strings.HasPrefix(providerID, "gce://") {
			vendor = "gcp"
		} else if strings.HasPrefix(providerID, "azure://") {
			vendor = "azure"
		}
	}

	// Vendor specific labels
	if vendor == "" {
		if _, ok := labels["eks.amazonaws.com/nodegroup"]; ok {
			vendor = "aws"
		} else if _, ok := labels["cloud.google.com/gke-nodepool"]; ok {
			vendor = "gcp"
		} else if _, ok := labels["kubernetes.azure.com/cluster"]; ok {
			vendor = "azure"
		}
	}

	if vendor == "" && instanceType != "" {
		vendor = DetectVendor(instanceType, region)
	}

	return vendor, region, instanceType
}

// DetectVendor uses heuristics to identify the cloud provider from instance/region metadata.
func DetectVendor(instanceType, region string) string {
	if strings.HasPrefix(instanceType, "Standard_") || strings.HasPrefix(instanceType, "Basic_") || !strings.Contains(region, "-") {
		return "azure"
	}
	if strings.Contains(instanceType, "-") && (strings.HasPrefix(instanceType, "n1-") || strings.HasPrefix(instanceType, "e2-") || strings.HasPrefix(instanceType, "c2-")) {
		return "gcp"
	}
	return "aws"
}

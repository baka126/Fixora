package finops

import "testing"

type recordingPricingProvider struct {
	vendor       string
	region       string
	instanceType string
}

func (p *recordingPricingProvider) GetProfileForInstance(vendor, region, instanceType string) (*PricingProfile, error) {
	p.vendor = vendor
	p.region = region
	p.instanceType = instanceType
	return &PricingProfile{
		Name:              "test",
		CPURatePerHour:    0.04,
		MemoryRatePerHour: 0.005,
	}, nil
}

func TestParseNodeMetadataUsesProviderID(t *testing.T) {
	labels := map[string]string{
		"topology.kubernetes.io/region":    "us-east-1",
		"node.kubernetes.io/instance-type": "m6i.large",
	}

	vendor, region, instanceType := ParseNodeMetadata(labels, "aws:///us-east-1a/i-123456")

	if vendor != "aws" || region != "us-east-1" || instanceType != "m6i.large" {
		t.Fatalf("ParseNodeMetadata() = (%q, %q, %q), want aws/us-east-1/m6i.large", vendor, region, instanceType)
	}
}

func TestCalculateClusterCostPassesProviderIDVendor(t *testing.T) {
	provider := &recordingPricingProvider{}

	cost, err := CalculateClusterCost([]NodeInfo{{
		Name:       "node-1",
		ProviderID: "gce://project/us-central1-a/node-1",
		Labels: map[string]string{
			"topology.kubernetes.io/region":    "us-central1",
			"node.kubernetes.io/instance-type": "e2-standard-4",
		},
	}}, provider)
	if err != nil {
		t.Fatalf("CalculateClusterCost returned error: %v", err)
	}
	if cost <= 0 {
		t.Fatalf("CalculateClusterCost cost = %f, want positive", cost)
	}
	if provider.vendor != "gcp" || provider.region != "us-central1" || provider.instanceType != "e2-standard-4" {
		t.Fatalf("provider received (%q, %q, %q), want gcp/us-central1/e2-standard-4", provider.vendor, provider.region, provider.instanceType)
	}
}

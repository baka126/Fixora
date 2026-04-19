package finops

import "fmt"

// GCPPricingClient is a placeholder for direct GCP pricing if needed.
// For now, we prefer using the unified Infracost API for GCP.
type GCPPricingClient struct{}

func NewGCPPricingClient() *GCPPricingClient {
	return &GCPPricingClient{}
}

func (c *GCPPricingClient) GetProfileForInstance(vendor, region, instanceType string) (*PricingProfile, error) {
	if vendor != "gcp" {
		return nil, fmt.Errorf("GCPPricingClient only handles 'gcp' vendor, got: %s", vendor)
	}
	return nil, fmt.Errorf("direct GCP pricing not implemented (use Infracost)")
}

var DefaultGCPClient = NewGCPPricingClient()

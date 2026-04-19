package finops

// GCPPricingClient is a placeholder for direct GCP pricing if needed.
// For now, we prefer using the unified Infracost API for GCP.
type GCPPricingClient struct{}

func NewGCPPricingClient() *GCPPricingClient {
	return &GCPPricingClient{}
}

var DefaultGCPClient = NewGCPPricingClient()

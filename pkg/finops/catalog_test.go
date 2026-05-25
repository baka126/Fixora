package finops

import (
	"fmt"
	"strings"
	"testing"
)

type failingPricingProvider struct{}

func (failingPricingProvider) GetProfileForInstance(vendor, region, instanceType string) (*PricingProfile, error) {
	return nil, fmt.Errorf("not configured")
}

func TestCatalogPricingProviderUsesCorootStyleUnitAllocation(t *testing.T) {
	profile, err := NewCatalogPricingProvider().GetProfileForInstance("aws", "us-east-1", "m6i.large")
	if err != nil {
		t.Fatalf("GetProfileForInstance() error = %v", err)
	}
	if profile.CPURatePerHour <= 0 || profile.MemoryRatePerHour <= 0 {
		t.Fatalf("profile rates = %#v, want positive rates", profile)
	}
	if profile.CPURatePerHour != profile.MemoryRatePerHour {
		t.Fatalf("catalog rates = cpu %.6f memory %.6f, want equal Coroot-style unit rates", profile.CPURatePerHour, profile.MemoryRatePerHour)
	}
	total := profile.CPURatePerHour*2 + profile.MemoryRatePerHour*8
	if total < 0.095 || total > 0.097 {
		t.Fatalf("reconstructed hourly total = %.6f, want about 0.096", total)
	}
	if got := CalculateMonthlyNodeCost(*profile); got < 70 || got > 71 {
		t.Fatalf("monthly node cost = %.2f, want about 70.08", got)
	}
	xlarge, err := NewCatalogPricingProvider().GetProfileForInstance("aws", "us-east-1", "m6i.xlarge")
	if err != nil {
		t.Fatalf("GetProfileForInstance(xlarge) error = %v", err)
	}
	if got := CalculateMonthlyNodeCost(*xlarge); got < 140 || got > 141 {
		t.Fatalf("xlarge monthly node cost = %.2f, want about 140.16", got)
	}
}

func TestCatalogPricingProviderFallsBackToDefaultVendorRegion(t *testing.T) {
	profile, err := NewCatalogPricingProvider().GetProfileForInstance("gcp", "europe-west1", "e2-standard-4")
	if err != nil {
		t.Fatalf("GetProfileForInstance() error = %v", err)
	}
	if !strings.Contains(profile.Name, "us-central1") {
		t.Fatalf("profile name = %q, want default catalog region", profile.Name)
	}
}

func TestMultiPricingProviderUsesCatalogAfterUnavailableProvider(t *testing.T) {
	provider := NewMultiPricingProvider(failingPricingProvider{}, NewCatalogPricingProvider())
	profile, err := provider.GetProfileForInstance("azure", "eastus", "Standard_D2s_v3")
	if err != nil {
		t.Fatalf("GetProfileForInstance() error = %v", err)
	}
	if profile == nil || !strings.Contains(profile.Name, "Catalog AZURE") {
		t.Fatalf("profile = %#v, want catalog profile", profile)
	}
}

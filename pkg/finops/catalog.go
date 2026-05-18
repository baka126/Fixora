package finops

import (
	"fmt"
	"strings"
)

type catalogPrice struct {
	Vendor       string
	Region       string
	InstanceType string
	VCPUs        float64
	MemoryGiB    float64
	HourlyUSD    float64
}

type CatalogPricingProvider struct {
	prices map[string]catalogPrice
}

func NewCatalogPricingProvider() *CatalogPricingProvider {
	prices := map[string]catalogPrice{}
	for _, price := range defaultCatalogPrices() {
		prices[catalogKey(price.Vendor, price.Region, price.InstanceType)] = price
	}
	return &CatalogPricingProvider{prices: prices}
}

func (p *CatalogPricingProvider) GetProfileForInstance(vendor, region, instanceType string) (*PricingProfile, error) {
	if p == nil {
		return nil, fmt.Errorf("catalog pricing provider is nil")
	}
	price, ok := p.prices[catalogKey(vendor, region, instanceType)]
	if !ok {
		price, ok = p.prices[catalogKey(vendor, defaultCatalogRegion(vendor), instanceType)]
	}
	if !ok {
		return nil, fmt.Errorf("catalog price not found for %s %s in %s", vendor, instanceType, region)
	}
	if price.HourlyUSD <= 0 || price.VCPUs <= 0 || price.MemoryGiB <= 0 {
		return nil, fmt.Errorf("invalid catalog price for %s %s in %s", vendor, instanceType, region)
	}

	// Coroot-style allocation: spread total node hourly price across one vCPU
	// unit plus one GiB memory unit. This keeps node and requested-resource
	// cost views internally consistent when exact provider billing APIs are not available.
	perUnit := price.HourlyUSD / (price.VCPUs + price.MemoryGiB)
	return &PricingProfile{
		Name:              fmt.Sprintf("Catalog %s %s (%s)", strings.ToUpper(price.Vendor), price.InstanceType, price.Region),
		CPURatePerHour:    perUnit,
		MemoryRatePerHour: perUnit,
	}, nil
}

func catalogKey(vendor, region, instanceType string) string {
	return strings.ToLower(strings.TrimSpace(vendor)) + "/" +
		strings.ToLower(strings.TrimSpace(region)) + "/" +
		strings.ToLower(strings.TrimSpace(instanceType))
}

func defaultCatalogRegion(vendor string) string {
	switch strings.ToLower(strings.TrimSpace(vendor)) {
	case "gcp":
		return "us-central1"
	case "azure":
		return "eastus"
	default:
		return "us-east-1"
	}
}

func defaultCatalogPrices() []catalogPrice {
	return []catalogPrice{
		// AWS on-demand Linux, representative us-east-1 prices.
		{Vendor: "aws", Region: "us-east-1", InstanceType: "t3.small", VCPUs: 2, MemoryGiB: 2, HourlyUSD: 0.0208},
		{Vendor: "aws", Region: "us-east-1", InstanceType: "t3.medium", VCPUs: 2, MemoryGiB: 4, HourlyUSD: 0.0416},
		{Vendor: "aws", Region: "us-east-1", InstanceType: "m5.large", VCPUs: 2, MemoryGiB: 8, HourlyUSD: 0.096},
		{Vendor: "aws", Region: "us-east-1", InstanceType: "m5.xlarge", VCPUs: 4, MemoryGiB: 16, HourlyUSD: 0.192},
		{Vendor: "aws", Region: "us-east-1", InstanceType: "m6i.large", VCPUs: 2, MemoryGiB: 8, HourlyUSD: 0.096},
		{Vendor: "aws", Region: "us-east-1", InstanceType: "m6i.xlarge", VCPUs: 4, MemoryGiB: 16, HourlyUSD: 0.192},
		{Vendor: "aws", Region: "us-east-1", InstanceType: "c6i.large", VCPUs: 2, MemoryGiB: 4, HourlyUSD: 0.085},
		{Vendor: "aws", Region: "us-east-1", InstanceType: "c6i.xlarge", VCPUs: 4, MemoryGiB: 8, HourlyUSD: 0.17},
		{Vendor: "aws", Region: "us-east-1", InstanceType: "r6i.large", VCPUs: 2, MemoryGiB: 16, HourlyUSD: 0.126},
		{Vendor: "aws", Region: "us-east-1", InstanceType: "r6i.xlarge", VCPUs: 4, MemoryGiB: 32, HourlyUSD: 0.252},

		// GCP on-demand Linux, representative us-central1 prices.
		{Vendor: "gcp", Region: "us-central1", InstanceType: "e2-standard-2", VCPUs: 2, MemoryGiB: 8, HourlyUSD: 0.06701},
		{Vendor: "gcp", Region: "us-central1", InstanceType: "e2-standard-4", VCPUs: 4, MemoryGiB: 16, HourlyUSD: 0.13402},
		{Vendor: "gcp", Region: "us-central1", InstanceType: "n2-standard-2", VCPUs: 2, MemoryGiB: 8, HourlyUSD: 0.0971},
		{Vendor: "gcp", Region: "us-central1", InstanceType: "n2-standard-4", VCPUs: 4, MemoryGiB: 16, HourlyUSD: 0.1942},
		{Vendor: "gcp", Region: "us-central1", InstanceType: "c2-standard-4", VCPUs: 4, MemoryGiB: 16, HourlyUSD: 0.2088},

		// Azure Linux consumption, representative eastus prices.
		{Vendor: "azure", Region: "eastus", InstanceType: "Standard_B2s", VCPUs: 2, MemoryGiB: 4, HourlyUSD: 0.0416},
		{Vendor: "azure", Region: "eastus", InstanceType: "Standard_D2s_v3", VCPUs: 2, MemoryGiB: 8, HourlyUSD: 0.096},
		{Vendor: "azure", Region: "eastus", InstanceType: "Standard_D4s_v3", VCPUs: 4, MemoryGiB: 16, HourlyUSD: 0.192},
		{Vendor: "azure", Region: "eastus", InstanceType: "Standard_D2s_v5", VCPUs: 2, MemoryGiB: 8, HourlyUSD: 0.096},
		{Vendor: "azure", Region: "eastus", InstanceType: "Standard_D4s_v5", VCPUs: 4, MemoryGiB: 16, HourlyUSD: 0.192},
	}
}

var DefaultCatalogClient = NewCatalogPricingProvider()

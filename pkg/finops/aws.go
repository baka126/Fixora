package finops

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type InstancePricingResponse struct {
	Results []InstancePricing `json:"results"`
}

type InstancePricing struct {
	InstanceType  string  `json:"instanceType"`
	Region        string  `json:"region"`
	OnDemandPrice float64 `json:"onDemandPrice"`
	Vcpus         float64 `json:"vcpus"`
	MemoryGiB     float64 `json:"memoryGiB"`
}

type AWSPricingClient struct {
	cache      map[string]*PricingProfile
	httpClient *http.Client
	mu         sync.RWMutex
}

func NewAWSPricingClient() *AWSPricingClient {
	return &AWSPricingClient{
		cache: make(map[string]*PricingProfile),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetProfileForInstance fetches live pricing for a specific instance type and region.
func (c *AWSPricingClient) GetProfileForInstance(vendor, region, instanceType string) (*PricingProfile, error) {
	if vendor != "aws" {
		return nil, fmt.Errorf("AWSPricingClient only handles 'aws' vendor, got: %s", vendor)
	}
	cacheKey := fmt.Sprintf("%s:%s", instanceType, region)

	c.mu.RLock()
	if profile, ok := c.cache[cacheKey]; ok {
		c.mu.RUnlock()
		return profile, nil
	}
	c.mu.RUnlock()

	// Fetch from API
	url := fmt.Sprintf("https://go.runs-on.com/api/instances/%s?region=%s&platform=Linux/UNIX", instanceType, region)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pricing API returned status: %d", resp.StatusCode)
	}

	var data InstancePricingResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	if len(data.Results) == 0 {
		return nil, fmt.Errorf("no pricing data found for %s in %s", instanceType, region)
	}

	// Use the first result (usually region/AZ specific)
	p := data.Results[0]
	if p.Vcpus <= 0 || p.MemoryGiB <= 0 {
		return nil, fmt.Errorf("invalid pricing dimensions for %s in %s: vcpus=%.2f memoryGiB=%.2f", instanceType, region, p.Vcpus, p.MemoryGiB)
	}

	// Derive CPU and Memory rates.
	// Heuristic: Allocate 50% of instance cost to CPU and 50% to Memory.
	// This is a common industry standard for resource-based cost allocation.
	cpuRate := (p.OnDemandPrice * 0.5) / p.Vcpus
	memRate := (p.OnDemandPrice * 0.5) / p.MemoryGiB

	profile := &PricingProfile{
		Name:             fmt.Sprintf("AWS %s (%s)", instanceType, region),
		CPURatePerHour:   cpuRate,
		MemoryRatePerHour: memRate,
	}

	c.mu.Lock()
	c.cache[cacheKey] = profile
	c.mu.Unlock()

	return profile, nil
}

// Global client instance
var DefaultAWSClient = NewAWSPricingClient()

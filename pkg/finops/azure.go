package finops

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
)

type AzurePricingResponse struct {
	Items []AzurePriceItem `json:"Items"`
}

type AzurePriceItem struct {
	RetailPrice   float64 `json:"retailPrice"`
	UnitOfMeasure string  `json:"unitOfMeasure"`
	ArmRegionName string  `json:"armRegionName"`
	ArmSkuName    string  `json:"armSkuName"`
	ProductName   string  `json:"productName"`
}

type AzurePricingClient struct {
	cache map[string]*PricingProfile
	mu    sync.RWMutex
}

func NewAzurePricingClient() *AzurePricingClient {
	return &AzurePricingClient{
		cache: make(map[string]*PricingProfile),
	}
}

// GetProfileForInstance fetches live pricing for an Azure VM SKU.
// Since the Azure Retail API doesn't return vCPU/RAM specs directly in a structured way,
// we use a heuristic based on the SKU name or return a profile that represents the VM cost.
func (c *AzurePricingClient) GetProfileForInstance(sku, region string) (*PricingProfile, error) {
	cacheKey := fmt.Sprintf("%s:%s", sku, region)

	c.mu.RLock()
	if profile, ok := c.cache[cacheKey]; ok {
		c.mu.RUnlock()
		return profile, nil
	}
	c.mu.RUnlock()

	// Filter for Virtual Machines, On-demand (not spot), and the specific SKU/Region
	filter := fmt.Sprintf("armRegionName eq '%s' and armSkuName eq '%s' and serviceName eq 'Virtual Machines' and priceType eq 'Consumption'", region, sku)
	endpoint := fmt.Sprintf("https://prices.azure.com/api/retail/prices?$filter=%s", url.QueryEscape(filter))

	resp, err := http.Get(endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("azure pricing API returned status: %d", resp.StatusCode)
	}

	var data AzurePricingResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	if len(data.Items) == 0 {
		return nil, fmt.Errorf("no azure pricing data found for %s in %s", sku, region)
	}

	// We pick the first item. Note: Azure API might return multiple meters for different OS types.
	// We'll prioritize Linux if possible, or just take the first.
	item := data.Items[0]

	// Because we don't have vCPU/RAM specs here, we'll use "Standard" estimation
	// for common Azure D-series (e.g. D2s v3 has 2 vCPU, 8GiB RAM).
	// For a fully robust solution, we'd need a SKU-to-Spec mapping table.
	// For now, we return a profile with the total hourly price and a generic unit.
	
	profile := &PricingProfile{
		Name:             fmt.Sprintf("Azure %s (%s)", sku, region),
		// We'll set these to 0 and handle the cost calculation differently if needed,
		// OR we can provide a decent fallback rate if we can't determine specs.
		CPURatePerHour:   item.RetailPrice * 0.5 / 2, // Assume 2 vCPU base
		MemoryRatePerHour: item.RetailPrice * 0.5 / 8, // Assume 8 GiB base
	}

	c.mu.Lock()
	c.cache[cacheKey] = profile
	c.mu.Unlock()

	return profile, nil
}

var DefaultAzureClient = NewAzurePricingClient()

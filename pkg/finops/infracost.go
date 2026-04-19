package finops

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

type InfracostGraphQLRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables"`
}

type InfracostResponse struct {
	Data struct {
		Products []struct {
			Prices []struct {
				USD  string `json:"USD"`
				Unit string `json:"unit"`
			} `json:"prices"`
			Attributes []struct {
				Key   string `json:"key"`
				Value string `json:"value"`
			} `json:"attributes"`
		} `json:"products"`
	} `json:"data"`
}

type InfracostClient struct {
	apiKey string
	cache  map[string]*PricingProfile
	mu     sync.RWMutex
}

func NewInfracostClient(apiKey string) *InfracostClient {
	return &InfracostClient{
		apiKey: apiKey,
		cache:  make(map[string]*PricingProfile),
	}
}

func (c *InfracostClient) GetProfileForInstance(vendor, region, instanceType string) (*PricingProfile, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("infracost API key not configured")
	}

	cacheKey := fmt.Sprintf("%s:%s:%s", vendor, region, instanceType)
	c.mu.RLock()
	if profile, ok := c.cache[cacheKey]; ok {
		c.mu.RUnlock()
		return profile, nil
	}
	c.mu.RUnlock()

	// Build GraphQL query
	query := `
	query($filter: ProductFilter!) {
		products(filter: $filter) {
			attributes { key value }
			prices(filter: { purchaseOption: "on_demand" }) {
				USD
				unit
			}
		}
	}`

	var attributeFilters []map[string]string
	var service string

	switch strings.ToLower(vendor) {
	case "aws":
		service = "AmazonEC2"
		attributeFilters = []map[string]string{
			{"key": "instanceType", "value": instanceType},
			{"key": "operatingSystem", "value": "Linux"},
			{"key": "tenancy", "value": "Shared"},
			{"key": "capacitystatus", "value": "Used"},
		}
	case "azure":
		service = "Virtual Machines"
		attributeFilters = []map[string]string{
			{"key": "armSkuName", "value": instanceType},
			{"key": "operatingSystem", "value": "Linux"},
		}
	case "gcp":
		service = "Compute Engine"
		attributeFilters = []map[string]string{
			{"key": "machineType", "value": instanceType},
			{"key": "usageType", "value": "OnDemand"},
		}
	default:
		return nil, fmt.Errorf("unsupported vendor: %s", vendor)
	}

	variables := map[string]interface{}{
		"filter": map[string]interface{}{
			"vendorName":       strings.ToLower(vendor),
			"service":          service,
			"region":           region,
			"attributeFilters": attributeFilters,
		},
	}

	reqBody, _ := json.Marshal(InfracostGraphQLRequest{
		Query:     query,
		Variables: variables,
	})

	httpClient := &http.Client{Timeout: 10 * time.Second}
	req, _ := http.NewRequest("POST", "https://pricing.api.infracost.io/graphql", bytes.NewBuffer(reqBody))
	req.Header.Set("X-Api-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("infracost API returned status: %d", resp.StatusCode)
	}

	var infraResp InfracostResponse
	if err := json.NewDecoder(resp.Body).Decode(&infraResp); err != nil {
		return nil, err
	}

	if len(infraResp.Data.Products) == 0 || len(infraResp.Data.Products[0].Prices) == 0 {
		return nil, fmt.Errorf("no pricing found for %s %s in %s", vendor, instanceType, region)
	}

	var hourlyPrice float64
	if _, err := fmt.Sscanf(infraResp.Data.Products[0].Prices[0].USD, "%f", &hourlyPrice); err != nil {
		slog.Error("Failed to parse hourly price from Infracost", "value", infraResp.Data.Products[0].Prices[0].USD, "error", err)
	}

	// Extract vCPU and Memory if available from attributes
	vcpus := 2.0 // Default heuristic
	memoryGiB := 8.0
	for _, attr := range infraResp.Data.Products[0].Attributes {
		if attr.Key == "vcpus" {
			fmt.Sscanf(attr.Value, "%f", &vcpus)
		}
		if attr.Key == "memoryGiB" {
			fmt.Sscanf(attr.Value, "%f", &memoryGiB)
		}
	}

	if hourlyPrice == 0 {
		return nil, fmt.Errorf("invalid hourly price from Infracost: %s", infraResp.Data.Products[0].Prices[0].USD)
	}

	profile := &PricingProfile{
		Name:              fmt.Sprintf("Infracost %s %s (%s)", vendor, instanceType, region),
		CPURatePerHour:    (hourlyPrice * 0.5) / vcpus,
		MemoryRatePerHour: (hourlyPrice * 0.5) / memoryGiB,
	}

	c.mu.Lock()
	c.cache[cacheKey] = profile
	c.mu.Unlock()

	return profile, nil
}

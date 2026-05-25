package finops

import (
	"fmt"
	"math"
)

// ServiceTier represents the business importance of a service.
type ServiceTier string

const (
	TierCritical ServiceTier = "Critical" // e.g. Payment, Auth
	TierHigh     ServiceTier = "High"     // e.g. API Gateway, Core Services
	TierMedium   ServiceTier = "Medium"   // e.g. Search, Analytics
	TierLow      ServiceTier = "Low"      // e.g. Documentation, Dev Tools
)

// PricingProfile holds the hourly rates for compute resources.
type PricingProfile struct {
	Name              string
	CPURatePerHour    float64 // Cost per vCPU hour
	MemoryRatePerHour float64 // Cost per GiB hour
	VCPUs             float64 // Instance vCPU capacity when known
	MemoryGiB         float64 // Instance memory capacity when known
	HourlyRateUSD     float64 // Full instance hourly price when known
}

// RiskMetrics represents the data needed to calculate smart financial impact.
type RiskMetrics struct {
	Tier              ServiceTier
	Replicas          int
	RequestsPerSecond float64
	ErrorRate         float64
	SLAThreshold      float64 // e.g. 99.9% (0.999)
	AvgRevenuePerReq  float64 // Dollar value of a successful request
}

// Default AWS On-Demand Profile (US-East-1 average)
var AWSDefaultProfile = PricingProfile{
	Name:              "AWS (us-east-1)",
	CPURatePerHour:    0.0405, // e.g. m5.large average
	MemoryRatePerHour: 0.00506,
}

// CalculateMonthlyCost estimates the cost of resources per month (730 hours).
func CalculateMonthlyCost(cpu float64, memBytes float64, profile PricingProfile, replicas int) float64 {
	memGiB := memBytes / (1024 * 1024 * 1024)
	hourlyCost := (cpu * profile.CPURatePerHour) + (memGiB * profile.MemoryRatePerHour)
	return hourlyCost * 730 * float64(replicas)
}

func CalculateMonthlyNodeCost(profile PricingProfile) float64 {
	if profile.HourlyRateUSD > 0 {
		return profile.HourlyRateUSD * 730
	}
	vcpus := profile.VCPUs
	if vcpus <= 0 {
		vcpus = 2
	}
	memoryGiB := profile.MemoryGiB
	if memoryGiB <= 0 {
		memoryGiB = 8
	}
	return ((profile.CPURatePerHour * vcpus) + (profile.MemoryRatePerHour * memoryGiB)) * 730
}

// FormatImpact returns a human-readable FinOps impact string.
func FormatImpact(oldCost, newCost float64, currency string) string {
	diff := newCost - oldCost
	if math.Abs(diff) < 0.01 {
		return "No significant cost change"
	}
	if diff > 0 {
		return fmt.Sprintf("+%s%.2f/mo", currency, diff)
	}
	return fmt.Sprintf("-%s%.2f/mo (Saving)", currency, math.Abs(diff))
}

// CalculateSmartImpact combines infrastructure costs with business risk valuation.
func CalculateSmartImpact(oldResources, newResources struct{ CPU, Mem float64 }, profile PricingProfile, metrics RiskMetrics) (impact string, details string) {
	oldMonthly := CalculateMonthlyCost(oldResources.CPU, oldResources.Mem, profile, metrics.Replicas)
	newMonthly := CalculateMonthlyCost(newResources.CPU, newResources.Mem, profile, metrics.Replicas)
	infraChange := newMonthly - oldMonthly

	// 1. Value of Lost Revenue (Direct)
	// lostRevenue = (Requests/sec * 3600 sec/hr) * errorRate * RevenueValue
	hourlyRequests := metrics.RequestsPerSecond * 3600
	directLossPerHour := hourlyRequests * metrics.ErrorRate * metrics.AvgRevenuePerReq

	// 2. Business Risk Multiplier (Intangible cost like Brand Reputation/Customer Support Load)
	var riskMultiplier float64
	switch metrics.Tier {
	case TierCritical:
		riskMultiplier = 5000.0 // Critical outages are extremely expensive
	case TierHigh:
		riskMultiplier = 1000.0
	case TierMedium:
		riskMultiplier = 200.0
	case TierLow:
		riskMultiplier = 50.0
	default:
		riskMultiplier = 100.0
	}

	// 3. Productivity/Engineering Loss
	// Assume an outage takes 2 engineers 2 hours to fix on average
	productivityLoss := 200.0 // Abstract flat cost of context switching/debugging

	totalRiskValue := (directLossPerHour * 24) + riskMultiplier + productivityLoss

	action := "preventing"
	if infraChange < 0 {
		action = "while maintaining"
	}

	impact = fmt.Sprintf("%s %s %s compute cost vs. %s a $%s risk profile",
		metrics.Tier,
		profile.Name,
		FormatImpact(oldMonthly, newMonthly, "$"),
		action,
		formatLargeMoney(totalRiskValue),
	)

	details = fmt.Sprintf("FinOps Deep Analysis:\n"+
		"- Deployment Scale: %d replicas\n"+
		"- Business Priority: %s Tier\n"+
		"- Infra Impact: %s\n"+
		"- Direct Revenue Loss: $%.2f/hr\n"+
		"- Total Risk Valuation: $%s (Revenue + Brand + Engineering Time)",
		metrics.Replicas,
		metrics.Tier,
		FormatImpact(oldMonthly, newMonthly, "$"),
		directLossPerHour,
		formatLargeMoney(totalRiskValue),
	)

	return impact, details
}

func formatLargeMoney(val float64) string {
	if val >= 1000000 {
		return fmt.Sprintf("%.1fM", val/1000000)
	}
	if val >= 1000 {
		return fmt.Sprintf("%.1fK", val/1000)
	}
	return fmt.Sprintf("%.2f", val)
}

// CalculateCoD (Legacy support)
func CalculateCoD(errorRate float64, requestsPerHour float64, revenuePerRequest float64, latency float64, threshold float64, penalty float64) string {
	lostRevenue := errorRate * requestsPerHour * revenuePerRequest
	latencyLoss := 0.0
	if latency > threshold {
		latencyLoss = penalty
	}
	totalLoss := lostRevenue + latencyLoss
	if totalLoss > 0 {
		return fmt.Sprintf("-$%.2f/hr (Projected Loss)", totalLoss)
	}
	return "No immediate financial loss"
}

// NodeInfo represents the basic information needed to calculate a node's cost.
type NodeInfo struct {
	Name       string
	Labels     map[string]string
	ProviderID string
}

// CalculateClusterCost estimates the total monthly cost of the cluster based on active nodes.
func CalculateClusterCost(nodes []NodeInfo, provider PricingProvider) (float64, error) {
	var totalMonthlyCost float64

	for _, node := range nodes {
		vendor, region, instanceType := ParseNodeMetadata(node.Labels, node.ProviderID)
		if vendor == "" || region == "" || instanceType == "" {
			continue // Skip nodes where we can't determine pricing info
		}

		profile, err := provider.GetProfileForInstance(vendor, region, instanceType)
		if err != nil || profile == nil {
			continue // Skip if pricing isn't available
		}

		totalMonthlyCost += CalculateMonthlyNodeCost(*profile)
	}

	return totalMonthlyCost, nil
}

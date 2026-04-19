package finops

import (
	"fmt"
)

// PricingProfile holds the hourly rates for compute resources.
type PricingProfile struct {
	Name             string
	CPURatePerHour   float64 // Cost per vCPU hour
	MemoryRatePerHour float64 // Cost per GiB hour
}

// Default AWS On-Demand Profile (US-East-1 average)
var AWSDefaultProfile = PricingProfile{
	Name:             "AWS (us-east-1)",
	CPURatePerHour:   0.0405, // e.g. m5.large average
	MemoryRatePerHour: 0.00506,
}

// CalculateMonthlyCost estimates the cost of a pod per month (730 hours).
func CalculateMonthlyCost(cpuRequests float64, memRequestsBytes float64, profile PricingProfile) float64 {
	memGiB := memRequestsBytes / (1024 * 1024 * 1024)
	hourlyCost := (cpuRequests * profile.CPURatePerHour) + (memGiB * profile.MemoryRatePerHour)
	return hourlyCost * 730 // Average hours in a month
}

// FormatImpact returns a human-readable FinOps impact string.
func FormatImpact(oldCost, newCost float64, currency string) string {
	diff := newCost - oldCost
	if diff > 0 {
		return fmt.Sprintf("+%s%.2f/mo", currency, diff)
	} else if diff < 0 {
		return fmt.Sprintf("-%s%.2f/mo", currency, -diff)
	}
	return "No cost change"
}

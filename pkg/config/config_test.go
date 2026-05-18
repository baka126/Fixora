package config

import (
	"testing"
	"time"
)

func TestNormalizeMode(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want OperatingMode
	}{
		{name: "auto fix", in: "auto-fix", want: AutoFix},
		{name: "click to fix", in: "click-to-fix", want: ClickToFix},
		{name: "dry run", in: "dry-run", want: DryRun},
		{name: "empty defaults to dry run", in: "", want: DryRun},
		{name: "invalid defaults to dry run", in: "experimental", want: DryRun},
		{name: "trimmed dry run", in: " dry-run ", want: DryRun},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeMode(tt.in); got != tt.want {
				t.Fatalf("normalizeMode(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestLoadInvestigationCooldowns(t *testing.T) {
	t.Setenv("INVESTIGATION_COOLDOWN", "6h")
	t.Setenv("ALERTMANAGER_DEDUP_WINDOW", "90m")

	cfg := Load()

	if cfg.InvestigationCooldown != 6*time.Hour {
		t.Fatalf("InvestigationCooldown = %s, want 6h", cfg.InvestigationCooldown)
	}
	if cfg.AlertmanagerDedupWindow != 90*time.Minute {
		t.Fatalf("AlertmanagerDedupWindow = %s, want 90m", cfg.AlertmanagerDedupWindow)
	}
}

func TestLoadInvestigationCooldownDefaults(t *testing.T) {
	t.Setenv("INVESTIGATION_COOLDOWN", "")
	t.Setenv("ALERTMANAGER_DEDUP_WINDOW", "")

	cfg := Load()

	if cfg.InvestigationCooldown != 12*time.Hour {
		t.Fatalf("InvestigationCooldown = %s, want 12h", cfg.InvestigationCooldown)
	}
	if cfg.AlertmanagerDedupWindow != 12*time.Hour {
		t.Fatalf("AlertmanagerDedupWindow = %s, want 12h", cfg.AlertmanagerDedupWindow)
	}
}

package config

import "testing"

func TestNormalizeMode(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want OperatingMode
	}{
		{name: "auto fix", in: "auto-fix", want: AutoFix},
		{name: "click to fix", in: "click-to-fix", want: ClickToFix},
		{name: "dry run", in: "dry-run", want: DryRun},
		{name: "empty defaults to auto", in: "", want: AutoFix},
		{name: "invalid defaults to auto", in: "experimental", want: AutoFix},
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

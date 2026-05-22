package security

import (
	"regexp"
	"strings"
	"testing"
)

func TestMaskingContextUsesStablePlaceholders(t *testing.T) {
	ctx := NewMaskingContext()
	input := "owner baka@example.com retried from baka@example.com with token: abc-123-def-456"

	masked := ctx.Mask(input)

	if strings.Contains(masked, "baka@example.com") || strings.Contains(masked, "abc-123-def-456") {
		t.Fatalf("masked output still contains sensitive values: %s", masked)
	}
	if got := strings.Count(masked, "[FIXORA_EMAIL_1]"); got != 2 {
		t.Fatalf("email placeholder count = %d, want 2 in %s", got, masked)
	}
	if !strings.Contains(masked, "[FIXORA_TOKEN_1]") {
		t.Fatalf("token placeholder missing from %s", masked)
	}
	if len(ctx.BlockedFindings()) == 0 {
		t.Fatalf("expected blocked finding for token-like value")
	}
}

func TestMaskingContextAppliesCustomRules(t *testing.T) {
	ctx := NewMaskingContext(regexp.MustCompile(`CORP-\d{3}`))

	masked := ctx.Mask("customer CORP-123 hit CORP-123 again")

	if strings.Contains(masked, "CORP-123") {
		t.Fatalf("custom value was not masked: %s", masked)
	}
	if got := strings.Count(masked, "[FIXORA_CUSTOM_1]"); got != 2 {
		t.Fatalf("custom placeholder count = %d, want 2 in %s", got, masked)
	}
}

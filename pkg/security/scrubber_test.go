package security

import (
	"regexp"
	"testing"
)

func TestScrubPII(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expected       string
		customPatterns []*regexp.Regexp
	}{
		{
			name:     "Scrub Email",
			input:    "Error from user baka@example.com: connection failed",
			expected: "Error from user [EMAIL]: connection failed",
		},
		{
			name:     "Scrub IPv4",
			input:    "Failed to connect to 192.168.1.100:8080",
			expected: "Failed to connect to [IP]:8080",
		},
		{
			name:     "Scrub Bearer Token",
			input:    "Authorization: Bearer secret-token-12345",
			expected: "Authorization: Bearer [REDACTED]",
		},
		{
			name:     "Scrub Password",
			input:    "login failed for user admin with password=my-super-secret-pass",
			expected: "login failed for user admin with password [REDACTED]",
		},
		{
			name:     "Mixed Content",
			input:    "User test@dev.local on 10.0.0.1 failed with token: abc-123-def-456",
			expected: "User [EMAIL] on [IP] failed with token [REDACTED]",
		},
		{
			name:           "Custom SSN Scrub",
			input:          "Customer SSN: 123-45-6789 failed validation",
			expected:       "Customer [CUSTOM_REDACTED] failed validation",
			customPatterns: []*regexp.Regexp{regexp.MustCompile(`(?i)SSN:\s*\d{3}-\d{2}-\d{4}`)},
		},
		{
			name:           "Multiple Custom Patterns",
			input:          "Proprietary ID CORP-99X found with hash X99AAB",
			expected:       "Proprietary ID [CUSTOM_REDACTED] found with hash [CUSTOM_REDACTED]",
			customPatterns: []*regexp.Regexp{regexp.MustCompile(`CORP-\d{2}[A-Z]`), regexp.MustCompile(`X99[A-Z]{3}`)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := ScrubPII(tt.input, tt.customPatterns...)
			if actual != tt.expected {
				t.Errorf("ScrubPII() = %q, want %q", actual, tt.expected)
			}
		})
	}
}

package security

import (
	"testing"
)

func TestScrubPII(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := ScrubPII(tt.input)
			if actual != tt.expected {
				t.Errorf("ScrubPII() = %q, want %q", actual, tt.expected)
			}
		})
	}
}

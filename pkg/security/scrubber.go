package security

import (
	"regexp"
)

var (
	emailRegex = regexp.MustCompile(`(?i)[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}`)
	ipv4Regex  = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
	// Simplified token regex for common patterns like Bearer, jwt, etc.
	tokenRegex = regexp.MustCompile(`(?i)(bearer|token|auth|key|secret|password)[\s:=]+[a-z0-9._\-]{10,}`)
)

// ScrubPII removes sensitive information from logs before sending them to external APIs.
func ScrubPII(input string) string {
	scrubbed := input
	scrubbed = emailRegex.ReplaceAllString(scrubbed, "[EMAIL]")
	scrubbed = ipv4Regex.ReplaceAllString(scrubbed, "[IP]")
	scrubbed = tokenRegex.ReplaceAllString(scrubbed, "$1 [REDACTED]")
	return scrubbed
}

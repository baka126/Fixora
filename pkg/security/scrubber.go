package security

import (
	"regexp"
)

var (
	emailRegex = regexp.MustCompile(`(?i)[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}`)
	// IPv4 and IPv6 regexes
	ipv4Regex = regexp.MustCompile(`\b(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\b`)
	ipv6Regex = regexp.MustCompile(`(?i)\b(?:[a-f0-9]{1,4}:){7}[a-f0-9]{1,4}\b|\b(?:[a-f0-9]{1,4}:){1,7}:[a-f0-9]{1,4}\b|\b:[a-f0-9]{1,4}(?::[a-f0-9]{1,4}){1,7}\b`)
	// Robust token regex for common patterns
	tokenRegex = regexp.MustCompile(`(?i)(bearer|token|auth|key|secret|password|passwd|pwd)[\s:=]+["']?[a-z0-9._\-]{10,}`)
	jwtRegex   = regexp.MustCompile(`\b[A-Za-z0-9-_]+\.[A-Za-z0-9-_]+\.[A-Za-z0-9-_]+\b`)
)

// ScrubPII removes sensitive information from logs before sending them to external APIs.
func ScrubPII(input string) string {
	scrubbed := input
	scrubbed = emailRegex.ReplaceAllString(scrubbed, "[EMAIL]")
	scrubbed = ipv4Regex.ReplaceAllString(scrubbed, "[IP]")
	scrubbed = ipv6Regex.ReplaceAllString(scrubbed, "[IP]")
	scrubbed = jwtRegex.ReplaceAllString(scrubbed, "[JWT]")
	// Redact the value part of the token match, ensuring a space before [REDACTED] to match test expectations
	scrubbed = tokenRegex.ReplaceAllString(scrubbed, "$1 [REDACTED]")
	return scrubbed
}

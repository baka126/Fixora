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
	
	// Cloud Provider & generic secrets
	awsAccessKeyRegex = regexp.MustCompile(`\b(AKIA|A3T|AGPA|AIDA|AROA|AIPA|ANPA|ANVA|ASIA)[A-Z0-9]{16}\b`)
	awsSecretKeyRegex = regexp.MustCompile(`(?i)(aws_secret_access_key|aws_session_token)[\s:=]+["']?[A-Za-z0-9/+=]{40,}["']?`)
	privateKeyRegex   = regexp.MustCompile(`(?s)-----BEGIN.*?PRIVATE KEY-----.*?-----END.*?PRIVATE KEY-----`)
	
	// Generic base64-like data often found in k8s secrets
	k8sSecretDataRegex = regexp.MustCompile(`(?im)^(\s*)([a-zA-Z0-9_.-]+):\s+([A-Za-z0-9+/]{16,}={0,2})$`)
)

// ScrubPII removes sensitive information from logs before sending them to external APIs.
func ScrubPII(input string) string {
	scrubbed := input
	scrubbed = emailRegex.ReplaceAllString(scrubbed, "[EMAIL]")
	scrubbed = ipv4Regex.ReplaceAllString(scrubbed, "[IP]")
	scrubbed = ipv6Regex.ReplaceAllString(scrubbed, "[IP]")
	scrubbed = jwtRegex.ReplaceAllString(scrubbed, "[JWT]")
	
	scrubbed = privateKeyRegex.ReplaceAllString(scrubbed, "-----BEGIN PRIVATE KEY-----\n[REDACTED]\n-----END PRIVATE KEY-----")
	scrubbed = awsAccessKeyRegex.ReplaceAllString(scrubbed, "[AWS_ACCESS_KEY]")
	scrubbed = awsSecretKeyRegex.ReplaceAllString(scrubbed, "$1 [REDACTED]")
	
	// Redact generic yaml keys that look like base64 secret data
	scrubbed = k8sSecretDataRegex.ReplaceAllString(scrubbed, "$1$2: [BASE64_REDACTED]")
	
	// Redact the value part of the token match, ensuring a space before [REDACTED] to match test expectations
	scrubbed = tokenRegex.ReplaceAllString(scrubbed, "$1 [REDACTED]")
	
	return scrubbed
}

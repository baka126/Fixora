package security

import (
	"crypto/rand"
	"fmt"
	"regexp"
	"strings"
)

var (
	// yamlEnvValueRegex captures YAML environment variable blocks. e.g. "  value: secret_value"
	yamlEnvValueRegex = regexp.MustCompile(`(?im)^(\s*value:\s*)(.+)$`)

	// genericTokenValueRegex captures prefix ($1) and value ($2) for structural secrets
	genericTokenValueRegex = regexp.MustCompile(`(?i)([a-zA-Z0-9_]*(?:bearer|token|auth|key|secret|password|passwd|pwd)[a-zA-Z0-9_]*[\s:=]+)(["']?[a-zA-Z0-9._\-+=/]{10,}["']?)`)
)

// Tokenizer statefully replaces sensitive strings with symmetric tokens.
// This allows Fixora to send scrubbed config files to AI models and then
// safely map the tokens back to their original values when the AI returns a patch.
type Tokenizer struct {
	mapping map[string]string
}

func NewTokenizer() *Tokenizer {
	return &Tokenizer{
		mapping: make(map[string]string),
	}
}

func (t *Tokenizer) generateToken() string {
	b := make([]byte, 8)
	rand.Read(b)
	return fmt.Sprintf("TOKEN_%x", b)
}

func (t *Tokenizer) tokenizeMatch(match string) string {
	tok := t.generateToken()
	t.mapping[tok] = match
	return tok
}

// Tokenize processes an input string, replacing recognized secrets, PII, and
// likely-sensitive structural YAML fields with symmetric UUID tokens.
func (t *Tokenizer) Tokenize(input string) string {
	tokenized := input

	// 1. YAML Specific fields (env values)
	tokenized = yamlEnvValueRegex.ReplaceAllStringFunc(tokenized, func(match string) string {
		parts := yamlEnvValueRegex.FindStringSubmatch(match)
		if len(parts) == 3 {
			prefix := parts[1]
			val := strings.TrimSpace(parts[2])
			lowerVal := strings.ToLower(val)
			// Skip tokenizing non-sensitive values like booleans to avoid confusing AI structural logic
			if len(val) > 4 && !strings.Contains(lowerVal, "true") && !strings.Contains(lowerVal, "false") && !strings.Contains(val, "[") && !strings.Contains(val, "{") {
				tok := t.generateToken()
				t.mapping[tok] = val
				return prefix + tok
			}
		}
		return match
	})

	// 2. K8s Secret data block heuristics
	tokenized = k8sSecretDataRegex.ReplaceAllStringFunc(tokenized, func(match string) string {
		parts := k8sSecretDataRegex.FindStringSubmatch(match)
		if len(parts) == 4 {
			prefix := parts[1] + parts[2] + ": "
			val := strings.TrimSpace(parts[3])
			tok := t.generateToken()
			t.mapping[tok] = val
			return prefix + tok
		}
		return match
	})

	// 3. Generic Secrets & Tokens (AWS, generic passwords, etc)
	tokenized = genericTokenValueRegex.ReplaceAllStringFunc(tokenized, func(match string) string {
		parts := genericTokenValueRegex.FindStringSubmatch(match)
		if len(parts) == 3 {
			prefix := parts[1]
			val := parts[2]
			tok := t.generateToken()
			t.mapping[tok] = val
			return prefix + tok
		}
		return match
	})

	// 4. Fallback to full match tokenization for explicit PII/Keys
	tokenized = emailRegex.ReplaceAllStringFunc(tokenized, t.tokenizeMatch)
	tokenized = ipv4Regex.ReplaceAllStringFunc(tokenized, t.tokenizeMatch)
	tokenized = ipv6Regex.ReplaceAllStringFunc(tokenized, t.tokenizeMatch)
	tokenized = jwtRegex.ReplaceAllStringFunc(tokenized, t.tokenizeMatch)
	tokenized = awsAccessKeyRegex.ReplaceAllStringFunc(tokenized, t.tokenizeMatch)
	tokenized = privateKeyRegex.ReplaceAllStringFunc(tokenized, t.tokenizeMatch)

	return tokenized
}

// Detokenize swaps all generated tokens back to their original raw values.
func (t *Tokenizer) Detokenize(input string) string {
	result := input
	for token, orig := range t.mapping {
		result = strings.ReplaceAll(result, token, orig)
	}
	return result
}

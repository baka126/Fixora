package security

import (
	"crypto/rand"
	"fmt"
	"regexp"
	"strings"
	"time"
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
	mapping        map[string]string
	reverseMapping map[string]string
}

func NewTokenizer() *Tokenizer {
	return &Tokenizer{
		mapping:        make(map[string]string),
		reverseMapping: make(map[string]string),
	}
}

func (t *Tokenizer) generateToken(hint string) string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("FIXORA_%s_%d", hint, time.Now().UnixNano())
	}
	return fmt.Sprintf("FIXORA_%s_%x", hint, b)
}

func (t *Tokenizer) tokenizeMatch(match string, hint string) string {
	if tok, exists := t.reverseMapping[match]; exists {
		return tok
	}
	tok := t.generateToken(hint)
	t.mapping[tok] = match
	t.reverseMapping[match] = tok
	return tok
}

func isGeneratedToken(value string) bool {
	return strings.HasPrefix(strings.Trim(value, `"'`), "FIXORA_")
}

// Tokenize processes an input string, replacing recognized secrets, PII, and
// likely-sensitive structural YAML fields with symmetric tokens.
func (t *Tokenizer) Tokenize(input string) string {
	tokenized := input

	// 1. Provider-specific secrets before broad YAML/generic key matching.
	tokenized = awsAccessKeyRegex.ReplaceAllStringFunc(tokenized, func(m string) string { return t.tokenizeMatch(m, "AWS") })
	tokenized = awsSecretKeyRegex.ReplaceAllStringFunc(tokenized, func(m string) string { return t.tokenizeMatch(m, "AWS_SECRET") })
	tokenized = privateKeyRegex.ReplaceAllStringFunc(tokenized, func(m string) string { return t.tokenizeMatch(m, "PRIVATE_KEY") })

	// 2. YAML Specific fields (env values)
	tokenized = yamlEnvValueRegex.ReplaceAllStringFunc(tokenized, func(match string) string {
		parts := yamlEnvValueRegex.FindStringSubmatch(match)
		if len(parts) == 3 {
			prefix := parts[1]
			val := strings.TrimSpace(parts[2])
			lowerVal := strings.ToLower(val)
			// Skip tokenizing non-sensitive/structural values to preserve AI context
			if len(val) > 4 && !isGeneratedToken(val) && !strings.Contains(lowerVal, "true") && !strings.Contains(lowerVal, "false") && !strings.Contains(val, "[") && !strings.Contains(val, "{") {
				tok := t.tokenizeMatch(val, "VAL")
				return prefix + tok
			}
		}
		return match
	})

	// 3. K8s Secret data block heuristics
	tokenized = k8sSecretDataRegex.ReplaceAllStringFunc(tokenized, func(match string) string {
		parts := k8sSecretDataRegex.FindStringSubmatch(match)
		if len(parts) == 4 {
			prefix := parts[1] + parts[2] + ": "
			val := strings.TrimSpace(parts[3])
			if isGeneratedToken(val) {
				return match
			}
			tok := t.tokenizeMatch(val, "SEC")
			return prefix + tok
		}
		return match
	})

	// 4. Generic Secrets & Tokens
	tokenized = genericTokenValueRegex.ReplaceAllStringFunc(tokenized, func(match string) string {
		parts := genericTokenValueRegex.FindStringSubmatch(match)
		if len(parts) == 3 {
			prefix := parts[1]
			val := parts[2]
			if isGeneratedToken(val) {
				return match
			}
			tok := t.tokenizeMatch(val, "KEY")
			return prefix + tok
		}
		return match
	})

	// 5. PII/Network Tokenization
	tokenized = emailRegex.ReplaceAllStringFunc(tokenized, func(m string) string { return t.tokenizeMatch(m, "EMAIL") })
	tokenized = ipv4Regex.ReplaceAllStringFunc(tokenized, func(m string) string { return t.tokenizeMatch(m, "IP") })
	tokenized = ipv6Regex.ReplaceAllStringFunc(tokenized, func(m string) string { return t.tokenizeMatch(m, "IP") })
	tokenized = jwtRegex.ReplaceAllStringFunc(tokenized, func(m string) string { return t.tokenizeMatch(m, "JWT") })

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

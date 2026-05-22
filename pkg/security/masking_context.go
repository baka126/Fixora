package security

import (
	"fmt"
	"regexp"
	"strings"
)

type MaskRule struct {
	Name        string
	Pattern     *regexp.Regexp
	Replacement string
	Block       bool
}

type MaskingContext struct {
	rules          []MaskRule
	mapping        map[string]string
	reverseMapping map[string]string
	counters       map[string]int
	blocked        []string
}

func NewMaskingContext(customPatterns ...*regexp.Regexp) *MaskingContext {
	rules := []MaskRule{
		{Name: "private_key", Pattern: privateKeyRegex, Replacement: "PRIVATE_KEY", Block: true},
		{Name: "aws_access_key", Pattern: awsAccessKeyRegex, Replacement: "AWS_ACCESS_KEY", Block: true},
		{Name: "aws_secret_key", Pattern: awsSecretKeyRegex, Replacement: "AWS_SECRET", Block: true},
		{Name: "jwt", Pattern: jwtRegex, Replacement: "JWT", Block: true},
		{Name: "secret_data", Pattern: k8sSecretDataRegex, Replacement: "SECRET", Block: true},
		{Name: "token", Pattern: tokenRegex, Replacement: "TOKEN", Block: true},
		{Name: "email", Pattern: emailRegex, Replacement: "EMAIL"},
		{Name: "ipv4", Pattern: ipv4Regex, Replacement: "IP"},
		{Name: "ipv6", Pattern: ipv6Regex, Replacement: "IP"},
	}
	for i, pattern := range customPatterns {
		if pattern == nil {
			continue
		}
		rules = append(rules, MaskRule{
			Name:        fmt.Sprintf("custom_%d", i+1),
			Pattern:     pattern,
			Replacement: "CUSTOM",
		})
	}
	return NewMaskingContextWithRules(rules...)
}

func NewMaskingContextWithRules(rules ...MaskRule) *MaskingContext {
	return &MaskingContext{
		rules:          append([]MaskRule(nil), rules...),
		mapping:        make(map[string]string),
		reverseMapping: make(map[string]string),
		counters:       make(map[string]int),
	}
}

func (m *MaskingContext) Mask(input string) string {
	if m == nil || input == "" {
		return input
	}
	masked := input
	for _, rule := range m.rules {
		if rule.Pattern == nil {
			continue
		}
		label := normalizeMaskLabel(firstNonEmptyMask(rule.Replacement, rule.Name))
		masked = rule.Pattern.ReplaceAllStringFunc(masked, func(match string) string {
			placeholder := m.placeholder(label, match)
			if rule.Block {
				m.recordBlocked(rule.Name, placeholder)
			}
			return placeholder
		})
	}
	return masked
}

func (m *MaskingContext) BlockedFindings() []string {
	if m == nil || len(m.blocked) == 0 {
		return nil
	}
	out := append([]string(nil), m.blocked...)
	return out
}

func (m *MaskingContext) placeholder(label, value string) string {
	if existing, ok := m.reverseMapping[value]; ok {
		return existing
	}
	m.counters[label]++
	placeholder := fmt.Sprintf("[FIXORA_%s_%d]", label, m.counters[label])
	m.mapping[placeholder] = value
	m.reverseMapping[value] = placeholder
	return placeholder
}

func (m *MaskingContext) recordBlocked(ruleName, placeholder string) {
	finding := normalizeMaskLabel(ruleName) + ":" + placeholder
	for _, existing := range m.blocked {
		if existing == finding {
			return
		}
	}
	m.blocked = append(m.blocked, finding)
}

func normalizeMaskLabel(label string) string {
	label = strings.ToUpper(strings.TrimSpace(label))
	if label == "" {
		return "VALUE"
	}
	var b strings.Builder
	for _, r := range label {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('_')
	}
	return strings.Trim(b.String(), "_")
}

func firstNonEmptyMask(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

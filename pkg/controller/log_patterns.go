package controller

import (
	"regexp"
	"sort"
	"strings"
)

type LogPattern struct {
	Pattern string
	Count   int
	Example string
}

var logPatternTokenRE = regexp.MustCompile(`(?i)([0-9]{4}-[0-9]{2}-[0-9]{2}T[^\s]+|[[:xdigit:]]{8}-[[:xdigit:]]{4}-[[:xdigit:]]{4}-[[:xdigit:]]{4}-[[:xdigit:]]{12}|0x[0-9a-f]+|[a-f0-9]{12,}|[0-9]+(?:\.[0-9]+)?)`)

func ClusterLogPatterns(logs string, limit int) []LogPattern {
	if limit <= 0 {
		limit = 5
	}
	type bucket struct {
		count   int
		example string
	}
	buckets := map[string]bucket{}
	for _, raw := range strings.Split(logs, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		pattern := normalizeLogPattern(line)
		if pattern == "" {
			continue
		}
		b := buckets[pattern]
		b.count++
		if b.example == "" {
			b.example = line
		}
		buckets[pattern] = b
	}
	out := make([]LogPattern, 0, len(buckets))
	for pattern, b := range buckets {
		out = append(out, LogPattern{Pattern: pattern, Count: b.count, Example: b.example})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count == out[j].Count {
			return out[i].Pattern < out[j].Pattern
		}
		return out[i].Count > out[j].Count
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func normalizeLogPattern(line string) string {
	line = strings.TrimSpace(line)
	line = logPatternTokenRE.ReplaceAllString(line, "<var>")
	fields := strings.Fields(line)
	if len(fields) > 32 {
		fields = fields[:32]
	}
	return strings.Join(fields, " ")
}

func formatLogPatterns(patterns []LogPattern) string {
	if len(patterns) == 0 {
		return ""
	}
	lines := make([]string, 0, len(patterns))
	for _, p := range patterns {
		lines = append(lines, "- "+p.Pattern+" ("+itoa(p.Count)+" occurrences)")
	}
	return "Log Patterns:\n" + strings.Join(lines, "\n")
}

func itoa(v int) string {
	const digits = "0123456789"
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = digits[v%10]
		v /= 10
	}
	return string(buf[i:])
}

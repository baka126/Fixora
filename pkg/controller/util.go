package controller

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"

	v1 "k8s.io/api/core/v1"
)

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return "Unknown"
}

func containsAny(s string, needles ...string) bool {
	s = strings.ToLower(s)
	for _, needle := range needles {
		if strings.Contains(s, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func selectorMatches(selector, labels map[string]string) bool {
	if len(selector) == 0 {
		return false
	}
	for key, value := range selector {
		if labels[key] != value {
			return false
		}
	}
	return true
}

func endpointsIncludePod(endpoints *v1.Endpoints, podName string) bool {
	if endpoints == nil {
		return false
	}
	for _, subset := range endpoints.Subsets {
		for _, address := range subset.Addresses {
			if address.TargetRef != nil && address.TargetRef.Kind == "Pod" && address.TargetRef.Name == podName {
				return true
			}
		}
	}
	return false
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func CalculateEvidenceHash(logs, events, metrics string) string {
	h := sha256.New()
	h.Write([]byte(logs))
	h.Write([]byte(events))
	h.Write([]byte(metrics))
	return hex.EncodeToString(h.Sum(nil))
}

package ai

import (
	"bufio"
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	// Matches common log timestamps to ignore during similarity checks
	timestampRegex = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}[T\s]\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})?\s*`)
)

// CompressLogs takes raw log lines and deduplicates highly similar consecutive lines.
func CompressLogs(logs string) string {
	if logs == "" {
		return ""
	}

	scanner := bufio.NewScanner(strings.NewReader(logs))
	var compressed []string
	var lastLineNormalized string
	var repeatCount int
	var lastOriginalLine string

	for scanner.Scan() {
		line := scanner.Text()
		
		// Strip timestamp for comparison
		normalized := timestampRegex.ReplaceAllString(line, "")
		normalized = strings.TrimSpace(normalized)

		if normalized == lastLineNormalized && normalized != "" {
			repeatCount++
		} else {
			if repeatCount > 0 {
				compressed = append(compressed, fmt.Sprintf("... [Repeated %d times] ...", repeatCount))
			}
			if lastOriginalLine != "" {
				compressed = append(compressed, lastOriginalLine)
			}
			lastLineNormalized = normalized
			lastOriginalLine = line
			repeatCount = 0
		}
	}

	if repeatCount > 0 {
		compressed = append(compressed, fmt.Sprintf("... [Repeated %d times] ...", repeatCount))
	}
	if lastOriginalLine != "" {
		compressed = append(compressed, lastOriginalLine)
	}

	return strings.Join(compressed, "\n")
}

// CompressYAML aggressively trims a Kubernetes YAML document for AI consumption.
// It removes status, managedFields, and empty values to save context tokens.
func CompressYAML(yamlStr string) string {
	var parsed map[string]interface{}
	if err := yaml.Unmarshal([]byte(yamlStr), &parsed); err != nil {
		// Fallback to original if not valid YAML
		return yamlStr
	}

	cleanYAML(parsed)

	bytes, err := yaml.Marshal(parsed)
	if err != nil {
		return yamlStr
	}

	return string(bytes)
}

func cleanYAML(m map[string]interface{}) {
	// Remove common noisy top-level fields
	delete(m, "status")

	if meta, ok := m["metadata"].(map[string]interface{}); ok {
		delete(meta, "managedFields")
		delete(meta, "creationTimestamp")
		delete(meta, "resourceVersion")
		delete(meta, "uid")
		delete(meta, "generation")
		
		// Remove empty maps
		if ann, ok := meta["annotations"].(map[string]interface{}); ok && len(ann) == 0 {
			delete(meta, "annotations")
		}
		if lbl, ok := meta["labels"].(map[string]interface{}); ok && len(lbl) == 0 {
			delete(meta, "labels")
		}
	}

	// Recursively clean
	for k, v := range m {
		switch typed := v.(type) {
		case map[string]interface{}:
			cleanYAML(typed)
			if len(typed) == 0 {
				delete(m, k)
			}
		case []interface{}:
			for _, item := range typed {
				if itemMap, ok := item.(map[string]interface{}); ok {
					cleanYAML(itemMap)
				}
			}
		}
	}
}

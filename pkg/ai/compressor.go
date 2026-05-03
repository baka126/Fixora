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

// ExtractSnippet finds specific fields for a container in a K8s YAML and returns them as a combined YAML snippet.
func ExtractSnippet(yamlStr, containerName string, fields []string) (string, error) {
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(yamlStr), &root); err != nil {
		return "", err
	}

	containerNode := findContainerNode(&root, containerName)
	if containerNode == nil {
		// Fallback to top-level fields (e.g. for Replica count in Deployment)
		snippet := make(map[string]interface{})
		var fullMap map[string]interface{}
		yaml.Unmarshal([]byte(yamlStr), &fullMap)
		for _, f := range fields {
			if val, ok := fullMap[f]; ok {
				snippet[f] = val
			}
		}
		if len(snippet) > 0 {
			bytes, _ := yaml.Marshal(snippet)
			return string(bytes), nil
		}
		return "", fmt.Errorf("container or fields not found")
	}

	snippetMap := make(map[string]interface{})
	for _, field := range fields {
		if node := findKey(containerNode, field); node != nil {
			var val interface{}
			node.Decode(&val)
			snippetMap[field] = val
		}
	}

	if len(snippetMap) == 0 {
		return "", fmt.Errorf("no target fields found in container %s", containerName)
	}

	bytes, err := yaml.Marshal(snippetMap)
	return string(bytes), err
}

func findContainerNode(node *yaml.Node, containerName string) *yaml.Node {
	if node.Kind == yaml.MappingNode {
		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i].Value
			if key == "containers" && node.Content[i+1].Kind == yaml.SequenceNode {
				for _, container := range node.Content[i+1].Content {
					if nameNode := findKey(container, "name"); nameNode != nil && nameNode.Value == containerName {
						return container
					}
				}
			}
			if found := findContainerNode(node.Content[i+1], containerName); found != nil {
				return found
			}
		}
	}
	if node.Kind == yaml.DocumentNode || node.Kind == yaml.SequenceNode {
		for _, child := range node.Content {
			if found := findContainerNode(child, containerName); found != nil {
				return found
			}
		}
	}
	return nil
}

// SurgicalUpdate replaces specific fields in the original YAML with new values from a snippet.
func SurgicalUpdate(originalYaml, containerName, snippetYaml string) (string, error) {
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(originalYaml), &root); err != nil {
		return "", err
	}

	var snippetNode yaml.Node
	if err := yaml.Unmarshal([]byte(snippetYaml), &snippetNode); err != nil {
		return "", err
	}
	
	// Snippet is usually a map of fields
	targetSnippet := &snippetNode
	if snippetNode.Kind == yaml.DocumentNode && len(snippetNode.Content) > 0 {
		targetSnippet = snippetNode.Content[0]
	}

	if targetSnippet.Kind != yaml.MappingNode {
		return "", fmt.Errorf("invalid snippet format: expected mapping")
	}

	containerNode := findContainerNode(&root, containerName)
	if containerNode == nil {
		// Try top-level update (e.g. replicas)
		if root.Kind == yaml.DocumentNode && len(root.Content) > 0 {
			applyMapping(root.Content[0], targetSnippet)
		} else {
			applyMapping(&root, targetSnippet)
		}
	} else {
		applyMapping(containerNode, targetSnippet)
	}

	bytes, err := yaml.Marshal(&root)
	return string(bytes), err
}

func applyMapping(dest, src *yaml.Node) {
	for i := 0; i < len(src.Content); i += 2 {
		key := src.Content[i].Value
		val := src.Content[i+1]
		
		found := false
		for j := 0; j < len(dest.Content); j += 2 {
			if dest.Content[j].Value == key {
				dest.Content[j+1] = val
				found = true
				break
			}
		}
		if !found {
			dest.Content = append(dest.Content, src.Content[i], val)
		}
	}
}

func findKey(node *yaml.Node, targetKey string) *yaml.Node {
	if node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == targetKey {
			return node.Content[i+1]
		}
	}
	return nil
}

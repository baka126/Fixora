package controller

import (
	"strings"

	"fixora/pkg/ai"
	"fixora/pkg/gitops"
	"gopkg.in/yaml.v3"
	v1 "k8s.io/api/core/v1"
)

func isSurgicalContainerSnippet(content string) bool {
	if ai.IsKubernetesManifest(content) {
		return false
	}
	return strings.Contains(content, "limits:") || strings.Contains(content, "requests:")
}

func applyRawManifestPatch(original, generated string, source gitops.WorkloadSource, diagnosis Diagnosis, pod *v1.Pod) (string, bool, error) {
	if source.ManifestType == gitops.ManifestHelm || source.ManifestType == gitops.ManifestKustomize || source.ManifestType == gitops.ManifestFluxHelmRelease {
		return generated, false, nil
	}
	if !ai.IsKubernetesManifest(generated) {
		return generated, false, nil
	}

	var originalRoot yaml.Node
	if err := yaml.Unmarshal([]byte(original), &originalRoot); err != nil {
		return "", false, err
	}
	originalDoc := documentMapping(&originalRoot)
	if originalDoc == nil {
		return generated, false, nil
	}

	var generatedRoot yaml.Node
	if err := yaml.Unmarshal([]byte(generated), &generatedRoot); err != nil {
		return "", false, err
	}
	generatedDoc := documentMapping(&generatedRoot)
	if generatedDoc == nil || scalarValue(originalDoc, "kind") != scalarValue(generatedDoc, "kind") {
		return generated, false, nil
	}
	if scalarValue(originalDoc, "kind") != "Pod" {
		return generated, false, nil
	}

	containerName := ""
	if len(pod.Spec.Containers) > 0 {
		containerName = pod.Spec.Containers[0].Name
	}
	originalContainer := podContainerMapping(originalDoc, containerName)
	generatedContainer := podContainerMapping(generatedDoc, containerName)
	if originalContainer == nil || generatedContainer == nil {
		return generated, false, nil
	}

	changed := copyContainerPatchFields(originalContainer, generatedContainer, diagnosis.PatchStrategy)
	if !changed {
		return original, false, nil
	}

	bytes, err := yaml.Marshal(&originalRoot)
	if err != nil {
		return "", false, err
	}
	return string(bytes), true, nil
}

func copyContainerPatchFields(dest, src *yaml.Node, strategy PatchStrategy) bool {
	changed := false
	for _, key := range containerPatchFields(strategy) {
		srcValue := mappingValue(src, key)
		if srcValue == nil {
			continue
		}
		destValue := mappingValue(dest, key)
		if destValue != nil && yamlNodesEqual(destValue, srcValue) {
			continue
		}
		setMappingValue(dest, key, cloneYAMLNode(srcValue))
		changed = true
	}
	return changed
}

func containerPatchFields(strategy PatchStrategy) []string {
	switch strategy {
	case PatchImage:
		return []string{"image", "imagePullPolicy"}
	case PatchResources:
		return []string{"resources"}
	case PatchEnvOrVolumeRef:
		return []string{"env", "envFrom", "volumeMounts", "command", "args"}
	case PatchProbe:
		return []string{"livenessProbe", "readinessProbe", "startupProbe"}
	default:
		return []string{"image", "imagePullPolicy", "resources", "env", "envFrom", "volumeMounts", "command", "args", "livenessProbe", "readinessProbe", "startupProbe"}
	}
}

func podContainerMapping(doc *yaml.Node, containerName string) *yaml.Node {
	spec := mappingValue(doc, "spec")
	if spec == nil {
		return nil
	}
	containers := mappingValue(spec, "containers")
	if containers == nil || containers.Kind != yaml.SequenceNode {
		return nil
	}
	for _, container := range containers.Content {
		if container.Kind != yaml.MappingNode {
			continue
		}
		if containerName == "" || scalarValue(container, "name") == containerName {
			return container
		}
	}
	return nil
}

func yamlNodesEqual(a, b *yaml.Node) bool {
	aBytes, aErr := yaml.Marshal(a)
	bBytes, bErr := yaml.Marshal(b)
	return aErr == nil && bErr == nil && string(aBytes) == string(bBytes)
}

func cloneYAMLNode(node *yaml.Node) *yaml.Node {
	if node == nil {
		return nil
	}
	clone := *node
	if len(node.Content) > 0 {
		clone.Content = make([]*yaml.Node, len(node.Content))
		for i, child := range node.Content {
			clone.Content[i] = cloneYAMLNode(child)
		}
	}
	return &clone
}

func documentMapping(root *yaml.Node) *yaml.Node {
	if root.Kind == yaml.DocumentNode && len(root.Content) > 0 && root.Content[0].Kind == yaml.MappingNode {
		return root.Content[0]
	}
	if root.Kind == yaml.MappingNode {
		return root
	}
	return nil
}

func mappingValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

func scalarValue(node *yaml.Node, key string) string {
	value := mappingValue(node, key)
	if value == nil || value.Kind != yaml.ScalarNode {
		return ""
	}
	return value.Value
}

func setScalarValue(node *yaml.Node, key, value string) {
	setMappingValue(node, key, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value})
}

func setMappingValue(node *yaml.Node, key string, value *yaml.Node) {
	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			node.Content[i+1] = value
			return
		}
	}
	node.Content = append(node.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, value)
}

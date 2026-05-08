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

func normalizeRawPodPatchIdentity(pod *v1.Pod, source gitops.WorkloadSource, content string) (string, bool, error) {
	if source.ManifestType == gitops.ManifestHelm || source.ManifestType == gitops.ManifestKustomize || source.ManifestType == gitops.ManifestFluxHelmRelease {
		return content, false, nil
	}

	var root yaml.Node
	if err := yaml.Unmarshal([]byte(content), &root); err != nil {
		return "", false, err
	}
	doc := documentMapping(&root)
	if doc == nil || scalarValue(doc, "kind") != "Pod" {
		return content, false, nil
	}
	metadata := mappingValue(doc, "metadata")
	if metadata == nil {
		metadata = &yaml.Node{Kind: yaml.MappingNode}
		setMappingValue(doc, "metadata", metadata)
	}

	changed := false
	if pod.Name != "" && scalarValue(metadata, "name") != pod.Name {
		setScalarValue(metadata, "name", pod.Name)
		changed = true
	}
	if pod.Namespace != "" && scalarValue(metadata, "namespace") != "" && scalarValue(metadata, "namespace") != pod.Namespace {
		setScalarValue(metadata, "namespace", pod.Namespace)
		changed = true
	}
	if !changed {
		return content, false, nil
	}

	bytes, err := yaml.Marshal(&root)
	if err != nil {
		return "", false, err
	}
	return string(bytes), true, nil
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

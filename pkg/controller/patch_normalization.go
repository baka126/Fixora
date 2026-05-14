package controller

import (
	"fmt"
	"regexp"
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

func applyRawManifestPatch(original, generated string, source gitops.WorkloadSource, diagnosis Diagnosis, pod *v1.Pod, identity workloadIdentity, failingContainerName string) (string, bool, error) {
	if source.ManifestType == gitops.ManifestHelm || source.ManifestType == gitops.ManifestFluxHelmRelease {
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
	kind := scalarValue(originalDoc, "kind")
	if !isPatchableWorkloadKind(kind) {
		return generated, false, nil
	}
	if err := validateRawWorkloadManifestIdentity(originalDoc, kind, pod, identity); err != nil {
		return "", false, err
	}
	if source.ManifestType == gitops.ManifestKustomize {
		return generated, false, nil
	}

	containerName := targetContainerName(pod, failingContainerName)
	originalContainer := workloadContainerMapping(originalDoc, kind, containerName)
	generatedContainer := workloadContainerMapping(generatedDoc, kind, containerName)
	if originalContainer == nil || generatedContainer == nil {
		return generated, false, nil
	}

	originalImage := scalarValue(originalContainer, "image")
	generatedImage := scalarValue(generatedContainer, "image")
	changed := copyContainerPatchFields(originalContainer, generatedContainer, diagnosis.PatchStrategy)
	if !changed {
		return original, false, nil
	}

	if diagnosis.PatchStrategy == PatchImage {
		patched, ok := patchYAMLScalar(original, originalImage, generatedImage)
		if ok {
			return patched, true, nil
		}
	}

	bytes, err := yaml.Marshal(&originalRoot)
	if err != nil {
		return "", false, err
	}
	return string(bytes), true, nil
}

func validateRawWorkloadManifestIdentity(doc *yaml.Node, kind string, pod *v1.Pod, identity workloadIdentity) error {
	metadata := mappingValue(doc, "metadata")
	manifestName := scalarValue(metadata, "name")
	manifestNamespace := scalarValue(metadata, "namespace")
	targetKind := identity.Kind
	targetName := identity.Name
	if targetKind == "" || targetName == "" {
		targetKind = "Pod"
		if pod != nil {
			targetName = pod.Name
		}
	}
	if kind == "Pod" && targetKind != "Pod" {
		return fmt.Errorf("raw Pod manifest is not a safe source for controller-owned workload: source declares Pod/%s but incident is owned by %s/%s", manifestName, targetKind, targetName)
	}
	if targetKind != "" && kind != targetKind {
		return fmt.Errorf("raw workload manifest kind mismatch: source declares %s/%s but incident target is %s/%s", kind, manifestName, targetKind, targetName)
	}
	if targetName != "" && manifestName != "" && manifestName != targetName {
		return fmt.Errorf("raw workload manifest identity mismatch: source declares %s/%s but incident target is %s/%s", kind, manifestName, targetKind, targetName)
	}
	if pod != nil && pod.Namespace != "" && manifestNamespace != "" && manifestNamespace != pod.Namespace {
		return fmt.Errorf("raw workload manifest namespace mismatch: source declares namespace %s but incident is namespace %s", manifestNamespace, pod.Namespace)
	}
	return nil
}

func targetContainerName(pod *v1.Pod, failingContainerName string) string {
	if strings.TrimSpace(failingContainerName) != "" {
		return strings.TrimSpace(failingContainerName)
	}
	if pod != nil && len(pod.Spec.Containers) > 0 {
		return pod.Spec.Containers[0].Name
	}
	if pod != nil && len(pod.Spec.InitContainers) > 0 {
		return pod.Spec.InitContainers[0].Name
	}
	return ""
}

func failingContainerNameForPod(pod *v1.Pod) string {
	if pod == nil {
		return ""
	}
	for _, status := range pod.Status.InitContainerStatuses {
		if isFailingContainerStatus(status) {
			return status.Name
		}
	}
	for _, status := range pod.Status.ContainerStatuses {
		if isFailingContainerStatus(status) {
			return status.Name
		}
	}
	return ""
}

func isFailingContainerStatus(status v1.ContainerStatus) bool {
	if status.State.Waiting != nil {
		switch status.State.Waiting.Reason {
		case "CrashLoopBackOff", "CreateContainerConfigError", "ImagePullBackOff", "ErrImagePull", "CreateContainerError":
			return true
		}
	}
	if status.State.Terminated != nil {
		return status.State.Terminated.ExitCode != 0 ||
			status.State.Terminated.Reason == "OOMKilled" ||
			status.State.Terminated.Reason == "ContainerCannotRun" ||
			status.State.Terminated.Reason == "DeadlineExceeded"
	}
	return false
}

func isPatchableWorkloadKind(kind string) bool {
	switch kind {
	case "Pod", "Deployment", "StatefulSet", "DaemonSet", "ReplicaSet", "Job", "CronJob":
		return true
	default:
		return false
	}
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

func patchYAMLScalar(content, oldValue, newValue string) (string, bool) {
	if strings.TrimSpace(oldValue) == "" || strings.TrimSpace(newValue) == "" || oldValue == newValue {
		return content, false
	}
	pattern := regexp.MustCompile(`(?m)^(\s*image:\s*)(["']?)` + regexp.QuoteMeta(oldValue) + `(["']?)\s*$`)
	matches := pattern.FindAllStringSubmatchIndex(content, -1)
	if len(matches) != 1 {
		return content, false
	}
	return pattern.ReplaceAllString(content, "${1}${2}"+newValue+"${3}"), true
}

func workloadContainerMapping(doc *yaml.Node, kind, containerName string) *yaml.Node {
	for _, containers := range workloadContainerNodes(doc, kind) {
		if containers == nil || containers.Kind != yaml.SequenceNode {
			continue
		}
		for _, container := range containers.Content {
			if container.Kind != yaml.MappingNode {
				continue
			}
			if containerName == "" || scalarValue(container, "name") == containerName {
				return container
			}
		}
	}
	return nil
}

func workloadContainerNodes(doc *yaml.Node, kind string) []*yaml.Node {
	spec := mappingValue(doc, "spec")
	if spec == nil {
		return nil
	}
	switch kind {
	case "Pod":
		return containerNodesFromSpec(spec)
	case "Deployment", "StatefulSet", "DaemonSet", "ReplicaSet":
		return templateContainerNodes(spec)
	case "Job":
		return templateContainerNodes(spec)
	case "CronJob":
		jobTemplate := mappingValue(spec, "jobTemplate")
		jobSpec := mappingValue(jobTemplate, "spec")
		return templateContainerNodes(jobSpec)
	default:
		return nil
	}
}

func templateContainerNodes(spec *yaml.Node) []*yaml.Node {
	template := mappingValue(spec, "template")
	templateSpec := mappingValue(template, "spec")
	return containerNodesFromSpec(templateSpec)
}

func containerNodesFromSpec(spec *yaml.Node) []*yaml.Node {
	if spec == nil {
		return nil
	}
	return []*yaml.Node{
		mappingValue(spec, "containers"),
		mappingValue(spec, "initContainers"),
	}
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

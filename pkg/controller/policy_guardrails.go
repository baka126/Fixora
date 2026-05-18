package controller

import (
	"bytes"
	"fmt"
	"io"
	"path"
	"strings"

	"fixora/pkg/gitops"
	"fixora/pkg/vcs"
	"gopkg.in/yaml.v3"
)

var deniedRBACKinds = map[string]bool{
	"Role":               true,
	"RoleBinding":        true,
	"ClusterRole":        true,
	"ClusterRoleBinding": true,
}

type manifestIdentity struct {
	Namespace string
	Kind      string
	Name      string
}

func enforcePatchGuardrails(source gitops.WorkloadSource, changes []vcs.FileChange, allowedImageRegistries, allowedReplacementImages []string, incidentNamespace string, expectedTargets []manifestIdentity, excludedNamespaces []string) error {
	for _, change := range changes {
		lowerPath := strings.ToLower(change.FilePath)
		base := path.Base(lowerPath)
		if strings.Contains(lowerPath, ".github/workflows/") || strings.Contains(lowerPath, ".gitlab-ci") {
			return fmt.Errorf("CI workflow changes are not allowed: %s", change.FilePath)
		}
		if strings.Contains(lowerPath, "/rbac") || strings.Contains(base, "rbac") {
			return fmt.Errorf("RBAC manifest path changes are not allowed: %s", change.FilePath)
		}
		if strings.Contains(base, "secret") {
			return fmt.Errorf("Secret manifest file changes are not allowed: %s", change.FilePath)
		}
		if change.Delete {
			continue
		}
		if err := enforceManifestGuardrails(source, change.FilePath, change.NewContent, change.PreviousContent, allowedImageRegistries, allowedReplacementImages, incidentNamespace, expectedTargets, excludedNamespaces); err != nil {
			return err
		}
	}
	return nil
}

func enforceArchitectureImageGuardrail(diagnosis Diagnosis, changes []vcs.FileChange, allowedReplacementImages []string) error {
	if !isArchitectureImageDiagnosis(diagnosis) || len(allowedReplacementImages) > 0 {
		return nil
	}
	for _, change := range changes {
		previousImages := manifestImageSet(change.PreviousContent)
		decoder := yaml.NewDecoder(bytes.NewReader(change.NewContent))
		for {
			var doc map[string]interface{}
			err := decoder.Decode(&doc)
			if err == io.EOF {
				break
			}
			if err != nil || len(doc) == 0 {
				continue
			}
			for _, image := range policyDocumentImages(doc) {
				if !previousImages[image] {
					return fmt.Errorf("replacement image %s must be configured in ALLOWED_REPLACEMENT_IMAGES for CPU architecture remediation", image)
				}
			}
		}
	}
	return nil
}

func isArchitectureImageDiagnosis(diagnosis Diagnosis) bool {
	if diagnosis.PatchStrategy != PatchImage {
		return false
	}
	haystack := strings.ToLower(strings.Join(append([]string{diagnosis.Symptom, diagnosis.LikelyCause}, diagnosis.Evidence...), "\n"))
	return strings.Contains(haystack, "architecture") ||
		strings.Contains(haystack, "exec format") ||
		strings.Contains(haystack, "cpu instruction") ||
		strings.Contains(haystack, "wrong architecture")
}

func enforceManifestGuardrails(source gitops.WorkloadSource, filePath string, content, previousContent []byte, allowedImageRegistries, allowedReplacementImages []string, incidentNamespace string, expectedTargets []manifestIdentity, excludedNamespaces []string) error {
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	previousImages := manifestImageSet(previousContent)
	matchedExpectedTarget := len(expectedTargets) == 0 || source.Controller == gitops.ControllerAnnotation || source.ManifestType == gitops.ManifestHelm || source.ManifestType == gitops.ManifestFluxHelmRelease
	checkedWorkloadDoc := false
	for {
		var doc map[string]interface{}
		err := decoder.Decode(&doc)
		if err == io.EOF {
			if checkedWorkloadDoc && !matchedExpectedTarget {
				return fmt.Errorf("manifest %s does not target the incident workload", filePath)
			}
			return nil
		}
		if err != nil {
			return fmt.Errorf("cannot parse %s for policy guardrails: %w", filePath, err)
		}
		if len(doc) == 0 {
			continue
		}
		if nestedPath := nestedKubernetesObjectPath(doc); nestedPath != "" {
			return fmt.Errorf("nested Kubernetes object is not allowed in %s at %s", filePath, nestedPath)
		}

		// Cross-Namespace guardrail
		name := ""
		if metadata, ok := doc["metadata"].(map[string]interface{}); ok {
			name, _ = metadata["name"].(string)
			if ns, ok := metadata["namespace"].(string); ok && ns != "" {
				if incidentNamespace != "" && !strings.EqualFold(ns, strings.TrimSpace(incidentNamespace)) {
					return fmt.Errorf("cross-namespace modification from %s to %s is not allowed in %s", incidentNamespace, ns, filePath)
				}
				for _, excluded := range excludedNamespaces {
					if strings.EqualFold(ns, strings.TrimSpace(excluded)) {
						return fmt.Errorf("modifying resources in protected namespace %s is not allowed in %s", ns, filePath)
					}
				}
			}
		}

		kind, _ := doc["kind"].(string)
		if deniedRBACKinds[kind] {
			return fmt.Errorf("RBAC kind %s is not allowed in %s", kind, filePath)
		}
		if kind == "Secret" {
			return fmt.Errorf("Secret manifests are not allowed in %s", filePath)
		}
		if isWorkloadKind(kind) && source.ManifestType != gitops.ManifestHelm && source.ManifestType != gitops.ManifestFluxHelmRelease {
			checkedWorkloadDoc = true
			if manifestMatchesExpectedTarget(kind, name, incidentNamespace, expectedTargets) {
				matchedExpectedTarget = true
			}
		}
		images := policyDocumentImages(doc)
		if len(allowedImageRegistries) > 0 {
			for _, image := range images {
				registry := imageRegistry(image)
				if !registryAllowed(registry, allowedImageRegistries) {
					return fmt.Errorf("image registry %s is not allowlisted for image %s in %s", registry, image, filePath)
				}
			}
		}
		for _, image := range images {
			if previousImages[image] {
				continue
			}
			if !imagePinned(image) {
				return fmt.Errorf("replacement image %s must be pinned to a non-latest tag or digest in %s", image, filePath)
			}
			if len(allowedReplacementImages) > 0 && !imageAllowed(image, allowedReplacementImages) {
				return fmt.Errorf("replacement image %s is not allowlisted in %s", image, filePath)
			}
		}
	}
}

func nestedKubernetesObjectPath(root map[string]interface{}) string {
	return nestedKubernetesObjectPathAt(root, "$", true)
}

func nestedKubernetesObjectPathAt(node interface{}, currentPath string, root bool) string {
	switch typed := node.(type) {
	case map[string]interface{}:
		if !root {
			_, hasAPIVersion := typed["apiVersion"]
			_, hasKind := typed["kind"]
			if hasAPIVersion && hasKind {
				return currentPath
			}
		}
		for key, value := range typed {
			if path := nestedKubernetesObjectPathAt(value, currentPath+"."+key, false); path != "" {
				return path
			}
		}
	case []interface{}:
		for i, item := range typed {
			if path := nestedKubernetesObjectPathAt(item, fmt.Sprintf("%s[%d]", currentPath, i), false); path != "" {
				return path
			}
		}
	}
	return ""
}

func isWorkloadKind(kind string) bool {
	switch kind {
	case "Pod", "Deployment", "StatefulSet", "DaemonSet", "ReplicaSet", "Job", "CronJob":
		return true
	default:
		return false
	}
}

func manifestMatchesExpectedTarget(kind, name, namespace string, expected []manifestIdentity) bool {
	for _, target := range expected {
		if target.Kind != "" && !strings.EqualFold(kind, strings.TrimSpace(target.Kind)) {
			continue
		}
		if target.Name != "" && !strings.EqualFold(name, strings.TrimSpace(target.Name)) {
			continue
		}
		if target.Namespace != "" && namespace != "" && !strings.EqualFold(namespace, strings.TrimSpace(target.Namespace)) {
			continue
		}
		return true
	}
	return false
}

func manifestImages(doc map[string]interface{}) []string {
	var images []string
	visitManifestContainers(doc, func(container map[string]interface{}) {
		if image, ok := container["image"].(string); ok && image != "" {
			images = append(images, image)
		}
	})
	return images
}

func policyDocumentImages(doc map[string]interface{}) []string {
	seen := map[string]bool{}
	var images []string
	for _, image := range append(manifestImages(doc), helmValuesImages(doc)...) {
		image = strings.TrimSpace(image)
		if image == "" || seen[image] {
			continue
		}
		seen[image] = true
		images = append(images, image)
	}
	return images
}

func visitManifestContainers(node interface{}, visit func(map[string]interface{})) {
	switch typed := node.(type) {
	case map[string]interface{}:
		for key, value := range typed {
			if key == "containers" || key == "initContainers" {
				if list, ok := value.([]interface{}); ok {
					for _, item := range list {
						if container, ok := item.(map[string]interface{}); ok {
							visit(container)
						}
					}
				}
			}
			visitManifestContainers(value, visit)
		}
	case []interface{}:
		for _, item := range typed {
			visitManifestContainers(item, visit)
		}
	}
}

func helmValuesImages(node interface{}) []string {
	var images []string
	visitHelmValuesImages(node, "", func(image string) {
		images = append(images, image)
	})
	return images
}

func visitHelmValuesImages(node interface{}, parentKey string, visit func(string)) {
	switch typed := node.(type) {
	case map[string]interface{}:
		if image := helmImageFromMap(typed, parentKey); image != "" {
			visit(image)
		}
		for key, value := range typed {
			lowerKey := strings.ToLower(strings.TrimSpace(key))
			if image, ok := value.(string); ok && lowerKey == "image" && strings.TrimSpace(image) != "" {
				visit(image)
			}
			visitHelmValuesImages(value, lowerKey, visit)
		}
	case []interface{}:
		for _, item := range typed {
			visitHelmValuesImages(item, parentKey, visit)
		}
	}
}

func helmImageFromMap(values map[string]interface{}, parentKey string) string {
	repository := stringMapValue(values, "repository")
	if repository == "" {
		return ""
	}
	registry := stringMapValue(values, "registry")
	tag := stringMapValue(values, "tag")
	digest := stringMapValue(values, "digest")
	image := repository
	if registry != "" && !strings.Contains(repository, "/") {
		image = strings.TrimSuffix(registry, "/") + "/" + repository
	}
	if digest != "" {
		digest = strings.TrimPrefix(digest, "@")
		return image + "@" + digest
	}
	if tag != "" {
		return image + ":" + tag
	}
	if strings.Contains(strings.ToLower(parentKey), "image") {
		return image
	}
	return ""
}

func stringMapValue(values map[string]interface{}, key string) string {
	for currentKey, value := range values {
		if !strings.EqualFold(strings.TrimSpace(currentKey), key) {
			continue
		}
		if str, ok := value.(string); ok {
			return strings.TrimSpace(str)
		}
	}
	return ""
}

func imageRegistry(image string) string {
	first := strings.SplitN(image, "/", 2)[0]
	if strings.Contains(first, ".") || strings.Contains(first, ":") || first == "localhost" {
		return first
	}
	return "docker.io"
}

func registryAllowed(registry string, allowed []string) bool {
	for _, item := range allowed {
		item = strings.TrimSpace(item)
		if item == "*" || strings.EqualFold(item, registry) {
			return true
		}
	}
	return false
}

func imageAllowed(image string, allowed []string) bool {
	image = strings.TrimSpace(image)
	for _, item := range allowed {
		item = strings.TrimSpace(item)
		if item == "*" || item == image {
			return true
		}
		if strings.HasSuffix(item, ":*") && strings.HasPrefix(image, strings.TrimSuffix(item, "*")) {
			return true
		}
		if strings.HasSuffix(item, "/*") && strings.HasPrefix(image, strings.TrimSuffix(item, "*")) {
			return true
		}
	}
	return false
}

func imagePinned(image string) bool {
	image = strings.TrimSpace(image)
	if image == "" {
		return false
	}
	if strings.Contains(image, "@sha256:") {
		return true
	}
	lastSlash := strings.LastIndex(image, "/")
	lastColon := strings.LastIndex(image, ":")
	if lastColon <= lastSlash {
		return false
	}
	tag := strings.TrimSpace(image[lastColon+1:])
	return tag != "" && !strings.EqualFold(tag, "latest")
}

func manifestImageSet(content []byte) map[string]bool {
	images := map[string]bool{}
	if len(content) == 0 {
		return images
	}
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	for {
		var doc map[string]interface{}
		err := decoder.Decode(&doc)
		if err == io.EOF {
			return images
		}
		if err != nil {
			return images
		}
		for _, image := range policyDocumentImages(doc) {
			images[image] = true
		}
	}
}

func policyRejectionReason(err error) string {
	if err == nil {
		return "unknown"
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "ci workflow"):
		return "ci-workflow"
	case strings.Contains(msg, "rbac"):
		return "rbac"
	case strings.Contains(msg, "secret"):
		return "secret"
	case strings.Contains(msg, "image registry"):
		return "image-registry"
	case strings.Contains(msg, "replacement image"):
		return "replacement-image"
	default:
		return "policy"
	}
}

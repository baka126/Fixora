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

func enforcePatchGuardrails(_ gitops.WorkloadSource, changes []vcs.FileChange, allowedImageRegistries []string) error {
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
		if err := enforceManifestGuardrails(change.FilePath, change.NewContent, allowedImageRegistries); err != nil {
			return err
		}
	}
	return nil
}

func enforceManifestGuardrails(filePath string, content []byte, allowedImageRegistries []string) error {
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	for {
		var doc map[string]interface{}
		err := decoder.Decode(&doc)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("cannot parse %s for policy guardrails: %w", filePath, err)
		}
		if len(doc) == 0 {
			continue
		}
		kind, _ := doc["kind"].(string)
		if deniedRBACKinds[kind] {
			return fmt.Errorf("RBAC kind %s is not allowed in %s", kind, filePath)
		}
		if kind == "Secret" {
			return fmt.Errorf("Secret manifests are not allowed in %s", filePath)
		}
		if len(allowedImageRegistries) > 0 {
			for _, image := range manifestImages(doc) {
				registry := imageRegistry(image)
				if !registryAllowed(registry, allowedImageRegistries) {
					return fmt.Errorf("image registry %s is not allowlisted for image %s in %s", registry, image, filePath)
				}
			}
		}
	}
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
	default:
		return "policy"
	}
}

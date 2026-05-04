package controller

import (
	"context"
	"fmt"
	"path"
	"strings"

	"fixora/pkg/gitops"
	"fixora/pkg/validation"
	"fixora/pkg/vcs"
	v1 "k8s.io/api/core/v1"
)

func (c *Controller) resolveGitOpsSources(ctx context.Context, pod *v1.Pod) []gitops.WorkloadSource {
	resolver := gitops.NewResolver(c.clientset, c.dynamicClient, gitops.ResolverConfig{
		ArgoCDEnabled:   c.config.ArgoCDEnabled,
		ArgoCDNamespace: c.config.ArgoCDNamespace,
	})
	sources, err := resolver.ResolvePod(ctx, pod)
	if err != nil {
		return nil
	}
	return sources
}

func gitOpsPatchInstructions(source gitops.WorkloadSource, pod *v1.Pod, diagnosis Diagnosis) string {
	switch source.ManifestType {
	case gitops.ManifestKustomize:
		return fmt.Sprintf("%s. Kustomize overlay detected: do not edit rendered/base workload manifests directly. Update kustomization.yaml and create or update a StrategicMergePatch under an allowed patch path for pod %s. Patch strategy: %s", source.Summary(), pod.Name, diagnosis.PatchStrategy)
	case gitops.ManifestHelm:
		return fmt.Sprintf("%s. Helm source detected: prefer values.yaml for environment-specific changes; edit templates only when the structural chart template is the root cause. Patch strategy: %s", source.Summary(), diagnosis.PatchStrategy)
	case gitops.ManifestFluxHelmRelease:
		return fmt.Sprintf("%s. Flux HelmRelease detected: prefer HelmRelease values or referenced values files, not rendered workload YAML. Patch strategy: %s", source.Summary(), diagnosis.PatchStrategy)
	default:
		return fmt.Sprintf("%s. Raw manifest source detected: edit only the source manifest for the affected workload. Patch strategy: %s", source.Summary(), diagnosis.PatchStrategy)
	}
}

func isGitOpsContextFile(source gitops.WorkloadSource, filePath string) bool {
	if !isRemediableManifest(filePath) {
		return false
	}
	switch source.ManifestType {
	case gitops.ManifestKustomize:
		return true
	case gitops.ManifestHelm, gitops.ManifestFluxHelmRelease:
		return isHelmRelevantFile(filePath)
	default:
		return true
	}
}

func isGitOpsEditableFile(source gitops.WorkloadSource, filePath string) bool {
	switch source.ManifestType {
	case gitops.ManifestKustomize:
		return isKustomizeControlFile(filePath) || isKustomizePatchFile(filePath)
	case gitops.ManifestHelm, gitops.ManifestFluxHelmRelease:
		return isHelmEditableFile(filePath)
	default:
		return isRemediableManifest(filePath)
	}
}

func allowedNewPatchFiles(pod *v1.Pod, source gitops.WorkloadSource, diagnosis Diagnosis) []string {
	if source.ManifestType != gitops.ManifestKustomize {
		return nil
	}
	name := fmt.Sprintf("%s-%s.yaml", slugify(pod.Name), slugify(string(diagnosis.PatchStrategy)))
	return []string{path.Join(source.Path, "fixora-patches", name)}
}

func isKustomizeControlFile(filePath string) bool {
	base := strings.ToLower(path.Base(filePath))
	return base == "kustomization.yaml" || base == "kustomization.yml"
}

func isKustomizePatchFile(filePath string) bool {
	lower := strings.ToLower(filePath)
	base := path.Base(lower)
	return strings.Contains(lower, "/patches/") || strings.Contains(lower, "fixora-patches/") || strings.Contains(base, "patch")
}

func isHelmRelevantFile(filePath string) bool {
	lower := strings.ToLower(filePath)
	base := path.Base(lower)
	return isRemediableManifest(filePath) &&
		(base == "values.yaml" || base == "values.yml" || base == "chart.yaml" || strings.Contains(lower, "/templates/") || strings.Contains(lower, "helmrelease"))
}

func isHelmEditableFile(filePath string) bool {
	lower := strings.ToLower(filePath)
	base := path.Base(lower)
	return base == "values.yaml" || base == "values.yml" || strings.Contains(lower, "helmrelease") || strings.Contains(lower, "/templates/")
}

func validateManifestAwarePatchSet(source gitops.WorkloadSource, changes []vcs.FileChange) error {
	if source.ManifestType != gitops.ManifestKustomize {
		return nil
	}

	hasGeneratedPatch := false
	hasKustomization := false
	for _, change := range changes {
		if change.Create && isKustomizePatchFile(change.FilePath) {
			hasGeneratedPatch = true
		}
		if isKustomizeControlFile(change.FilePath) {
			hasKustomization = true
		}
	}
	if hasGeneratedPatch && !hasKustomization {
		return fmt.Errorf("Kustomize remediation creates a patch file but does not update kustomization.yaml")
	}
	return nil
}

func validationMessage(result validation.ValidationResult) string {
	if result.Output != "" {
		return result.Output
	}
	if result.Error != nil {
		return result.Error.Error()
	}
	return "validation failed"
}

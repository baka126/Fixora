package controller

import (
	"strings"
	"testing"

	"fixora/pkg/gitops"
	"fixora/pkg/vcs"
)

func TestValidateManifestAwarePatchSetRequiresKustomizationForGeneratedPatch(t *testing.T) {
	source := gitops.WorkloadSource{ManifestType: gitops.ManifestKustomize}
	err := validateManifestAwarePatchSet(source, []vcs.FileChange{{
		FilePath: "overlays/prod/fixora-patches/api-resources.yaml",
		Create:   true,
	}})
	if err == nil {
		t.Fatal("expected generated Kustomize patch without kustomization.yaml to be rejected")
	}
}

func TestValidateManifestAwarePatchSetAllowsKustomizePatchWithControlFile(t *testing.T) {
	source := gitops.WorkloadSource{ManifestType: gitops.ManifestKustomize}
	err := validateManifestAwarePatchSet(source, []vcs.FileChange{
		{FilePath: "overlays/prod/fixora-patches/api-resources.yaml", Create: true},
		{FilePath: "overlays/prod/kustomization.yaml"},
	})
	if err != nil {
		t.Fatalf("expected Kustomize patch with kustomization.yaml to pass, got %v", err)
	}
}

func TestGitOpsPatchInstructionsTargetOwnerWorkload(t *testing.T) {
	got := gitOpsPatchInstructions(
		gitops.WorkloadSource{ManifestType: gitops.ManifestHelm},
		nil,
		workloadIdentity{Kind: "Deployment", Name: "api"},
		Diagnosis{PatchStrategy: PatchResources},
	)

	for _, want := range []string{"Deployment/api", "values.yaml", "Do not patch a rendered Pod manifest"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected instructions to contain %q, got %q", want, got)
		}
	}
}

func TestAllowedNewPatchFilesUseOwnerWorkload(t *testing.T) {
	got := allowedNewPatchFiles(nil, workloadIdentity{Kind: "StatefulSet", Name: "postgres"}, gitops.WorkloadSource{
		ManifestType: gitops.ManifestKustomize,
		Path:         "overlays/prod",
	}, Diagnosis{PatchStrategy: PatchResources})

	if len(got) != 1 || got[0] != "overlays/prod/fixora-patches/statefulset-postgres-resources.yaml" {
		t.Fatalf("expected owner workload patch path, got %#v", got)
	}
}

func TestFluxHelmReleaseFleetSourceAllowsArbitraryYamlFileName(t *testing.T) {
	source := gitops.WorkloadSource{
		ManifestType: gitops.ManifestFluxHelmRelease,
		RepoURL:      "https://github.com/acme/fleet.git",
		Helm: gitops.HelmSource{
			RepoURL: "https://charts.example.com",
		},
	}

	if !isGitOpsContextFile(source, "clusters/prod/api.yaml") {
		t.Fatal("expected Flux HelmRelease fleet source to include arbitrary YAML manifest")
	}
	if !isGitOpsEditableFile(source, "clusters/prod/api.yaml") {
		t.Fatal("expected Flux HelmRelease fleet source YAML to be editable")
	}
}

func TestValidateRemediationFileChangeDefersHelmTemplateDryRun(t *testing.T) {
	result := validateRemediationFileChange(gitops.WorkloadSource{ManifestType: gitops.ManifestHelm}, vcs.FileChange{
		FilePath:   "charts/api/templates/deployment.yaml",
		NewContent: []byte("image: {{ .Values.image.repository }}:{{ .Values.image.tag }}\n"),
	})

	if !result.Valid || !result.Skipped {
		t.Fatalf("expected Helm template validation to be deferred, got %+v", result)
	}
}

func TestValidateRemediationFileChangeValidatesFluxHelmReleaseAsYAML(t *testing.T) {
	result := validateRemediationFileChange(gitops.WorkloadSource{ManifestType: gitops.ManifestFluxHelmRelease}, vcs.FileChange{
		FilePath: "clusters/prod/api.yaml",
		NewContent: []byte(`
apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
spec:
  values:
    resources:
      limits:
        memory: 512Mi
`),
	})

	if !result.Valid {
		t.Fatalf("expected Flux HelmRelease YAML validation to pass, got %+v", result)
	}
}

package controller

import (
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

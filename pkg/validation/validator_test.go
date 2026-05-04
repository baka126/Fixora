package validation

import (
	"strings"
	"testing"
	"time"

	"fixora/pkg/gitops"
	"fixora/pkg/vcs"
)

func TestValidateRenderSandboxSkipsMissingOptionalRenderer(t *testing.T) {
	got := ValidateRenderSandbox(
		gitops.WorkloadSource{ManifestType: gitops.ManifestKustomize},
		map[string][]byte{"overlays/prod/deployment.yaml": []byte("kind: Deployment")},
		nil,
		SandboxOptions{Enabled: true, RequireRender: false, Timeout: time.Second},
	)

	if !got.Valid || !got.Skipped {
		t.Fatalf("expected optional render to be skipped successfully, got %#v", got)
	}
}

func TestValidateRenderSandboxRequiresRendererInputWhenConfigured(t *testing.T) {
	got := ValidateRenderSandbox(
		gitops.WorkloadSource{ManifestType: gitops.ManifestKustomize},
		map[string][]byte{"overlays/prod/deployment.yaml": []byte("kind: Deployment")},
		nil,
		SandboxOptions{Enabled: true, RequireRender: true, Timeout: time.Second},
	)

	if got.Valid || !strings.Contains(got.Output, "no kustomization.yaml") {
		t.Fatalf("expected required render to fail, got %#v", got)
	}
}

func TestValidateRenderSandboxRejectsUnsafePaths(t *testing.T) {
	got := ValidateRenderSandbox(
		gitops.WorkloadSource{ManifestType: gitops.ManifestRaw},
		nil,
		[]vcs.FileChange{{FilePath: "../escape.yaml", NewContent: []byte("kind: Pod")}},
		SandboxOptions{Enabled: true, Timeout: time.Second},
	)

	if got.Valid {
		t.Fatalf("expected unsafe path to fail, got %#v", got)
	}
}

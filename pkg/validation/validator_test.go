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

func TestBuildHelmTemplateArgsUsesConfiguredValueFilesInOrder(t *testing.T) {
	source := gitops.WorkloadSource{
		Helm: gitops.HelmSource{
			ReleaseName: "checkout",
			Namespace:   "prod",
			ValueFiles:  []string{"values.yaml", "values-prod.yaml"},
			Parameters:  map[string]string{"image.pullPolicy": "IfNotPresent"},
		},
	}

	args := buildHelmTemplateArgs("/tmp/fixora", "charts/api/Chart.yaml", []string{
		"charts/api/Chart.yaml",
		"charts/api/values.yaml",
		"charts/api/values-prod.yaml",
	}, source, nil)
	got := strings.Join(args, " ")
	for _, want := range []string{
		"template checkout /tmp/fixora/charts/api",
		"--namespace prod",
		"-f /tmp/fixora/charts/api/values.yaml -f /tmp/fixora/charts/api/values-prod.yaml",
		"--set image.pullPolicy=IfNotPresent",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected args to contain %q, got %q", want, got)
		}
	}
}

func TestHelmValueFileOrderIncludesChangedValuesWhenNoConfiguredFiles(t *testing.T) {
	got := helmValueFileOrder("charts/api/Chart.yaml", []string{
		"charts/api/Chart.yaml",
		"charts/api/values.yaml",
		"charts/api/values-prod.yaml",
	}, gitops.WorkloadSource{}, map[string]bool{"charts/api/values-prod.yaml": true})

	want := []string{"charts/api/values.yaml", "charts/api/values-prod.yaml"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("helm value files = %#v, want %#v", got, want)
	}
}

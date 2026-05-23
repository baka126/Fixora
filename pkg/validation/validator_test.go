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

func TestValidateSemanticRenderRawImageChangePasses(t *testing.T) {
	original := []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: api
  namespace: prod
spec:
  template:
    spec:
      containers:
      - name: api
        image: example/api:v1
`)
	changed := []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: api
  namespace: prod
spec:
  template:
    spec:
      containers:
      - name: api
        image: example/api:v2
`)

	got := ValidateSemanticRender(
		gitops.WorkloadSource{ManifestType: gitops.ManifestRaw},
		map[string][]byte{"deploy.yaml": original},
		[]vcs.FileChange{{FilePath: "deploy.yaml", NewContent: changed}},
		SemanticTarget{Kind: "Deployment", Name: "api", Namespace: "prod", ContainerName: "api", PatchStrategy: "image"},
		SandboxOptions{Enabled: true, Timeout: time.Second},
	)

	if !got.Valid || got.Skipped {
		t.Fatalf("expected semantic image validation to pass, got %#v", got)
	}
}

func TestValidateSemanticRenderRejectsUnrelatedRawChange(t *testing.T) {
	original := []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: api
  namespace: prod
  labels:
    app: api
spec:
  template:
    spec:
      containers:
      - name: api
        image: example/api:v1
`)
	changed := []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: api
  namespace: prod
  labels:
    app: api
    owner: platform
spec:
  template:
    spec:
      containers:
      - name: api
        image: example/api:v1
`)

	got := ValidateSemanticRender(
		gitops.WorkloadSource{ManifestType: gitops.ManifestRaw},
		map[string][]byte{"deploy.yaml": original},
		[]vcs.FileChange{{FilePath: "deploy.yaml", NewContent: changed}},
		SemanticTarget{Kind: "Deployment", Name: "api", Namespace: "prod", ContainerName: "api", PatchStrategy: "image"},
		SandboxOptions{Enabled: true, Timeout: time.Second},
	)

	if got.Valid || !strings.Contains(got.Output, "did not change expected image fields") {
		t.Fatalf("expected unrelated change to fail semantic image validation, got %#v", got)
	}
}

func TestValidateSemanticRenderSchedulingChangePasses(t *testing.T) {
	original := []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: api
spec:
  template:
    spec:
      containers:
      - name: api
        image: example/api:v1
`)
	changed := []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: api
spec:
  template:
    spec:
      nodeSelector:
        kubernetes.io/arch: arm64
      containers:
      - name: api
        image: example/api:v1
`)

	got := ValidateSemanticRender(
		gitops.WorkloadSource{ManifestType: gitops.ManifestRaw},
		map[string][]byte{"deploy.yaml": original},
		[]vcs.FileChange{{FilePath: "deploy.yaml", NewContent: changed}},
		SemanticTarget{Kind: "Deployment", Name: "api", PatchStrategy: "scheduling-policy"},
		SandboxOptions{Enabled: true, Timeout: time.Second},
	)

	if !got.Valid || got.Skipped {
		t.Fatalf("expected semantic scheduling validation to pass, got %#v", got)
	}
}

func TestValidateSemanticRenderSecurityContextChangePasses(t *testing.T) {
	original := []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: api
spec:
  template:
    spec:
      containers:
      - name: api
        image: example/api:v1
`)
	changed := []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: api
spec:
  template:
    spec:
      volumes:
      - name: tmp
        emptyDir: {}
      containers:
      - name: api
        image: example/api:v1
        securityContext:
          readOnlyRootFilesystem: true
          allowPrivilegeEscalation: false
        volumeMounts:
        - name: tmp
          mountPath: /tmp
`)

	got := ValidateSemanticRender(
		gitops.WorkloadSource{ManifestType: gitops.ManifestRaw},
		map[string][]byte{"deploy.yaml": original},
		[]vcs.FileChange{{FilePath: "deploy.yaml", NewContent: changed}},
		SemanticTarget{Kind: "Deployment", Name: "api", ContainerName: "api", PatchStrategy: "security-context"},
		SandboxOptions{Enabled: true, Timeout: time.Second},
	)

	if !got.Valid || got.Skipped {
		t.Fatalf("expected semantic security validation to pass, got %#v", got)
	}
}

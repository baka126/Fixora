package ai

import (
	"strings"
	"testing"
)

func TestSurgicalUpdateRejectsFullManifestSnippet(t *testing.T) {
	original := `
apiVersion: v1
kind: Pod
metadata:
  name: oom-test
spec:
  containers:
  - name: stress
    image: polinux/stress
    resources:
      limits:
        memory: "100Mi"
`
	fullManifest := `
apiVersion: v1
kind: Pod
metadata:
  name: oom-test
spec:
  containers:
  - name: stress
    image: alexeiled/stress-ng
    resources:
      limits:
        memory: "100Mi"
`

	merged, err := SurgicalUpdate(original, "stress", fullManifest)
	if err == nil {
		t.Fatalf("expected full manifest snippet to be rejected, got merged yaml:\n%s", merged)
	}
	if !strings.Contains(err.Error(), "full Kubernetes manifest") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSurgicalUpdateStillAppliesContainerSnippet(t *testing.T) {
	original := `
apiVersion: v1
kind: Pod
metadata:
  name: oom-test
spec:
  containers:
  - name: stress
    image: polinux/stress
    resources:
      limits:
        memory: "100Mi"
`
	snippet := `
resources:
  limits:
    memory: "256Mi"
`

	merged, err := SurgicalUpdate(original, "stress", snippet)
	if err != nil {
		t.Fatalf("expected snippet to merge, got %v", err)
	}
	if !strings.Contains(merged, `memory: "256Mi"`) {
		t.Fatalf("expected merged resources, got:\n%s", merged)
	}
	if strings.Contains(merged, "apiVersion: v1\n    kind: Pod") {
		t.Fatalf("unexpected nested manifest in merged yaml:\n%s", merged)
	}
}

func TestIsKubernetesManifest(t *testing.T) {
	if !IsKubernetesManifest("apiVersion: v1\nkind: Pod\nmetadata:\n  name: api\n") {
		t.Fatal("expected full manifest to be detected")
	}
	if IsKubernetesManifest("resources:\n  limits:\n    memory: 256Mi\n") {
		t.Fatal("expected field snippet not to be detected as a full manifest")
	}
}

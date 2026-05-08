package controller

import (
	"strings"
	"testing"

	"fixora/pkg/gitops"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIsSurgicalContainerSnippetRejectsFullManifest(t *testing.T) {
	fullManifest := `
apiVersion: v1
kind: Pod
metadata:
  name: oom-test
spec:
  containers:
  - name: stress
    resources:
      limits:
        memory: "100Mi"
`
	if isSurgicalContainerSnippet(fullManifest) {
		t.Fatal("expected full manifest not to be treated as a surgical container snippet")
	}
}

func TestNormalizeRawPodPatchIdentityRetargetsStandalonePod(t *testing.T) {
	pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "oom-test-2", Namespace: "default"}}
	content := `
apiVersion: v1
kind: Pod
metadata:
  name: oom-test
spec:
  containers:
  - name: stress
    image: alexeiled/stress-ng
`

	got, changed, err := normalizeRawPodPatchIdentity(pod, gitops.WorkloadSource{ManifestType: gitops.ManifestRaw}, content)
	if err != nil {
		t.Fatalf("normalize raw pod patch: %v", err)
	}
	if !changed {
		t.Fatal("expected pod identity to be normalized")
	}
	if !strings.Contains(got, "name: oom-test-2") {
		t.Fatalf("expected normalized pod name, got:\n%s", got)
	}
	if !strings.Contains(got, "image: alexeiled/stress-ng") {
		t.Fatalf("expected image fix to remain, got:\n%s", got)
	}
}

func TestNormalizeRawPodPatchIdentitySkipsKustomize(t *testing.T) {
	pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "oom-test-2", Namespace: "default"}}
	content := "apiVersion: v1\nkind: Pod\nmetadata:\n  name: oom-test\n"

	got, changed, err := normalizeRawPodPatchIdentity(pod, gitops.WorkloadSource{ManifestType: gitops.ManifestKustomize}, content)
	if err != nil {
		t.Fatalf("normalize kustomize patch: %v", err)
	}
	if changed {
		t.Fatalf("expected kustomize content not to be retargeted, got:\n%s", got)
	}
	if got != content {
		t.Fatalf("expected original content to be preserved, got:\n%s", got)
	}
}

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

func TestApplyRawManifestPatchPreservesMetadataAndOrder(t *testing.T) {
	pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "oom-test", Namespace: "default"}}
	pod.Spec.Containers = []v1.Container{{Name: "stress"}}
	original := `
apiVersion: v1
kind: Pod
metadata:
  name: oom-test
  annotations:
    fixora.io/repo-url: "https://github.com/baka126/fixora-demo.git"
    fixora.io/repo-path: "pod.yaml"
spec:
  restartPolicy: Never
  containers:
  - name: stress
    image: polinux/stress
    resources:
      limits:
        memory: "100Mi"
    args: ["stress", "--vm", "1", "--vm-bytes", "500M", "--vm-hang", "1"]
`
	generated := `
apiVersion: v1
kind: Pod
metadata:
  name: oom-test-2
spec:
  containers:
  - name: stress
    image: alexeiled/stress-ng
    args:
    - --vm
    - "1"
    - --vm-bytes
    - 500M
    - --vm-hang
    - "1"
`

	got, changed, err := applyRawManifestPatch(original, generated, gitops.WorkloadSource{ManifestType: gitops.ManifestRaw}, Diagnosis{PatchStrategy: PatchImage}, pod)
	if err != nil {
		t.Fatalf("apply raw manifest patch: %v", err)
	}
	if !changed {
		t.Fatal("expected image patch to be applied")
	}
	want := `
apiVersion: v1
kind: Pod
metadata:
  name: oom-test
  annotations:
    fixora.io/repo-url: "https://github.com/baka126/fixora-demo.git"
    fixora.io/repo-path: "pod.yaml"
spec:
  restartPolicy: Never
  containers:
  - name: stress
    image: alexeiled/stress-ng
    resources:
      limits:
        memory: "100Mi"
    args: ["stress", "--vm", "1", "--vm-bytes", "500M", "--vm-hang", "1"]
`
	if got != want {
		t.Fatalf("expected scalar-only image patch preserving formatting.\nwant:\n%s\ngot:\n%s", want, got)
	}
	if !strings.Contains(got, "name: oom-test\n") {
		t.Fatalf("expected original pod name to be preserved, got:\n%s", got)
	}
	if !strings.Contains(got, "image: alexeiled/stress-ng") {
		t.Fatalf("expected image fix to remain, got:\n%s", got)
	}
	if !strings.Contains(got, `args: ["stress", "--vm", "1", "--vm-bytes", "500M", "--vm-hang", "1"]`) {
		t.Fatalf("expected immutable pod args to be preserved for image patch, got:\n%s", got)
	}
	if !strings.Contains(got, "fixora.io/repo-url") {
		t.Fatalf("expected annotations to be preserved, got:\n%s", got)
	}
	restartPolicyIndex := strings.Index(got, "restartPolicy: Never")
	containersIndex := strings.Index(got, "containers:")
	if restartPolicyIndex < 0 || containersIndex < 0 || restartPolicyIndex > containersIndex {
		t.Fatalf("expected restartPolicy to remain before containers, got:\n%s", got)
	}
}

func TestApplyRawManifestPatchRejectsIdentityMismatch(t *testing.T) {
	pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "oom-test-5", Namespace: "default"}}
	pod.Spec.Containers = []v1.Container{{Name: "stress"}}
	original := `
apiVersion: v1
kind: Pod
metadata:
  name: oom-test-4
spec:
  containers:
  - name: stress
    image: polinux/stress
`
	generated := `
apiVersion: v1
kind: Pod
metadata:
  name: oom-test-4
spec:
  containers:
  - name: stress
    image: alexeiled/stress-ng
`

	_, _, err := applyRawManifestPatch(original, generated, gitops.WorkloadSource{ManifestType: gitops.ManifestRaw}, Diagnosis{PatchStrategy: PatchImage}, pod)
	if err == nil {
		t.Fatal("expected raw Pod manifest identity mismatch to be rejected")
	}
	if !strings.Contains(err.Error(), "identity mismatch") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestApplyRawManifestPatchSkipsKustomize(t *testing.T) {
	pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "oom-test-2", Namespace: "default"}}
	content := "apiVersion: v1\nkind: Pod\nmetadata:\n  name: oom-test\n"

	got, changed, err := applyRawManifestPatch(content, content, gitops.WorkloadSource{ManifestType: gitops.ManifestKustomize}, Diagnosis{PatchStrategy: PatchImage}, pod)
	if err != nil {
		t.Fatalf("apply kustomize patch: %v", err)
	}
	if changed {
		t.Fatalf("expected kustomize content not to be retargeted, got:\n%s", got)
	}
	if got != content {
		t.Fatalf("expected original content to be preserved, got:\n%s", got)
	}
}

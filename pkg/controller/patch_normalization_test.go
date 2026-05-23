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

	got, changed, err := applyRawManifestPatch(original, generated, gitops.WorkloadSource{ManifestType: gitops.ManifestRaw}, Diagnosis{PatchStrategy: PatchImage}, pod, workloadIdentity{Kind: "Pod", Name: "oom-test"}, "")
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

	_, _, err := applyRawManifestPatch(original, generated, gitops.WorkloadSource{ManifestType: gitops.ManifestRaw}, Diagnosis{PatchStrategy: PatchImage}, pod, workloadIdentity{Kind: "Pod", Name: "oom-test-5"}, "")
	if err == nil {
		t.Fatal("expected raw Pod manifest identity mismatch to be rejected")
	}
	if !strings.Contains(err.Error(), "identity mismatch") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestApplyRawManifestPatchSkipsKustomize(t *testing.T) {
	pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "oom-test-2", Namespace: "default"}}
	content := "apiVersion: v1\nkind: Pod\nmetadata:\n  name: oom-test-2\n"

	got, changed, err := applyRawManifestPatch(content, content, gitops.WorkloadSource{ManifestType: gitops.ManifestKustomize}, Diagnosis{PatchStrategy: PatchImage}, pod, workloadIdentity{Kind: "Pod", Name: "oom-test-2"}, "")
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

func TestApplyRawManifestPatchUpdatesDeploymentTemplate(t *testing.T) {
	pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "api-7d9f-abc", Namespace: "default"}}
	pod.Spec.Containers = []v1.Container{{Name: "api"}}
	original := `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: api
spec:
  template:
    spec:
      containers:
      - name: api
        image: ghcr.io/acme/api:v1
        args: ["serve"]
`
	generated := `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: api
spec:
  template:
    spec:
      containers:
      - name: api
        image: ghcr.io/acme/api:v2
        args: ["serve", "--log-level=info"]
`

	got, changed, err := applyRawManifestPatch(original, generated, gitops.WorkloadSource{ManifestType: gitops.ManifestRaw}, Diagnosis{PatchStrategy: PatchEnvOrVolumeRef}, pod, workloadIdentity{Kind: "Deployment", Name: "api"}, "")
	if err != nil {
		t.Fatalf("apply deployment patch: %v", err)
	}
	if !changed {
		t.Fatal("expected deployment template patch to be applied")
	}
	if !strings.Contains(got, `args: ["serve", "--log-level=info"]`) && !strings.Contains(got, `- --log-level=info`) {
		t.Fatalf("expected deployment pod template args to be updated, got:\n%s", got)
	}
}

func TestApplyRawManifestPatchUpdatesSecurityContextAndEmptyDir(t *testing.T) {
	pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "api-7d9f-abc", Namespace: "default"}}
	pod.Spec.Containers = []v1.Container{{Name: "api"}}
	original := `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: api
spec:
  template:
    spec:
      containers:
      - name: api
        image: ghcr.io/acme/api:v1
`
	generated := `
apiVersion: apps/v1
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
        image: ghcr.io/acme/api:v1
        securityContext:
          readOnlyRootFilesystem: true
          allowPrivilegeEscalation: false
        volumeMounts:
        - name: tmp
          mountPath: /tmp
`

	got, changed, err := applyRawManifestPatch(original, generated, gitops.WorkloadSource{ManifestType: gitops.ManifestRaw}, Diagnosis{PatchStrategy: PatchSecurityContext}, pod, workloadIdentity{Kind: "Deployment", Name: "api"}, "api")
	if err != nil {
		t.Fatalf("apply security patch: %v", err)
	}
	if !changed {
		t.Fatal("expected security patch to be applied")
	}
	for _, want := range []string{"emptyDir", "mountPath: /tmp", "readOnlyRootFilesystem: true", "allowPrivilegeEscalation: false"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in patch, got:\n%s", want, got)
		}
	}
}

func TestApplyRawManifestPatchRejectsBarePodForOwnedWorkload(t *testing.T) {
	pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "api-7d9f-abc", Namespace: "default"}}
	pod.Spec.Containers = []v1.Container{{Name: "api"}}
	original := `
apiVersion: v1
kind: Pod
metadata:
  name: api-7d9f-abc
spec:
  containers:
  - name: api
    image: ghcr.io/acme/api:v1
`
	generated := `
apiVersion: v1
kind: Pod
metadata:
  name: api-7d9f-abc
spec:
  containers:
  - name: api
    image: ghcr.io/acme/api:v2
`

	_, _, err := applyRawManifestPatch(original, generated, gitops.WorkloadSource{ManifestType: gitops.ManifestRaw}, Diagnosis{PatchStrategy: PatchImage}, pod, workloadIdentity{Kind: "Deployment", Name: "api"}, "")
	if err == nil {
		t.Fatal("expected bare Pod source to be rejected for controller-owned workload")
	}
	if !strings.Contains(err.Error(), "controller-owned workload") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestApplyRawManifestPatchTargetsSecondaryContainer(t *testing.T) {
	pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "api-7d9f-abc", Namespace: "default"}}
	pod.Spec.Containers = []v1.Container{{Name: "api"}, {Name: "sidecar"}}
	original := `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: api
spec:
  template:
    spec:
      containers:
      - name: api
        image: ghcr.io/acme/api:v1
      - name: sidecar
        image: ghcr.io/acme/sidecar:v1
`
	generated := `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: api
spec:
  template:
    spec:
      containers:
      - name: api
        image: ghcr.io/acme/api:v2
      - name: sidecar
        image: ghcr.io/acme/sidecar:v2
`

	got, changed, err := applyRawManifestPatch(original, generated, gitops.WorkloadSource{ManifestType: gitops.ManifestRaw}, Diagnosis{PatchStrategy: PatchImage}, pod, workloadIdentity{Kind: "Deployment", Name: "api"}, "sidecar")
	if err != nil {
		t.Fatalf("apply sidecar patch: %v", err)
	}
	if !changed {
		t.Fatal("expected sidecar image patch to be applied")
	}
	if !strings.Contains(got, "image: ghcr.io/acme/api:v1") {
		t.Fatalf("expected primary container image to remain unchanged, got:\n%s", got)
	}
	if !strings.Contains(got, "image: ghcr.io/acme/sidecar:v2") {
		t.Fatalf("expected sidecar image to be updated, got:\n%s", got)
	}
}

func TestApplyRawManifestPatchTargetsInitContainer(t *testing.T) {
	pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "api-7d9f-abc", Namespace: "default"}}
	pod.Spec.InitContainers = []v1.Container{{Name: "migrate"}}
	pod.Spec.Containers = []v1.Container{{Name: "api"}}
	original := `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: api
spec:
  template:
    spec:
      initContainers:
      - name: migrate
        image: ghcr.io/acme/migrate:v1
      containers:
      - name: api
        image: ghcr.io/acme/api:v1
`
	generated := `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: api
spec:
  template:
    spec:
      initContainers:
      - name: migrate
        image: ghcr.io/acme/migrate:v2
      containers:
      - name: api
        image: ghcr.io/acme/api:v2
`

	got, changed, err := applyRawManifestPatch(original, generated, gitops.WorkloadSource{ManifestType: gitops.ManifestRaw}, Diagnosis{PatchStrategy: PatchImage}, pod, workloadIdentity{Kind: "Deployment", Name: "api"}, "migrate")
	if err != nil {
		t.Fatalf("apply initContainer patch: %v", err)
	}
	if !changed {
		t.Fatal("expected initContainer patch to be applied")
	}
	if !strings.Contains(got, "image: ghcr.io/acme/migrate:v2") {
		t.Fatalf("expected initContainer image to be updated, got:\n%s", got)
	}
	if !strings.Contains(got, "image: ghcr.io/acme/api:v1") {
		t.Fatalf("expected app container image to remain unchanged, got:\n%s", got)
	}
}

func TestApplyRawManifestPatchRejectsKustomizePatchIdentityMismatch(t *testing.T) {
	pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "api-7d9f-abc", Namespace: "default"}}
	original := `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: worker
spec:
  template:
    spec:
      containers:
      - name: api
        image: ghcr.io/acme/api:v1
`

	_, _, err := applyRawManifestPatch(original, original, gitops.WorkloadSource{ManifestType: gitops.ManifestKustomize}, Diagnosis{PatchStrategy: PatchImage}, pod, workloadIdentity{Kind: "Deployment", Name: "api"}, "api")
	if err == nil {
		t.Fatal("expected Kustomize patch identity mismatch to be rejected")
	}
	if !strings.Contains(err.Error(), "identity mismatch") {
		t.Fatalf("unexpected error: %v", err)
	}
}

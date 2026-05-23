package controller

import (
	"testing"

	"fixora/pkg/gitops"
	"fixora/pkg/vcs"
)

func TestEnforcePatchGuardrailsRejectsRBAC(t *testing.T) {
	err := enforcePatchGuardrails(gitops.WorkloadSource{}, []vcs.FileChange{{
		FilePath: "deploy/role.yaml",
		NewContent: []byte(`
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: broad
`),
	}}, nil, nil, "default", nil, nil)
	if err == nil {
		t.Fatal("expected RBAC manifest to be rejected")
	}
}

func TestEnforcePatchGuardrailsRejectsSecret(t *testing.T) {
	err := enforcePatchGuardrails(gitops.WorkloadSource{}, []vcs.FileChange{{
		FilePath: "deploy/app.yaml",
		NewContent: []byte(`
apiVersion: v1
kind: Secret
metadata:
  name: app-secret
`),
	}}, nil, nil, "default", nil, nil)
	if err == nil {
		t.Fatal("expected Secret manifest to be rejected")
	}
}

func TestEnforcePatchGuardrailsRejectsPrivilegeEscalation(t *testing.T) {
	err := enforcePatchGuardrails(gitops.WorkloadSource{}, []vcs.FileChange{{
		FilePath: "deploy/app.yaml",
		NewContent: []byte(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: api
spec:
  template:
    spec:
      containers:
      - name: api
        image: ghcr.io/acme/api:1
        securityContext:
          privileged: true
`),
	}}, nil, nil, "default", nil, nil)
	if err == nil {
		t.Fatal("expected privileged securityContext to be rejected")
	}
}

func TestEnforcePatchGuardrailsRejectsUnapprovedImageRegistry(t *testing.T) {
	err := enforcePatchGuardrails(gitops.WorkloadSource{}, []vcs.FileChange{{
		FilePath: "deploy/app.yaml",
		NewContent: []byte(`
apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      containers:
      - name: app
        image: untrusted.example.com/app:1
`),
	}}, []string{"ghcr.io"}, nil, "default", nil, nil)
	if err == nil {
		t.Fatal("expected unapproved image registry to be rejected")
	}
}

func TestEnforcePatchGuardrailsAllowsApprovedImageRegistry(t *testing.T) {
	err := enforcePatchGuardrails(gitops.WorkloadSource{}, []vcs.FileChange{{
		FilePath: "deploy/app.yaml",
		NewContent: []byte(`
apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      containers:
      - name: app
        image: ghcr.io/acme/app:1
`),
	}}, []string{"ghcr.io"}, nil, "default", nil, nil)
	if err != nil {
		t.Fatalf("expected approved image registry to pass, got %v", err)
	}
}

func TestEnforcePatchGuardrailsRejectsCrossNamespace(t *testing.T) {
	err := enforcePatchGuardrails(gitops.WorkloadSource{}, []vcs.FileChange{{
		FilePath: "deploy/app.yaml",
		NewContent: []byte(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  namespace: kube-system
`),
	}}, nil, nil, "default", nil, []string{"kube-system"})
	if err == nil {
		t.Fatal("expected cross-namespace modification to be rejected")
	}
}

func TestEnforcePatchGuardrailsRejectsNonExcludedCrossNamespace(t *testing.T) {
	err := enforcePatchGuardrails(gitops.WorkloadSource{}, []vcs.FileChange{{
		FilePath: "deploy/app.yaml",
		NewContent: []byte(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  namespace: payments
`),
	}}, nil, nil, "orders", nil, nil)
	if err == nil {
		t.Fatal("expected non-excluded cross-namespace modification to be rejected")
	}
}

func TestEnforcePatchGuardrailsRejectsNestedKubernetesObjectInContainer(t *testing.T) {
	err := enforcePatchGuardrails(gitops.WorkloadSource{}, []vcs.FileChange{{
		FilePath: "pod.yaml",
		NewContent: []byte(`
apiVersion: v1
kind: Pod
metadata:
  name: oom-test-2
spec:
  containers:
  - name: stress
    image: polinux/stress
    apiVersion: v1
    kind: Pod
    metadata:
      name: oom-test-2
`),
	}}, nil, nil, "default", []manifestIdentity{{Namespace: "default", Kind: "Pod", Name: "oom-test-2"}}, nil)
	if err == nil {
		t.Fatal("expected nested Kubernetes object to be rejected")
	}
}

func TestEnforcePatchGuardrailsRejectsRawManifestTargetMismatch(t *testing.T) {
	err := enforcePatchGuardrails(gitops.WorkloadSource{}, []vcs.FileChange{{
		FilePath: "pod.yaml",
		NewContent: []byte(`
apiVersion: v1
kind: Pod
metadata:
  name: oom-test
spec:
  containers:
  - name: stress
    image: alexeiled/stress-ng:0.12.05
`),
	}}, nil, nil, "default", []manifestIdentity{{Namespace: "default", Kind: "Pod", Name: "oom-test-2"}}, nil)
	if err == nil {
		t.Fatal("expected target mismatch to be rejected")
	}
}

func TestEnforcePatchGuardrailsAllowsRawManifestTargetMatch(t *testing.T) {
	err := enforcePatchGuardrails(gitops.WorkloadSource{}, []vcs.FileChange{{
		FilePath: "pod.yaml",
		NewContent: []byte(`
apiVersion: v1
kind: Pod
metadata:
  name: oom-test-2
spec:
  containers:
  - name: stress
    image: alexeiled/stress-ng:0.12.05
`),
	}}, nil, nil, "default", []manifestIdentity{{Namespace: "default", Kind: "Pod", Name: "oom-test-2"}}, nil)
	if err != nil {
		t.Fatalf("expected matching target to pass, got %v", err)
	}
}

func TestEnforcePatchGuardrailsRejectsUnpinnedReplacementImage(t *testing.T) {
	previous := []byte(`
apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      containers:
      - name: app
        image: polinux/stress
`)
	err := enforcePatchGuardrails(gitops.WorkloadSource{}, []vcs.FileChange{{
		FilePath:        "deploy/app.yaml",
		PreviousContent: previous,
		NewContent: []byte(`
apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      containers:
      - name: app
        image: alexeiled/stress-ng
`),
	}}, nil, nil, "default", nil, nil)
	if err == nil {
		t.Fatal("expected unpinned replacement image to be rejected")
	}
}

func TestEnforcePatchGuardrailsRejectsUnpinnedHelmValuesImageTag(t *testing.T) {
	previous := []byte(`
image:
  repository: polinux/stress
  tag: "1.0.0"
`)
	err := enforcePatchGuardrails(gitops.WorkloadSource{ManifestType: gitops.ManifestHelm}, []vcs.FileChange{{
		FilePath:        "charts/stress/values.yaml",
		PreviousContent: previous,
		NewContent: []byte(`
image:
  repository: alexeiled/stress-ng
`),
	}}, nil, nil, "default", nil, nil)
	if err == nil {
		t.Fatal("expected unpinned Helm values replacement image to be rejected")
	}
}

func TestEnforcePatchGuardrailsAllowsPinnedHelmValuesImageTag(t *testing.T) {
	previous := []byte(`
image:
  repository: polinux/stress
  tag: "1.0.0"
`)
	err := enforcePatchGuardrails(gitops.WorkloadSource{ManifestType: gitops.ManifestHelm}, []vcs.FileChange{{
		FilePath:        "charts/stress/values.yaml",
		PreviousContent: previous,
		NewContent: []byte(`
image:
  repository: alexeiled/stress-ng
  tag: "0.12.05"
`),
	}}, nil, []string{"alexeiled/stress-ng:0.12.05"}, "default", nil, nil)
	if err != nil {
		t.Fatalf("expected pinned allowlisted Helm values image to pass, got %v", err)
	}
}

func TestEnforceArchitectureImageGuardrailCoversHelmValues(t *testing.T) {
	previous := []byte(`
image:
  repository: polinux/stress
  tag: "1.0.0"
`)
	err := enforceArchitectureImageGuardrail(Diagnosis{
		PatchStrategy: PatchImage,
		LikelyCause:   "exec format error from wrong CPU architecture",
	}, []vcs.FileChange{{
		FilePath:        "charts/stress/values.yaml",
		PreviousContent: previous,
		NewContent: []byte(`
image:
  repository: alexeiled/stress-ng
  tag: "0.12.05"
`),
	}}, nil)
	if err == nil {
		t.Fatal("expected architecture Helm values image replacement without allowlist to be rejected")
	}
}

func TestEnforcePatchGuardrailsRejectsLatestReplacementImage(t *testing.T) {
	previous := []byte(`
apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      containers:
      - name: app
        image: polinux/stress
`)
	err := enforcePatchGuardrails(gitops.WorkloadSource{}, []vcs.FileChange{{
		FilePath:        "deploy/app.yaml",
		PreviousContent: previous,
		NewContent: []byte(`
apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      containers:
      - name: app
        image: alexeiled/stress-ng:latest
`),
	}}, nil, nil, "default", nil, nil)
	if err == nil {
		t.Fatal("expected latest replacement image to be rejected")
	}
}

func TestEnforcePatchGuardrailsRejectsUnapprovedReplacementImage(t *testing.T) {
	err := enforcePatchGuardrails(gitops.WorkloadSource{}, []vcs.FileChange{{
		FilePath: "deploy/app.yaml",
		NewContent: []byte(`
apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      containers:
      - name: app
        image: ghcr.io/pao-pao/stress:1.0.0
`),
	}}, nil, []string{"alexeiled/stress-ng:0.12.05"}, "default", nil, nil)
	if err == nil {
		t.Fatal("expected unapproved replacement image to be rejected")
	}
}

func TestEnforcePatchGuardrailsAllowsApprovedReplacementImage(t *testing.T) {
	err := enforcePatchGuardrails(gitops.WorkloadSource{}, []vcs.FileChange{{
		FilePath: "deploy/app.yaml",
		NewContent: []byte(`
apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      containers:
      - name: app
        image: alexeiled/stress-ng:0.12.05
`),
	}}, nil, []string{"alexeiled/stress-ng:0.12.05"}, "default", nil, nil)
	if err != nil {
		t.Fatalf("expected approved replacement image to pass, got %v", err)
	}
}

func TestEnforcePatchGuardrailsAllowsExistingUnapprovedImage(t *testing.T) {
	previous := []byte(`
apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      containers:
      - name: app
        image: legacy.example.com/app:1
      - name: stress
        image: polinux/stress
`)
	err := enforcePatchGuardrails(gitops.WorkloadSource{}, []vcs.FileChange{{
		FilePath:        "deploy/app.yaml",
		PreviousContent: previous,
		NewContent: []byte(`
apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      containers:
      - name: app
        image: legacy.example.com/app:1
      - name: stress
        image: alexeiled/stress-ng:0.12.05
`),
	}}, nil, []string{"alexeiled/stress-ng:0.12.05"}, "default", nil, nil)
	if err != nil {
		t.Fatalf("expected only newly introduced image to need allowlisting, got %v", err)
	}
}

func TestEnforceArchitectureImageGuardrailRequiresVerifiedAllowlist(t *testing.T) {
	previous := []byte(`
apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      containers:
      - name: app
        image: polinux/stress
`)
	err := enforceArchitectureImageGuardrail(Diagnosis{
		PatchStrategy: PatchImage,
		LikelyCause:   "The container image is incompatible with the node CPU architecture and fails with exec format error.",
	}, []vcs.FileChange{{
		FilePath:        "deploy/app.yaml",
		PreviousContent: previous,
		NewContent: []byte(`
apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      containers:
      - name: app
        image: alexeiled/stress-ng:0.12.05
`),
	}}, nil)
	if err == nil {
		t.Fatal("expected architecture image replacement without allowlist to be rejected")
	}
}

func TestEnforceArchitectureImageGuardrailAllowsVerifiedAllowlist(t *testing.T) {
	previous := []byte(`
apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      containers:
      - name: app
        image: polinux/stress
`)
	err := enforceArchitectureImageGuardrail(Diagnosis{
		PatchStrategy: PatchImage,
		LikelyCause:   "The container image is incompatible with the node CPU architecture and fails with exec format error.",
	}, []vcs.FileChange{{
		FilePath:        "deploy/app.yaml",
		PreviousContent: previous,
		NewContent: []byte(`
apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      containers:
      - name: app
        image: alexeiled/stress-ng:0.12.05
`),
	}}, []string{"alexeiled/stress-ng:0.12.05"})
	if err != nil {
		t.Fatalf("expected allowlisted architecture replacement to pass, got %v", err)
	}
}

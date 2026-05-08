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
	}}, nil, "default", nil, nil)
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
	}}, nil, "default", nil, nil)
	if err == nil {
		t.Fatal("expected Secret manifest to be rejected")
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
	}}, []string{"ghcr.io"}, "default", nil, nil)
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
	}}, []string{"ghcr.io"}, "default", nil, nil)
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
	}}, nil, "default", nil, []string{"kube-system"})
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
	}}, nil, "orders", nil, nil)
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
	}}, nil, "default", []manifestIdentity{{Namespace: "default", Kind: "Pod", Name: "oom-test-2"}}, nil)
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
    image: alexeiled/stress-ng
`),
	}}, nil, "default", []manifestIdentity{{Namespace: "default", Kind: "Pod", Name: "oom-test-2"}}, nil)
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
    image: alexeiled/stress-ng
`),
	}}, nil, "default", []manifestIdentity{{Namespace: "default", Kind: "Pod", Name: "oom-test-2"}}, nil)
	if err != nil {
		t.Fatalf("expected matching target to pass, got %v", err)
	}
}

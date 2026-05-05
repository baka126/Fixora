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
	}}, nil, nil)
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
	}}, nil, nil)
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
	}}, []string{"ghcr.io"}, nil)
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
	}}, []string{"ghcr.io"}, nil)
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
	}}, nil, []string{"kube-system"})
	if err == nil {
		t.Fatal("expected cross-namespace modification to be rejected")
	}
}

package controller

import (
	"context"
	"strings"
	"testing"

	"fixora/pkg/vcs"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type remediationStatusProvider struct {
	branchStatus vcs.PullRequestStatus
	urlStatus    vcs.PullRequestStatus
}

func (p remediationStatusProvider) CreatePullRequest(context.Context, vcs.PullRequestOptions) (string, error) {
	return "", nil
}

func (p remediationStatusProvider) AppendCommit(context.Context, string, string, string, []vcs.FileChange, string) error {
	return nil
}

func (p remediationStatusProvider) GetFileContent(context.Context, string, string, string, string) ([]byte, error) {
	return nil, nil
}

func (p remediationStatusProvider) ListFiles(context.Context, string, string, string, string) (map[string][]byte, error) {
	return nil, nil
}

func (p remediationStatusProvider) PullRequestExists(context.Context, string, string, string) (bool, string, error) {
	return false, "", nil
}

func (p remediationStatusProvider) GetPullRequestStatus(context.Context, string, string, string) (vcs.PullRequestStatus, error) {
	return p.branchStatus, nil
}

func (p remediationStatusProvider) GetPullRequestStatusByURL(context.Context, string) (vcs.PullRequestStatus, error) {
	return p.urlStatus, nil
}

func TestRemediationPRStatusPrefersStoredURL(t *testing.T) {
	rec := RemediationRecord{
		PRURL: "https://github.com/acme/platform/pull/42",
		Options: vcs.PullRequestOptions{
			RepoOwner: "acme",
			RepoName:  "platform",
			Head:      "fixora/old-branch",
		},
	}
	provider := remediationStatusProvider{
		branchStatus: vcs.PullRequestStatus{State: "not_found"},
		urlStatus:    vcs.PullRequestStatus{State: "closed", Merged: true, URL: rec.PRURL},
	}

	got, err := remediationPRStatus(context.Background(), provider, rec)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !got.Merged || got.URL != rec.PRURL {
		t.Fatalf("expected merged status from PR URL, got %#v", got)
	}
}

func TestRemediationPRStatusFallsBackToBranch(t *testing.T) {
	rec := RemediationRecord{
		Options: vcs.PullRequestOptions{
			RepoOwner: "acme",
			RepoName:  "platform",
			Head:      "fixora/current-branch",
		},
	}
	provider := remediationStatusProvider{
		branchStatus: vcs.PullRequestStatus{State: "open", URL: "https://github.com/acme/platform/pull/43"},
	}

	got, err := remediationPRStatus(context.Background(), provider, rec)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.State != "open" || got.URL == "" {
		t.Fatalf("expected open branch status, got %#v", got)
	}
}

func TestRemediationContainerExpectationsFromDeploymentManifest(t *testing.T) {
	rec := RemediationRecord{
		Namespace:    "default",
		WorkloadKind: "Deployment",
		WorkloadName: "api",
		ChangedFiles: []remediationChangedFile{{
			FilePath: "deploy/api.yaml",
			ApplyHints: remediationApplyHintsFromContent([]byte(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: api
  namespace: default
spec:
  template:
    spec:
      containers:
      - name: api
        image: ghcr.io/acme/api:1.2.3
        env:
        - name: DB_HOST
          value: postgres.default.svc
        resources:
          requests:
            memory: 256Mi
          limits:
            memory: 512Mi
`)),
		}},
	}

	got := remediationContainerExpectations(rec)
	if len(got) != 1 {
		t.Fatalf("expected one container expectation, got %#v", got)
	}
	if got[0].Name != "api" || got[0].Image != "ghcr.io/acme/api:1.2.3" {
		t.Fatalf("unexpected image expectation: %#v", got[0])
	}
	if got[0].EnvHashes["DB_HOST"] != hashApplyValue("postgres.default.svc") || got[0].Requests["memory"] != "256Mi" || got[0].Limits["memory"] != "512Mi" {
		t.Fatalf("unexpected env/resource expectations: %#v", got[0])
	}
}

func TestRemediationContainerExpectationsFromHelmValues(t *testing.T) {
	rec := RemediationRecord{
		Namespace:    "default",
		WorkloadKind: "Deployment",
		WorkloadName: "api",
		ChangedFiles: []remediationChangedFile{{
			FilePath: "values.yaml",
			ApplyHints: remediationApplyHintsFromContent([]byte(`
image:
  repository: ghcr.io/acme/api
  tag: 1.2.3
`)),
		}},
	}

	got := remediationContainerExpectations(rec)
	if len(got) != 1 {
		t.Fatalf("expected one image expectation, got %#v", got)
	}
	if got[0].Name != "" || got[0].Image != "ghcr.io/acme/api:1.2.3" {
		t.Fatalf("unexpected Helm image expectation: %#v", got[0])
	}
	if !got[0].matchesAny(map[string]liveContainerState{"api": {Image: "ghcr.io/acme/api:1.2.3"}}) {
		t.Fatal("expected unnamed Helm image expectation to match live container image")
	}
}

func TestFluxObjectReadyReportsFailure(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{
					"type":    "Ready",
					"status":  "False",
					"reason":  "BuildFailed",
					"message": "kustomize build failed",
				},
			},
		},
	}}

	ready, failure := fluxObjectReady(obj, "payments")
	if !ready || !strings.Contains(failure, "BuildFailed") {
		t.Fatalf("expected ready-for-observation failure, got ready=%v failure=%q", ready, failure)
	}
}

func TestFluxObjectReadyWaitsWhenReadyUnknown(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{"type": "Ready", "status": "Unknown"},
			},
		},
	}}

	ready, failure := fluxObjectReady(obj, "payments")
	if ready || failure != "" {
		t.Fatalf("expected monitor to keep waiting, got ready=%v failure=%q", ready, failure)
	}
}

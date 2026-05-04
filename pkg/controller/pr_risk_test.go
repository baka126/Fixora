package controller

import (
	"testing"

	"fixora/pkg/gitops"
	"fixora/pkg/vcs"
)

func TestScorePRRiskRequiresApprovalForProdBaseImageChange(t *testing.T) {
	got := scorePRRisk(
		vcs.PullRequestOptions{Files: []vcs.FileChange{{FilePath: "base/deployment.yaml"}}},
		gitops.WorkloadSource{OverlayRole: gitops.OverlayBase, Environment: "prod", Path: "overlays/prod"},
		Diagnosis{PatchStrategy: PatchImage},
		90,
	)

	if !got.RequiresApproval {
		t.Fatalf("expected high-risk PR to require approval, got %#v", got)
	}
}

func TestScorePRRiskAllowsLowRiskOverlayResourceChange(t *testing.T) {
	got := scorePRRisk(
		vcs.PullRequestOptions{Files: []vcs.FileChange{{FilePath: "overlays/dev/values.yaml"}}},
		gitops.WorkloadSource{OverlayRole: gitops.OverlayEnv, Environment: "dev", Path: "overlays/dev"},
		Diagnosis{PatchStrategy: PatchResources},
		93,
	)

	if got.RequiresApproval {
		t.Fatalf("expected low-risk PR to avoid forced approval, got %#v", got)
	}
}

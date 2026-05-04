package controller

import (
	"fmt"
	"path"
	"strings"

	"fixora/pkg/gitops"
	"fixora/pkg/vcs"
)

type PRRisk struct {
	Score            int
	RequiresApproval bool
	Reasons          []string
}

func scorePRRisk(opts vcs.PullRequestOptions, source gitops.WorkloadSource, diagnosis Diagnosis, aiConfidence int) PRRisk {
	var risk PRRisk
	add := func(points int, reason string) {
		risk.Score += points
		risk.Reasons = append(risk.Reasons, reason)
	}

	if aiConfidence < 85 {
		add(20, fmt.Sprintf("AI confidence is %d%%", aiConfidence))
	}
	if source.OverlayRole == gitops.OverlayBase {
		add(25, "targets a base GitOps template")
	}
	if strings.EqualFold(source.Environment, "prod") || strings.EqualFold(source.Environment, "production") || strings.Contains(strings.ToLower(source.Path), "/prod") {
		add(25, "targets a production overlay")
	}
	switch diagnosis.PatchStrategy {
	case PatchImage:
		add(20, "changes image rollout behavior")
	case PatchSchedulingPolicy:
		add(15, "changes scheduling policy")
	case PatchEnvOrVolumeRef, PatchPVC:
		add(15, "changes runtime dependencies")
	}

	for _, file := range opts.Files {
		lower := strings.ToLower(file.FilePath)
		base := path.Base(lower)
		if file.Create || file.Delete {
			add(10, "creates or deletes files")
		}
		if strings.Contains(lower, ".github/workflows/") || strings.Contains(lower, ".gitlab-ci") {
			add(50, "touches CI workflow files")
		}
		if strings.Contains(lower, "clusterrole") || strings.Contains(lower, "rolebinding") || strings.Contains(lower, "/rbac") || strings.Contains(base, "rbac") {
			add(45, "touches RBAC manifests")
		}
		if strings.Contains(base, "secret") {
			add(40, "touches Secret manifests")
		}
		if strings.Contains(lower, "/base/") {
			add(20, "touches a base path")
		}
	}

	risk.RequiresApproval = risk.Score >= 60
	return risk
}

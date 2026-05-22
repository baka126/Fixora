package controller

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"fixora/pkg/gitops"
	"fixora/pkg/notifications"
	"fixora/pkg/vcs"
	v1 "k8s.io/api/core/v1"
)

var branchUnsafeChars = regexp.MustCompile(`[^a-z0-9._/-]+`)

func buildTargetedPROptions(
	pod *v1.Pod,
	evidence notifications.EvidenceChain,
	diagnosis Diagnosis,
	aiConfidence int,
	repoOwner string,
	repoName string,
	baseBranch string,
	changes []vcs.FileChange,
	branchSubject string,
	timestamp int64,
) []vcs.PullRequestOptions {
	if strings.TrimSpace(branchSubject) == "" {
		branchSubject = pod.Name
	}
	groups := splitChangesByFile(changes)
	opts := make([]vcs.PullRequestOptions, 0, len(groups))
	for _, group := range groups {
		scope := prScopeFromChangeGroup(diagnosis, group)
		branch := fmt.Sprintf("fixora/%s-%s-%s-%d", slugify(string(diagnosis.PatchStrategy)), slugify(branchSubject), slugify(scope.BranchPart), timestamp)
		opts = append(opts, vcs.PullRequestOptions{
			Title:         fmt.Sprintf("Fixora: %s for %s/%s", scope.TitleAction, pod.Namespace, pod.Name),
			Body:          targetedPRBody(evidence, diagnosis, aiConfidence, group),
			Head:          branch,
			Base:          baseBranch,
			RepoOwner:     repoOwner,
			RepoName:      repoName,
			Files:         group,
			CommitMessage: fmt.Sprintf("fix: %s for %s/%s", scope.CommitAction, pod.Namespace, pod.Name),
		})
	}
	return opts
}

func buildManifestAwarePROptions(
	pod *v1.Pod,
	evidence notifications.EvidenceChain,
	diagnosis Diagnosis,
	aiConfidence int,
	repoOwner string,
	repoName string,
	baseBranch string,
	source gitops.WorkloadSource,
	changes []vcs.FileChange,
	branchSubject string,
	timestamp int64,
) []vcs.PullRequestOptions {
	if source.ManifestType != gitops.ManifestKustomize {
		return buildTargetedPROptions(pod, evidence, diagnosis, aiConfidence, repoOwner, repoName, baseBranch, changes, branchSubject, timestamp)
	}
	if strings.TrimSpace(branchSubject) == "" {
		branchSubject = pod.Name
	}

	sorted := append([]vcs.FileChange(nil), changes...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].FilePath < sorted[j].FilePath
	})
	scope := prScopeFromChangeGroup(diagnosis, sorted)
	branch := fmt.Sprintf("fixora/%s-%s-kustomize-%s-%d", slugify(string(diagnosis.PatchStrategy)), slugify(branchSubject), slugify(scope.BranchPart), timestamp)
	return []vcs.PullRequestOptions{{
		Title:         fmt.Sprintf("Fixora: update Kustomize overlay for %s/%s", pod.Namespace, pod.Name),
		Body:          targetedPRBody(evidence, diagnosis, aiConfidence, sorted),
		Head:          branch,
		Base:          baseBranch,
		RepoOwner:     repoOwner,
		RepoName:      repoName,
		Files:         sorted,
		CommitMessage: fmt.Sprintf("fix: update Kustomize overlay for %s/%s", pod.Namespace, pod.Name),
	}}
}

func splitChangesByFile(changes []vcs.FileChange) [][]vcs.FileChange {
	sorted := append([]vcs.FileChange(nil), changes...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].FilePath < sorted[j].FilePath
	})

	groups := make([][]vcs.FileChange, 0, len(sorted))
	for _, change := range sorted {
		groups = append(groups, []vcs.FileChange{change})
	}
	return groups
}

type prScope struct {
	TitleAction  string
	CommitAction string
	BranchPart   string
}

func prScopeFromChangeGroup(diagnosis Diagnosis, changes []vcs.FileChange) prScope {
	action := patchStrategyAction(diagnosis.PatchStrategy)
	filePart := "manifest"
	if len(changes) > 0 {
		filePart = filepath.Base(changes[0].FilePath)
		filePart = strings.TrimSuffix(strings.TrimSuffix(filePart, ".yaml"), ".yml")
	}
	return prScope{
		TitleAction:  action.title,
		CommitAction: action.commit,
		BranchPart:   filePart,
	}
}

type strategyAction struct {
	title  string
	commit string
}

func patchStrategyAction(strategy PatchStrategy) strategyAction {
	switch strategy {
	case PatchResources:
		return strategyAction{title: "adjust resources", commit: "adjust Kubernetes resources"}
	case PatchImage:
		return strategyAction{title: "correct image reference", commit: "correct image reference"}
	case PatchEnvOrVolumeRef:
		return strategyAction{title: "fix config references", commit: "fix config references"}
	case PatchSchedulingPolicy:
		return strategyAction{title: "fix scheduling policy", commit: "fix scheduling policy"}
	case PatchProbe:
		return strategyAction{title: "fix health probe", commit: "fix health probe"}
	case PatchServiceSelector:
		return strategyAction{title: "fix service routing", commit: "fix service routing"}
	case PatchPVC:
		return strategyAction{title: "fix volume dependency", commit: "fix volume dependency"}
	default:
		return strategyAction{title: "apply targeted remediation", commit: "apply targeted remediation"}
	}
}

func targetedPRBody(evidence notifications.EvidenceChain, diagnosis Diagnosis, aiConfidence int, changes []vcs.FileChange) string {
	return fmt.Sprintf(`### Targeted Fix

* **Symptom:** %s
* **Category:** %s
* **Patch Strategy:** %s
* **Deterministic Confidence:** %d%%
* **AI Confidence:** %d%%

### 🔍 Glass Box: The Evidence Chain
<details>
<summary>Click to view the mathematical and diagnostic evidence used to generate this fix</summary>

**Root Cause:**
%s

**Metric Proof:**
%s

**Event Timeline:**
%s

**Historical Pattern:**
%s

**Cluster Context:**
%s

**Validated Claims:**
%s

**Claims Requiring Review:**
%s
</details>

### Files Changed

%s

Generated by Fixora.`,
		diagnosis.Symptom,
		diagnosis.Category,
		diagnosis.PatchStrategy,
		diagnosis.Confidence,
		aiConfidence,
		firstNonEmpty(evidence.RootCause, diagnosis.LikelyCause),
		firstNonEmpty(evidence.MetricProof, "No metric proof available."),
		firstNonEmpty(evidence.EventTimeline, "No relevant events."),
		firstNonEmpty(evidence.HistoricalPattern, "No historical pattern found."),
		firstNonEmpty(evidence.ClusterContext, "No cluster context available."),
		formatClaimList(evidence.ValidatedClaims, "No deterministic claims recorded."),
		formatClaimList(evidence.UnvalidatedClaims, "No unvalidated claims recorded."),
		formatChangedFiles(changes),
	)
}

func targetedPRPreview(opts vcs.PullRequestOptions, diagnosis Diagnosis) string {
	return fmt.Sprintf("Targeted %s patch\nTitle: %s\nFiles:\n%s", diagnosis.PatchStrategy, opts.Title, formatChangedFiles(opts.Files))
}

func formatChangedFiles(changes []vcs.FileChange) string {
	if len(changes) == 0 {
		return "- No files"
	}
	lines := make([]string, 0, len(changes))
	for _, change := range changes {
		lines = append(lines, "- "+change.FilePath)
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

func formatClaimList(claims []string, fallback string) string {
	claims = nonEmptyDashboard(claims...)
	if len(claims) == 0 {
		return fallback
	}
	lines := make([]string, 0, len(claims))
	for _, claim := range claims {
		lines = append(lines, "- "+claim)
	}
	return strings.Join(lines, "\n")
}

func slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, ":", "-")
	value = strings.ReplaceAll(value, " ", "-")
	value = branchUnsafeChars.ReplaceAllString(value, "-")
	value = strings.Trim(value, "-._/")
	if value == "" {
		return "patch"
	}
	if len(value) > 48 {
		return value[:48]
	}
	return value
}

func remediationBranchPrefix(head string) string {
	head = strings.TrimSpace(head)
	idx := strings.LastIndex(head, "-")
	if idx < 0 || idx == len(head)-1 {
		return head
	}
	for _, r := range head[idx+1:] {
		if r < '0' || r > '9' {
			return head
		}
	}
	return head[:idx+1]
}

func remediationBranchSubject(pod *v1.Pod, identity workloadIdentity) string {
	if identity.Kind != "" && identity.Name != "" {
		return identity.Kind + "-" + identity.Name
	}
	if pod == nil {
		return "workload"
	}
	return pod.Name
}

func remediationSourceBranchSubject(subject string, source gitops.WorkloadSource) string {
	parts := []string{subject}
	for _, item := range []string{source.Environment, source.Region, source.AppName, source.Path} {
		item = strings.TrimSpace(item)
		if item != "" {
			parts = append(parts, item)
		}
	}
	return strings.Join(parts, "-")
}

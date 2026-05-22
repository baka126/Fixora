package controller

import (
	"context"
	"strings"
	"testing"

	"fixora/pkg/gitops"
	"fixora/pkg/notifications"
	"fixora/pkg/vcs"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type duplicateCheckProvider struct {
	existing map[string]string
	files    map[string][]byte
	checked  []string
}

func (p *duplicateCheckProvider) CreatePullRequest(ctx context.Context, opts vcs.PullRequestOptions) (string, error) {
	return "", nil
}

func (p *duplicateCheckProvider) AppendCommit(ctx context.Context, repoOwner, repoName, branch string, files []vcs.FileChange, message string) error {
	return nil
}

func (p *duplicateCheckProvider) GetFileContent(ctx context.Context, repoOwner, repoName, path, ref string) ([]byte, error) {
	if p.files != nil {
		return p.files[path], nil
	}
	return nil, nil
}

func (p *duplicateCheckProvider) ListFiles(ctx context.Context, repoOwner, repoName, path, ref string) (map[string][]byte, error) {
	return nil, nil
}

func (p *duplicateCheckProvider) PullRequestExists(ctx context.Context, repoOwner, repoName, headBranch string) (bool, string, error) {
	p.checked = append(p.checked, headBranch)
	if p.existing == nil {
		return false, "", nil
	}
	url, ok := p.existing[headBranch]
	return ok, url, nil
}

func (p *duplicateCheckProvider) GetPullRequestStatus(ctx context.Context, repoOwner, repoName, headBranch string) (vcs.PullRequestStatus, error) {
	return vcs.PullRequestStatus{}, nil
}

func TestBuildTargetedPROptionsSplitsByFile(t *testing.T) {
	pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "api-7d9f", Namespace: "payments"}}
	evidence := notifications.EvidenceChain{
		RootCause:   "Container memory limit is too low.",
		MetricProof: "Memory usage exceeded 95% of limit.",
	}
	diagnosis := Diagnosis{
		Symptom:       "Container was killed by the kernel OOM killer",
		Category:      CategoryRuntime,
		LikelyCause:   "The workload exceeded its memory limit.",
		Confidence:    90,
		PatchStrategy: PatchResources,
	}
	changes := []vcs.FileChange{
		{FilePath: "deploy/api.yaml", NewContent: []byte("kind: Deployment")},
		{FilePath: "deploy/worker.yaml", NewContent: []byte("kind: Deployment")},
	}

	got := buildTargetedPROptions(pod, evidence, diagnosis, 91, "acme", "platform", "main", changes, "Deployment-api", 123)

	if len(got) != 2 {
		t.Fatalf("expected 2 targeted PRs, got %d", len(got))
	}
	for _, opt := range got {
		if len(opt.Files) != 1 {
			t.Fatalf("expected one file per targeted PR, got %d", len(opt.Files))
		}
		if !strings.Contains(opt.Title, "adjust resources") {
			t.Fatalf("expected resource-specific title, got %q", opt.Title)
		}
		if !strings.Contains(opt.CommitMessage, "adjust Kubernetes resources") {
			t.Fatalf("expected resource-specific commit, got %q", opt.CommitMessage)
		}
		if !strings.HasPrefix(opt.Head, "fixora/resources-deployment-api-") {
			t.Fatalf("expected targeted branch prefix, got %q", opt.Head)
		}
	}
	if got[0].Head == got[1].Head {
		t.Fatalf("expected unique branch names, got %q", got[0].Head)
	}
}

func TestBuildManifestAwarePROptionsKeepsKustomizeOverlayTogether(t *testing.T) {
	pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "api-7d9f", Namespace: "payments"}}
	evidence := notifications.EvidenceChain{RootCause: "Probe path is wrong."}
	diagnosis := Diagnosis{
		Symptom:       "Health probe is failing",
		Category:      CategoryRuntime,
		LikelyCause:   "Probe does not match app behavior.",
		Confidence:    78,
		PatchStrategy: PatchProbe,
	}
	changes := []vcs.FileChange{
		{FilePath: "overlays/prod/kustomization.yaml", NewContent: []byte("patches: []")},
		{FilePath: "overlays/prod/fixora-patches/api-probe.yaml", NewContent: []byte("kind: Deployment"), Create: true},
	}

	got := buildManifestAwarePROptions(
		pod, evidence, diagnosis, 90, "acme", "platform", "main",
		gitops.WorkloadSource{ManifestType: gitops.ManifestKustomize},
		changes, "Deployment-api", 123,
	)

	if len(got) != 1 {
		t.Fatalf("expected one Kustomize PR, got %d", len(got))
	}
	if len(got[0].Files) != 2 {
		t.Fatalf("expected patch and kustomization in one PR, got %d files", len(got[0].Files))
	}
	if !strings.Contains(got[0].Title, "Kustomize overlay") {
		t.Fatalf("expected Kustomize-specific title, got %q", got[0].Title)
	}
}

func TestRemediationSourceBranchSubjectIncludesOverlayHints(t *testing.T) {
	got := slugify(remediationSourceBranchSubject("Deployment-api", gitops.WorkloadSource{
		Environment: "prod",
		Region:      "us-east-1",
		AppName:     "checkout",
		Path:        "overlays/prod/us-east-1",
	}))

	for _, want := range []string{"deployment-api", "prod", "us-east-1"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected branch subject %q to contain %q", got, want)
		}
	}
}

func TestFilterActiveRemediationPlansSkipsOpenPRByBranchPrefix(t *testing.T) {
	pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "oom-test", Namespace: "default"}}
	plan := remediationPROption{Options: vcs.PullRequestOptions{
		RepoOwner: "baka126",
		RepoName:  "fixora-demo",
		Head:      "fixora/image-oom-test-pod-1715520000",
	}}
	provider := &duplicateCheckProvider{
		existing: map[string]string{
			"fixora/image-oom-test-pod-": "https://github.com/baka126/fixora-demo/pull/9",
		},
	}

	got := (&Controller{}).filterActiveRemediationPlans(context.Background(), provider, pod, workloadIdentity{Kind: "Pod", Name: "oom-test"}, []remediationPROption{plan})

	if len(got) != 0 {
		t.Fatalf("expected duplicate active remediation to be skipped, got %d plans", len(got))
	}
	if len(provider.checked) != 1 || provider.checked[0] != "fixora/image-oom-test-pod-" {
		t.Fatalf("expected provider check with stable branch prefix, got %#v", provider.checked)
	}
}

func TestFilterActiveRemediationPlansAllowsNewPrefix(t *testing.T) {
	pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "oom-test", Namespace: "default"}}
	plan := remediationPROption{Options: vcs.PullRequestOptions{
		RepoOwner: "baka126",
		RepoName:  "fixora-demo",
		Head:      "fixora/image-oom-test-pod-1715520000",
	}}
	provider := &duplicateCheckProvider{}

	got := (&Controller{}).filterActiveRemediationPlans(context.Background(), provider, pod, workloadIdentity{Kind: "Pod", Name: "oom-test"}, []remediationPROption{plan})

	if len(got) != 1 {
		t.Fatalf("expected new remediation plan to be allowed, got %d", len(got))
	}
}

func TestFilterActiveRemediationPlansSkipsInMemoryPendingApproval(t *testing.T) {
	pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "oom-test", Namespace: "default"}}
	plan := remediationPROption{Options: vcs.PullRequestOptions{
		RepoOwner: "baka126",
		RepoName:  "fixora-demo",
		Head:      "fixora/image-pod-oom-test-pod-1715520000",
	}}
	ctrl := &Controller{
		pendingFixes: map[string]PendingFix{
			"fix-1": {
				PodNamespace: "default",
				Options: vcs.PullRequestOptions{
					RepoOwner: "baka126",
					RepoName:  "fixora-demo",
					Head:      "fixora/image-pod-oom-test-pod-1715510000",
				},
			},
		},
	}

	got := ctrl.filterActiveRemediationPlans(context.Background(), &duplicateCheckProvider{}, pod, workloadIdentity{Kind: "Pod", Name: "oom-test"}, []remediationPROption{plan})

	if len(got) != 0 {
		t.Fatalf("expected in-memory pending approval to suppress duplicate plan, got %d", len(got))
	}
}

func TestFilterNoopFileChangesDropsUnchangedContent(t *testing.T) {
	got := filterNoopFileChanges([]vcs.FileChange{
		{FilePath: "pod.yaml", PreviousContent: []byte("image: app:v1\n"), NewContent: []byte("image: app:v1\n")},
		{FilePath: "other.yaml", PreviousContent: []byte("image: app:v1\n"), NewContent: []byte("image: app:v2\n")},
	})

	if len(got) != 1 || got[0].FilePath != "other.yaml" {
		t.Fatalf("expected only changed file to remain, got %#v", got)
	}
}

func TestValidatePROptionsFreshRejectsChangedSource(t *testing.T) {
	err := validatePROptionsFresh(context.Background(), &duplicateCheckProvider{
		files: map[string][]byte{"pod.yaml": []byte("image: app:v2\n")},
	}, vcs.PullRequestOptions{
		RepoOwner: "baka126",
		RepoName:  "fixora-demo",
		Base:      "main",
		Files: []vcs.FileChange{{
			FilePath:        "pod.yaml",
			PreviousContent: []byte("image: app:v1\n"),
			NewContent:      []byte("image: app:v3\n"),
		}},
	})

	if err == nil {
		t.Fatal("expected stale source content to be rejected")
	}
}

func TestTargetedPRBodyIncludesDiagnosisAndFiles(t *testing.T) {
	body := targetedPRBody(
		notifications.EvidenceChain{
			RootCause:         "Probe path is wrong.",
			MetricProof:       "No metrics.",
			ValidatedClaims:   []string{"Kubernetes events were collected."},
			UnvalidatedClaims: []string{"Patch strategy needs review."},
		},
		Diagnosis{
			Symptom:       "Health probe is failing",
			Category:      CategoryRuntime,
			LikelyCause:   "Probe does not match app behavior.",
			Confidence:    78,
			PatchStrategy: PatchProbe,
		},
		86,
		[]vcs.FileChange{{FilePath: "charts/api/values.yaml"}},
	)

	for _, want := range []string{"Patch Strategy", "Health probe", "AI Confidence", "charts/api/values.yaml", "Validated Claims", "Kubernetes events were collected.", "Claims Requiring Review"} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected body to contain %q, got:\n%s", want, body)
		}
	}
}

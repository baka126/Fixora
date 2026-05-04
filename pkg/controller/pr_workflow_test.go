package controller

import (
	"strings"
	"testing"

	"fixora/pkg/gitops"
	"fixora/pkg/notifications"
	"fixora/pkg/vcs"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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

	got := buildTargetedPROptions(pod, evidence, diagnosis, 91, "acme", "platform", "main", changes, 123)

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
		if !strings.HasPrefix(opt.Head, "fixora/resources-api-7d9f-") {
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
		changes, 123,
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

func TestTargetedPRBodyIncludesDiagnosisAndFiles(t *testing.T) {
	body := targetedPRBody(
		notifications.EvidenceChain{RootCause: "Probe path is wrong.", MetricProof: "No metrics."},
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

	for _, want := range []string{"Patch Strategy", "Health probe", "AI Confidence", "charts/api/values.yaml"} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected body to contain %q, got:\n%s", want, body)
		}
	}
}

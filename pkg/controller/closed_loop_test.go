package controller

import (
	"strings"
	"testing"
	"time"

	"fixora/pkg/vcs"
)

func TestRemediationChangedFilesSorted(t *testing.T) {
	got := remediationChangedFiles([]vcs.FileChange{
		{FilePath: "overlays/prod/kustomization.yaml", PreviousContent: []byte("resources: []")},
		{FilePath: "overlays/prod/fixora-patches/api.yaml", Create: true},
	})

	if got[0].FilePath != "overlays/prod/fixora-patches/api.yaml" || !got[0].Create {
		t.Fatalf("expected created patch file first, got %#v", got)
	}
	if got[1].FilePath != "overlays/prod/kustomization.yaml" {
		t.Fatalf("expected sorted kustomization path second, got %#v", got)
	}
	if !got[1].HasPrevious || string(got[1].PreviousContent) != "resources: []" {
		t.Fatalf("expected previous content to be preserved, got %#v", got[1])
	}
}

func TestFormatRemediationFeedback(t *testing.T) {
	raw := `[{"file_path":"deploy/api.yaml","create":false}]`
	got := formatRemediationFeedback([]remediationFeedbackRow{{
		PatchStrategy:    string(PatchResources),
		Status:           string(RemediationProductionFailed),
		RepoOwner:        "acme",
		RepoName:         "platform",
		HeadBranch:       "fixora/resources-api",
		PRURL:            "https://github.com/acme/platform/pull/42",
		FailureReason:    "CrashLoopBackOff after sync",
		ChangedFilesJSON: raw,
		UpdatedAt:        time.Now(),
	}})

	for _, want := range []string{
		"Previous Fixora remediation attempts failed",
		string(PatchResources),
		"https://github.com/acme/platform/pull/42",
		"deploy/api.yaml",
		"CrashLoopBackOff after sync",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected feedback to contain %q, got %q", want, got)
		}
	}
}

func TestBuildRevertFileChanges(t *testing.T) {
	got, err := buildRevertFileChanges([]remediationChangedFile{
		{FilePath: "overlays/prod/kustomization.yaml", PreviousContent: []byte("resources: []"), HasPrevious: true},
		{FilePath: "overlays/prod/fixora-patches/api.yaml", Create: true},
	})
	if err != nil {
		t.Fatalf("expected revert changes, got error: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 revert changes, got %d", len(got))
	}
	if string(got[0].NewContent) != "resources: []" {
		t.Fatalf("expected first change to restore previous content, got %#v", got[0])
	}
	if !got[1].Delete {
		t.Fatalf("expected second change to delete generated file, got %#v", got[1])
	}
}

func TestBuildRevertFileChangesRejectsMissingPreviousContent(t *testing.T) {
	_, err := buildRevertFileChanges([]remediationChangedFile{
		{FilePath: "deploy/api.yaml"},
	})
	if err == nil {
		t.Fatal("expected missing previous content to be rejected")
	}
}

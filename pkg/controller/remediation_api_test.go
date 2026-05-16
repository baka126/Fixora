package controller

import (
	"encoding/json"
	"testing"
)

func TestRemediationChangedFilePathsParsesStoredObjects(t *testing.T) {
	raw, err := json.Marshal([]remediationChangedFile{
		{FilePath: "overlays/prod/fixora-patches/api.yaml", Create: true},
		{FilePath: "../escape.yaml"},
		{FilePath: "charts/api/values.yaml"},
	})
	if err != nil {
		t.Fatal(err)
	}

	got := remediationChangedFilePaths(raw)
	want := []string{"charts/api/values.yaml", "overlays/prod/fixora-patches/api.yaml"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestRemediationEditableFileForPathRequiresAllowlistMatch(t *testing.T) {
	files := []remediationEditableFile{
		{FilePath: "charts/api/values.yaml"},
	}

	if _, ok := remediationEditableFileForPath("charts/api/values.yaml", files); !ok {
		t.Fatal("expected allowlisted file to be editable")
	}
	if _, ok := remediationEditableFileForPath(".github/workflows/deploy.yaml", files); ok {
		t.Fatal("unexpectedly allowed file outside remediation changed files")
	}
	if _, ok := remediationEditableFileForPath("../charts/api/values.yaml", files); ok {
		t.Fatal("unexpectedly allowed path traversal")
	}
}

package controller

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"

	"fixora/pkg/vcs"
)

type FileDiff struct {
	FilePath string `json:"filePath"`
	Original string `json:"original"`
	Patched  string `json:"patched"`
}

type remediationEditableFile struct {
	FilePath     string `json:"file_path"`
	FilePathAlt  string `json:"filePath"`
	NewFilePath  string `json:"new_file_path"`
	OldFilePath  string `json:"old_file_path"`
	PreviousPath string `json:"previous_path"`
	Create       bool   `json:"create"`
}

func (c *Controller) GetRemediationDiff(ctx context.Context, id int64) ([]FileDiff, error) {
	if c.history == nil || c.history.db == nil {
		return nil, fmt.Errorf("database not configured")
	}

	query := `
		SELECT COALESCE(vcs_type, ''), COALESCE(repo_owner, ''), COALESCE(repo_name, ''),
		       COALESCE(base_branch, ''), COALESCE(head_branch, ''), COALESCE(changed_files, '[]'::jsonb)
		FROM remediation_outcomes
		WHERE id = $1
	`
	var vcsType, repoOwner, repoName, baseBranch, headBranch string
	var changedFilesJSON []byte
	err := c.history.db.QueryRowContext(ctx, query, id).Scan(
		&vcsType, &repoOwner, &repoName, &baseBranch, &headBranch, &changedFilesJSON,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("remediation not found")
	}
	if err != nil {
		return nil, err
	}

	changedFiles := remediationChangedFilePaths(changedFilesJSON)
	if len(changedFiles) == 0 {
		return nil, fmt.Errorf("remediation has no editable changed files")
	}

	var provider vcs.Provider
	if vcsType == "github" && c.ghProvider != nil {
		provider = c.ghProvider
	} else if vcsType == "gitlab" && c.glProvider != nil {
		provider = c.glProvider
	} else {
		return nil, fmt.Errorf("vcs provider not configured or unsupported type: %s", vcsType)
	}

	var diffs []FileDiff
	for _, file := range changedFiles {
		originalBytes, err := provider.GetFileContent(ctx, repoOwner, repoName, file, baseBranch)
		if err != nil {
			originalBytes = []byte{} // File might be new
		}
		patchedBytes, err := provider.GetFileContent(ctx, repoOwner, repoName, file, headBranch)
		if err != nil {
			patchedBytes = []byte{}
		}

		diffs = append(diffs, FileDiff{
			FilePath: file,
			Original: string(originalBytes),
			Patched:  string(patchedBytes),
		})
	}

	return diffs, nil
}

func (c *Controller) AppendCommitToRemediation(ctx context.Context, id int64, filePath, content, message string) error {
	if c.history == nil || c.history.db == nil {
		return fmt.Errorf("database not configured")
	}

	query := `
		SELECT COALESCE(vcs_type, ''), COALESCE(repo_owner, ''), COALESCE(repo_name, ''),
		       COALESCE(head_branch, ''), COALESCE(changed_files, '[]'::jsonb), COALESCE(status, '')
		FROM remediation_outcomes
		WHERE id = $1
	`
	var vcsType, repoOwner, repoName, headBranch, status string
	var changedFilesJSON []byte
	err := c.history.db.QueryRowContext(ctx, query, id).Scan(
		&vcsType, &repoOwner, &repoName, &headBranch, &changedFilesJSON, &status,
	)
	if err == sql.ErrNoRows {
		return fmt.Errorf("remediation not found")
	}
	if err != nil {
		return err
	}
	if headBranch == "" {
		return fmt.Errorf("remediation has no editable PR branch")
	}
	if remediationStatusClosed(status) {
		return fmt.Errorf("remediation is not editable in status %q", status)
	}
	allowedFiles := remediationEditableFiles(changedFilesJSON)
	editableFile, ok := remediationEditableFileForPath(filePath, allowedFiles)
	if !ok {
		return fmt.Errorf("file path is not part of the remediation changed files")
	}

	var provider vcs.Provider
	if vcsType == "github" && c.ghProvider != nil {
		provider = c.ghProvider
	} else if vcsType == "gitlab" && c.glProvider != nil {
		provider = c.glProvider
	} else {
		return fmt.Errorf("vcs provider not configured or unsupported type: %s", vcsType)
	}

	files := []vcs.FileChange{
		{
			FilePath:   filePath,
			NewContent: []byte(content),
			Create:     editableFile.Create,
		},
	}

	return provider.AppendCommit(ctx, repoOwner, repoName, headBranch, files, message)
}

func remediationChangedFilePaths(raw []byte) []string {
	return remediationChangedFilePathsFromChanges(remediationEditableFiles(raw))
}

func remediationEditableFiles(raw []byte) []remediationEditableFile {
	var direct []string
	if err := json.Unmarshal(raw, &direct); err == nil {
		files := make([]remediationEditableFile, 0, len(direct))
		for _, filePath := range direct {
			files = append(files, remediationEditableFile{FilePath: filePath})
		}
		return cleanRemediationFiles(files)
	}

	var structured []remediationEditableFile
	if err := json.Unmarshal(raw, &structured); err != nil {
		return nil
	}
	return cleanRemediationFiles(structured)
}

func cleanRemediationFiles(files []remediationEditableFile) []remediationEditableFile {
	seen := make(map[string]bool)
	out := make([]remediationEditableFile, 0, len(files))
	for _, file := range files {
		cleaned := strings.TrimSpace(firstNonEmpty(file.FilePath, file.FilePathAlt, file.NewFilePath, file.OldFilePath, file.PreviousPath))
		if !isSafeRepositoryPath(cleaned) || seen[cleaned] {
			continue
		}
		seen[cleaned] = true
		file.FilePath = cleaned
		out = append(out, file)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].FilePath < out[j].FilePath
	})
	return out
}

func remediationChangedFilePathsFromChanges(files []remediationEditableFile) []string {
	paths := make([]string, 0, len(files))
	for _, file := range files {
		paths = append(paths, file.FilePath)
	}
	sort.Strings(paths)
	return paths
}

func remediationEditableFileForPath(filePath string, allowed []remediationEditableFile) (remediationEditableFile, bool) {
	cleaned := strings.TrimSpace(filePath)
	if !isSafeRepositoryPath(cleaned) {
		return remediationEditableFile{}, false
	}
	for _, allowedFile := range allowed {
		if cleaned == allowedFile.FilePath {
			return allowedFile, true
		}
	}
	return remediationEditableFile{}, false
}

func isSafeRepositoryPath(filePath string) bool {
	if filePath == "" || strings.HasPrefix(filePath, "/") || strings.Contains(filePath, "\x00") {
		return false
	}
	cleaned := path.Clean(filePath)
	if cleaned == "." || cleaned != filePath || strings.HasPrefix(cleaned, "../") || cleaned == ".." {
		return false
	}
	for _, part := range strings.Split(cleaned, "/") {
		if part == ".git" || part == "" {
			return false
		}
	}
	return true
}

func remediationStatusClosed(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "merged", "closed", "succeeded", "reverted", "production_failed", "revert_failed":
		return true
	default:
		return false
	}
}

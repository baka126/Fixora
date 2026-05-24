package controller

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"

	"fixora/pkg/vcs"
)

type RemediationActionResult struct {
	ID          int64  `json:"id"`
	Status      string `json:"status"`
	Message     string `json:"message"`
	PRURL       string `json:"prUrl,omitempty"`
	RevertPRURL string `json:"revertPrUrl,omitempty"`
}

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

func (c *Controller) MarkRemediationApplied(ctx context.Context, id int64) (RemediationActionResult, error) {
	rec, err := c.remediationRecordForAction(ctx, id)
	if err != nil {
		return RemediationActionResult{}, err
	}
	switch rec.Status {
	case RemediationAwaitingApply, RemediationPROpened, RemediationObserving:
	default:
		return RemediationActionResult{}, fmt.Errorf("remediation cannot be marked applied from status %q", rec.Status)
	}
	c.saveRemediationWorkloadSnapshot(ctx, rec.ID, "manual_apply", rec.Namespace, firstNonEmpty(rec.WorkloadKind, "Pod"), firstNonEmpty(rec.WorkloadName, rec.PodName))
	c.markRemediationStatus(ctx, rec.ID, RemediationObserving, rec.PRURL, "Operator marked remediation changes as applied; observing workload health")
	return RemediationActionResult{ID: rec.ID, Status: string(RemediationObserving), Message: "Remediation marked applied; Fixora is observing workload health.", PRURL: rec.PRURL}, nil
}

func (c *Controller) RerunRemediationValidation(ctx context.Context, id int64) (RemediationActionResult, error) {
	rec, err := c.remediationRecordForAction(ctx, id)
	if err != nil {
		return RemediationActionResult{}, err
	}
	switch rec.Status {
	case RemediationAwaitingApply:
		applied, failure := c.remediationAppliedForObservation(ctx, rec)
		if failure != "" {
			c.markProductionRemediationFailure(ctx, rec, failure)
			return RemediationActionResult{ID: rec.ID, Status: string(RemediationProductionFailed), Message: failure, PRURL: rec.PRURL}, nil
		}
		if !applied {
			return RemediationActionResult{}, fmt.Errorf("remediation changes are not detected in the cluster yet")
		}
		c.saveRemediationWorkloadSnapshot(ctx, rec.ID, "manual_validation", rec.Namespace, firstNonEmpty(rec.WorkloadKind, "Pod"), firstNonEmpty(rec.WorkloadName, rec.PodName))
		c.markRemediationStatus(ctx, rec.ID, RemediationObserving, rec.PRURL, "Manual validation detected remediation changes; observing workload health")
		return RemediationActionResult{ID: rec.ID, Status: string(RemediationObserving), Message: "Remediation changes are live; observation restarted.", PRURL: rec.PRURL}, nil
	case RemediationObserving:
		ready, gitOpsFailure := c.gitOpsReadyForObservation(ctx, rec)
		if gitOpsFailure != "" {
			c.markProductionRemediationFailure(ctx, rec, gitOpsFailure)
			return RemediationActionResult{ID: rec.ID, Status: string(RemediationProductionFailed), Message: gitOpsFailure, PRURL: rec.PRURL}, nil
		}
		if !ready {
			return RemediationActionResult{}, fmt.Errorf("GitOps controller has not reported the remediation healthy yet")
		}
		if failure := c.workloadRegressionReason(ctx, rec); failure != "" {
			c.markProductionRemediationFailure(ctx, rec, failure)
			return RemediationActionResult{ID: rec.ID, Status: string(RemediationProductionFailed), Message: failure, PRURL: rec.PRURL}, nil
		}
		c.markRemediationStatus(ctx, rec.ID, RemediationSucceeded, rec.PRURL, "Manual validation completed without regression")
		return RemediationActionResult{ID: rec.ID, Status: string(RemediationSucceeded), Message: "Manual validation completed without regression.", PRURL: rec.PRURL}, nil
	default:
		return RemediationActionResult{}, fmt.Errorf("remediation cannot be revalidated from status %q", rec.Status)
	}
}

func (c *Controller) OpenRevertForRemediation(ctx context.Context, id int64) (RemediationActionResult, error) {
	rec, err := c.remediationRecordForAction(ctx, id)
	if err != nil {
		return RemediationActionResult{}, err
	}
	switch rec.Status {
	case RemediationProductionFailed, RemediationRevertFailed:
	default:
		return RemediationActionResult{}, fmt.Errorf("revert PR can only be opened after production_failed or revert_failed status")
	}
	c.openRevertPR(ctx, rec)
	updated, err := c.remediationRecordForAction(ctx, id)
	if err != nil {
		return RemediationActionResult{}, err
	}
	if updated.Status != RemediationRevertOpened {
		return RemediationActionResult{}, errors.New(firstNonEmpty(updated.FailureReason, "revert PR was not opened"))
	}
	return RemediationActionResult{ID: updated.ID, Status: string(updated.Status), Message: "Revert PR opened.", PRURL: updated.PRURL, RevertPRURL: updated.RevertPRURL}, nil
}

func (c *Controller) DismissRemediation(ctx context.Context, id int64) (RemediationActionResult, error) {
	rec, err := c.remediationRecordForAction(ctx, id)
	if err != nil {
		return RemediationActionResult{}, err
	}
	switch rec.Status {
	case RemediationSucceeded, RemediationReverted, RemediationDismissed:
		return RemediationActionResult{}, fmt.Errorf("remediation is already closed with status %q", rec.Status)
	}
	c.markRemediationStatus(ctx, rec.ID, RemediationDismissed, rec.PRURL, "Operator dismissed remediation workflow")
	return RemediationActionResult{ID: rec.ID, Status: string(RemediationDismissed), Message: "Remediation dismissed.", PRURL: rec.PRURL, RevertPRURL: rec.RevertPRURL}, nil
}

func (c *Controller) remediationRecordForAction(ctx context.Context, id int64) (RemediationRecord, error) {
	if c.history == nil || c.history.db == nil {
		return RemediationRecord{}, fmt.Errorf("database not configured")
	}
	rec, ok, err := c.history.RemediationByID(ctx, id)
	if err != nil {
		return RemediationRecord{}, err
	}
	if !ok {
		return RemediationRecord{}, fmt.Errorf("remediation not found")
	}
	return rec, nil
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
	case "merged", "closed", "succeeded", "reverted", "dismissed":
		return true
	default:
		return false
	}
}

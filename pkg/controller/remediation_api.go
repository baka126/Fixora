package controller

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"fixora/pkg/vcs"
)

type FileDiff struct {
	FilePath string `json:"filePath"`
	Original string `json:"original"`
	Patched  string `json:"patched"`
}

func (c *Controller) GetRemediationDiff(ctx context.Context, id int64) ([]FileDiff, error) {
	if c.history == nil || c.history.db == nil {
		return nil, fmt.Errorf("database not configured")
	}

	query := `
		SELECT vcs_type, repo_owner, repo_name, base_branch, head_branch, changed_files
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

	var changedFiles []string
	if len(changedFilesJSON) > 0 {
		_ = json.Unmarshal(changedFilesJSON, &changedFiles)
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
		SELECT vcs_type, repo_owner, repo_name, head_branch
		FROM remediation_outcomes
		WHERE id = $1
	`
	var vcsType, repoOwner, repoName, headBranch string
	err := c.history.db.QueryRowContext(ctx, query, id).Scan(
		&vcsType, &repoOwner, &repoName, &headBranch,
	)
	if err == sql.ErrNoRows {
		return fmt.Errorf("remediation not found")
	}
	if err != nil {
		return err
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
		},
	}

	return provider.AppendCommit(ctx, repoOwner, repoName, headBranch, files, message)
}

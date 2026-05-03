package vcs

import (
	"context"
)

type FileChange struct {
	FilePath   string
	NewContent []byte
}

type PullRequestOptions struct {
	Title         string
	Body          string
	Head          string // Branch name
	Base          string // usually "main" or "master"
	RepoOwner     string
	RepoName      string
	Files         []FileChange
	CommitMessage string
}

type Provider interface {
	CreatePullRequest(ctx context.Context, opts PullRequestOptions) (string, error) // Returns PR URL
	GetFileContent(ctx context.Context, repoOwner, repoName, path, ref string) ([]byte, error)
	ListFiles(ctx context.Context, repoOwner, repoName, path, ref string) (map[string][]byte, error)
	PullRequestExists(ctx context.Context, repoOwner, repoName, headBranch string) (bool, string, error) // Returns true and URL if exists
}

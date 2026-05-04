package vcs

import (
	"context"
)

type FileChange struct {
	FilePath        string
	NewContent      []byte
	PreviousContent []byte
	Create          bool
	Delete          bool
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

type PullRequestStatus struct {
	URL            string
	State          string
	Merged         bool
	MergeCommitSHA string
}

type Provider interface {
	CreatePullRequest(ctx context.Context, opts PullRequestOptions) (string, error) // Returns PR URL
	GetFileContent(ctx context.Context, repoOwner, repoName, path, ref string) ([]byte, error)
	ListFiles(ctx context.Context, repoOwner, repoName, path, ref string) (map[string][]byte, error)
	PullRequestExists(ctx context.Context, repoOwner, repoName, headBranch string) (bool, string, error) // Returns true and URL if exists
	GetPullRequestStatus(ctx context.Context, repoOwner, repoName, headBranch string) (PullRequestStatus, error)
}

package vcs

import (
	"context"
)

type PullRequestOptions struct {
	Title        string
	Body         string
	Head         string // Branch name
	Base         string // usually "main" or "master"
	RepoOwner    string
	RepoName     string
	FilePath     string
	NewContent   []byte
	CommitMessage string
}

type Provider interface {
	CreatePullRequest(ctx context.Context, opts PullRequestOptions) (string, error) // Returns PR URL
	GetFileContent(ctx context.Context, repoOwner, repoName, path, ref string) ([]byte, error)
}

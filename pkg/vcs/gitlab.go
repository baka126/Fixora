package vcs

import (
	"context"
	"fmt"

	"github.com/xanzy/go-gitlab"
)

type GitLabProvider struct {
	client *gitlab.Client
}

func NewGitLabProvider(token, baseURL string) (*GitLabProvider, error) {
	opts := []gitlab.ClientOptionFunc{}
	if baseURL != "" {
		opts = append(opts, gitlab.WithBaseURL(baseURL))
	}
	client, err := gitlab.NewClient(token, opts...)
	if err != nil {
		return nil, err
	}
	return &GitLabProvider{
		client: client,
	}, nil
}

func (g *GitLabProvider) CreatePullRequest(ctx context.Context, opts PullRequestOptions) (string, error) {
	projectID := fmt.Sprintf("%s/%s", opts.RepoOwner, opts.RepoName)

	// 1. Create a new branch
	_, _, err := g.client.Branches.CreateBranch(projectID, &gitlab.CreateBranchOptions{
		Branch: gitlab.String(opts.Head),
		Ref:    gitlab.String(opts.Base),
	})
	if err != nil {
		return "", err
	}

	// 2. Commit file change
	actions := []*gitlab.CommitActionOptions{
		{
			Action:   gitlab.Ptr(gitlab.FileUpdate),
			FilePath: gitlab.String(opts.FilePath),
			Content:  gitlab.String(string(opts.NewContent)),
		},
	}

	_, _, err = g.client.Commits.CreateCommit(projectID, &gitlab.CreateCommitOptions{
		Branch:        gitlab.String(opts.Head),
		CommitMessage: gitlab.String(opts.CommitMessage),
		Actions:       actions,
	})
	if err != nil {
		return "", err
	}

	// 3. Create Merge Request
	mr, _, err := g.client.MergeRequests.CreateMergeRequest(projectID, &gitlab.CreateMergeRequestOptions{
		Title:        gitlab.String(opts.Title),
		SourceBranch: gitlab.String(opts.Head),
		TargetBranch: gitlab.String(opts.Base),
		Description:  gitlab.String(opts.Body),
	})
	if err != nil {
		return "", err
	}

	return mr.WebURL, nil
}

func (g *GitLabProvider) GetFileContent(ctx context.Context, repoOwner, repoName, path, ref string) ([]byte, error) {
	projectID := fmt.Sprintf("%s/%s", repoOwner, repoName)
	content, _, err := g.client.RepositoryFiles.GetRawFile(projectID, path, &gitlab.GetRawFileOptions{Ref: gitlab.String(ref)})
	if err != nil {
		return nil, err
	}
	return content, nil
}

func (g *GitLabProvider) PullRequestExists(ctx context.Context, repoOwner, repoName, headBranch string) (bool, string, error) {
	projectID := fmt.Sprintf("%s/%s", repoOwner, repoName)
	opts := &gitlab.ListProjectMergeRequestsOptions{
		State:        gitlab.Ptr("opened"),
		SourceBranch: gitlab.Ptr(headBranch),
	}
	mrs, _, err := g.client.MergeRequests.ListProjectMergeRequests(projectID, opts)
	if err != nil {
		return false, "", err
	}

	if len(mrs) > 0 {
		return true, mrs[0].WebURL, nil
	}

	return false, "", nil
}

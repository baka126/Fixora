package vcs

import (
	"context"
	"fmt"
	"strings"

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

	// 2. Commit file changes
	var actions []*gitlab.CommitActionOptions
	for _, file := range opts.Files {
		action := gitlab.FileUpdate
		if file.Delete {
			action = gitlab.FileDelete
		} else if file.Create {
			action = gitlab.FileCreate
		}
		commitAction := &gitlab.CommitActionOptions{
			Action:   gitlab.Ptr(action),
			FilePath: gitlab.String(file.FilePath),
		}
		if !file.Delete {
			commitAction.Content = gitlab.String(string(file.NewContent))
		}
		actions = append(actions, commitAction)
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

func (g *GitLabProvider) ListFiles(ctx context.Context, repoOwner, repoName, path, ref string) (map[string][]byte, error) {
	projectID := fmt.Sprintf("%s/%s", repoOwner, repoName)

	tree, _, err := g.client.Repositories.ListTree(projectID, &gitlab.ListTreeOptions{
		Path: gitlab.Ptr(path),
		Ref:  gitlab.Ptr(ref),
	})
	if err != nil {
		// Try as single file
		content, fileErr := g.GetFileContent(ctx, repoOwner, repoName, path, ref)
		if fileErr == nil {
			return map[string][]byte{path: content}, nil
		}
		return nil, err
	}

	files := make(map[string][]byte)
	for _, item := range tree {
		if item.Type == "blob" && (strings.HasSuffix(item.Name, ".yaml") || strings.HasSuffix(item.Name, ".yml")) {
			content, fileErr := g.GetFileContent(ctx, repoOwner, repoName, item.Path, ref)
			if fileErr == nil {
				files[item.Path] = content
			}
		}
	}
	return files, nil
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

func (g *GitLabProvider) GetPullRequestStatus(ctx context.Context, repoOwner, repoName, headBranch string) (PullRequestStatus, error) {
	projectID := fmt.Sprintf("%s/%s", repoOwner, repoName)
	opts := &gitlab.ListProjectMergeRequestsOptions{
		State:        gitlab.Ptr("all"),
		SourceBranch: gitlab.Ptr(headBranch),
	}
	mrs, _, err := g.client.MergeRequests.ListProjectMergeRequests(projectID, opts)
	if err != nil {
		return PullRequestStatus{}, err
	}
	if len(mrs) == 0 {
		return PullRequestStatus{State: "not_found"}, nil
	}

	mr := mrs[0]
	status := PullRequestStatus{
		URL:            mr.WebURL,
		State:          mr.State,
		Merged:         mr.State == "merged",
		MergeCommitSHA: mr.MergeCommitSHA,
	}
	return status, nil
}

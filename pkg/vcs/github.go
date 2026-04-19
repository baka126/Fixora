package vcs

import (
	"context"
	"fmt"

	"github.com/google/go-github/v50/github"
	"golang.org/x/oauth2"
)

type GitHubProvider struct {
	client *github.Client
}

func NewGitHubProvider(token string) *GitHubProvider {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)
	return &GitHubProvider{
		client: github.NewClient(tc),
	}
}

func (g *GitHubProvider) CreatePullRequest(ctx context.Context, opts PullRequestOptions) (string, error) {
	// 1. Get current commit of base branch
	ref, _, err := g.client.Git.GetRef(ctx, opts.RepoOwner, opts.RepoName, "refs/heads/"+opts.Base)
	if err != nil {
		return "", err
	}

	// 2. Create a new branch
	newRef := &github.Reference{
		Ref: github.String("refs/heads/" + opts.Head),
		Object: &github.GitObject{
			SHA: ref.Object.SHA,
		},
	}
	_, _, err = g.client.Git.CreateRef(ctx, opts.RepoOwner, opts.RepoName, newRef)
	if err != nil {
		return "", err
	}

	// 3. Create or Update file
	fileOpts := &github.RepositoryContentFileOptions{
		Message: github.String(opts.CommitMessage),
		Content: opts.NewContent,
		Branch:  github.String(opts.Head),
	}

	// Get file SHA if it exists
	file, _, _, _ := g.client.Repositories.GetContents(ctx, opts.RepoOwner, opts.RepoName, opts.FilePath, &github.RepositoryContentGetOptions{Ref: opts.Head})
	if file != nil {
		fileOpts.SHA = file.SHA
	}

	_, _, err = g.client.Repositories.UpdateFile(ctx, opts.RepoOwner, opts.RepoName, opts.FilePath, fileOpts)
	if err != nil {
		return "", err
	}

	// 4. Create Pull Request
	newPR := &github.NewPullRequest{
		Title:               github.String(opts.Title),
		Head:                github.String(opts.Head),
		Base:                github.String(opts.Base),
		Body:                github.String(opts.Body),
		MaintainerCanModify: github.Bool(true),
	}

	pr, _, err := g.client.PullRequests.Create(ctx, opts.RepoOwner, opts.RepoName, newPR)
	if err != nil {
		return "", err
	}

	return pr.GetHTMLURL(), nil
}

func (g *GitHubProvider) GetFileContent(ctx context.Context, repoOwner, repoName, path, ref string) ([]byte, error) {
	opts := &github.RepositoryContentGetOptions{Ref: ref}
	file, _, _, err := g.client.Repositories.GetContents(ctx, repoOwner, repoName, path, opts)
	if err != nil {
		return nil, err
	}
	if file == nil {
		return nil, fmt.Errorf("file not found: %s", path)
	}
	content, err := file.GetContent()
	if err != nil {
		return nil, err
	}
	return []byte(content), nil
}

func (g *GitHubProvider) PullRequestExists(ctx context.Context, repoOwner, repoName, headBranch string) (bool, string, error) {
	opts := &github.PullRequestListOptions{
		State: "open",
	}
	prs, _, err := g.client.PullRequests.List(ctx, repoOwner, repoName, opts)
	if err != nil {
		return false, "", err
	}

	for _, pr := range prs {
		if pr.Head.GetLabel() == headBranch || pr.Head.GetRef() == headBranch {
			return true, pr.GetHTMLURL(), nil
		}
	}

	return false, "", nil
}

package vcs

import (
	"context"
	"fmt"
	"strings"

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

	baseCommit, _, err := g.client.Git.GetCommit(ctx, opts.RepoOwner, opts.RepoName, ref.Object.GetSHA())
	if err != nil {
		return "", err
	}

	// 2. Create a new Tree with all file changes
	var entries []*github.TreeEntry
	for _, file := range opts.Files {
		if file.Delete {
			entries = append(entries, &github.TreeEntry{
				Path: github.String(file.FilePath),
				SHA:  github.String(""),
			})
			continue
		}
		entries = append(entries, &github.TreeEntry{
			Path:    github.String(file.FilePath),
			Mode:    github.String("100644"),
			Type:    github.String("blob"),
			Content: github.String(string(file.NewContent)),
		})
	}

	tree, _, err := g.client.Git.CreateTree(ctx, opts.RepoOwner, opts.RepoName, baseCommit.Tree.GetSHA(), entries)
	if err != nil {
		return "", err
	}

	// 3. Create a new Commit
	commit := &github.Commit{
		Message: github.String(opts.CommitMessage),
		Tree:    tree,
		Parents: []*github.Commit{baseCommit},
	}
	newCommit, _, err := g.client.Git.CreateCommit(ctx, opts.RepoOwner, opts.RepoName, commit)
	if err != nil {
		return "", err
	}

	// 4. Create a new Branch
	newRef := &github.Reference{
		Ref: github.String("refs/heads/" + opts.Head),
		Object: &github.GitObject{
			SHA: newCommit.SHA,
		},
	}
	_, _, err = g.client.Git.CreateRef(ctx, opts.RepoOwner, opts.RepoName, newRef)
	if err != nil {
		return "", err
	}

	// 5. Create Pull Request
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

func (g *GitHubProvider) ListFiles(ctx context.Context, repoOwner, repoName, path, ref string) (map[string][]byte, error) {
	_, dirContent, _, err := g.client.Repositories.GetContents(ctx, repoOwner, repoName, path, &github.RepositoryContentGetOptions{Ref: ref})
	if err != nil {
		// Try as single file
		fileContent, fileErr := g.GetFileContent(ctx, repoOwner, repoName, path, ref)
		if fileErr == nil {
			return map[string][]byte{path: fileContent}, nil
		}
		return nil, err
	}

	files := make(map[string][]byte)
	if dirContent != nil {
		for _, item := range dirContent {
			if item.GetType() == "file" && (strings.HasSuffix(item.GetName(), ".yaml") || strings.HasSuffix(item.GetName(), ".yml")) {
				file, _, _, err := g.client.Repositories.GetContents(ctx, repoOwner, repoName, item.GetPath(), &github.RepositoryContentGetOptions{Ref: ref})
				if err == nil && file != nil {
					content, _ := file.GetContent()
					files[item.GetPath()] = []byte(content)
				}
			}
		}
	}
	return files, nil
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
		if pr.Head.GetLabel() == headBranch || pr.Head.GetRef() == headBranch ||
			strings.HasPrefix(pr.Head.GetLabel(), headBranch) || strings.HasPrefix(pr.Head.GetRef(), headBranch) {
			return true, pr.GetHTMLURL(), nil
		}
	}

	return false, "", nil
}

func (g *GitHubProvider) GetPullRequestStatus(ctx context.Context, repoOwner, repoName, headBranch string) (PullRequestStatus, error) {
	opts := &github.PullRequestListOptions{
		State: "all",
		Head:  fmt.Sprintf("%s:%s", repoOwner, headBranch),
	}
	prs, _, err := g.client.PullRequests.List(ctx, repoOwner, repoName, opts)
	if err != nil {
		return PullRequestStatus{}, err
	}

	for _, pr := range prs {
		if !githubPRHeadMatches(pr, headBranch) {
			continue
		}
		status := PullRequestStatus{
			URL:   pr.GetHTMLURL(),
			State: pr.GetState(),
		}
		if pr.GetState() == "closed" {
			full, _, err := g.client.PullRequests.Get(ctx, repoOwner, repoName, pr.GetNumber())
			if err != nil {
				return status, err
			}
			status.URL = full.GetHTMLURL()
			status.State = full.GetState()
			status.Merged = full.GetMerged()
			status.MergeCommitSHA = full.GetMergeCommitSHA()
		}
		return status, nil
	}

	return PullRequestStatus{State: "not_found"}, nil
}

func githubPRHeadMatches(pr *github.PullRequest, headBranch string) bool {
	if pr == nil || pr.Head == nil {
		return false
	}
	return pr.Head.GetLabel() == headBranch ||
		pr.Head.GetRef() == headBranch ||
		strings.HasPrefix(pr.Head.GetLabel(), headBranch) ||
		strings.HasPrefix(pr.Head.GetRef(), headBranch)
}

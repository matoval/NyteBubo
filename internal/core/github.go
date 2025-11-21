package core

import (
	"context"
	"fmt"

	"github.com/google/go-github/v63/github"
	"golang.org/x/oauth2"
)

// GitHubClient wraps the GitHub API client
type GitHubClient struct {
	client *github.Client
	ctx    context.Context
}

// GetPullRequest retrieves a pull request
func (gc *GitHubClient) GetPullRequest(owner, repo string, number int) (*github.PullRequest, error) {
	pr, _, err := gc.client.PullRequests.Get(gc.ctx, owner, repo, number)
	if err != nil {
		return nil, fmt.Errorf("failed to get pull request: %w", err)
	}
	return pr, nil
}

// NewGitHubClient creates a new GitHub API client
func NewGitHubClient(token string) *GitHubClient {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)

	return &GitHubClient{
		client: github.NewClient(tc),
		ctx:    ctx,
	}
}

// GetIssue retrieves an issue from a repository
func (gc *GitHubClient) GetIssue(owner, repo string, number int) (*github.Issue, error) {
	issue, _, err := gc.client.Issues.Get(gc.ctx, owner, repo, number)
	if err != nil {
		return nil, fmt.Errorf("failed to get issue: %w", err)
	}
	return issue, nil
}

// CreateIssueComment adds a comment to an issue
func (gc *GitHubClient) CreateIssueComment(owner, repo string, number int, body string) error {
	comment := &github.IssueComment{
		Body: github.String(body),
	}
	_, _, err := gc.client.Issues.CreateComment(gc.ctx, owner, repo, number, comment)
	if err != nil {
		return fmt.Errorf("failed to create comment: %w", err)
	}
	return nil
}

// ListIssueComments retrieves all comments for an issue
func (gc *GitHubClient) ListIssueComments(owner, repo string, number int) ([]*github.IssueComment, error) {
	opts := &github.IssueListCommentsOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}
	comments, _, err := gc.client.Issues.ListComments(gc.ctx, owner, repo, number, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list comments: %w", err)
	}
	return comments, nil
}

// GetRepository retrieves repository information
func (gc *GitHubClient) GetRepository(owner, repo string) (*github.Repository, error) {
	repository, _, err := gc.client.Repositories.Get(gc.ctx, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository: %w", err)
	}
	return repository, nil
}

// CreatePullRequest creates a new pull request
func (gc *GitHubClient) CreatePullRequest(owner, repo, title, body, head, base string) (*github.PullRequest, error) {
	pr := &github.NewPullRequest{
		Title: github.String(title),
		Body:  github.String(body),
		Head:  github.String(head),
		Base:  github.String(base),
	}

	pullRequest, _, err := gc.client.PullRequests.Create(gc.ctx, owner, repo, pr)
	if err != nil {
		return nil, fmt.Errorf("failed to create pull request: %w", err)
	}
	return pullRequest, nil
}

// ListPRComments retrieves all comments (review comments + issue comments) for a PR
func (gc *GitHubClient) ListPRComments(owner, repo string, number int) ([]*github.PullRequestComment, error) {
	opts := &github.PullRequestListCommentsOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}
	comments, _, err := gc.client.PullRequests.ListComments(gc.ctx, owner, repo, number, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list PR comments: %w", err)
	}
	return comments, nil
}

// GetFileContent retrieves the content of a file from a repository
func (gc *GitHubClient) GetFileContent(owner, repo, path, ref string) (string, error) {
	opts := &github.RepositoryContentGetOptions{Ref: ref}
	fileContent, _, _, err := gc.client.Repositories.GetContents(gc.ctx, owner, repo, path, opts)
	if err != nil {
		return "", fmt.Errorf("failed to get file content: %w", err)
	}

	if fileContent == nil {
		return "", fmt.Errorf("file not found: %s", path)
	}

	content, err := fileContent.GetContent()
	if err != nil {
		return "", fmt.Errorf("failed to decode file content: %w", err)
	}

	return content, nil
}

// CreateOrUpdateFile creates or updates a file in a repository
func (gc *GitHubClient) CreateOrUpdateFile(owner, repo, path, message, content, branch string, sha *string) error {
	opts := &github.RepositoryContentFileOptions{
		Message: github.String(message),
		Content: []byte(content),
		Branch:  github.String(branch),
		SHA:     sha,
	}

	_, _, err := gc.client.Repositories.CreateFile(gc.ctx, owner, repo, path, opts)
	if err != nil {
		return fmt.Errorf("failed to create/update file: %w", err)
	}

	return nil
}

// GetDefaultBranch retrieves the default branch name for a repository
func (gc *GitHubClient) GetDefaultBranch(owner, repo string) (string, error) {
	repository, err := gc.GetRepository(owner, repo)
	if err != nil {
		return "", err
	}
	return repository.GetDefaultBranch(), nil
}

// CreateBranch creates a new branch from a reference
func (gc *GitHubClient) CreateBranch(owner, repo, newBranch, baseBranch string) error {
	// Get the reference of the base branch
	baseRef, _, err := gc.client.Git.GetRef(gc.ctx, owner, repo, "refs/heads/"+baseBranch)
	if err != nil {
		return fmt.Errorf("failed to get base branch: %w", err)
	}

	// Create new branch
	newRef := &github.Reference{
		Ref:    github.String("refs/heads/" + newBranch),
		Object: &github.GitObject{SHA: baseRef.Object.SHA},
	}

	_, _, err = gc.client.Git.CreateRef(gc.ctx, owner, repo, newRef)
	if err != nil {
		return fmt.Errorf("failed to create branch: %w", err)
	}

	return nil
}

// GetAuthenticatedUser retrieves the currently authenticated user
func (gc *GitHubClient) GetAuthenticatedUser() (*github.User, error) {
	user, _, err := gc.client.Users.Get(gc.ctx, "")
	if err != nil {
		return nil, fmt.Errorf("failed to get authenticated user: %w", err)
	}
	return user, nil
}

// ListAssignedIssues retrieves all issues assigned to a specific user across specified repositories
func (gc *GitHubClient) ListAssignedIssues(username string, repositories []string) ([]*github.Issue, error) {
	var allIssues []*github.Issue

	// Build repository filter query
	repoQuery := ""
	for i, repo := range repositories {
		if i > 0 {
			repoQuery += " "
		}
		repoQuery += "repo:" + repo
	}

	// Search for issues assigned to the user
	query := fmt.Sprintf("is:issue is:open assignee:%s %s", username, repoQuery)
	opts := &github.SearchOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}

	result, _, err := gc.client.Search.Issues(gc.ctx, query, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to search issues: %w", err)
	}

	allIssues = append(allIssues, result.Issues...)

	return allIssues, nil
}

// ListRepositoryIssues retrieves all open issues from a specific repository
func (gc *GitHubClient) ListRepositoryIssues(owner, repo, assignee string) ([]*github.Issue, error) {
	opts := &github.IssueListByRepoOptions{
		State:     "open",
		Assignee:  assignee,
		Sort:      "created",
		Direction: "desc",
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	issues, _, err := gc.client.Issues.ListByRepo(gc.ctx, owner, repo, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list repository issues: %w", err)
	}

	// Filter out pull requests (GitHub API includes PRs in issues endpoint)
	var issuesOnly []*github.Issue
	for _, issue := range issues {
		if !issue.IsPullRequest() {
			issuesOnly = append(issuesOnly, issue)
		}
	}

	return issuesOnly, nil
}

package githubapi

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/google/go-github/v69/github"
)

// ListOpenIssues fetches all open issues for a repository.
func (c *Client) ListOpenIssues(ctx context.Context, owner, repo string) ([]IssueDocument, error) {
	if err := c.ensureInstallation(ctx, owner, repo); err != nil {
		return nil, err
	}

	opts := &github.IssueListByRepoOptions{
		State:       "open",
		ListOptions: github.ListOptions{PerPage: 100},
	}

	var docs []IssueDocument
	for {
		issues, resp, err := c.gh.Issues.ListByRepo(ctx, owner, repo, opts)
		if err != nil {
			return nil, fmt.Errorf("list issues: %w", err)
		}
		for _, issue := range issues {
			// Skip pull requests (GitHub returns PRs as issues in this endpoint).
			if issue.IsPullRequest() {
				continue
			}
			doc, err := c.fetchIssueComments(ctx, owner, repo, issue)
			if err != nil {
				return nil, err
			}
			docs = append(docs, *doc)
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return docs, nil
}

// ListOpenPullRequests fetches all open pull requests for a repository.
func (c *Client) ListOpenPullRequests(ctx context.Context, owner, repo string) ([]PullRequestDocument, error) {
	if err := c.ensureInstallation(ctx, owner, repo); err != nil {
		return nil, err
	}

	opts := &github.PullRequestListOptions{
		State:       "open",
		ListOptions: github.ListOptions{PerPage: 100},
	}

	var docs []PullRequestDocument
	for {
		prs, resp, err := c.gh.PullRequests.List(ctx, owner, repo, opts)
		if err != nil {
			return nil, fmt.Errorf("list pull requests: %w", err)
		}
		for _, pr := range prs {
			doc, err := c.fetchPRComments(ctx, owner, repo, pr)
			if err != nil {
				return nil, err
			}
			docs = append(docs, *doc)
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return docs, nil
}

// FetchIssue retrieves a single issue (with comments) by number.
func (c *Client) FetchIssue(ctx context.Context, owner, repo string, number int) (*IssueDocument, error) {
	if err := c.ensureInstallation(ctx, owner, repo); err != nil {
		return nil, err
	}
	issue, _, err := c.gh.Issues.Get(ctx, owner, repo, number)
	if err != nil {
		return nil, fmt.Errorf("get issue: %w", err)
	}
	return c.fetchIssueComments(ctx, owner, repo, issue)
}

// FetchPullRequest retrieves a single PR (with comments) by number.
func (c *Client) FetchPullRequest(ctx context.Context, owner, repo string, number int) (*PullRequestDocument, error) {
	if err := c.ensureInstallation(ctx, owner, repo); err != nil {
		return nil, err
	}
	pr, _, err := c.gh.PullRequests.Get(ctx, owner, repo, number)
	if err != nil {
		return nil, fmt.Errorf("get pull request: %w", err)
	}
	return c.fetchPRComments(ctx, owner, repo, pr)
}

func (c *Client) fetchIssueComments(ctx context.Context, owner, repo string, issue *github.Issue) (*IssueDocument, error) {
	comments, _, err := c.gh.Issues.ListComments(ctx, owner, repo, issue.GetNumber(), &github.IssueListCommentsOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	})
	if err != nil {
		return nil, fmt.Errorf("list issue comments: %w", err)
	}

	doc := &IssueDocument{
		Number:    issue.GetNumber(),
		Title:     issue.GetTitle(),
		Body:      issue.GetBody(),
		URL:       issue.GetHTMLURL(),
		UpdatedAt: issue.GetUpdatedAt().Time,
	}
	for _, cm := range comments {
		if body := cm.GetBody(); body != "" {
			doc.Comments = append(doc.Comments, body)
		}
	}
	return doc, nil
}

func (c *Client) fetchPRComments(ctx context.Context, owner, repo string, pr *github.PullRequest) (*PullRequestDocument, error) {
	comments, _, err := c.gh.PullRequests.ListComments(ctx, owner, repo, pr.GetNumber(), &github.PullRequestListCommentsOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	})
	if err != nil {
		return nil, fmt.Errorf("list pr comments: %w", err)
	}

	doc := &PullRequestDocument{
		Number:    pr.GetNumber(),
		Title:     pr.GetTitle(),
		Body:      pr.GetBody(),
		URL:       pr.GetHTMLURL(),
		UpdatedAt: pr.GetUpdatedAt().Time,
	}
	for _, cm := range comments {
		if body := cm.GetBody(); body != "" {
			doc.Comments = append(doc.Comments, body)
		}
	}
	return doc, nil
}

// ContentHash builds a deterministic hash string for deduplication.
func ContentHash(title, body string, comments []string) string {
	b := strings.Builder{}
	b.WriteString(title)
	b.WriteByte('\n')
	b.WriteString(body)
	for _, c := range comments {
		b.WriteByte('\n')
		b.WriteString(c)
	}
	return fmt.Sprintf("%x", sha256.Sum256([]byte(b.String())))
}

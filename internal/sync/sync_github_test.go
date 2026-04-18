package sync

import (
	"context"
	"fmt"
	"testing"

	"marrow/internal/db"
	"marrow/internal/embed"
	"marrow/internal/githubapi"
)

type fakeGitHubClient struct {
	issues []githubapi.IssueDocument
	prs    []githubapi.PullRequestDocument
}

func (f *fakeGitHubClient) ListOpenIssues(ctx context.Context, owner, repo string) ([]githubapi.IssueDocument, error) {
	return f.issues, nil
}

func (f *fakeGitHubClient) ListOpenPullRequests(ctx context.Context, owner, repo string) ([]githubapi.PullRequestDocument, error) {
	return f.prs, nil
}

func (f *fakeGitHubClient) FetchIssue(ctx context.Context, owner, repo string, number int) (*githubapi.IssueDocument, error) {
	for _, i := range f.issues {
		if i.Number == number {
			return &i, nil
		}
	}
	return nil, fmt.Errorf("issue %d not found", number)
}

func (f *fakeGitHubClient) FetchPullRequest(ctx context.Context, owner, repo string, number int) (*githubapi.PullRequestDocument, error) {
	for _, p := range f.prs {
		if p.Number == number {
			return &p, nil
		}
	}
	return nil, fmt.Errorf("pr %d not found", number)
}

func TestRunGitHubIndexesIssuesAndPRs(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()

	orch := &Orchestrator{
		DB:          database,
		EmbedFn:     embed.NewMock(),
		Source:      "github-api",
		DefaultLang: "en",
	}

	client := &fakeGitHubClient{
		issues: []githubapi.IssueDocument{
			{Number: 1, Title: "Bug", Body: "It is broken", Comments: []string{"confirm"}},
		},
		prs: []githubapi.PullRequestDocument{
			{Number: 2, Title: "Fix", Body: "Here is the fix", Comments: []string{"lgtm"}},
		},
	}

	ctx := context.Background()
	if err := orch.RunGitHub(ctx, client, "owner", "repo"); err != nil {
		t.Fatalf("run github: %v", err)
	}

	paths, err := db.NewDocumentRepo(database).GetDocumentPathsBySource(ctx, "github-api")
	if err != nil {
		t.Fatalf("get paths: %v", err)
	}
	if len(paths) != 2 {
		t.Fatalf("expected 2 paths, got %d: %v", len(paths), paths)
	}

	// Verify doc types by querying directly
	var docType string
	err = database.QueryRowContext(ctx, `SELECT doc_type FROM documents WHERE path = ?`, "gh:owner/repo/issues/1").Scan(&docType)
	if err != nil || docType != "issue" {
		t.Errorf("expected issue doc_type, got %q (err=%v)", docType, err)
	}
	err = database.QueryRowContext(ctx, `SELECT doc_type FROM documents WHERE path = ?`, "gh:owner/repo/pull/2").Scan(&docType)
	if err != nil || docType != "pull_request" {
		t.Errorf("expected pull_request doc_type, got %q (err=%v)", docType, err)
	}
}

func TestRunGitHubDeletesClosedItems(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()

	orch := &Orchestrator{
		DB:          database,
		EmbedFn:     embed.NewMock(),
		Source:      "github-api",
		DefaultLang: "en",
	}

	client := &fakeGitHubClient{
		issues: []githubapi.IssueDocument{
			{Number: 1, Title: "Bug", Body: "It is broken"},
		},
		prs: []githubapi.PullRequestDocument{
			{Number: 2, Title: "Fix", Body: "Here is the fix"},
		},
	}

	ctx := context.Background()
	if err := orch.RunGitHub(ctx, client, "owner", "repo"); err != nil {
		t.Fatalf("run github: %v", err)
	}

	// Now simulate issue 1 and PR 2 being closed
	client.issues = nil
	client.prs = nil
	if err := orch.RunGitHub(ctx, client, "owner", "repo"); err != nil {
		t.Fatalf("run github second: %v", err)
	}

	paths, err := db.NewDocumentRepo(database).GetDocumentPathsBySource(ctx, "github-api")
	if err != nil {
		t.Fatalf("get paths: %v", err)
	}
	if len(paths) != 0 {
		t.Errorf("expected 0 paths after deletion, got %d: %v", len(paths), paths)
	}
}

func TestIndexSingleIssue(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()

	orch := &Orchestrator{
		DB:          database,
		EmbedFn:     embed.NewMock(),
		Source:      "github-api",
		DefaultLang: "en",
	}

	client := &fakeGitHubClient{
		issues: []githubapi.IssueDocument{
			{Number: 42, Title: "Bug", Body: "It is broken"},
		},
	}

	ctx := context.Background()
	if err := orch.IndexSingleIssue(ctx, client, "owner", "repo", 42); err != nil {
		t.Fatalf("index single issue: %v", err)
	}

	var title string
	err = database.QueryRowContext(ctx, `SELECT title FROM documents WHERE path = ?`, "gh:owner/repo/issues/42").Scan(&title)
	if err != nil || title != "Bug" {
		t.Errorf("expected title Bug, got %q (err=%v)", title, err)
	}
}

func TestDeleteGitHubDocument(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()

	orch := &Orchestrator{
		DB:          database,
		EmbedFn:     embed.NewMock(),
		Source:      "github-api",
		DefaultLang: "en",
	}

	client := &fakeGitHubClient{
		issues: []githubapi.IssueDocument{
			{Number: 1, Title: "Bug", Body: "It is broken"},
		},
	}

	ctx := context.Background()
	if err := orch.RunGitHub(ctx, client, "owner", "repo"); err != nil {
		t.Fatalf("run github: %v", err)
	}

	if err := orch.DeleteGitHubDocument(ctx, "owner", "repo", "issues", 1); err != nil {
		t.Fatalf("delete: %v", err)
	}

	paths, err := db.NewDocumentRepo(database).GetDocumentPathsBySource(ctx, "github-api")
	if err != nil {
		t.Fatalf("get paths: %v", err)
	}
	if len(paths) != 0 {
		t.Errorf("expected 0 paths, got %d", len(paths))
	}
}

func TestIndexSinglePullRequest(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()

	orch := &Orchestrator{
		DB:          database,
		EmbedFn:     embed.NewMock(),
		Source:      "github-api",
		DefaultLang: "en",
	}

	client := &fakeGitHubClient{
		prs: []githubapi.PullRequestDocument{
			{Number: 7, Title: "Feature", Body: "Adds feature"},
		},
	}

	ctx := context.Background()
	if err := orch.IndexSinglePullRequest(ctx, client, "owner", "repo", 7); err != nil {
		t.Fatalf("index single pr: %v", err)
	}

	var title string
	err = database.QueryRowContext(ctx, `SELECT title FROM documents WHERE path = ?`, "gh:owner/repo/pull/7").Scan(&title)
	if err != nil || title != "Feature" {
		t.Errorf("expected title Feature, got %q (err=%v)", title, err)
	}
}

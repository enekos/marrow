package sync

import (
	"context"
	"fmt"
	"strings"
	"time"

	"marrow/internal/db"
	"marrow/internal/embed"
	"marrow/internal/githubapi"
	"marrow/internal/index"
	"marrow/internal/stemmer"
)

// GitHubSyncer fetches open issues and pull requests via the GitHub API and indexes them.
type GitHubSyncer struct {
	DB          *db.DB
	EmbedFn     embed.Func
	Source      string
	DefaultLang string
	Client      GitHubAPIClient
	Owner       string
	Repo        string
}

// Sync fetches all open issues and PRs and indexes them.
func (s *GitHubSyncer) Sync(ctx context.Context) error {
	stateRepo := db.NewSyncStateRepo(s.DB)
	state, err := loadOrCreateState(ctx, stateRepo, s.Source)
	if err != nil {
		return err
	}

	issues, err := s.Client.ListOpenIssues(ctx, s.Owner, s.Repo)
	if err != nil {
		return fmt.Errorf("list issues: %w", err)
	}
	prs, err := s.Client.ListOpenPullRequests(ctx, s.Owner, s.Repo)
	if err != nil {
		return fmt.Errorf("list pull requests: %w", err)
	}

	prefix := fmt.Sprintf("gh:%s/%s/", s.Owner, s.Repo)
	indexed := make(map[string]struct{})
	lang := s.DefaultLang
	if lang == "" {
		lang = "en"
	}

	for _, issue := range issues {
		path := fmt.Sprintf("%sissues/%d", prefix, issue.Number)
		if err := s.indexItem(ctx, path, issue.Title, issue.Body, issue.Comments, "issue"); err != nil {
			return err
		}
		indexed[path] = struct{}{}
	}

	for _, pr := range prs {
		path := fmt.Sprintf("%spull/%d", prefix, pr.Number)
		if err := s.indexItem(ctx, path, pr.Title, pr.Body, pr.Comments, "pull_request"); err != nil {
			return err
		}
		indexed[path] = struct{}{}
	}

	tracked, err := db.NewDocumentRepo(s.DB).GetDocumentPathsBySource(ctx, s.Source)
	if err != nil {
		return fmt.Errorf("list tracked: %w", err)
	}
	var toDelete []string
	for _, p := range tracked {
		if strings.HasPrefix(p, prefix) {
			if _, ok := indexed[p]; !ok {
				toDelete = append(toDelete, p)
			}
		}
	}
	orch := &Orchestrator{DB: s.DB, Source: s.Source}
	if err := orch.removePaths(ctx, toDelete); err != nil {
		return fmt.Errorf("remove: %w", err)
	}

	now := time.Now().UTC()
	state.LastSyncAt = &now
	return stateRepo.Upsert(ctx, state)
}

// IndexIssue fetches and indexes one issue.
func (s *GitHubSyncer) IndexIssue(ctx context.Context, number int) error {
	issue, err := s.Client.FetchIssue(ctx, s.Owner, s.Repo, number)
	if err != nil {
		return err
	}
	path := fmt.Sprintf("gh:%s/%s/issues/%d", s.Owner, s.Repo, number)
	return s.indexItem(ctx, path, issue.Title, issue.Body, issue.Comments, "issue")
}

// IndexPullRequest fetches and indexes one pull request.
func (s *GitHubSyncer) IndexPullRequest(ctx context.Context, number int) error {
	pr, err := s.Client.FetchPullRequest(ctx, s.Owner, s.Repo, number)
	if err != nil {
		return err
	}
	path := fmt.Sprintf("gh:%s/%s/pull/%d", s.Owner, s.Repo, number)
	return s.indexItem(ctx, path, pr.Title, pr.Body, pr.Comments, "pull_request")
}

// DeleteDocument removes a synthetic GitHub document by path.
func (s *GitHubSyncer) DeleteDocument(ctx context.Context, docType string, number int) error {
	path := fmt.Sprintf("gh:%s/%s/%s/%d", s.Owner, s.Repo, docType, number)
	orch := &Orchestrator{DB: s.DB, Source: s.Source}
	return orch.removePaths(ctx, []string{path})
}

func (s *GitHubSyncer) indexItem(ctx context.Context, path, title, body string, comments []string, docType string) error {
	content := buildContent(title, body, comments)
	hash := githubapi.ContentHash(title, body, comments)
	lang := s.DefaultLang
	if lang == "" {
		lang = "en"
	}
	stemmed := stemmer.StemText(content, lang)
	chunks, err := embedChunks(ctx, s.EmbedFn, content)
	if err != nil {
		return fmt.Errorf("embed %s: %w", path, err)
	}
	ix := index.NewIndexer(s.DB)
	return ix.Index(ctx, index.Document{
		Path:        path,
		Hash:        hash,
		Title:       title,
		Lang:        lang,
		StemmedText: stemmed,
		Chunks:      chunks,
		Source:      s.Source,
		DocType:     docType,
	})
}

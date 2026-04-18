package service

import (
	"context"
	"fmt"

	"marrow/internal/config"
	"marrow/internal/db"
	"marrow/internal/embed"
	"marrow/internal/sync"
)

// Syncer coordinates indexing operations.
type Syncer struct {
	DB      *db.DB
	EmbedFn embed.Func
}

// SyncLocal runs an incremental sync for a local directory.
func (s *Syncer) SyncLocal(ctx context.Context, source, defaultLang, root string) error {
	orch := &sync.Orchestrator{
		DB:          s.DB,
		EmbedFn:     s.EmbedFn,
		Source:      source,
		DefaultLang: defaultLang,
	}
	return orch.RunLocal(ctx, root)
}

// SyncGit runs a git pull/clone sync.
func (s *Syncer) SyncGit(ctx context.Context, source, defaultLang, repoURL, token, localPath string) error {
	orch := &sync.Orchestrator{
		DB:          s.DB,
		EmbedFn:     s.EmbedFn,
		Source:      source,
		DefaultLang: defaultLang,
	}
	return orch.RunGit(ctx, repoURL, token, localPath)
}

// SyncGitHubAPI fetches and indexes open issues and PRs.
func (s *Syncer) SyncGitHubAPI(ctx context.Context, source, defaultLang string, client sync.GitHubAPIClient, owner, repo string) error {
	orch := &sync.Orchestrator{
		DB:          s.DB,
		EmbedFn:     s.EmbedFn,
		Source:      source,
		DefaultLang: defaultLang,
	}
	return orch.RunGitHub(ctx, client, owner, repo)
}

// SyncSource dispatches to the appropriate syncer based on source config.
func (s *Syncer) SyncSource(ctx context.Context, cfg config.SourceConfig, fallbackLang string) error {
	src := cfg.Name
	if src == "" {
		src = cfg.Type
	}
	lang := cfg.DefaultLang
	if lang == "" {
		lang = fallbackLang
	}
	if lang == "" {
		lang = "en"
	}
	switch cfg.Type {
	case "local":
		return s.SyncLocal(ctx, src, lang, cfg.Dir)
	case "git":
		lp := cfg.LocalPath
		if lp == "" {
			lp = sync.LocalPathFromSource("./repo", src)
		}
		return s.SyncGit(ctx, src, lang, cfg.RepoURL, cfg.Token, lp)
	default:
		return fmt.Errorf("unsupported source type: %s", cfg.Type)
	}
}

// IndexGitHubIssue indexes a single GitHub issue.
func (s *Syncer) IndexGitHubIssue(ctx context.Context, source, defaultLang string, client sync.GitHubAPIClient, owner, repo string, number int) error {
	orch := &sync.Orchestrator{
		DB:          s.DB,
		EmbedFn:     s.EmbedFn,
		Source:      source,
		DefaultLang: defaultLang,
	}
	return orch.IndexSingleIssue(ctx, client, owner, repo, number)
}

// IndexGitHubPullRequest indexes a single GitHub pull request.
func (s *Syncer) IndexGitHubPullRequest(ctx context.Context, source, defaultLang string, client sync.GitHubAPIClient, owner, repo string, number int) error {
	orch := &sync.Orchestrator{
		DB:          s.DB,
		EmbedFn:     s.EmbedFn,
		Source:      source,
		DefaultLang: defaultLang,
	}
	return orch.IndexSinglePullRequest(ctx, client, owner, repo, number)
}

// DeleteGitHubDocument removes a synthetic GitHub document.
func (s *Syncer) DeleteGitHubDocument(ctx context.Context, source, defaultLang, owner, repo, docType string, number int) error {
	orch := &sync.Orchestrator{
		DB:          s.DB,
		EmbedFn:     s.EmbedFn,
		Source:      source,
		DefaultLang: defaultLang,
	}
	return orch.DeleteGitHubDocument(ctx, owner, repo, docType, number)
}

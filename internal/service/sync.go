package service

import (
	"context"
	"fmt"

	"github.com/enekos/marrow/internal/config"
	"github.com/enekos/marrow/internal/db"
	"github.com/enekos/marrow/internal/embed"
	"github.com/enekos/marrow/internal/sync"
)

// Syncer coordinates indexing operations.
type Syncer struct {
	DB      *db.DB
	EmbedFn embed.Func
}

// orchestrator returns a pre-wired sync.Orchestrator for the given source.
func (s *Syncer) orchestrator(source, defaultLang string) *sync.Orchestrator {
	return &sync.Orchestrator{
		DB:          s.DB,
		EmbedFn:     s.EmbedFn,
		Source:      source,
		DefaultLang: defaultLang,
	}
}

// SyncLocal runs an incremental sync for a local directory.
func (s *Syncer) SyncLocal(ctx context.Context, source, defaultLang, root string) error {
	return s.orchestrator(source, defaultLang).RunLocal(ctx, root)
}

// SyncGit runs a git pull/clone sync.
func (s *Syncer) SyncGit(ctx context.Context, source, defaultLang, repoURL, token, localPath string) error {
	return s.orchestrator(source, defaultLang).RunGit(ctx, repoURL, token, localPath)
}

// SyncGitHubAPI fetches and indexes open issues and PRs.
func (s *Syncer) SyncGitHubAPI(ctx context.Context, source, defaultLang string, client sync.GitHubAPIClient, owner, repo string) error {
	return s.orchestrator(source, defaultLang).RunGitHub(ctx, client, owner, repo)
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
		lang = config.DefaultLang
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
	return s.orchestrator(source, defaultLang).IndexSingleIssue(ctx, client, owner, repo, number)
}

// IndexGitHubPullRequest indexes a single GitHub pull request.
func (s *Syncer) IndexGitHubPullRequest(ctx context.Context, source, defaultLang string, client sync.GitHubAPIClient, owner, repo string, number int) error {
	return s.orchestrator(source, defaultLang).IndexSinglePullRequest(ctx, client, owner, repo, number)
}

// DeleteGitHubDocument removes a synthetic GitHub document.
func (s *Syncer) DeleteGitHubDocument(ctx context.Context, source, defaultLang, owner, repo, docType string, number int) error {
	return s.orchestrator(source, defaultLang).DeleteGitHubDocument(ctx, owner, repo, docType, number)
}

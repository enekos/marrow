package sync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"marrow/internal/db"
	"marrow/internal/embed"
	"marrow/internal/githubapi"
	"marrow/internal/index"
	"marrow/internal/markdown"
	"marrow/internal/stemmer"
	"marrow/internal/watcher"
)

// Orchestrator coordinates indexing for a given source.
// It delegates to source-specific syncers.
type Orchestrator struct {
	DB          *db.DB
	EmbedFn     embed.Func
	Source      string
	DefaultLang string
}

// GitHubAPIClient is the subset of githubapi.Client used by the syncers.
type GitHubAPIClient interface {
	ListOpenIssues(ctx context.Context, owner, repo string) ([]githubapi.IssueDocument, error)
	ListOpenPullRequests(ctx context.Context, owner, repo string) ([]githubapi.PullRequestDocument, error)
	FetchIssue(ctx context.Context, owner, repo string, number int) (*githubapi.IssueDocument, error)
	FetchPullRequest(ctx context.Context, owner, repo string, number int) (*githubapi.PullRequestDocument, error)
}

// RunLocal performs an incremental sync over a local directory tree.
func (o *Orchestrator) RunLocal(ctx context.Context, root string) error {
	syncer := &LocalSyncer{
		DB:          o.DB,
		EmbedFn:     o.EmbedFn,
		Source:      o.Source,
		DefaultLang: o.DefaultLang,
		Root:        root,
	}
	return syncer.Sync(ctx)
}

// RunGit clones or pulls a GitHub repo and indexes changed markdown files.
func (o *Orchestrator) RunGit(ctx context.Context, repoURL, token, localPath string) error {
	syncer := &GitSyncer{
		DB:          o.DB,
		EmbedFn:     o.EmbedFn,
		Source:      o.Source,
		DefaultLang: o.DefaultLang,
		RepoURL:     repoURL,
		Token:       token,
		LocalPath:   localPath,
	}
	return syncer.Sync(ctx)
}

// RunGitHub fetches open issues and pull requests via the GitHub API and indexes them.
func (o *Orchestrator) RunGitHub(ctx context.Context, client GitHubAPIClient, owner, repo string) error {
	syncer := &GitHubSyncer{
		DB:          o.DB,
		EmbedFn:     o.EmbedFn,
		Source:      o.Source,
		DefaultLang: o.DefaultLang,
		Client:      client,
		Owner:       owner,
		Repo:        repo,
	}
	return syncer.Sync(ctx)
}

// IndexSingleIssue fetches and indexes one issue. Used by webhooks.
func (o *Orchestrator) IndexSingleIssue(ctx context.Context, client GitHubAPIClient, owner, repo string, number int) error {
	syncer := &GitHubSyncer{
		DB:          o.DB,
		EmbedFn:     o.EmbedFn,
		Source:      o.Source,
		DefaultLang: o.DefaultLang,
		Client:      client,
		Owner:       owner,
		Repo:        repo,
	}
	return syncer.IndexIssue(ctx, number)
}

// IndexSinglePullRequest fetches and indexes one pull request. Used by webhooks.
func (o *Orchestrator) IndexSinglePullRequest(ctx context.Context, client GitHubAPIClient, owner, repo string, number int) error {
	syncer := &GitHubSyncer{
		DB:          o.DB,
		EmbedFn:     o.EmbedFn,
		Source:      o.Source,
		DefaultLang: o.DefaultLang,
		Client:      client,
		Owner:       owner,
		Repo:        repo,
	}
	return syncer.IndexPullRequest(ctx, number)
}

// DeleteGitHubDocument removes a synthetic GitHub document by path.
func (o *Orchestrator) DeleteGitHubDocument(ctx context.Context, owner, repo, docType string, number int) error {
	syncer := &GitHubSyncer{
		DB:     o.DB,
		Source: o.Source,
		Owner:  owner,
		Repo:   repo,
	}
	return syncer.DeleteDocument(ctx, docType, number)
}

func loadOrCreateState(ctx context.Context, repo *db.SyncStateRepo, source string) (*db.SyncState, error) {
	state, err := repo.Get(ctx, source)
	if err == nil {
		return state, nil
	}
	// Not found: create default
	return &db.SyncState{Source: source}, nil
}

func (o *Orchestrator) indexFiles(ctx context.Context, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	ix := index.NewIndexer(o.DB)
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			return fmt.Errorf("read %s: %w", p, err)
		}
		defaultLang := o.DefaultLang
		if defaultLang == "" {
			defaultLang = "en"
		}
		md, err := markdown.ParseWithDefault(data, defaultLang)
		if err != nil {
			return fmt.Errorf("parse %s: %w", p, err)
		}
		hash := fmt.Sprintf("%x", watcher.HashBytes(data))
		stemmed := stemmer.StemText(md.Text, md.Lang)
		vec, err := o.EmbedFn(ctx, md.Text)
		if err != nil {
			return fmt.Errorf("embed %s: %w", p, err)
		}
		doc := index.Document{
			Path:        p,
			Hash:        hash,
			Title:       md.Title,
			Lang:        md.Lang,
			StemmedText: stemmed,
			Embedding:   vec,
			Source:      o.Source,
			DocType:     "markdown",
		}
		if err := ix.Index(ctx, doc); err != nil {
			return fmt.Errorf("index %s: %w", p, err)
		}
	}
	return nil
}

func (o *Orchestrator) removePaths(ctx context.Context, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	return db.NewDocumentRepo(o.DB).DeleteDocumentsByPaths(ctx, paths)
}

func buildContent(title, body string, comments []string) string {
	b := strings.Builder{}
	b.WriteString(title)
	if body != "" {
		b.WriteString("\n")
		b.WriteString(body)
	}
	for _, c := range comments {
		if c != "" {
			b.WriteString("\n")
			b.WriteString(c)
		}
	}
	return b.String()
}

// LocalPathFromSource derives a local directory name from the source identifier.
func LocalPathFromSource(base, source string) string {
	clean := strings.ReplaceAll(source, "/", "-")
	return filepath.Join(base, clean)
}

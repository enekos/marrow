package sync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"marrow/internal/db"
	"marrow/internal/embed"
	"marrow/internal/gitpull"
	"marrow/internal/githubapi"
	"marrow/internal/index"
	"marrow/internal/markdown"
	"marrow/internal/stemmer"
	"marrow/internal/watcher"
)

// Orchestrator coordinates indexing for a given source.
type Orchestrator struct {
	DB          *db.DB
	EmbedFn     embed.Func
	Source      string
	DefaultLang string
}

// GitHubAPIClient is the subset of githubapi.Client used by the orchestrator.
type GitHubAPIClient interface {
	ListOpenIssues(ctx context.Context, owner, repo string) ([]githubapi.IssueDocument, error)
	ListOpenPullRequests(ctx context.Context, owner, repo string) ([]githubapi.PullRequestDocument, error)
	FetchIssue(ctx context.Context, owner, repo string, number int) (*githubapi.IssueDocument, error)
	FetchPullRequest(ctx context.Context, owner, repo string, number int) (*githubapi.PullRequestDocument, error)
}

// RunLocal performs an incremental sync over a local directory tree.
func (o *Orchestrator) RunLocal(ctx context.Context, root string) error {
	state, err := o.loadOrCreateState(ctx)
	if err != nil {
		return err
	}

	var since time.Time
	if state.LastSyncAt != nil {
		since = *state.LastSyncAt
	}

	crawler := watcher.NewCrawler(o.DB)
	changed, deleted, err := crawler.ScanIncremental(ctx, root, since, o.Source)
	if err != nil {
		return fmt.Errorf("crawl: %w", err)
	}

	paths := make([]string, len(changed))
	for i, fi := range changed {
		paths[i] = fi.Path
	}
	if err := o.indexFiles(ctx, paths); err != nil {
		return fmt.Errorf("index: %w", err)
	}
	if err := o.removePaths(ctx, deleted); err != nil {
		return fmt.Errorf("remove: %w", err)
	}

	now := time.Now().UTC()
	state.LastSyncAt = &now
	return o.DB.UpsertSyncState(ctx, state)
}

// RunGit clones or pulls a GitHub repo and indexes changed markdown files.
func (o *Orchestrator) RunGit(ctx context.Context, repoURL, token, localPath string) error {
	state, err := o.loadOrCreateState(ctx)
	if err != nil {
		return err
	}
	state.RepoURL = repoURL
	state.LocalPath = localPath
	state.Token = token
	if err := o.DB.UpsertSyncState(ctx, state); err != nil {
		return err
	}

	changed, err := gitpull.Sync(repoURL, token, localPath)
	if err != nil {
		return fmt.Errorf("git sync: %w", err)
	}

	// Filter to markdown only (gitpull already does this, but be safe)
	var mdFiles []string
	for _, p := range changed {
		if strings.HasSuffix(strings.ToLower(p), ".md") {
			mdFiles = append(mdFiles, p)
		}
	}

	if err := o.indexFiles(ctx, mdFiles); err != nil {
		return fmt.Errorf("index: %w", err)
	}

	// Cleanup: remove DB entries for .md files that no longer exist under localPath
	tracked, err := o.DB.GetDocumentPathsBySource(ctx, o.Source)
	if err != nil {
		return fmt.Errorf("list tracked: %w", err)
	}
	var toDelete []string
	for _, p := range tracked {
		if _, err := os.Stat(p); os.IsNotExist(err) {
			toDelete = append(toDelete, p)
		}
	}
	if err := o.removePaths(ctx, toDelete); err != nil {
		return fmt.Errorf("remove: %w", err)
	}

	now := time.Now().UTC()
	state.LastSyncAt = &now
	return o.DB.UpsertSyncState(ctx, state)
}

// RunGitHub fetches open issues and pull requests via the GitHub API and indexes them.
func (o *Orchestrator) RunGitHub(ctx context.Context, client GitHubAPIClient, owner, repo string) error {
	state, err := o.loadOrCreateState(ctx)
	if err != nil {
		return err
	}

	issues, err := client.ListOpenIssues(ctx, owner, repo)
	if err != nil {
		return fmt.Errorf("list issues: %w", err)
	}
	prs, err := client.ListOpenPullRequests(ctx, owner, repo)
	if err != nil {
		return fmt.Errorf("list pull requests: %w", err)
	}

	ix := index.NewIndexer(o.DB)
	prefix := fmt.Sprintf("gh:%s/%s/", owner, repo)
	indexed := make(map[string]struct{})
	defaultLang := o.DefaultLang
	if defaultLang == "" {
		defaultLang = "en"
	}

	for _, issue := range issues {
		path := fmt.Sprintf("%sissues/%d", prefix, issue.Number)
		content := buildContent(issue.Title, issue.Body, issue.Comments)
		hash := githubapi.ContentHash(issue.Title, issue.Body, issue.Comments)
		stemmed := stemmer.StemText(content, defaultLang)
		vec, err := o.EmbedFn(ctx, content)
		if err != nil {
			return fmt.Errorf("embed %s: %w", path, err)
		}
		doc := index.Document{
			Path:        path,
			Hash:        hash,
			Title:       issue.Title,
			Lang:        defaultLang,
			StemmedText: stemmed,
			Embedding:   vec,
			Source:      o.Source,
			DocType:     "issue",
		}
		if err := ix.Index(ctx, doc); err != nil {
			return fmt.Errorf("index %s: %w", path, err)
		}
		indexed[path] = struct{}{}
	}

	for _, pr := range prs {
		path := fmt.Sprintf("%spull/%d", prefix, pr.Number)
		content := buildContent(pr.Title, pr.Body, pr.Comments)
		hash := githubapi.ContentHash(pr.Title, pr.Body, pr.Comments)
		stemmed := stemmer.StemText(content, defaultLang)
		vec, err := o.EmbedFn(ctx, content)
		if err != nil {
			return fmt.Errorf("embed %s: %w", path, err)
		}
		doc := index.Document{
			Path:        path,
			Hash:        hash,
			Title:       pr.Title,
			Lang:        defaultLang,
			StemmedText: stemmed,
			Embedding:   vec,
			Source:      o.Source,
			DocType:     "pull_request",
		}
		if err := ix.Index(ctx, doc); err != nil {
			return fmt.Errorf("index %s: %w", path, err)
		}
		indexed[path] = struct{}{}
	}

	// Cleanup: delete tracked items for this repo that are no longer open.
	tracked, err := o.DB.GetDocumentPathsBySource(ctx, o.Source)
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
	if err := o.removePaths(ctx, toDelete); err != nil {
		return fmt.Errorf("remove: %w", err)
	}

	now := time.Now().UTC()
	state.LastSyncAt = &now
	return o.DB.UpsertSyncState(ctx, state)
}

// IndexSingleIssue fetches and indexes one issue. Used by webhooks.
func (o *Orchestrator) IndexSingleIssue(ctx context.Context, client GitHubAPIClient, owner, repo string, number int) error {
	issue, err := client.FetchIssue(ctx, owner, repo, number)
	if err != nil {
		return err
	}
	path := fmt.Sprintf("gh:%s/%s/issues/%d", owner, repo, number)
	content := buildContent(issue.Title, issue.Body, issue.Comments)
	hash := githubapi.ContentHash(issue.Title, issue.Body, issue.Comments)
	defaultLang := o.DefaultLang
	if defaultLang == "" {
		defaultLang = "en"
	}
	stemmed := stemmer.StemText(content, defaultLang)
	vec, err := o.EmbedFn(ctx, content)
	if err != nil {
		return fmt.Errorf("embed %s: %w", path, err)
	}
	ix := index.NewIndexer(o.DB)
	return ix.Index(ctx, index.Document{
		Path:        path,
		Hash:        hash,
		Title:       issue.Title,
		Lang:        defaultLang,
		StemmedText: stemmed,
		Embedding:   vec,
		Source:      o.Source,
		DocType:     "issue",
	})
}

// IndexSinglePullRequest fetches and indexes one pull request. Used by webhooks.
func (o *Orchestrator) IndexSinglePullRequest(ctx context.Context, client GitHubAPIClient, owner, repo string, number int) error {
	pr, err := client.FetchPullRequest(ctx, owner, repo, number)
	if err != nil {
		return err
	}
	path := fmt.Sprintf("gh:%s/%s/pull/%d", owner, repo, number)
	content := buildContent(pr.Title, pr.Body, pr.Comments)
	hash := githubapi.ContentHash(pr.Title, pr.Body, pr.Comments)
	defaultLang := o.DefaultLang
	if defaultLang == "" {
		defaultLang = "en"
	}
	stemmed := stemmer.StemText(content, defaultLang)
	vec, err := o.EmbedFn(ctx, content)
	if err != nil {
		return fmt.Errorf("embed %s: %w", path, err)
	}
	ix := index.NewIndexer(o.DB)
	return ix.Index(ctx, index.Document{
		Path:        path,
		Hash:        hash,
		Title:       pr.Title,
		Lang:        defaultLang,
		StemmedText: stemmed,
		Embedding:   vec,
		Source:      o.Source,
		DocType:     "pull_request",
	})
}

// DeleteGitHubDocument removes a synthetic GitHub document by path.
func (o *Orchestrator) DeleteGitHubDocument(ctx context.Context, owner, repo, docType string, number int) error {
	path := fmt.Sprintf("gh:%s/%s/%s/%d", owner, repo, docType, number)
	return o.removePaths(ctx, []string{path})
}

func (o *Orchestrator) loadOrCreateState(ctx context.Context) (*db.SyncState, error) {
	state, err := o.DB.GetSyncState(ctx, o.Source)
	if err == nil {
		return state, nil
	}
	// Not found: create default
	return &db.SyncState{Source: o.Source}, nil
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
	return o.DB.DeleteDocumentsByPaths(ctx, paths)
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

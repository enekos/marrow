package sync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/enekos/marrow/internal/chunker"
	"github.com/enekos/marrow/internal/config"
	"github.com/enekos/marrow/internal/db"
	"github.com/enekos/marrow/internal/embed"
	"github.com/enekos/marrow/internal/githubapi"
	"github.com/enekos/marrow/internal/index"
	"github.com/enekos/marrow/internal/markdown"
	"github.com/enekos/marrow/internal/stemmer"
	"github.com/enekos/marrow/internal/watcher"
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
	defaultLang := o.DefaultLang
	if defaultLang == "" {
		defaultLang = config.DefaultLang
	}

	// Pipeline: N CPU workers parse/stem/embed files in parallel; a single
	// writer goroutine commits results in batched transactions. SQLite allows
	// only one writer, so we never fan out DB work — but CPU prep dominates
	// sync time on large repos and parallelizes cleanly.
	workers := runtime.NumCPU()
	if workers < 2 {
		workers = 2
	}
	if workers > len(paths) {
		workers = len(paths)
	}

	in := make(chan string, workers*2)
	out := make(chan indexJob, workers*2)
	errCh := make(chan error, workers+1)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for p := range in {
				doc, err := prepareDocument(ctx, o.EmbedFn, p, defaultLang, o.Source)
				if err != nil {
					select {
					case errCh <- err:
					default:
					}
					cancel()
					return
				}
				select {
				case out <- indexJob{path: p, doc: doc}:
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	// Feeder.
	go func() {
		defer close(in)
		for _, p := range paths {
			select {
			case in <- p:
			case <-ctx.Done():
				return
			}
		}
	}()

	// Closer: wait for workers then close out.
	go func() {
		wg.Wait()
		close(out)
	}()

	// Writer: batch into transactions of up to batchSize docs each.
	const batchSize = 256
	pending := make([]index.Document, 0, batchSize)
	flush := func() error {
		if len(pending) == 0 {
			return nil
		}
		tx, err := o.DB.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin tx: %w", err)
		}
		for _, d := range pending {
			if err := index.IndexTx(ctx, tx, d); err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("index %s: %w", d.Path, err)
			}
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit: %w", err)
		}
		pending = pending[:0]
		return nil
	}

	for job := range out {
		pending = append(pending, job.doc)
		if len(pending) >= batchSize {
			if err := flush(); err != nil {
				cancel()
				// Drain workers before returning.
				for range out {
				}
				return err
			}
		}
	}
	if err := flush(); err != nil {
		return err
	}
	select {
	case err := <-errCh:
		return err
	default:
	}
	return nil
}

type indexJob struct {
	path string
	doc  index.Document
}

func prepareDocument(ctx context.Context, embedFn embed.Func, path, defaultLang, source string) (index.Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return index.Document{}, fmt.Errorf("read %s: %w", path, err)
	}
	md, err := markdown.ParseWithDefault(data, defaultLang)
	if err != nil {
		return index.Document{}, fmt.Errorf("parse %s: %w", path, err)
	}
	hash := fmt.Sprintf("%x", watcher.HashBytes(data))
	stemmed := stemmer.StemText(md.Text, md.Lang)
	chunks, err := embedChunks(ctx, embedFn, md.Text)
	if err != nil {
		return index.Document{}, fmt.Errorf("embed %s: %w", path, err)
	}
	return index.Document{
		Path:        path,
		Hash:        hash,
		Title:       md.Title,
		Lang:        md.Lang,
		StemmedText: stemmed,
		Chunks:      chunks,
		Source:      source,
		DocType:     "markdown",
	}, nil
}

// embedChunks splits text into chunks and embeds each. Empty text produces a
// single chunk containing the empty string so the document still has a
// vector (consistent with historic behaviour).
func embedChunks(ctx context.Context, fn embed.Func, text string) ([]index.Chunk, error) {
	pieces := chunker.Chunk(text, chunker.DefaultMaxChars)
	if len(pieces) == 0 {
		pieces = []string{""}
	}
	chunks := make([]index.Chunk, 0, len(pieces))
	for i, p := range pieces {
		vec, err := fn(ctx, p)
		if err != nil {
			return nil, err
		}
		chunks = append(chunks, index.Chunk{Index: i, Text: p, Embedding: vec})
	}
	return chunks, nil
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

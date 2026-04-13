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
	"marrow/internal/index"
	"marrow/internal/markdown"
	"marrow/internal/stemmer"
	"marrow/internal/watcher"
)

// Orchestrator coordinates indexing for a given source.
type Orchestrator struct {
	DB      *db.DB
	EmbedFn embed.Func
	Source  string
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
		md, err := markdown.Parse(data)
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

// LocalPathFromSource derives a local directory name from the source identifier.
func LocalPathFromSource(base, source string) string {
	clean := strings.ReplaceAll(source, "/", "-")
	return filepath.Join(base, clean)
}

package sync

import (
	"context"
	"fmt"
	"time"

	"github.com/enekos/marrow/internal/db"
	"github.com/enekos/marrow/internal/embed"
	"github.com/enekos/marrow/internal/watcher"
)

// LocalSyncer performs incremental sync over a local directory tree.
type LocalSyncer struct {
	DB          *db.DB
	EmbedFn     embed.Func
	Source      string
	DefaultLang string
	Root        string
}

// Sync runs an incremental crawl and indexes changed markdown files.
func (s *LocalSyncer) Sync(ctx context.Context) error {
	stateRepo := db.NewSyncStateRepo(s.DB)
	state, err := loadOrCreateState(ctx, stateRepo, s.Source)
	if err != nil {
		return err
	}

	var since time.Time
	if state.LastSyncAt != nil {
		since = *state.LastSyncAt
	}

	crawler := watcher.NewCrawler(s.DB)
	changed, deleted, err := crawler.ScanIncremental(ctx, s.Root, since, s.Source)
	if err != nil {
		return fmt.Errorf("crawl: %w", err)
	}

	paths := make([]string, len(changed))
	for i, fi := range changed {
		paths[i] = fi.Path
	}

	orch := &Orchestrator{DB: s.DB, EmbedFn: s.EmbedFn, DefaultLang: s.DefaultLang, Source: s.Source}
	if err := orch.indexFiles(ctx, paths); err != nil {
		return fmt.Errorf("index: %w", err)
	}
	if err := orch.removePaths(ctx, deleted); err != nil {
		return fmt.Errorf("remove: %w", err)
	}

	now := time.Now().UTC()
	state.LastSyncAt = &now
	return stateRepo.Upsert(ctx, state)
}

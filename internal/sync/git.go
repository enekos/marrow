package sync

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/enekos/marrow/internal/db"
	"github.com/enekos/marrow/internal/embed"
	"github.com/enekos/marrow/internal/gitpull"
)

// GitSyncer clones or pulls a GitHub repo and indexes changed markdown files.
type GitSyncer struct {
	DB          *db.DB
	EmbedFn     embed.Func
	Source      string
	DefaultLang string
	RepoURL     string
	Token       string
	LocalPath   string
}

// Sync runs git pull/clone and indexes changed files.
func (s *GitSyncer) Sync(ctx context.Context) error {
	stateRepo := db.NewSyncStateRepo(s.DB)
	state, err := loadOrCreateState(ctx, stateRepo, s.Source)
	if err != nil {
		return err
	}
	state.RepoURL = s.RepoURL
	state.LocalPath = s.LocalPath
	state.Token = s.Token
	if err := stateRepo.Upsert(ctx, state); err != nil {
		return err
	}

	changed, err := gitpull.Sync(s.RepoURL, s.Token, s.LocalPath)
	if err != nil {
		return fmt.Errorf("git sync: %w", err)
	}

	var mdFiles []string
	for _, p := range changed {
		if strings.HasSuffix(strings.ToLower(p), ".md") {
			mdFiles = append(mdFiles, p)
		}
	}

	orch := &Orchestrator{DB: s.DB, EmbedFn: s.EmbedFn, DefaultLang: s.DefaultLang, Source: s.Source}
	if err := orch.indexFiles(ctx, mdFiles); err != nil {
		return fmt.Errorf("index: %w", err)
	}

	tracked, err := db.NewDocumentRepo(s.DB).GetDocumentPathsBySource(ctx, s.Source)
	if err != nil {
		return fmt.Errorf("list tracked: %w", err)
	}
	var toDelete []string
	for _, p := range tracked {
		if _, err := os.Stat(p); os.IsNotExist(err) {
			toDelete = append(toDelete, p)
		}
	}
	if err := orch.removePaths(ctx, toDelete); err != nil {
		return fmt.Errorf("remove: %w", err)
	}

	now := time.Now().UTC()
	state.LastSyncAt = &now
	return stateRepo.Upsert(ctx, state)
}

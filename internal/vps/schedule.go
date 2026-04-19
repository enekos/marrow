//go:build vps

package vps

import (
	"context"
	"sync"
	"time"

	"log/slog"

	"marrow/internal/config"
	"marrow/internal/service"
)

// StartScheduledSync begins a background ticker that re-syncs all configured
// sources at the given interval. It skips overlapping syncs per source.
func StartScheduledSync(ctx context.Context, logger *slog.Logger, syncer *service.Syncer, cfg *config.Config, interval time.Duration) {
	if interval <= 0 {
		return
	}
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		var mu sync.Mutex
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if !mu.TryLock() {
					logger.Warn("scheduled sync skipped: previous sync still running")
					continue
				}
				go func() {
					defer mu.Unlock()
					runAllSyncs(ctx, logger, syncer, cfg)
				}()
			}
		}
	}()
}

func runAllSyncs(ctx context.Context, logger *slog.Logger, syncer *service.Syncer, cfg *config.Config) {
	for _, s := range cfg.Sources {
		s := s
		src := s.Name
		if src == "" {
			src = s.Type
		}
		lang := s.DefaultLang
		if lang == "" {
			lang = cfg.Search.DefaultLang
		}
		if lang == "" {
			lang = config.DefaultLang
		}
		syncCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
		cfgCopy := s
		cfgCopy.Name = src
		cfgCopy.DefaultLang = lang
		logger.Info("scheduled sync starting", "source", src)
		start := time.Now()
		if err := syncer.SyncSource(syncCtx, cfgCopy, cfg.Search.DefaultLang); err != nil {
			logger.Error("scheduled sync failed", "source", src, "err", err)
		} else {
			logger.Info("scheduled sync complete", "source", src, "duration", time.Since(start).String())
		}
		cancel()
	}
}

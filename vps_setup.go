//go:build vps

package main

import (
	"context"
	"time"

	"marrow/internal/config"
	"marrow/internal/server"
	"marrow/internal/service"
	"marrow/internal/vps"
)

func setupVPSServer(ctx context.Context, srv *server.Server, cfg *config.Config, syncer *service.Syncer) {
	vps.Setup(srv, cfg)

	interval, err := time.ParseDuration(cfg.Server.SyncInterval)
	if err != nil || interval <= 0 {
		interval = 15 * time.Minute
	}
	vps.StartScheduledSync(ctx, srv.Logger, syncer, cfg, interval)
}

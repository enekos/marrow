//go:build !vps

package main

import (
	"context"

	"marrow/internal/config"
	"marrow/internal/server"
	"marrow/internal/service"
)

func setupVPSServer(ctx context.Context, srv *server.Server, cfg *config.Config, syncer *service.Syncer) {
	// No-op when VPS features are not compiled in.
}

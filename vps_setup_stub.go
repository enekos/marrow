//go:build !vps

package main

import (
	"context"

	"github.com/enekos/marrow/internal/config"
	"github.com/enekos/marrow/internal/server"
	"github.com/enekos/marrow/internal/service"
)

func setupVPSServer(ctx context.Context, srv *server.Server, cfg *config.Config, syncer *service.Syncer) {
	// No-op when VPS features are not compiled in.
}

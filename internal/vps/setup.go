//go:build vps

package vps

import (
	"github.com/enekos/marrow/internal/config"
	"github.com/enekos/marrow/internal/server"
)

// Setup wires all production middleware and background jobs into the server.
func Setup(srv *server.Server, cfg *config.Config) {
	maxBody := cfg.Server.MaxBodySize
	if maxBody <= 0 {
		maxBody = 1024 * 1024 // 1MB default
	}

	srv.WrapHandler = middlewareChain(
		securityHeadersMiddleware(),
		siteResolution(cfg),
		corsMiddleware(cfg),
		authMiddleware(cfg),
		rateLimitMiddleware(cfg),
		bodyLimitMiddleware(maxBody),
		requestLogMiddleware(srv.Logger),
	)
}

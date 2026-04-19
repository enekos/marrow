//go:build vps

package vps

import (
	"marrow/internal/config"
	"marrow/internal/server"
)

// Setup wires all production middleware and background jobs into the server.
func Setup(srv *server.Server, cfg *config.Config) {
	srv.WrapHandler = middlewareChain(
		siteResolution(cfg),
		corsMiddleware(cfg),
		authMiddleware(),
		rateLimitMiddleware(),
		requestLogMiddleware(srv.Logger),
	)
}

package server

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/enekos/marrow/internal/config"
	"github.com/enekos/marrow/internal/db"
	"github.com/enekos/marrow/internal/embed"
	"github.com/enekos/marrow/internal/githubapi"
	"github.com/enekos/marrow/internal/service"
)

const shutdownTimeout = 10 * time.Second

// contextKey is a private type for context values.
type contextKey string

const siteContextKey contextKey = "marrow.site"

// WithSite returns a context with the site config attached.
func WithSite(ctx context.Context, site *config.SiteConfig) context.Context {
	return context.WithValue(ctx, siteContextKey, site)
}

// SiteFromContext retrieves the site config from the context.
func SiteFromContext(ctx context.Context) *config.SiteConfig {
	site, _ := ctx.Value(siteContextKey).(*config.SiteConfig)
	return site
}

// Server holds HTTP handlers and their dependencies.
type Server struct {
	Logger          *slog.Logger
	Searcher        *service.Searcher
	Syncer          *service.Syncer
	StatsRepo       *db.StatsRepo
	StateRepo       *db.SyncStateRepo
	Database        *db.DB
	EmbedFn         embed.Func
	IndexHTML       []byte
	GHClient        *githubapi.Client
	GHRepoOwner     string
	GHRepoName      string
	GHWebhookSecret string
	DefaultLang     string
	WebhookSource   string
	Config          *config.Config
	WrapHandler     func(http.Handler) http.Handler
	httpServer      *http.Server
}

// New creates a new Server with the given dependencies.
func New(logger *slog.Logger, searcher *service.Searcher, syncer *service.Syncer, database *db.DB, embedFn embed.Func, indexHTML []byte) *Server {
	return &Server{
		Logger:    logger,
		Searcher:  searcher,
		Syncer:    syncer,
		StatsRepo: db.NewStatsRepo(database),
		StateRepo: db.NewSyncStateRepo(database),
		Database:  database,
		EmbedFn:   embedFn,
		IndexHTML: indexHTML,
	}
}

// RegisterRoutes wires all handlers to the provided mux.
func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /{$}", s.handleIndex)
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /stats", s.handleStats)
	mux.HandleFunc("POST /search", s.handleSearch)
	mux.HandleFunc("POST /webhook", s.handleWebhook)
}

// Run starts the HTTP server and blocks until the provided context is cancelled,
// then performs a graceful shutdown.
func (s *Server) Run(ctx context.Context, addr string) error {
	mux := http.NewServeMux()
	s.RegisterRoutes(mux)

	handler := http.Handler(mux)
	if s.WrapHandler != nil {
		handler = s.WrapHandler(handler)
	}
	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	errCh := make(chan error, 1)
	go func() {
		s.Logger.Info("server listening", "addr", addr)
		errCh <- s.httpServer.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		s.Logger.Info("server shutting down gracefully")
		return s.httpServer.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}

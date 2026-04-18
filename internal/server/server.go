package server

import (
	"log/slog"
	"net/http"

	"marrow/internal/db"
	"marrow/internal/embed"
	"marrow/internal/githubapi"
	"marrow/internal/service"
)

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

// ListenAndServe starts the HTTP server on addr.
func (s *Server) ListenAndServe(addr string) error {
	mux := http.NewServeMux()
	s.RegisterRoutes(mux)
	s.Logger.Info("server listening", "addr", addr)
	return http.ListenAndServe(addr, mux)
}

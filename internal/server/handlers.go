package server

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/enekos/marrow/internal/db"
)

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(s.IndexHTML)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	dbStatus := "ok"
	if err := s.Database.PingContext(r.Context()); err != nil {
		dbStatus = "error"
	}

	stats, err := s.StatsRepo.Get(r.Context())
	if err != nil {
		stats = &db.Stats{}
	}

	resp := map[string]any{
		"status":        "ok",
		"db":            dbStatus,
		"total_docs":    stats.TotalDocs,
		"sources_total": len(s.Config.Sources),
		"sites_total":   len(s.Config.Sites),
	}
	if stats.LastSyncAt != nil {
		resp["last_sync"] = stats.LastSyncAt.Format(time.RFC3339)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.StatsRepo.Get(r.Context())
	if err != nil {
		s.Logger.Error("stats error", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Query   string `json:"q"`
		Limit   int    `json:"limit"`
		Lang    string `json:"lang"`
		Source  string `json:"source"`
		DocType string `json:"doc_type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	if req.Limit <= 0 {
		req.Limit = 10
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// If a site is resolved via middleware, constrain search to its sources.
	site := SiteFromContext(r.Context())

	results, err := s.Searcher.Search(ctx, req.Query, req.Limit, req.Source, req.DocType, req.Lang, site)
	if err != nil {
		s.Logger.Error("search error", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"results": results,
	})
}

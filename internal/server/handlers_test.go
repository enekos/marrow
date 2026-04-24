package server

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/enekos/marrow/internal/config"
	"github.com/enekos/marrow/internal/db"
	"github.com/enekos/marrow/internal/index"
	"github.com/enekos/marrow/internal/search"
	"github.com/enekos/marrow/internal/service"
)

func mockEmbed(ctx context.Context, text string) ([]float32, error) {
	v := make([]float32, 384)
	for i := range v {
		v[i] = 0.01 * float32(i%10)
	}
	return v, nil
}

func setupTestServer(t *testing.T) (*Server, *db.DB) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	engine := search.NewEngine(database, mockEmbed)
	searcher := &service.Searcher{Engine: engine}
	syncer := &service.Syncer{DB: database, EmbedFn: mockEmbed}

	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	srv := New(logger, searcher, syncer, database, mockEmbed, []byte("<html></html>"))
	srv.Config = &config.Config{}
	return srv, database
}

func seedDocs(t *testing.T, database *db.DB) {
	ctx := context.Background()
	ix := index.NewIndexer(database)
	for _, doc := range []struct {
		path   string
		title  string
		source string
	}{
		{"blog/post1.md", "Hello Blog", "blog-md"},
		{"docs/page1.md", "Hello Docs", "docs-md"},
	} {
		vec, _ := mockEmbed(ctx, doc.title)
		if err := ix.Index(ctx, index.Document{
			Path:        doc.path,
			Hash:        "hash",
			Title:       doc.title,
			Lang:        "en",
			Source:      doc.source,
			DocType:     "markdown",
			StemmedText: doc.title,
			Embedding:   vec,
		}); err != nil {
			t.Fatalf("index %q: %v", doc.path, err)
		}
	}
}

func TestHandleHealth(t *testing.T) {
	srv, database := setupTestServer(t)
	defer database.Close()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	srv.handleHealth(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("status = %v, want ok", resp["status"])
	}
	if resp["db"] != "ok" {
		t.Errorf("db = %v, want ok", resp["db"])
	}
	if resp["sources_total"] == nil {
		t.Error("expected sources_total in response")
	}
	if resp["sites_total"] == nil {
		t.Error("expected sites_total in response")
	}
}

func TestHandleSearch_NoSite(t *testing.T) {
	srv, database := setupTestServer(t)
	defer database.Close()
	seedDocs(t, database)

	body := `{"q":"hello","limit":10}`
	req := httptest.NewRequest(http.MethodPost, "/search", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.handleSearch(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var resp struct {
		Results []search.Result `json:"results"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Results) != 2 {
		t.Errorf("results count = %d, want 2", len(resp.Results))
	}
}

func TestHandleSearch_WithSite(t *testing.T) {
	srv, database := setupTestServer(t)
	defer database.Close()
	seedDocs(t, database)

	srv.Config.Sites = []config.SiteConfig{
		{Name: "blog", Sources: []string{"blog-md"}},
	}

	body := `{"q":"hello","limit":10}`
	req := httptest.NewRequest(http.MethodPost, "/search", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(WithSite(req.Context(), &srv.Config.Sites[0]))
	rr := httptest.NewRecorder()
	srv.handleSearch(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var resp struct {
		Results []search.Result `json:"results"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Results) != 1 {
		t.Errorf("results count = %d, want 1", len(resp.Results))
	}
	if resp.Results[0].Path != "blog/post1.md" {
		t.Errorf("path = %q, want blog/post1.md", resp.Results[0].Path)
	}
}

func TestHandleSearch_InvalidJSON(t *testing.T) {
	srv, database := setupTestServer(t)
	defer database.Close()

	req := httptest.NewRequest(http.MethodPost, "/search", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.handleSearch(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleSearch_EmptyQuery(t *testing.T) {
	srv, database := setupTestServer(t)
	defer database.Close()

	body := `{"q":"","limit":10}`
	req := httptest.NewRequest(http.MethodPost, "/search", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.handleSearch(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp struct {
		Results []search.Result `json:"results"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Results) != 0 {
		t.Errorf("results count = %d, want 0", len(resp.Results))
	}
}

func TestHandleStats(t *testing.T) {
	srv, database := setupTestServer(t)
	defer database.Close()
	seedDocs(t, database)

	req := httptest.NewRequest(http.MethodGet, "/stats", nil)
	rr := httptest.NewRecorder()
	srv.handleStats(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp db.Stats
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.TotalDocs != 2 {
		t.Errorf("total_docs = %d, want 2", resp.TotalDocs)
	}
}

func TestHandleIndex(t *testing.T) {
	srv, database := setupTestServer(t)
	defer database.Close()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	srv.handleIndex(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Errorf("content-type = %q, want text/html; charset=utf-8", ct)
	}
	if body := rr.Body.String(); body != "<html></html>" {
		t.Errorf("body = %q, want <html></html>", body)
	}
}

func TestServerContextHelpers(t *testing.T) {
	ctx := context.Background()
	if SiteFromContext(ctx) != nil {
		t.Error("expected nil site from empty context")
	}

	site := &config.SiteConfig{Name: "test"}
	ctx = WithSite(ctx, site)
	if got := SiteFromContext(ctx); got == nil || got.Name != "test" {
		t.Errorf("site = %v, want test", got)
	}
}

func TestServerWrapHandler(t *testing.T) {
	srv, database := setupTestServer(t)
	defer database.Close()

	called := false
	srv.WrapHandler = func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			next.ServeHTTP(w, r)
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Run server in background.
	go func() {
		_ = srv.Run(ctx, "127.0.0.1:19090")
	}()

	// Give server a moment to start.
	time.Sleep(100 * time.Millisecond)

	req, _ := http.NewRequest(http.MethodGet, "http://127.0.0.1:19090/health", nil)
	client := &http.Client{Timeout: 1 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request error: %v", err)
	}
	resp.Body.Close()

	if !called {
		t.Error("expected WrapHandler to be called")
	}
	cancel()
}

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

func TestHandleSearch_HighlightHTML(t *testing.T) {
	srv, database := setupTestServer(t)
	defer database.Close()

	ctx := context.Background()
	ix := index.NewIndexer(database)
	vec, _ := mockEmbed(ctx, "anything")
	if err := ix.Index(ctx, index.Document{
		Path:        "blog/script.md",
		Hash:        "h",
		Title:       "Script Post",
		Lang:        "en",
		Source:      "blog-md",
		DocType:     "markdown",
		StemmedText: "the rocket flies past <script> tags safely",
		Embedding:   vec,
	}); err != nil {
		t.Fatalf("index: %v", err)
	}

	body := `{"q":"rocket","limit":10,"highlight_format":"html"}`
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
	if len(resp.Results) == 0 {
		t.Fatalf("expected at least one result")
	}
	snip := resp.Results[0].Snippet
	if !strings.Contains(snip, "<mark>") || !strings.Contains(snip, "</mark>") {
		t.Errorf("snippet missing <mark> tags: %q", snip)
	}
	if !strings.Contains(snip, "&lt;script&gt;") {
		t.Errorf("snippet did not HTML-escape <script>: %q", snip)
	}
	if strings.Contains(snip, "<script>") {
		t.Errorf("snippet contains raw <script>, unsafe: %q", snip)
	}
}

func TestHandleSearch_HighlightPlainDefault(t *testing.T) {
	srv, database := setupTestServer(t)
	defer database.Close()

	ctx := context.Background()
	ix := index.NewIndexer(database)
	vec, _ := mockEmbed(ctx, "anything")
	if err := ix.Index(ctx, index.Document{
		Path:        "blog/plain.md",
		Hash:        "h",
		Title:       "Plain",
		Lang:        "en",
		Source:      "blog-md",
		DocType:     "markdown",
		StemmedText: "rocket science is fun",
		Embedding:   vec,
	}); err != nil {
		t.Fatalf("index: %v", err)
	}

	body := `{"q":"rocket","limit":10}`
	req := httptest.NewRequest(http.MethodPost, "/search", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.handleSearch(rr, req)

	var resp struct {
		Results []search.Result `json:"results"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Results) == 0 {
		t.Fatalf("expected at least one result")
	}
	if strings.Contains(resp.Results[0].Snippet, "<mark>") {
		t.Errorf("default snippet should not contain <mark>: %q", resp.Results[0].Snippet)
	}
}

func TestHandleFacets_NoSite(t *testing.T) {
	srv, database := setupTestServer(t)
	defer database.Close()
	seedDocs(t, database)

	req := httptest.NewRequest(http.MethodGet, "/facets", nil)
	rr := httptest.NewRecorder()
	srv.handleFacets(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var resp db.Facets
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Sources) != 2 {
		t.Errorf("sources count = %d, want 2 (got %+v)", len(resp.Sources), resp.Sources)
	}
	gotSources := map[string]int64{}
	for _, fv := range resp.Sources {
		gotSources[fv.Value] = fv.Count
	}
	if gotSources["blog-md"] != 1 || gotSources["docs-md"] != 1 {
		t.Errorf("source counts = %v, want blog-md:1 docs-md:1", gotSources)
	}
	if len(resp.DocTypes) != 1 || resp.DocTypes[0].Value != "markdown" || resp.DocTypes[0].Count != 2 {
		t.Errorf("doc_types = %+v, want [{markdown 2}]", resp.DocTypes)
	}
	if len(resp.Langs) != 1 || resp.Langs[0].Value != "en" || resp.Langs[0].Count != 2 {
		t.Errorf("langs = %+v, want [{en 2}]", resp.Langs)
	}
}

func TestHandleFacets_WithSite(t *testing.T) {
	srv, database := setupTestServer(t)
	defer database.Close()
	seedDocs(t, database)

	srv.Config.Sites = []config.SiteConfig{
		{Name: "blog", Sources: []string{"blog-md"}},
	}

	req := httptest.NewRequest(http.MethodGet, "/facets", nil)
	req = req.WithContext(WithSite(req.Context(), &srv.Config.Sites[0]))
	rr := httptest.NewRecorder()
	srv.handleFacets(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var resp db.Facets
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Sources) != 1 || resp.Sources[0].Value != "blog-md" {
		t.Errorf("sources = %+v, want only blog-md", resp.Sources)
	}
	if resp.DocTypes[0].Count != 1 {
		t.Errorf("doc_types count = %d, want 1 (site-restricted)", resp.DocTypes[0].Count)
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

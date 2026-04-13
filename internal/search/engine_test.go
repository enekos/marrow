package search

import (
	"context"
	"path/filepath"
	"testing"

	"marrow/internal/db"
	"marrow/internal/embed"
	"marrow/internal/index"
)

func setupTestDB(t *testing.T) *db.DB {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

func TestSearch_HybridRankingAndTitleBoost(t *testing.T) {
	ctx := context.Background()
	database := setupTestDB(t)
	ix := index.NewIndexer(database)
	embedFn := embed.NewMock()

	docs := []index.Document{
		{Path: "/a.md", Hash: "1", Title: "Go Programming", Lang: "en", Source: "test", StemmedText: "go program languag", Embedding: mustEmbed(embedFn, "go programming language")},
		{Path: "/b.md", Hash: "2", Title: "Rust Programming", Lang: "en", Source: "test", StemmedText: "rust program languag", Embedding: mustEmbed(embedFn, "rust programming language")},
		{Path: "/c.md", Hash: "3", Title: "Go Best Practices", Lang: "en", Source: "test", StemmedText: "go best practic", Embedding: mustEmbed(embedFn, "go best practices")},
	}
	for _, d := range docs {
		if err := ix.Index(ctx, d); err != nil {
			t.Fatalf("index doc: %v", err)
		}
	}

	engine := NewEngine(database, embedFn)

	// Search for "go" — all three docs contain it, but /a.md and /c.md have "go" in title.
	results, err := engine.Search(ctx, "go", 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// Title-boosted docs should outrank the non-title doc.
	if results[0].Title != "Go Programming" && results[0].Title != "Go Best Practices" {
		t.Errorf("expected title-boosted doc first, got %s", results[0].Title)
	}
	if results[1].Title != "Go Programming" && results[1].Title != "Go Best Practices" {
		t.Errorf("expected title-boosted doc second, got %s", results[1].Title)
	}
	if results[2].Title != "Rust Programming" {
		t.Errorf("expected non-title doc last, got %s", results[2].Title)
	}

	// Scores should reflect title boost
	if results[0].Score < results[2].Score {
		t.Errorf("expected highest score for top result, got %f < %f", results[0].Score, results[2].Score)
	}
}

func TestSearch_EmptyQuery(t *testing.T) {
	ctx := context.Background()
	database := setupTestDB(t)
	engine := NewEngine(database, embed.NewMock())
	results, err := engine.Search(ctx, "", 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results for empty query, got %d", len(results))
	}
}

func mustEmbed(fn embed.Func, text string) []float32 {
	v, err := fn(context.Background(), text)
	if err != nil {
		panic(err)
	}
	return v
}

package index

import (
	"context"
	"path/filepath"
	"testing"

	"marrow/internal/db"
	"marrow/internal/embed"
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

func TestIndex_InsertAndUpdate(t *testing.T) {
	ctx := context.Background()
	database := setupTestDB(t)
	ix := NewIndexer(database)
	embedFn := embed.NewMock()

	doc := Document{
		Path:        "/notes/go.md",
		Hash:        "abc123",
		Title:       "Go Notes",
		Lang:        "en",
		Source:      "local",
		StemmedText: "go note",
		Embedding:   mustEmbed(embedFn, "go notes"),
	}

	// Insert
	if err := ix.Index(ctx, doc); err != nil {
		t.Fatalf("first index: %v", err)
	}

	var count int
	if err := database.QueryRow(`SELECT COUNT(*) FROM documents WHERE path = ?`, doc.Path).Scan(&count); err != nil {
		t.Fatalf("count documents: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 document, got %d", count)
	}

	// Update with new hash
	doc.Hash = "def456"
	doc.Title = "Updated Go Notes"
	doc.StemmedText = "updat go note"
	doc.Embedding = mustEmbed(embedFn, "updated go notes")
	if err := ix.Index(ctx, doc); err != nil {
		t.Fatalf("second index: %v", err)
	}

	var title string
	if err := database.QueryRow(`SELECT title FROM documents WHERE path = ?`, doc.Path).Scan(&title); err != nil {
		t.Fatalf("select title: %v", err)
	}
	if title != "Updated Go Notes" {
		t.Fatalf("expected updated title, got %s", title)
	}

	// Verify FTS updated
	var ftsTitle string
	if err := database.QueryRow(`SELECT title FROM documents_fts WHERE rowid = (SELECT id FROM documents WHERE path = ?)`, doc.Path).Scan(&ftsTitle); err != nil {
		t.Fatalf("select fts title: %v", err)
	}
	if ftsTitle != "Updated Go Notes" {
		t.Fatalf("expected fts title updated, got %s", ftsTitle)
	}
}

func mustEmbed(fn embed.Func, text string) []float32 {
	v, err := fn(context.Background(), text)
	if err != nil {
		panic(err)
	}
	return v
}

package sync

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

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

func TestRunLocal_IncrementalAndDelete(t *testing.T) {
	ctx := context.Background()
	database := setupTestDB(t)
	dir := t.TempDir()

	orch := &Orchestrator{
		DB:      database,
		EmbedFn: embed.NewMock(),
		Source:  "test",
	}

	// First sync
	f1 := filepath.Join(dir, "a.md")
	if err := os.WriteFile(f1, []byte("# Hello\nWorld"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := orch.RunLocal(ctx, dir); err != nil {
		t.Fatalf("first run: %v", err)
	}

	var count1 int
	if err := database.QueryRow(`SELECT COUNT(*) FROM documents WHERE source = 'test'`).Scan(&count1); err != nil {
		t.Fatal(err)
	}
	if count1 != 1 {
		t.Fatalf("expected 1 doc, got %d", count1)
	}

	// Wait a bit to ensure mtime changes
	time.Sleep(50 * time.Millisecond)

	// Second sync: add one, delete one
	f2 := filepath.Join(dir, "b.md")
	if err := os.WriteFile(f2, []byte("# Second\nDoc"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(f1); err != nil {
		t.Fatal(err)
	}
	if err := orch.RunLocal(ctx, dir); err != nil {
		t.Fatalf("second run: %v", err)
	}

	var count2 int
	if err := database.QueryRow(`SELECT COUNT(*) FROM documents WHERE source = 'test'`).Scan(&count2); err != nil {
		t.Fatal(err)
	}
	if count2 != 1 {
		t.Fatalf("expected 1 doc after deletion, got %d", count2)
	}

	var title string
	if err := database.QueryRow(`SELECT title FROM documents WHERE source = 'test'`).Scan(&title); err != nil {
		t.Fatal(err)
	}
	if title != "Second" {
		t.Fatalf("expected title 'Second', got %s", title)
	}
}

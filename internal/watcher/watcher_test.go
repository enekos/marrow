package watcher

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/enekos/marrow/internal/db"
)

func openTestDB(t *testing.T) *db.DB {
	t.Helper()
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

func TestHashBytes(t *testing.T) {
	data := []byte("hello")
	got := HashBytes(data)
	want := sha256.Sum256(data)
	if got != want {
		t.Errorf("HashBytes(%q) = %x, want %x", data, got, want)
	}
}

func TestCrawler_checkFile(t *testing.T) {
	ctx := context.Background()
	source := "test-source"

	t.Run("new file", func(t *testing.T) {
		database := openTestDB(t)
		c := NewCrawler(database)
		dir := t.TempDir()
		path := filepath.Join(dir, "new.md")
		content := []byte("new content")
		if err := os.WriteFile(path, content, 0644); err != nil {
			t.Fatal(err)
		}

		fi, err := c.checkFile(ctx, path, source)
		if err != nil {
			t.Fatalf("checkFile error: %v", err)
		}
		if fi == nil {
			t.Fatal("expected FileInfo, got nil")
		}
		if fi.Path != path {
			t.Errorf("Path = %q, want %q", fi.Path, path)
		}
		wantHash := fmt.Sprintf("%x", HashBytes(content))
		if fi.Hash != wantHash {
			t.Errorf("Hash = %q, want %q", fi.Hash, wantHash)
		}
	})

	t.Run("unchanged file", func(t *testing.T) {
		database := openTestDB(t)
		c := NewCrawler(database)
		dir := t.TempDir()
		path := filepath.Join(dir, "unchanged.md")
		content := []byte("unchanged content")
		if err := os.WriteFile(path, content, 0644); err != nil {
			t.Fatal(err)
		}
		hash := fmt.Sprintf("%x", HashBytes(content))
		if _, err := database.ExecContext(ctx, `INSERT INTO documents (path, hash, source) VALUES (?, ?, ?)`, path, hash, source); err != nil {
			t.Fatalf("insert doc: %v", err)
		}

		fi, err := c.checkFile(ctx, path, source)
		if err != nil {
			t.Fatalf("checkFile error: %v", err)
		}
		if fi != nil {
			t.Errorf("expected nil, got %+v", fi)
		}
	})

	t.Run("modified file", func(t *testing.T) {
		database := openTestDB(t)
		c := NewCrawler(database)
		dir := t.TempDir()
		path := filepath.Join(dir, "modified.md")
		content := []byte("original content")
		if err := os.WriteFile(path, content, 0644); err != nil {
			t.Fatal(err)
		}
		oldHash := fmt.Sprintf("%x", HashBytes(content))
		if _, err := database.ExecContext(ctx, `INSERT INTO documents (path, hash, source) VALUES (?, ?, ?)`, path, oldHash, source); err != nil {
			t.Fatalf("insert doc: %v", err)
		}

		// Modify file
		newContent := []byte("modified content")
		if err := os.WriteFile(path, newContent, 0644); err != nil {
			t.Fatal(err)
		}

		fi, err := c.checkFile(ctx, path, source)
		if err != nil {
			t.Fatalf("checkFile error: %v", err)
		}
		if fi == nil {
			t.Fatal("expected FileInfo, got nil")
		}
		wantHash := fmt.Sprintf("%x", HashBytes(newContent))
		if fi.Hash != wantHash {
			t.Errorf("Hash = %q, want %q", fi.Hash, wantHash)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		database := openTestDB(t)
		c := NewCrawler(database)
		path := filepath.Join(t.TempDir(), "missing.md")

		_, err := c.checkFile(ctx, path, source)
		if err == nil {
			t.Fatal("expected error for missing file")
		}
	})
}

func TestCrawler_ScanIncremental(t *testing.T) {
	ctx := context.Background()
	source := "test-source"

	t.Run("empty directory", func(t *testing.T) {
		database := openTestDB(t)
		c := NewCrawler(database)
		dir := t.TempDir()

		files, deleted, err := c.ScanIncremental(ctx, dir, time.Time{}, source)
		if err != nil {
			t.Fatalf("ScanIncremental error: %v", err)
		}
		if len(files) != 0 {
			t.Errorf("expected 0 files, got %d", len(files))
		}
		if len(deleted) != 0 {
			t.Errorf("expected 0 deleted, got %d", len(deleted))
		}
	})

	t.Run("finds new files", func(t *testing.T) {
		database := openTestDB(t)
		c := NewCrawler(database)
		dir := t.TempDir()
		path := filepath.Join(dir, "new.md")
		content := []byte("hello world")
		if err := os.WriteFile(path, content, 0644); err != nil {
			t.Fatal(err)
		}

		files, deleted, err := c.ScanIncremental(ctx, dir, time.Time{}, source)
		if err != nil {
			t.Fatalf("ScanIncremental error: %v", err)
		}
		if len(files) != 1 {
			t.Fatalf("expected 1 file, got %d", len(files))
		}
		if len(deleted) != 0 {
			t.Errorf("expected 0 deleted, got %d", len(deleted))
		}
		if files[0].Path != path {
			t.Errorf("Path = %q, want %q", files[0].Path, path)
		}
		wantHash := fmt.Sprintf("%x", HashBytes(content))
		if files[0].Hash != wantHash {
			t.Errorf("Hash = %q, want %q", files[0].Hash, wantHash)
		}
	})

	t.Run("skips unchanged files", func(t *testing.T) {
		database := openTestDB(t)
		c := NewCrawler(database)
		dir := t.TempDir()
		path := filepath.Join(dir, "unchanged.md")
		content := []byte("no change")
		if err := os.WriteFile(path, content, 0644); err != nil {
			t.Fatal(err)
		}
		hash := fmt.Sprintf("%x", HashBytes(content))
		if _, err := database.ExecContext(ctx, `INSERT INTO documents (path, hash, source) VALUES (?, ?, ?)`, path, hash, source); err != nil {
			t.Fatalf("insert doc: %v", err)
		}

		files, deleted, err := c.ScanIncremental(ctx, dir, time.Time{}, source)
		if err != nil {
			t.Fatalf("ScanIncremental error: %v", err)
		}
		if len(files) != 0 {
			t.Errorf("expected 0 files, got %d", len(files))
		}
		if len(deleted) != 0 {
			t.Errorf("expected 0 deleted, got %d", len(deleted))
		}
	})

	t.Run("detects modified files", func(t *testing.T) {
		database := openTestDB(t)
		c := NewCrawler(database)
		dir := t.TempDir()
		path := filepath.Join(dir, "modified.md")
		content := []byte("original")
		if err := os.WriteFile(path, content, 0644); err != nil {
			t.Fatal(err)
		}
		oldHash := fmt.Sprintf("%x", HashBytes(content))
		if _, err := database.ExecContext(ctx, `INSERT INTO documents (path, hash, source) VALUES (?, ?, ?)`, path, oldHash, source); err != nil {
			t.Fatalf("insert doc: %v", err)
		}

		// Modify file
		newContent := []byte("modified")
		if err := os.WriteFile(path, newContent, 0644); err != nil {
			t.Fatal(err)
		}

		files, deleted, err := c.ScanIncremental(ctx, dir, time.Time{}, source)
		if err != nil {
			t.Fatalf("ScanIncremental error: %v", err)
		}
		if len(files) != 1 {
			t.Fatalf("expected 1 file, got %d", len(files))
		}
		if len(deleted) != 0 {
			t.Errorf("expected 0 deleted, got %d", len(deleted))
		}
		wantHash := fmt.Sprintf("%x", HashBytes(newContent))
		if files[0].Hash != wantHash {
			t.Errorf("Hash = %q, want %q", files[0].Hash, wantHash)
		}
	})

	t.Run("since skips old unchanged files", func(t *testing.T) {
		database := openTestDB(t)
		c := NewCrawler(database)
		dir := t.TempDir()
		path := filepath.Join(dir, "old.md")
		content := []byte("old content")
		if err := os.WriteFile(path, content, 0644); err != nil {
			t.Fatal(err)
		}
		// Set file modification time to the past
		oldTime := time.Now().Add(-24 * time.Hour)
		if err := os.Chtimes(path, oldTime, oldTime); err != nil {
			t.Fatal(err)
		}
		hash := fmt.Sprintf("%x", HashBytes(content))
		if _, err := database.ExecContext(ctx, `INSERT INTO documents (path, hash, source) VALUES (?, ?, ?)`, path, hash, source); err != nil {
			t.Fatalf("insert doc: %v", err)
		}

		since := time.Now().Add(-1 * time.Hour)
		files, deleted, err := c.ScanIncremental(ctx, dir, since, source)
		if err != nil {
			t.Fatalf("ScanIncremental error: %v", err)
		}
		if len(files) != 0 {
			t.Errorf("expected 0 files, got %d", len(files))
		}
		if len(deleted) != 0 {
			t.Errorf("expected 0 deleted, got %d", len(deleted))
		}
	})

	t.Run("detects deleted files", func(t *testing.T) {
		database := openTestDB(t)
		c := NewCrawler(database)
		dir := t.TempDir()
		path := filepath.Join(dir, "deleted.md")
		// Do not create the file
		hash := fmt.Sprintf("%x", HashBytes([]byte("does not matter")))
		if _, err := database.ExecContext(ctx, `INSERT INTO documents (path, hash, source) VALUES (?, ?, ?)`, path, hash, source); err != nil {
			t.Fatalf("insert doc: %v", err)
		}

		files, deleted, err := c.ScanIncremental(ctx, dir, time.Time{}, source)
		if err != nil {
			t.Fatalf("ScanIncremental error: %v", err)
		}
		if len(files) != 0 {
			t.Errorf("expected 0 files, got %d", len(files))
		}
		if len(deleted) != 1 {
			t.Fatalf("expected 1 deleted, got %d", len(deleted))
		}
		if deleted[0] != path {
			t.Errorf("deleted path = %q, want %q", deleted[0], path)
		}
	})

	t.Run("ignores non-md files", func(t *testing.T) {
		database := openTestDB(t)
		c := NewCrawler(database)
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("text"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "note.md"), []byte("markdown"), 0644); err != nil {
			t.Fatal(err)
		}

		files, deleted, err := c.ScanIncremental(ctx, dir, time.Time{}, source)
		if err != nil {
			t.Fatalf("ScanIncremental error: %v", err)
		}
		if len(files) != 1 {
			t.Fatalf("expected 1 file, got %d", len(files))
		}
		if filepath.Base(files[0].Path) != "note.md" {
			t.Errorf("expected note.md, got %q", files[0].Path)
		}
		if len(deleted) != 0 {
			t.Errorf("expected 0 deleted, got %d", len(deleted))
		}
	})
}

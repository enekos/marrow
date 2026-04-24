package sync

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/enekos/marrow/internal/db"
	"github.com/enekos/marrow/internal/embed"
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

func runGit(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %v: %w (output: %s)", args, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func TestRunGit(t *testing.T) {
	ctx := context.Background()
	database := setupTestDB(t)

	// Create a bare upstream repo
	upstreamDir := t.TempDir()
	if err := runGit(upstreamDir, "init", "--bare"); err != nil {
		t.Fatalf("init bare: %v", err)
	}
	if err := runGit(upstreamDir, "symbolic-ref", "HEAD", "refs/heads/main"); err != nil {
		t.Fatalf("set head: %v", err)
	}

	// Create a local clone, add a markdown file, and push
	cloneDir := t.TempDir()
	if err := runGit(cloneDir, "clone", "file://"+upstreamDir, "."); err != nil {
		t.Fatalf("clone: %v", err)
	}
	if err := runGit(cloneDir, "config", "user.email", "test@test.com"); err != nil {
		t.Fatal(err)
	}
	if err := runGit(cloneDir, "config", "user.name", "Test"); err != nil {
		t.Fatal(err)
	}

	mdFile := filepath.Join(cloneDir, "readme.md")
	if err := os.WriteFile(mdFile, []byte("# Hello\nWorld"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runGit(cloneDir, "add", "."); err != nil {
		t.Fatal(err)
	}
	if err := runGit(cloneDir, "commit", "-m", "initial"); err != nil {
		t.Fatal(err)
	}
	if err := runGit(cloneDir, "push", "origin", "HEAD:main"); err != nil {
		_ = runGit(cloneDir, "push", "origin", "HEAD:master")
	}

	targetDir := t.TempDir()
	upstreamURL := "file://" + upstreamDir

	orch := &Orchestrator{
		DB:          database,
		EmbedFn:     embed.NewMock(),
		Source:      "git-test",
		DefaultLang: "en",
	}

	if err := orch.RunGit(ctx, upstreamURL, "", targetDir); err != nil {
		t.Fatalf("first run: %v", err)
	}

	var count1 int
	if err := database.QueryRow(`SELECT COUNT(*) FROM documents WHERE source = 'git-test'`).Scan(&count1); err != nil {
		t.Fatal(err)
	}
	if count1 != 1 {
		t.Fatalf("expected 1 doc, got %d", count1)
	}

	var title1 string
	if err := database.QueryRow(`SELECT title FROM documents WHERE source = 'git-test'`).Scan(&title1); err != nil {
		t.Fatal(err)
	}
	if title1 != "Hello" {
		t.Fatalf("expected title 'Hello', got %s", title1)
	}

	// Add another markdown file and push
	mdFile2 := filepath.Join(cloneDir, "guide.md")
	if err := os.WriteFile(mdFile2, []byte("# Guide\nContent"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runGit(cloneDir, "add", "."); err != nil {
		t.Fatal(err)
	}
	if err := runGit(cloneDir, "commit", "-m", "second"); err != nil {
		t.Fatal(err)
	}
	if err := runGit(cloneDir, "push", "origin", "HEAD:main"); err != nil {
		_ = runGit(cloneDir, "push", "origin", "HEAD:master")
	}

	if err := orch.RunGit(ctx, upstreamURL, "", targetDir); err != nil {
		t.Fatalf("second run: %v", err)
	}

	var count2 int
	if err := database.QueryRow(`SELECT COUNT(*) FROM documents WHERE source = 'git-test'`).Scan(&count2); err != nil {
		t.Fatal(err)
	}
	if count2 != 2 {
		t.Fatalf("expected 2 docs after incremental update, got %d", count2)
	}
}

func TestLocalPathFromSource(t *testing.T) {
	got := LocalPathFromSource("./repo", "foo/bar")
	want := filepath.Join("repo", "foo-bar")
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

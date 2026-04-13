package gitpull

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSync_CloneAndPull(t *testing.T) {
	// Create a bare upstream repo
	upstreamDir := t.TempDir()
	if err := runGit(upstreamDir, "init", "--bare"); err != nil {
		t.Fatalf("init bare: %v", err)
	}

	// Create a local clone, add a markdown file, and push
	cloneDir := t.TempDir()
	if err := runGit(cloneDir, "clone", upstreamDir, "."); err != nil {
		t.Fatalf("clone: %v", err)
	}
	if err := runGit(cloneDir, "config", "user.email", "test@test.com"); err != nil {
		t.Fatal(err)
	}
	if err := runGit(cloneDir, "config", "user.name", "Test"); err != nil {
		t.Fatal(err)
	}

	mdFile := filepath.Join(cloneDir, "readme.md")
	if err := os.WriteFile(mdFile, []byte("# Hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runGit(cloneDir, "add", "."); err != nil {
		t.Fatal(err)
	}
	if err := runGit(cloneDir, "commit", "-m", "initial"); err != nil {
		t.Fatal(err)
	}
	if err := runGit(cloneDir, "push", "origin", "HEAD:main"); err != nil {
		// try master if main fails
		_ = runGit(cloneDir, "push", "origin", "HEAD:master")
	}

	// Test Sync (clone)
	targetDir := t.TempDir()
	changed, err := Sync(upstreamDir, "", targetDir)
	if err != nil {
		t.Fatalf("first sync: %v", err)
	}
	if len(changed) == 0 {
		t.Fatalf("expected files on first clone, got none")
	}
	found := false
	for _, p := range changed {
		if strings.HasSuffix(p, "readme.md") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected readme.md in changed list, got %v", changed)
	}

	// Modify upstream and push again
	mdFile2 := filepath.Join(cloneDir, "guide.md")
	if err := os.WriteFile(mdFile2, []byte("# Guide"), 0o644); err != nil {
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

	// Test Sync (pull)
	changed2, err := Sync(upstreamDir, "", targetDir)
	if err != nil {
		t.Fatalf("second sync: %v", err)
	}
	if len(changed2) == 0 {
		t.Fatalf("expected files on pull, got none")
	}
	found2 := false
	for _, p := range changed2 {
		if strings.HasSuffix(p, "guide.md") {
			found2 = true
			break
		}
	}
	if !found2 {
		t.Fatalf("expected guide.md in changed list, got %v", changed2)
	}
}

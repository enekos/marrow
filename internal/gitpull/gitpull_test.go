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
	// Set default branch so clones know what to checkout
	if err := runGit(upstreamDir, "symbolic-ref", "HEAD", "refs/heads/main"); err != nil {
		t.Fatalf("set head: %v", err)
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

	// Test Sync (clone) — use file:// protocol so --depth works with local paths
	targetDir := t.TempDir()
	upstreamURL := "file://" + upstreamDir
	changed, err := Sync(upstreamURL, "", targetDir)
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
	changed2, err := Sync(upstreamURL, "", targetDir)
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

func TestInjectToken(t *testing.T) {
	tests := []struct {
		name    string
		repoURL string
		token   string
		want    string
		wantErr bool
	}{
		{
			name:    "empty token returns URL unchanged",
			repoURL: "https://github.com/owner/repo.git",
			token:   "",
			want:    "https://github.com/owner/repo.git",
			wantErr: false,
		},
		{
			name:    "token injected into HTTPS URL",
			repoURL: "https://github.com/owner/repo.git",
			token:   "abc123",
			want:    "https://abc123:x-oauth-basic@github.com/owner/repo.git",
			wantErr: false,
		},
		{
			name:    "token injected into URL with existing path",
			repoURL: "https://github.com/owner/repo.git/path/to/file",
			token:   "abc123",
			want:    "https://abc123:x-oauth-basic@github.com/owner/repo.git/path/to/file",
			wantErr: false,
		},
		{
			name:    "invalid URL returns error",
			repoURL: "://invalid-url",
			token:   "abc123",
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := injectToken(tt.repoURL, tt.token)
			if (err != nil) != tt.wantErr {
				t.Fatalf("injectToken(%q, %q) error = %v, wantErr %v", tt.repoURL, tt.token, err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("injectToken(%q, %q) = %q, want %q", tt.repoURL, tt.token, got, tt.want)
			}
		})
	}
}

func TestFindMarkdownFiles(t *testing.T) {
	t.Run("empty dir returns empty slice", func(t *testing.T) {
		dir := t.TempDir()
		got, err := findMarkdownFiles(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("expected empty slice, got %v", got)
		}
	})

	t.Run("dir with .md and non-.md files returns only .md files", func(t *testing.T) {
		dir := t.TempDir()
		_ = os.WriteFile(filepath.Join(dir, "readme.md"), []byte("# Hello"), 0o644)
		_ = os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0o644)
		_ = os.WriteFile(filepath.Join(dir, "notes.md"), []byte("notes"), 0o644)

		got, err := findMarkdownFiles(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("expected 2 files, got %d", len(got))
		}
		for _, p := range got {
			if !strings.HasSuffix(strings.ToLower(p), ".md") {
				t.Fatalf("expected only .md files, got %v", got)
			}
		}
	})

	t.Run("nested dirs find .md files recursively", func(t *testing.T) {
		dir := t.TempDir()
		subDir := filepath.Join(dir, "subdir")
		_ = os.MkdirAll(subDir, 0o755)
		_ = os.WriteFile(filepath.Join(dir, "top.md"), []byte("# Top"), 0o644)
		_ = os.WriteFile(filepath.Join(subDir, "nested.md"), []byte("# Nested"), 0o644)
		_ = os.WriteFile(filepath.Join(subDir, "script.js"), []byte("console.log()"), 0o644)

		got, err := findMarkdownFiles(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("expected 2 files, got %d", len(got))
		}
		gotMap := make(map[string]bool)
		for _, p := range got {
			gotMap[filepath.Base(p)] = true
		}
		if !gotMap["top.md"] || !gotMap["nested.md"] {
			t.Fatalf("expected top.md and nested.md, got %v", got)
		}
	})
}

func TestGitOutput(t *testing.T) {
	dir := t.TempDir()
	if err := runGit(dir, "init"); err != nil {
		t.Fatalf("git init: %v", err)
	}
	if err := runGit(dir, "config", "user.email", "test@test.com"); err != nil {
		t.Fatal(err)
	}
	if err := runGit(dir, "config", "user.name", "Test"); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(dir, "file.txt"), []byte("hello"), 0o644)
	_ = runGit(dir, "add", "file.txt")
	_ = runGit(dir, "commit", "-m", "initial")

	out, err := gitOutput(dir, "log", "--pretty=format:%s", "-n", "1")
	if err != nil {
		t.Fatalf("gitOutput error: %v", err)
	}
	if out != "initial" {
		t.Fatalf("expected output %q, got %q", "initial", out)
	}
}

func TestRunGit_error(t *testing.T) {
	dir := t.TempDir()
	err := runGit(dir, "not-a-valid-git-command")
	if err == nil {
		t.Fatalf("expected error for invalid git command, got nil")
	}
}

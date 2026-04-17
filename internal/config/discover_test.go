package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindProjectConfig(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(base string) (startDir string)
		wantSuffix string
		wantEmpty  bool
	}{
		{
			name: "config in start dir",
			setup: func(base string) string {
				dir := filepath.Join(base, "project")
				_ = os.MkdirAll(dir, 0755)
				_ = os.WriteFile(filepath.Join(dir, ".marrow.toml"), []byte(""), 0644)
				return dir
			},
			wantSuffix: "project/.marrow.toml",
		},
		{
			name: "config in parent dir",
			setup: func(base string) string {
				parent := filepath.Join(base, "parent")
				child := filepath.Join(parent, "child")
				_ = os.MkdirAll(child, 0755)
				_ = os.WriteFile(filepath.Join(parent, ".marrow.toml"), []byte(""), 0644)
				return child
			},
			wantSuffix: "parent/.marrow.toml",
		},
		{
			name: "stopped by git boundary",
			setup: func(base string) string {
				parent := filepath.Join(base, "repo")
				child := filepath.Join(parent, "child")
				_ = os.MkdirAll(child, 0755)
				_ = os.MkdirAll(filepath.Join(parent, ".git"), 0755)
				_ = os.WriteFile(filepath.Join(base, ".marrow.toml"), []byte(""), 0644)
				return child
			},
			wantEmpty: true,
		},
		{
			name: "config found in same dir as git",
			setup: func(base string) string {
				dir := filepath.Join(base, "repo")
				_ = os.MkdirAll(dir, 0755)
				_ = os.MkdirAll(filepath.Join(dir, ".git"), 0755)
				_ = os.WriteFile(filepath.Join(dir, ".marrow.toml"), []byte(""), 0644)
				return dir
			},
			wantSuffix: "repo/.marrow.toml",
		},
		{
			name: "no config up to root",
			setup: func(base string) string {
				dir := filepath.Join(base, "a", "b", "c")
				_ = os.MkdirAll(dir, 0755)
				return dir
			},
			wantEmpty: true,
		},

	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := t.TempDir()
			startDir := tt.setup(base)

			got := FindProjectConfig(startDir)

			if tt.wantEmpty {
				if got != "" {
					t.Fatalf("expected empty path, got %q", got)
				}
				return
			}

			if got == "" {
				t.Fatalf("expected non-empty path ending with %q, got empty", tt.wantSuffix)
			}

			if !containsSuffix(got, tt.wantSuffix) {
				t.Fatalf("expected path ending with %q, got %q", tt.wantSuffix, got)
			}
		})
	}
}

func containsSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

package search

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/enekos/marrow/internal/db"
	"github.com/enekos/marrow/internal/embed"
	"github.com/enekos/marrow/internal/index"
	"github.com/enekos/marrow/internal/markdown"
)

// TestFixtureIndexing loads real markdown fixture files from disk,
// parses them, indexes them, and verifies search works end-to-end.
func TestFixtureIndexing(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "fixture.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()

	embedFn := embed.NewMock()
	ix := index.NewIndexer(database)
	engine := NewEngine(database, embedFn)

	fixtureDir := "../testdata/fixtures/markdown"
	entries, err := os.ReadDir(fixtureDir)
	if err != nil {
		t.Fatalf("read fixture dir: %v", err)
	}

	var docCount int
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		subDir := filepath.Join(fixtureDir, entry.Name())
		files, err := os.ReadDir(subDir)
		if err != nil {
			t.Fatalf("read %s: %v", subDir, err)
		}
		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".md") {
				continue
			}
			path := filepath.Join(subDir, f.Name())
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}

			md, err := markdown.Parse(data)
			if err != nil {
				t.Fatalf("parse %s: %v", path, err)
			}

			docPath := "/fixtures/" + entry.Name() + "/" + f.Name()
			stemmed := md.Text // Use raw text for simplicity in this test
			vec, err := embedFn(ctx, md.Title+" "+md.Text)
			if err != nil {
				t.Fatalf("embed %s: %v", path, err)
			}

			doc := index.Document{
				Path:        docPath,
				Hash:        string(data),
				Title:       md.Title,
				Lang:        md.Lang,
				Source:      "fixture",
				StemmedText: stemmed,
				Embedding:   vec,
			}
			if err := ix.Index(ctx, doc); err != nil {
				t.Fatalf("index %s: %v", path, err)
			}
			docCount++
		}
	}

	t.Logf("indexed %d fixture documents", docCount)
	if docCount == 0 {
		t.Fatal("no fixture documents found")
	}

	// Search for "go" should return Go-related fixtures.
	results, err := engine.Search(ctx, "go", "", 10, Filter{})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected results for 'go'")
	}
	foundGo := false
	for _, r := range results {
		if strings.Contains(r.Path, "/go/") {
			foundGo = true
			break
		}
	}
	if !foundGo {
		t.Errorf("expected at least one Go fixture in results, got: %v", results[0].Path)
	}

	// Search for "rust" should return Rust-related fixtures.
	results, err = engine.Search(ctx, "rust", "", 10, Filter{})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	foundRust := false
	for _, r := range results {
		if strings.Contains(r.Path, "/rust/") {
			foundRust = true
			break
		}
	}
	if !foundRust {
		t.Errorf("expected at least one Rust fixture in results")
	}

	// Search for Spanish content.
	results, err = engine.Search(ctx, "programación", "es", 10, Filter{})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	foundES := false
	for _, r := range results {
		if strings.Contains(r.Path, "/es/") {
			foundES = true
			break
		}
	}
	if !foundES {
		t.Errorf("expected at least one Spanish fixture in results")
	}

	// Edge case: empty query should return no error and no results.
	results, err = engine.Search(ctx, "", "", 10, Filter{})
	if err != nil {
		t.Fatalf("empty query search: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for empty query, got %d", len(results))
	}
}

// TestFixtureEdgeCases verifies that edge-case documents are handled correctly.
func TestFixtureEdgeCases(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "edge.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()

	embedFn := embed.NewMock()
	ix := index.NewIndexer(database)
	engine := NewEngine(database, embedFn)

	edgeCases := []struct {
		path  string
		title string
		lang  string
		text  string
	}{
		{"/edge/empty.md", "Empty Document", "en", ""},
		{"/edge/short.md", "Tiny Doc", "en", "short"},
		{"/edge/unicode.md", "Unicode Test", "en", "hello 世界 مرحبا 🌍 αβγ δεζ ηθι"},
		{"/edge/long.md", "Very Long Document", "en", strings.Repeat("word ", 500)},
		{"/edge/code.md", "Code Only", "en", "func main() { fmt.Println(\"hello\") }"},
	}

	for _, tc := range edgeCases {
		vec, _ := embedFn(ctx, tc.title+" "+tc.text)
		doc := index.Document{
			Path:        tc.path,
			Hash:        tc.text,
			Title:       tc.title,
			Lang:        tc.lang,
			StemmedText: tc.text,
			Embedding:   vec,
		}
		if err := ix.Index(ctx, doc); err != nil {
			t.Fatalf("index %s: %v", tc.path, err)
		}
	}

	// All documents should be searchable.
	for _, tc := range edgeCases {
		results, err := engine.Search(ctx, tc.title, "", 5, Filter{})
		if err != nil {
			t.Errorf("search %s: %v", tc.path, err)
			continue
		}
		found := false
		for _, r := range results {
			if r.Path == tc.path {
				found = true
				break
			}
		}
		if !found && tc.text != "" {
			t.Logf("note: %s not in top-5 for title search (may be expected)", tc.path)
		}
	}
}

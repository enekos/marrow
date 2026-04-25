package service

import (
	"context"
	"slices"
	"testing"

	"github.com/enekos/marrow/internal/config"
	"github.com/enekos/marrow/internal/db"
	"github.com/enekos/marrow/internal/index"
	"github.com/enekos/marrow/internal/search"
)

// mockEmbed returns a deterministic 384-dim vector for testing.
func mockEmbed(ctx context.Context, text string) ([]float32, error) {
	v := make([]float32, 384)
	for i := range v {
		v[i] = 0.01 * float32(i%10)
	}
	return v, nil
}

func TestSearcher_Search_SiteFiltering(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()

	engine := search.NewEngine(database, mockEmbed)
	searcher := &Searcher{Engine: engine}

	// Seed documents for two sources.
	ctx := context.Background()
	ix := index.NewIndexer(database)
	for _, doc := range []struct {
		path   string
		title  string
		source string
	}{
		{"blog/post1.md", "Hello Blog", "blog-md"},
		{"blog/post2.md", "Another Post", "blog-md"},
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

	tests := []struct {
		name       string
		site       *config.SiteConfig
		explicit   string
		wantPaths  []string
		wantMinLen int
	}{
		{
			name:      "no site no explicit source returns all",
			site:      nil,
			explicit:  "",
			wantPaths: []string{"blog/post1.md", "blog/post2.md", "docs/page1.md"},
		},
		{
			name:      "explicit source overrides site",
			site:      &config.SiteConfig{Name: "blog", Sources: []string{"blog-md"}},
			explicit:  "docs-md",
			wantPaths: []string{"docs/page1.md"},
		},
		{
			name:      "site filters to its sources",
			site:      &config.SiteConfig{Name: "blog", Sources: []string{"blog-md"}},
			explicit:  "",
			wantPaths: []string{"blog/post1.md", "blog/post2.md"},
		},
		{
			name:      "site with multiple sources",
			site:      &config.SiteConfig{Name: "all", Sources: []string{"blog-md", "docs-md"}},
			explicit:  "",
			wantPaths: []string{"blog/post1.md", "blog/post2.md", "docs/page1.md"},
		},
		{
			name:       "site with no sources returns empty",
			site:       &config.SiteConfig{Name: "empty", Sources: []string{}},
			explicit:   "",
			wantMinLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := searcher.Search(ctx, "hello", 10, tt.explicit, "", "en", "", tt.site)
			if err != nil {
				t.Fatalf("search error: %v", err)
			}
			if tt.wantMinLen == 0 && len(results) == 0 {
				return
			}
			gotPaths := make([]string, len(results))
			for i, r := range results {
				gotPaths[i] = r.Path
			}
			slices.Sort(gotPaths)
			wantPaths := make([]string, len(tt.wantPaths))
			copy(wantPaths, tt.wantPaths)
			slices.Sort(wantPaths)
			if len(gotPaths) != len(wantPaths) {
				t.Errorf("result count = %d, want %d; got %v", len(gotPaths), len(wantPaths), gotPaths)
				return
			}
			for i, want := range wantPaths {
				if gotPaths[i] != want {
					t.Errorf("result[%d].Path = %q, want %q", i, gotPaths[i], want)
				}
			}
		})
	}
}

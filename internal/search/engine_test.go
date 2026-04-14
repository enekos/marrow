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
	results, err := engine.Search(ctx, "go", "", 10)
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
	results, err := engine.Search(ctx, "", "", 10)
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

func TestSearch_DetectLangOption(t *testing.T) {
	ctx := context.Background()
	database := setupTestDB(t)
	ix := index.NewIndexer(database)
	embedFn := embed.NewMock()

	docs := []index.Document{
		{Path: "/en.md", Hash: "1", Title: "Coding Guide", Lang: "en", Source: "test", StemmedText: "code guid", Embedding: mustEmbed(embedFn, "coding guide")},
		{Path: "/es.md", Hash: "2", Title: "Programación", Lang: "es", Source: "test", StemmedText: "gui program", Embedding: mustEmbed(embedFn, "guía de programación")},
	}
	for _, d := range docs {
		if err := ix.Index(ctx, d); err != nil {
			t.Fatalf("index doc: %v", err)
		}
	}

	engineOn := NewEngine(database, embedFn)
	resultsOn, err := engineOn.Search(ctx, "programación", "", 10)
	if err != nil {
		t.Fatalf("search with detection on: %v", err)
	}
	if len(resultsOn) != 2 {
		t.Fatalf("expected 2 results, got %d", len(resultsOn))
	}
	if resultsOn[0].Title != "Programación" {
		t.Errorf("expected 'Programación' first with detection on, got %s", resultsOn[0].Title)
	}

	engineOff := NewEngine(database, embedFn)
	engineOff.DetectLang = false
	engineOff.DefaultLang = "en"
	resultsOff, err := engineOff.Search(ctx, "programación", "", 10)
	if err != nil {
		t.Fatalf("search with detection off: %v", err)
	}
	if len(resultsOff) != 2 {
		t.Fatalf("expected 2 results, got %d", len(resultsOff))
	}

	var scoreOn, scoreOff float64
	for _, r := range resultsOn {
		if r.Title == "Programación" {
			scoreOn = r.Score
		}
	}
	for _, r := range resultsOff {
		if r.Title == "Programación" {
			scoreOff = r.Score
		}
	}
	if scoreOn <= scoreOff {
		t.Errorf("expected higher score for 'Programación' with detection on (%f) than off (%f)", scoreOn, scoreOff)
	}
}

func TestSearch_DefaultLang(t *testing.T) {
	ctx := context.Background()
	database := setupTestDB(t)
	ix := index.NewIndexer(database)
	embedFn := embed.NewMock()

	docs := []index.Document{
		{Path: "/en.md", Hash: "1", Title: "Coding Guide", Lang: "en", Source: "test", StemmedText: "code guid", Embedding: mustEmbed(embedFn, "coding guide")},
		{Path: "/es.md", Hash: "2", Title: "Programación", Lang: "es", Source: "test", StemmedText: "gui program", Embedding: mustEmbed(embedFn, "guía de programación")},
	}
	for _, d := range docs {
		if err := ix.Index(ctx, d); err != nil {
			t.Fatalf("index doc: %v", err)
		}
	}

	engine := NewEngine(database, embedFn)
	engine.DetectLang = false
	engine.DefaultLang = "es"
	results, err := engine.Search(ctx, "programación", "", 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) == 0 {
		t.Fatalf("expected results")
	}
	if results[0].Title != "Programación" {
		t.Errorf("expected 'Programación' first with default-lang=es, got %s", results[0].Title)
	}
	if results[0].Score <= 0.018 {
		t.Errorf("expected boosted score for 'Programación' with default-lang=es, got %f", results[0].Score)
	}
}

func TestDetectQueryLang(t *testing.T) {
	tests := []struct {
		query string
		want  string
	}{
		// English – basic stopwords and common terms
		{"the quick brown fox", "en"},
		{"how to use git", "en"},
		{"software configuration", "en"},
		{"Go Programming", "en"},
		{"a b c", "en"},
		{"123 456", "en"},

		// Spanish – stopwords and unique characters
		{"el libro", "es"},
		{"la casa", "es"},
		{"configuración", "es"},
		{"¿qué es esto?", "es"},
		{"señor García", "es"},
		{"cómo funciona esto", "es"},
		{"qué", "es"},
		{"ñ", "es"},
		{"¡hola!", "es"},
		{"más o menos", "es"},

		// Basque – digraphs and common words
		{"txistu", "eu"},
		{"etxe", "eu"},
		{"hitz", "eu"},
		{"Euskal Herria", "eu"},
		{"eta", "eu"},
		{"ez da", "eu"},
		{"nire etxea", "eu"},
		{"tx", "eu"},
		{"tz", "eu"},
		{"zer da hau", "eu"},
		{"Donostia kalean", "eu"},

		// Edge cases that previously mis-detected
		{"meta analysis", "en"},            // "eta" inside "meta" must not trigger Basque
		{"cats and dogs", "en"},            // "ts" in "cats" must not trigger Basque
		{"next", "en"},                     // no false Basque from tx/tz
		{"matrix", "en"},                   // no false Basque
		{"how to configure eta", "en"},     // English context overrides lone "eta"
		{"who is señor Garcia", "es"},      // ñ overrides English stopwords
		{"a", "en"},                        // ambiguous single-letter word, default to English
		{"el", "es"},                       // unambiguous Spanish

		// Mixed-context edge cases
		{"el famoso txistu", "eu"},         // lone Spanish article loses to strong Basque digraph
		{"el famoso txistu vasco español", "es"}, // several Spanish words overcome txistu
		{"Go Programming en español", "es"}, // Spanish words at end win
		{"Git eta GitHub artean", "eu"},    // Basque context with technical terms
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			got := detectQueryLang(tt.query)
			if got != tt.want {
				t.Errorf("detectQueryLang(%q) = %q, want %q", tt.query, got, tt.want)
			}
		})
	}
}

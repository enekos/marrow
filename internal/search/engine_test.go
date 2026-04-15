package search

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"marrow/internal/db"
	"marrow/internal/embed"
	"marrow/internal/index"
	"marrow/internal/testutil"
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

	// Search for "go" — docs /a.md and /c.md have "go" in title.
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

// -----------------------------------------------------------------------------
// Approved-truth (golden-file) tests
// -----------------------------------------------------------------------------

func TestSearch_RankingApproved(t *testing.T) {
	ctx := context.Background()
	database := setupTestDB(t)
	ix := index.NewIndexer(database)
	embedFn := embed.NewMock()

	now := time.Now()
	docs := []index.Document{
		{Path: "/go-intro.md", Hash: "1", Title: "Go Introduction", Lang: "en", Source: "test", StemmedText: "go introduct languag program", Embedding: mustEmbed(embedFn, "go introduction language programming")},
		{Path: "/go-modules.md", Hash: "2", Title: "Go Modules Guide", Lang: "en", Source: "test", StemmedText: "go modul guid depend manag", Embedding: mustEmbed(embedFn, "go modules guide dependency management")},
		{Path: "/rust-book.md", Hash: "3", Title: "The Rust Programming Language", Lang: "en", Source: "test", StemmedText: "rust program languag memori safeti", Embedding: mustEmbed(embedFn, "rust programming language memory safety")},
		{Path: "/python-async.md", Hash: "4", Title: "Python AsyncIO", Lang: "en", Source: "test", StemmedText: "python asyncio async await program", Embedding: mustEmbed(embedFn, "python asyncio async await programming")},
		{Path: "/go-best-practices.md", Hash: "5", Title: "Go Best Practices", Lang: "en", Source: "test", StemmedText: "go best practic structur code review", Embedding: mustEmbed(embedFn, "go best practices structure code review")},
		{Path: "/old-go-tutorial.md", Hash: "6", Title: "Old Go Tutorial", Lang: "en", Source: "test", StemmedText: "go tutori old beginn start languag", Embedding: mustEmbed(embedFn, "go tutorial old beginner start language")},
	}
	for _, d := range docs {
		if err := ix.Index(ctx, d); err != nil {
			t.Fatalf("index doc: %v", err)
		}
	}

	// Override last_modified to create a recency spread.
	_, err := database.ExecContext(ctx,
		`UPDATE documents SET last_modified = ? WHERE path = ?`,
		now.Add(-200*24*time.Hour), "/old-go-tutorial.md")
	if err != nil {
		t.Fatalf("update last_modified: %v", err)
	}
	_, err = database.ExecContext(ctx,
		`UPDATE documents SET last_modified = ? WHERE path = ?`,
		now.Add(-2*24*time.Hour), "/go-modules.md")
	if err != nil {
		t.Fatalf("update last_modified: %v", err)
	}

	engine := NewEngine(database, embedFn)

	queries := []string{"go", "go modules", "programming language"}
	for _, q := range queries {
		q := q
		t.Run(strings.ReplaceAll(q, " ", "_"), func(t *testing.T) {
			results, err := engine.Search(ctx, q, "", 10)
			if err != nil {
				t.Fatalf("search %q: %v", q, err)
			}
			// Strip IDs and paths for stable golden files (scores are the signal).
			type slim struct {
				Title string  `json:"title"`
				Score float64 `json:"score"`
			}
			slimmed := make([]slim, len(results))
			for i, r := range results {
				// Round to 6 decimals to mask CGO float noise in sqlite-vec distances.
				slimmed[i] = slim{Title: r.Title, Score: math.Round(r.Score*1e6) / 1e6}
			}
			b, err := json.MarshalIndent(slimmed, "", "  ")
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			testutil.VerifyApproved(t, b, q)
		})
	}
}

func TestDetectQueryLang_Approved(t *testing.T) {
	tests := []struct {
		query    string
		expected string
	}{
		// English
		{"the quick brown fox", "en"},
		{"how to use git", "en"},
		{"software configuration", "en"},
		{"Go Programming", "en"},
		{"a b c", "en"},
		{"123 456", "en"},
		{"meta analysis", "en"},
		{"cats and dogs", "en"},
		{"next", "en"},
		{"matrix", "en"},
		{"how to configure eta", "en"},
		{"a", "en"},
		{"i have nothing to declare", "en"},
		{"this is very important", "en"},
		{"the following public announcement", "en"},
		{"can you do that", "en"},
		{"well done", "en"},
		{"good bad new old", "en"},
		{"first last long great", "en"},

		// Spanish
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
		{"who is señor Garcia", "es"},
		{"el", "es"},
		{"Go Programming en español", "es"},
		{"el famoso txistu vasco español", "es"},
		{"la mejor casa", "es"},
		{"todos los días", "es"},
		{"aquí y ahora", "es"},
		{"bien hecho", "es"},
		{"porque sí", "es"},
		{"una persona importante", "es"},

		// Basque
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
		{"el famoso txistu", "eu"},
		{"Git eta GitHub artean", "eu"},
		{"hemen eta orain", "eu"},
		{"nire etxe ondoan", "eu"},
		{"euskal herriko kaleak", "eu"},
		{"bat bi gutxi guzti", "eu"},
		{"inoiz beti hemendik", "eu"},
		{"non nola noiz zergatik", "eu"},

		// Ambiguous / edge
		{"", "en"},
		{"x y z", "en"},
		{"123", "en"},
		{"el meta tx", "eu"},
		{"la matrix tz", "eu"},
	}

	var sb strings.Builder
	for _, tt := range tests {
		got := detectQueryLang(tt.query)
		status := "OK"
		if got != tt.expected {
			status = "FAIL"
		}
		fmt.Fprintf(&sb, "%-40s -> %s (want %s) [%s]\n", fmt.Sprintf("%q", tt.query), got, tt.expected, status)
	}
	testutil.VerifyApprovedString(t, sb.String())
}

func TestSearch_ScoreComponentsApproved(t *testing.T) {
	ctx := context.Background()
	database := setupTestDB(t)
	ix := index.NewIndexer(database)
	embedFn := embed.NewMock()

	now := time.Now()
	// All docs share the same stemmed text and embedding text so base FTS/vector scores are identical.
	baseText := "go modules tutorial"
	baseStemmed := "go modul tutori"
	baseEmbedding := mustEmbed(embedFn, baseText)

	docs := []index.Document{
		{Path: "/exact-title-new.md", Hash: "1", Title: "Go Modules", Lang: "en", Source: "test", StemmedText: baseStemmed, Embedding: baseEmbedding},
		{Path: "/partial-title-new.md", Hash: "2", Title: "Go Tutorial", Lang: "en", Source: "test", StemmedText: baseStemmed, Embedding: baseEmbedding},
		{Path: "/phrase-old.md", Hash: "3", Title: "Something Else", Lang: "en", Source: "test", StemmedText: baseStemmed, Embedding: baseEmbedding},
		{Path: "/none-older.md", Hash: "4", Title: "Unrelated Document", Lang: "en", Source: "test", StemmedText: baseStemmed, Embedding: baseEmbedding},
	}
	for _, d := range docs {
		if err := ix.Index(ctx, d); err != nil {
			t.Fatalf("index doc: %v", err)
		}
	}

	// Set recency: exact-title-new is 1 day old, partial-title-new is 1 day old,
	// phrase-old is 90 days old, none-older is 200 days old.
	ages := map[string]time.Duration{
		"/exact-title-new.md":  1 * 24 * time.Hour,
		"/partial-title-new.md": 1 * 24 * time.Hour,
		"/phrase-old.md":       90 * 24 * time.Hour,
		"/none-older.md":       200 * 24 * time.Hour,
	}
	for path, age := range ages {
		_, err := database.ExecContext(ctx,
			`UPDATE documents SET last_modified = ? WHERE path = ?`,
			now.Add(-age), path)
		if err != nil {
			t.Fatalf("update last_modified: %v", err)
		}
	}

	engine := NewEngine(database, embedFn)
	results, err := engine.Search(ctx, "go modules", "", 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}

	var sb strings.Builder
	fmt.Fprintln(&sb, "Query: go modules")
	fmt.Fprintln(&sb, "Expected heuristics:")
	fmt.Fprintln(&sb, "  1. Go Modules          -> exact title + phrase + new")
	fmt.Fprintln(&sb, "  2. Go Tutorial         -> partial title + new")
	fmt.Fprintln(&sb, "  3. Something Else      -> phrase match in content + old")
	fmt.Fprintln(&sb, "  4. Unrelated Document  -> no boost + oldest")
	fmt.Fprintln(&sb, "")
	fmt.Fprintln(&sb, "Actual ranking:")
	for i, r := range results {
		fmt.Fprintf(&sb, "%d. %-20s score=%.6f\n", i+1, r.Title, r.Score)
	}
	testutil.VerifyApprovedString(t, sb.String())
}

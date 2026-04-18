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
	results, err := engine.Search(ctx, "go", "", 10, Filter{})
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
	results, err := engine.Search(ctx, "", "", 10, Filter{})
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
	resultsOn, err := engineOn.Search(ctx, "programación", "", 10, Filter{})
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
	resultsOff, err := engineOff.Search(ctx, "programación", "", 10, Filter{})
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
	results, err := engine.Search(ctx, "programación", "", 10, Filter{})
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
			results, err := engine.Search(ctx, q, "", 10, Filter{})
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
	results, err := engine.Search(ctx, "go modules", "", 10, Filter{})
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
func TestStripOuterQuotes(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want string
	}{
		{"double quotes", `"hello world"`, "hello world"},
		{"single quotes", `'hello world'`, "hello world"},
		{"no quotes", `hello world`, "hello world"},
		{"only one double quote", `"hello world`, `"hello world`},
		{"only one single quote", `'hello world`, `'hello world`},
		{"empty string", "", ""},
		{"two chars double quotes", `""`, ""},
		{"two chars single quotes", `''`, ""},
		{"mixed quotes", `"hello world'`, `"hello world'`},
		{"inner quotes preserved", `"hello 'world'"`, "hello 'world'"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripOuterQuotes(tt.s)
			if got != tt.want {
				t.Errorf("stripOuterQuotes(%q) = %q, want %q", tt.s, got, tt.want)
			}
		})
	}
}

func TestBuildFilterSQL(t *testing.T) {
	tests := []struct {
		name     string
		filter   Filter
		wantSQL  string
		wantArgs []interface{}
	}{
		{
			name:     "empty filter",
			filter:   Filter{},
			wantSQL:  "",
			wantArgs: nil,
		},
		{
			name:     "source only",
			filter:   Filter{Source: "test"},
			wantSQL:  "source = ?",
			wantArgs: []interface{}{"test"},
		},
		{
			name:     "doc type only",
			filter:   Filter{DocType: "issue"},
			wantSQL:  "doc_type = ?",
			wantArgs: []interface{}{"issue"},
		},
		{
			name:     "lang only",
			filter:   Filter{Lang: "en"},
			wantSQL:  "lang = ?",
			wantArgs: []interface{}{"en"},
		},
		{
			name:     "source and doc type",
			filter:   Filter{Source: "test", DocType: "issue"},
			wantSQL:  "source = ? AND doc_type = ?",
			wantArgs: []interface{}{"test", "issue"},
		},
		{
			name:     "all three",
			filter:   Filter{Source: "test", DocType: "issue", Lang: "es"},
			wantSQL:  "source = ? AND doc_type = ? AND lang = ?",
			wantArgs: []interface{}{"test", "issue", "es"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSQL, gotArgs := buildFilterSQL(tt.filter)
			if gotSQL != tt.wantSQL {
				t.Errorf("buildFilterSQL() sql = %q, want %q", gotSQL, tt.wantSQL)
			}
			if len(gotArgs) != len(tt.wantArgs) {
				t.Errorf("buildFilterSQL() args len = %d, want %d", len(gotArgs), len(tt.wantArgs))
			} else {
				for i := range gotArgs {
					if gotArgs[i] != tt.wantArgs[i] {
						t.Errorf("buildFilterSQL() args[%d] = %v, want %v", i, gotArgs[i], tt.wantArgs[i])
					}
				}
			}
		})
	}
}

func TestTitleBoost(t *testing.T) {
	tests := []struct {
		name          string
		stemmedTitle  string
		stemmedTokens []string
		want          float64
	}{
		{
			name:          "empty tokens",
			stemmedTitle:  "go program languag",
			stemmedTokens: []string{},
			want:          1.0,
		},
		{
			name:          "no match",
			stemmedTitle:  "rust program languag",
			stemmedTokens: []string{"go", "modul"},
			want:          1.0,
		},
		{
			name:          "full match",
			stemmedTitle:  "go modul tutori",
			stemmedTokens: []string{"go", "modul", "tutori"},
			want:          1.25,
		},
		{
			name:          "partial match",
			stemmedTitle:  "go modul guid",
			stemmedTokens: []string{"go", "modul", "tutori"},
			want:          1.0 + 0.25*(2.0/3.0),
		},
		{
			name:          "duplicate tokens in title counted once",
			stemmedTitle:  "go go go",
			stemmedTokens: []string{"go", "modul"},
			want:          1.0 + 0.25*(1.0/2.0),
		},
		{
			name:          "duplicate tokens in query counted once",
			stemmedTitle:  "go modul tutori",
			stemmedTokens: []string{"go", "go", "go"},
			want:          1.0 + 0.25*(1.0/3.0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := titleBoost(tt.stemmedTitle, tt.stemmedTokens)
			if got != tt.want {
				t.Errorf("titleBoost(%q, %v) = %f, want %f", tt.stemmedTitle, tt.stemmedTokens, got, tt.want)
			}
		})
	}
}

func TestRecencyBoost(t *testing.T) {
	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name         string
		lastModified time.Time
		now          time.Time
		want         float64
	}{
		{
			name:         "future time gets max boost",
			lastModified: now.Add(24 * time.Hour),
			now:          now,
			want:         1.0 + recencyBoostMax,
		},
		{
			name:         "same time gets max boost",
			lastModified: now,
			now:          now,
			want:         1.0 + recencyBoostMax,
		},
		{
			name:         "very old gets no boost",
			lastModified: now.Add(-365 * 24 * time.Hour),
			now:          now,
			want:         1.0,
		},
		{
			name:         "exactly at boundary gets no boost",
			lastModified: now.Add(-maxRecencyDays * 24 * time.Hour),
			now:          now,
			want:         1.0,
		},
		{
			name:         "halfway gets half boost",
			lastModified: now.Add(-90 * 24 * time.Hour),
			now:          now,
			want:         1.0 + recencyBoostMax*0.5,
		},
		{
			name:         "quarterway gets three quarter boost",
			lastModified: now.Add(-45 * 24 * time.Hour),
			now:          now,
			want:         1.0 + recencyBoostMax*(1.0-45.0/maxRecencyDays),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := recencyBoost(tt.lastModified, tt.now)
			if got != tt.want {
				t.Errorf("recencyBoost(%v, %v) = %f, want %f", tt.lastModified, tt.now, got, tt.want)
			}
		})
	}
}

func TestNewEngine_PanicsOnNilEmbedFn(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when embedFn is nil")
		}
	}()
	_ = NewEngine(nil, nil)
}

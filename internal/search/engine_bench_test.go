package search

import (
	"context"
	"fmt"
	"math/rand/v2"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/enekos/marrow/internal/db"
	"github.com/enekos/marrow/internal/embed"
	"github.com/enekos/marrow/internal/index"
	"github.com/enekos/marrow/internal/stemmer"
)

// ---------------------------------------------------------------------------
// Fixture generation
// ---------------------------------------------------------------------------

func generateCorpus(nDocs int, chunksPerDoc int) []index.Document {
	words := []string{
		"go", "rust", "python", "java", "javascript", "typescript", "c", "cpp",
		"programming", "language", "tutorial", "guide", "reference", "documentation",
		"concurrency", "async", "await", "goroutine", "channel", "mutex", "lock",
		"memory", "safety", "ownership", "borrowing", "lifetime", "generic", "trait",
		"interface", "struct", "enum", "pattern", "matching", "error", "handling",
		"testing", "benchmark", "performance", "optimization", "compiler", "runtime",
		"module", "package", "dependency", "management", "version", "control", "git",
		"http", "server", "client", "request", "response", "api", "rest", "graphql",
		"database", "sql", "query", "index", "transaction", "migration", "schema",
		"docker", "kubernetes", "container", "orchestration", "deployment", "cloud",
		"linux", "windows", "macos", "terminal", "shell", "script", "automation",
		"algorithm", "data", "structure", "array", "list", "map", "tree", "graph",
		"machine", "learning", "neural", "network", "model", "training", "inference",
		"security", "authentication", "authorization", "encryption", "hash", "token",
		"frontend", "backend", "fullstack", "framework", "library", "tool", "cli",
	}

	sources := []string{"docs", "blog", "github", "tutorial", "reference"}
	langs := []string{"en", "es", "de"}
	docTypes := []string{"markdown", "issue", "readme"}

	docs := make([]index.Document, nDocs)
	for i := range nDocs {
		// Deterministic pseudo-random based on index for reproducibility.
		r := rand.New(rand.NewPCG(uint64(i), uint64(i*31+7)))

		source := sources[r.IntN(len(sources))]
		lang := langs[r.IntN(len(langs))]
		docType := docTypes[r.IntN(len(docTypes))]

		// Build title from 2-4 random words.
		titleWords := make([]string, 2+r.IntN(3))
		for j := range titleWords {
			titleWords[j] = words[r.IntN(len(words))]
		}
		title := strings.Join(titleWords, " ")

		// Build content from 20-100 random words.
		contentWords := make([]string, 20+r.IntN(80))
		for j := range contentWords {
			contentWords[j] = words[r.IntN(len(words))]
		}
		content := strings.Join(contentWords, " ")

		stemmedTitle := stemmer.StemText(title, lang)
		stemmedContent := stemmer.StemText(content, lang)

		fullText := stemmedTitle + " " + stemmedContent
		vec := mustEmbedStatic(fullText)

		var chunks []index.Chunk
		if chunksPerDoc > 1 {
			chunkSize := max(1, len(contentWords)/chunksPerDoc)
			for c := 0; c < chunksPerDoc && c*chunkSize < len(contentWords); c++ {
				start := c * chunkSize
				end := min(start+chunkSize+10, len(contentWords))
				chunkText := strings.Join(contentWords[start:end], " ")
				chunks = append(chunks, index.Chunk{
					Index:     c,
					Text:      chunkText,
					Embedding: mustEmbedStatic(chunkText),
				})
			}
		}

		docs[i] = index.Document{
			Path:        fmt.Sprintf("/%s/%s/doc_%d.md", source, lang, i),
			Hash:        fmt.Sprintf("hash_%d", i),
			Title:       title,
			Lang:        lang,
			Source:      source,
			DocType:     docType,
			StemmedText: stemmedContent,
			Embedding:   vec,
			Chunks:      chunks,
		}
	}
	return docs
}

func mustEmbedStatic(text string) []float32 {
	fn := embed.NewMock()
	v, err := fn(context.Background(), text)
	if err != nil {
		panic(err)
	}
	return v
}

func setupBenchmarkDB(b *testing.B, nDocs, chunksPerDoc int) (*db.DB, *Engine) {
	b.Helper()
	dir := b.TempDir()
	database, err := db.Open(filepath.Join(dir, "bench.db"))
	if err != nil {
		b.Fatalf("open db: %v", err)
	}

	embedFn := embed.NewMock()
	ctx := context.Background()

	// Index documents in batches for speed.
	corpus := generateCorpus(nDocs, chunksPerDoc)
	const batchSize = 100
	for i := 0; i < len(corpus); i += batchSize {
		end := min(i+batchSize, len(corpus))
		tx, err := database.BeginTx(ctx, nil)
		if err != nil {
			b.Fatalf("begin tx: %v", err)
		}
		for _, doc := range corpus[i:end] {
			if err := index.IndexTx(ctx, tx, doc); err != nil {
				b.Fatalf("index tx: %v", err)
			}
		}
		if err := tx.Commit(); err != nil {
			b.Fatalf("commit tx: %v", err)
		}
	}

	engine := NewEngine(database, embedFn)
	return database, engine
}

// ---------------------------------------------------------------------------
// Benchmark: Search latency by corpus size
// ---------------------------------------------------------------------------

func BenchmarkSearch_CorpusSize(b *testing.B) {
	sizes := []struct {
		name         string
		nDocs        int
		chunksPerDoc int
	}{
		{"small_100_1chunk", 100, 1},
		{"small_100_3chunk", 100, 3},
		{"medium_1k_1chunk", 1000, 1},
		{"medium_1k_3chunk", 1000, 3},
		{"large_10k_1chunk", 10000, 1},
		{"large_10k_3chunk", 10000, 3},
	}

	for _, sz := range sizes {
		b.Run(sz.name, func(b *testing.B) {
			database, engine := setupBenchmarkDB(b, sz.nDocs, sz.chunksPerDoc)
			defer database.Close()
			ctx := context.Background()

			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, err := engine.Search(ctx, "go programming tutorial", "", 10, Filter{})
				if err != nil {
					b.Fatalf("search: %v", err)
				}
			}
			b.StopTimer()
		})
	}
}

// ---------------------------------------------------------------------------
// Benchmark: Search latency by query complexity
// ---------------------------------------------------------------------------

func BenchmarkSearch_QueryComplexity(b *testing.B) {
	database, engine := setupBenchmarkDB(b, 1000, 3)
	defer database.Close()
	ctx := context.Background()

	queries := []struct {
		name  string
		query string
	}{
		{"single_word", "go"},
		{"two_words", "go programming"},
		{"three_words", "go programming tutorial"},
		{"five_words", "go programming tutorial memory safety"},
		{"phrase_quoted", `"go programming"`},
		{"spanish", "programación guía"},
	}

	for _, q := range queries {
		b.Run(q.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, err := engine.Search(ctx, q.query, "", 10, Filter{})
				if err != nil {
					b.Fatalf("search: %v", err)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Benchmark: Search latency by result limit
// ---------------------------------------------------------------------------

func BenchmarkSearch_Limit(b *testing.B) {
	database, engine := setupBenchmarkDB(b, 1000, 3)
	defer database.Close()
	ctx := context.Background()

	limits := []int{1, 5, 10, 25, 50, 100}
	for _, limit := range limits {
		b.Run(fmt.Sprintf("limit_%d", limit), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, err := engine.Search(ctx, "programming", "", limit, Filter{})
				if err != nil {
					b.Fatalf("search: %v", err)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Benchmark: Search with filters
// ---------------------------------------------------------------------------

func BenchmarkSearch_Filters(b *testing.B) {
	database, engine := setupBenchmarkDB(b, 1000, 3)
	defer database.Close()
	ctx := context.Background()

	filters := []struct {
		name   string
		filter Filter
	}{
		{"none", Filter{}},
		{"source_single", Filter{Sources: []string{"docs"}}},
		{"source_multi", Filter{Sources: []string{"docs", "blog"}}},
		{"doctype", Filter{DocType: "markdown"}},
		{"lang_en", Filter{Lang: "en"}},
		{"combined", Filter{Sources: []string{"docs", "blog"}, DocType: "markdown", Lang: "en"}},
	}

	for _, f := range filters {
		b.Run(f.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, err := engine.Search(ctx, "programming", "", 10, f.filter)
				if err != nil {
					b.Fatalf("search: %v", err)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Benchmark: Search with HTML highlighting
// ---------------------------------------------------------------------------

func BenchmarkSearch_HighlightHTML(b *testing.B) {
	database, engine := setupBenchmarkDB(b, 1000, 3)
	defer database.Close()
	ctx := context.Background()

	b.Run("plain", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, err := engine.Search(ctx, "programming", "", 10, Filter{HighlightFormat: ""})
			if err != nil {
				b.Fatalf("search: %v", err)
			}
		}
	})

	b.Run("html", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, err := engine.Search(ctx, "programming", "", 10, Filter{HighlightFormat: "html"})
			if err != nil {
				b.Fatalf("search: %v", err)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// Benchmark: Concurrent throughput
// ---------------------------------------------------------------------------

func BenchmarkSearch_Parallel(b *testing.B) {
	sizes := []struct {
		name  string
		nDocs int
	}{
		{"1k", 1000},
		{"10k", 10000},
	}

	for _, sz := range sizes {
		b.Run(sz.name, func(b *testing.B) {
			database, engine := setupBenchmarkDB(b, sz.nDocs, 3)
			defer database.Close()
			ctx := context.Background()

			b.ResetTimer()
			b.ReportAllocs()
			b.RunParallel(func(pb *testing.PB) {
				// Each goroutine cycles through different queries to avoid cache bias.
				queries := []string{
					"go programming",
					"rust tutorial",
					"python async",
					"memory safety",
					"concurrency patterns",
				}
				idx := 0
				for pb.Next() {
					q := queries[idx%len(queries)]
					idx++
					_, err := engine.Search(ctx, q, "", 10, Filter{})
					if err != nil {
						b.Fatalf("search: %v", err)
					}
				}
			})
		})
	}
}

// ---------------------------------------------------------------------------
// Benchmark: Component-level breakdown
// ---------------------------------------------------------------------------

// instrumentedEngine wraps Engine to measure individual phase durations.
type instrumentedEngine struct {
	*Engine
	mu          sync.Mutex
	prepareTime time.Duration
	embedTime   time.Duration
	ftsTime     time.Duration
	vecTime     time.Duration
	phraseTime  time.Duration
	scoreTime   time.Duration
	metaTime    time.Duration
	buildTime   time.Duration
	enrichTime  time.Duration
}

func (ie *instrumentedEngine) Search(ctx context.Context, query string, langHint string, limit int, filter Filter) ([]Result, error) {
	if limit <= 0 {
		limit = 10
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return []Result{}, nil
	}

	t0 := time.Now()
	phrase, _, stemmedQuery, stemmedTokens, err := ie.prepareQuery(query, langHint)
	if err != nil {
		return nil, err
	}
	ftsExpr := buildFTSMatchExpr(stemmedTokens)
	if ftsExpr == "" {
		ftsExpr = stemmedQuery
	}
	prepareDur := time.Since(t0)

	t0 = time.Now()
	qblob, err := ie.embedQuery(ctx, phrase, query)
	if err != nil {
		return nil, err
	}
	embedDur := time.Since(t0)

	filterSQL, filterArgs := buildFilterSQL(filter)

	var ftsRes *ftsResult
	var vecRes *vecResult
	var ftsErr, vecErr error
	var wg sync.WaitGroup
	wg.Add(2)

	t0 = time.Now()
	go func() {
		defer wg.Done()
		ftsRes, ftsErr = ie.queryFTS(ctx, ftsExpr, limit, filter.HighlightFormat)
	}()

	t1 := time.Now()
	go func() {
		defer wg.Done()
		vecRes, vecErr = ie.queryVectors(ctx, qblob, limit)
	}()

	wg.Wait()
	ftsDur := time.Since(t0)
	vecDur := time.Since(t1)

	if ftsErr != nil {
		return nil, ftsErr
	}
	if vecErr != nil {
		return nil, vecErr
	}

	t0 = time.Now()
	phraseDocIDs, err := ie.detectPhraseMatches(ctx, phrase, limit)
	if err != nil {
		return nil, err
	}
	phraseDur := time.Since(t0)

	t0 = time.Now()
	scoredDocs := ie.computeScores(ftsRes, vecRes)
	scoreDur := time.Since(t0)

	t0 = time.Now()
	metadata, err := ie.fetchMetadata(ctx, scoredDocs, filterSQL, filterArgs)
	if err != nil {
		return nil, err
	}
	metaDur := time.Since(t0)

	t0 = time.Now()
	results := ie.buildResults(scoredDocs, metadata, ftsRes, vecRes, phraseDocIDs, phrase, stemmedTokens)
	buildDur := time.Since(t0)

	t0 = time.Now()
	slices.SortFunc(results, func(a, b Result) int {
		if b.Score > a.Score {
			return 1
		}
		if b.Score < a.Score {
			return -1
		}
		return 0
	})
	if len(results) > limit {
		results = results[:limit]
	}
	ie.enrichResults(results, stemmedTokens)
	enrichDur := time.Since(t0)

	ie.mu.Lock()
	ie.prepareTime += prepareDur
	ie.embedTime += embedDur
	ie.ftsTime += ftsDur
	ie.vecTime += vecDur
	ie.phraseTime += phraseDur
	ie.scoreTime += scoreDur
	ie.metaTime += metaDur
	ie.buildTime += buildDur
	ie.enrichTime += enrichDur
	ie.mu.Unlock()

	return results, nil
}

func BenchmarkSearch_ComponentBreakdown(b *testing.B) {
	database, baseEngine := setupBenchmarkDB(b, 1000, 3)
	defer database.Close()
	ctx := context.Background()

	ie := &instrumentedEngine{Engine: baseEngine}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := ie.Search(ctx, "go programming tutorial", "", 10, Filter{})
		if err != nil {
			b.Fatalf("search: %v", err)
		}
	}
	b.StopTimer()

	b.ReportMetric(float64(ie.prepareTime)/float64(b.N), "prepare_ns/op")
	b.ReportMetric(float64(ie.embedTime)/float64(b.N), "embed_ns/op")
	b.ReportMetric(float64(ie.ftsTime)/float64(b.N), "fts_ns/op")
	b.ReportMetric(float64(ie.vecTime)/float64(b.N), "vec_ns/op")
	b.ReportMetric(float64(ie.phraseTime)/float64(b.N), "phrase_ns/op")
	b.ReportMetric(float64(ie.scoreTime)/float64(b.N), "score_ns/op")
	b.ReportMetric(float64(ie.metaTime)/float64(b.N), "meta_ns/op")
	b.ReportMetric(float64(ie.buildTime)/float64(b.N), "build_ns/op")
	b.ReportMetric(float64(ie.enrichTime)/float64(b.N), "enrich_ns/op")
}

// ---------------------------------------------------------------------------
// Benchmark: Cold vs warm cache
// ---------------------------------------------------------------------------

func BenchmarkSearch_ColdCache(b *testing.B) {
	// Each iteration creates a fresh DB so SQLite has no page cache.
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		database, engine := setupBenchmarkDB(b, 500, 3)
		ctx := context.Background()
		_, err := engine.Search(ctx, "programming tutorial", "", 10, Filter{})
		if err != nil {
			b.Fatalf("search: %v", err)
		}
		database.Close()
	}
}

// ---------------------------------------------------------------------------
// Benchmark: Query type coverage
// ---------------------------------------------------------------------------

func BenchmarkSearch_QueryTypes(b *testing.B) {
	database, engine := setupBenchmarkDB(b, 1000, 3)
	defer database.Close()
	ctx := context.Background()

	cases := []struct {
		name    string
		query   string
		lang    string
		filter  Filter
		limit   int
	}{
		{"simple_english", "programming", "", Filter{}, 10},
		{"spanish_hint", "programación", "es", Filter{}, 10},
		{"filtered_source", "programming", "", Filter{Sources: []string{"docs"}}, 10},
		{"filtered_doctype", "programming", "", Filter{DocType: "markdown"}, 10},
		{"high_limit", "programming", "", Filter{}, 50},
		{"rare_term", "kubernetes orchestration deployment", "", Filter{}, 10},
		{"common_term", "go", "", Filter{}, 10},
		{"empty_result", "xyznonexistent", "", Filter{}, 10},
	}

	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, err := engine.Search(ctx, c.query, c.lang, c.limit, c.filter)
				if err != nil {
					b.Fatalf("search: %v", err)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Benchmark: Memory pressure
// ---------------------------------------------------------------------------

func BenchmarkSearch_MemoryPressure(b *testing.B) {
	// Test with large result sets to stress allocation paths.
	database, engine := setupBenchmarkDB(b, 5000, 5)
	defer database.Close()
	ctx := context.Background()

	b.Run("limit_100", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, err := engine.Search(ctx, "programming", "", 100, Filter{})
			if err != nil {
				b.Fatalf("search: %v", err)
			}
		}
	})

	b.Run("limit_100_filtered", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, err := engine.Search(ctx, "programming", "", 100, Filter{Lang: "en"})
			if err != nil {
				b.Fatalf("search: %v", err)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// Benchmark: Scale sweep (documents)
// ---------------------------------------------------------------------------

func BenchmarkSearch_ScaleDocs(b *testing.B) {
	sizes := []int{10, 50, 100, 500, 1000, 5000}
	for _, n := range sizes {
		b.Run(fmt.Sprintf("%d_docs", n), func(b *testing.B) {
			database, engine := setupBenchmarkDB(b, n, 3)
			defer database.Close()
			ctx := context.Background()

			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, err := engine.Search(ctx, "programming tutorial guide", "", 10, Filter{})
				if err != nil {
					b.Fatalf("search: %v", err)
				}
			}
		})
	}
}

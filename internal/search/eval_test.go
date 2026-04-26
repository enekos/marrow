package search

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/enekos/marrow/internal/db"
	"github.com/enekos/marrow/internal/embed"
	"github.com/enekos/marrow/internal/eval"
	"github.com/enekos/marrow/internal/index"
	"github.com/enekos/marrow/internal/testutil"
)

// evalCorpus is a deterministic document set for retrieval evaluation.
// It covers Go, Rust, Python, Spanish, Basque, general topics, and edge cases
// to test cross-language, cross-topic, semantic, and boundary-condition retrieval.
var evalCorpus = []index.Document{
	// ── Go documents ─────────────────────────────────────────────
	{Path: "/go/intro.md", Hash: "1", Title: "Go Introduction", Lang: "en", Source: "eval", StemmedText: "go introduct languag program code syntax"},
	{Path: "/go/modules.md", Hash: "2", Title: "Go Modules Guide", Lang: "en", Source: "eval", StemmedText: "go modul guid depend manag version vendor"},
	{Path: "/go/concurrency.md", Hash: "3", Title: "Go Concurrency Patterns", Lang: "en", Source: "eval", StemmedText: "go concurrenci pattern goroutin channel sync context worker pool"},
	{Path: "/go/best-practices.md", Hash: "4", Title: "Go Best Practices", Lang: "en", Source: "eval", StemmedText: "go best practic structur code review test benchmark table driven subtest mock interface"},
	{Path: "/go/standard-library.md", Hash: "5", Title: "Go Standard Library Overview", Lang: "en", Source: "eval", StemmedText: "go standard librari overview net http io fmt context sync"},
	{Path: "/go/generics.md", Hash: "g1", Title: "Go Generics Tutorial", Lang: "en", Source: "eval", StemmedText: "go generic tutori type paramet constraint ani compar interfac"},
	{Path: "/go/channels.md", Hash: "g2", Title: "Go Channels Deep Dive", Lang: "en", Source: "eval", StemmedText: "go channel communic goroutin select buffer close"},
	{Path: "/go/error-handling.md", Hash: "g3", Title: "Go Error Handling", Lang: "en", Source: "eval", StemmedText: "go error handl panic recov defer wrap sentinel"},
	{Path: "/go/interfaces.md", Hash: "g4", Title: "Go Interfaces Explained", Lang: "en", Source: "eval", StemmedText: "go interfac satisfi method set implicit compil time"},
	{Path: "/go/performance.md", Hash: "g5", Title: "Go Performance Tuning", Lang: "en", Source: "eval", StemmedText: "go perform profil benchmark optim pprof cach"},

	// ── Rust documents ───────────────────────────────────────────
	{Path: "/rust/book.md", Hash: "6", Title: "The Rust Programming Language", Lang: "en", Source: "eval", StemmedText: "rust program languag memori safeti ownership borrow checker"},
	{Path: "/rust/async.md", Hash: "7", Title: "Rust Async Programming", Lang: "en", Source: "eval", StemmedText: "rust async await program futur tokio stream select runtime"},
	{Path: "/rust/traits.md", Hash: "8", Title: "Rust Traits and Generics", Lang: "en", Source: "eval", StemmedText: "rust trait generic bound default implement associ type iterator"},
	{Path: "/rust/ownership.md", Hash: "9", Title: "Understanding Ownership", Lang: "en", Source: "eval", StemmedText: "rust ownership move semant borrow refer mutabl scope drop"},
	{Path: "/rust/error-handling.md", Hash: "10", Title: "Error Handling in Rust", Lang: "en", Source: "eval", StemmedText: "rust error handl result option panic unwrap expect match"},
	{Path: "/rust/lifetimes.md", Hash: "r1", Title: "Rust Lifetimes Guide", Lang: "en", Source: "eval", StemmedText: "rust lifetim borrow checker scope refer dangl"},
	{Path: "/rust/macros.md", Hash: "r2", Title: "Rust Macros", Lang: "en", Source: "eval", StemmedText: "rust macro declar procedur meta program deriv"},
	{Path: "/rust/cargo.md", Hash: "r3", Title: "Rust Cargo and Crates", Lang: "en", Source: "eval", StemmedText: "rust cargo packag manag crate registri build tool"},
	{Path: "/rust/patterns.md", Hash: "r4", Title: "Rust Pattern Matching", Lang: "en", Source: "eval", StemmedText: "rust pattern match enum option result exhaust guard"},
	{Path: "/rust/unsafe.md", Hash: "r5", Title: "Rust Unsafe Code", Lang: "en", Source: "eval", StemmedText: "rust unsaf raw pointer transmut undefin behavior unsaf block"},

	// ── Python documents ─────────────────────────────────────────
	{Path: "/python/asyncio.md", Hash: "11", Title: "Python AsyncIO Complete Guide", Lang: "en", Source: "eval", StemmedText: "python asyncio async await coroutine task gather event loop concurrent"},
	{Path: "/python/data-science.md", Hash: "12", Title: "Python Data Science with Pandas", Lang: "en", Source: "eval", StemmedText: "python data scienc pandas numpy dataframe read csv select groupby merge aggreg"},
	{Path: "/python/web-frameworks.md", Hash: "13", Title: "Python Web Frameworks", Lang: "en", Source: "eval", StemmedText: "python web framework flask django fastapi rest api server request respons"},
	{Path: "/python/testing.md", Hash: "14", Title: "Python Testing Strategies", Lang: "en", Source: "eval", StemmedText: "python test strategi pytest unittest mock fixtur parametriz coverag"},
	{Path: "/python/type-hints.md", Hash: "p1", Title: "Python Type Hints", Lang: "en", Source: "eval", StemmedText: "python type hint annot mypi static check duck type"},
	{Path: "/python/virtual-env.md", Hash: "p2", Title: "Python Virtual Environments", Lang: "en", Source: "eval", StemmedText: "python virtual environ venv pip depend isol requir"},
	{Path: "/python/decorators.md", Hash: "p3", Title: "Python Decorators", Lang: "en", Source: "eval", StemmedText: "python decor higher order function wrapper functool lru cach"},
	{Path: "/python/machine-learning.md", Hash: "p4", Title: "Python Machine Learning", Lang: "en", Source: "eval", StemmedText: "python machin learn scikit learn tensorflow pytorch neural network"},

	// ── Spanish documents ────────────────────────────────────────
	{Path: "/es/programacion.md", Hash: "15", Title: "Programación en Go", Lang: "es", Source: "eval", StemmedText: "program go lenguaj codigo practica goroutine canal concurrencia"},
	{Path: "/es/rust.md", Hash: "16", Title: "Rust para Principiantes", Lang: "es", Source: "eval", StemmedText: "rust principi lenguaj program seguridad memoria propiedad"},
	{Path: "/es/python.md", Hash: "17", Title: "Python para Ciencia de Datos", Lang: "es", Source: "eval", StemmedText: "python ciencia dato pandas numpy analisis dataframe"},
	{Path: "/es/concurrencia.md", Hash: "e1", Title: "Concurrencia en Go", Lang: "es", Source: "eval", StemmedText: "concurrent go rutin canal select buff cierr"},
	{Path: "/es/web.md", Hash: "e2", Title: "Desarrollo Web con Python", Lang: "es", Source: "eval", StemmedText: "desarroll web python flask djang fastapi rest api"},

	// ── Basque documents ─────────────────────────────────────────
	{Path: "/eu/programazioa.md", Hash: "18", Title: "Programazioa Go-n", Lang: "eu", Source: "eval", StemmedText: "program go lengoaia kodea praktika goroutine kanal concurrencia"},
	{Path: "/eu/rust.md", Hash: "e3", Title: "Rust Hizkuntza", Lang: "eu", Source: "eval", StemmedText: "rust hiz ikasi programa segur memoria"},
	{Path: "/eu/python.md", Hash: "e4", Title: "Python Datu Zientzia", Lang: "eu", Source: "eval", StemmedText: "python datu zientzia ikas automa sarea neu"},

	// ── General / cross-topic documents ──────────────────────────
	{Path: "/general/programming.md", Hash: "19", Title: "Programming Languages Comparison", Lang: "en", Source: "eval", StemmedText: "program languag comparison go rust python perform concurrenci learn curv"},
	{Path: "/general/devops.md", Hash: "20", Title: "DevOps Best Practices", Lang: "en", Source: "eval", StemmedText: "devop best practic ci cd deploy infrastructur monitor docker kubernet terraform"},
	{Path: "/general/databases.md", Hash: "21", Title: "Database Design Patterns", Lang: "en", Source: "eval", StemmedText: "databas design pattern sql postgr mysql index normal transact acid"},
	{Path: "/general/security.md", Hash: "22", Title: "Application Security Fundamentals", Lang: "en", Source: "eval", StemmedText: "applic secur fundament authent author oauth jwt encrypt tls https"},
	{Path: "/general/algorithms.md", Hash: "g6", Title: "Algorithms and Data Structures", Lang: "en", Source: "eval", StemmedText: "algorithm data structur big o complex sort graph dynam program"},
	{Path: "/general/design-patterns.md", Hash: "g7", Title: "Design Patterns", Lang: "en", Source: "eval", StemmedText: "design pattern singleton factori observ strategi depend inject"},
	{Path: "/general/cloud.md", Hash: "g8", Title: "Cloud Computing", Lang: "en", Source: "eval", StemmedText: "cloud comput aw gcp azur serverless contain orchestr scale"},
	{Path: "/general/testing.md", Hash: "g9", Title: "Testing Methodologies", Lang: "en", Source: "eval", StemmedText: "test methodolog unit integr end end mock stub coverag tdd"},

	// ── Edge cases ───────────────────────────────────────────────
	{Path: "/edge/empty.md", Hash: "23", Title: "Empty Document", Lang: "en", Source: "eval", StemmedText: ""},
	{Path: "/edge/short.md", Hash: "24", Title: "Tiny Doc", Lang: "en", Source: "eval", StemmedText: "short"},
	{Path: "/edge/unicode.md", Hash: "25", Title: "Unicode and Special Characters", Lang: "en", Source: "eval", StemmedText: "unicod special charact emoji cjk arab mathemat symbol mix script text process"},
	{Path: "/edge/code-heavy.md", Hash: "26", Title: "Code Heavy Document", Lang: "en", Source: "eval", StemmedText: "code heavi document snippet go rust python exampl worker thread spawn async function"},
	{Path: "/edge/stopwords.md", Hash: "e5", Title: "Stop Words Document", Lang: "en", Source: "eval", StemmedText: "document contain onli common stop word veri littl meaning content"},
	{Path: "/edge/numbers.md", Hash: "e6", Title: "Numeric Document", Lang: "en", Source: "eval", StemmedText: "123 456 789 zero one two three numer digit calcul formula math"},
}

// evalQrels defines ground-truth relevance judgments for the corpus.
var evalQrels = []eval.QRel{
	// ── language-specific ────────────────────────────────────────
	{Query: "go", Category: "language-specific", Description: "Matches all Go-related documents including Spanish and Basque counterparts",
		Relevant: []string{"/go/intro.md", "/go/modules.md", "/go/concurrency.md", "/go/best-practices.md", "/go/standard-library.md", "/go/generics.md", "/go/channels.md", "/go/error-handling.md", "/go/interfaces.md", "/go/performance.md", "/es/programacion.md", "/es/concurrencia.md", "/eu/programazioa.md", "/general/programming.md"}},
	{Query: "go modules", Category: "language-specific", Description: "Focused query for Go dependency management",
		Relevant: []string{"/go/modules.md", "/go/best-practices.md", "/go/intro.md"},
		MinMetrics: map[string]float64{"MRR": 0.5}},
	{Query: "goroutine", Category: "language-specific", Description: "Go-specific concurrency primitive",
		Relevant: []string{"/go/concurrency.md", "/go/channels.md", "/go/best-practices.md", "/es/programacion.md", "/es/concurrencia.md", "/eu/programazioa.md"}},
	{Query: "go channels", Category: "language-specific", Description: "Go channel communication",
		Relevant: []string{"/go/channels.md", "/go/concurrency.md", "/es/concurrencia.md"}},
	{Query: "rust", Category: "language-specific", Description: "Matches all Rust-related documents",
		Relevant: []string{"/rust/book.md", "/rust/async.md", "/rust/traits.md", "/rust/ownership.md", "/rust/error-handling.md", "/rust/lifetimes.md", "/rust/macros.md", "/rust/cargo.md", "/rust/patterns.md", "/rust/unsafe.md", "/es/rust.md", "/eu/rust.md", "/general/programming.md"}},
	{Query: "rust ownership", Category: "language-specific", Description: "Rust ownership system",
		Relevant: []string{"/rust/ownership.md", "/rust/book.md", "/rust/lifetimes.md"}},
	{Query: "rust cargo", Category: "language-specific", Description: "Rust package manager",
		Relevant: []string{"/rust/cargo.md", "/rust/book.md"}},
	{Query: "python", Category: "language-specific", Description: "Matches all Python-related documents",
		Relevant: []string{"/python/asyncio.md", "/python/data-science.md", "/python/web-frameworks.md", "/python/testing.md", "/python/type-hints.md", "/python/virtual-env.md", "/python/decorators.md", "/python/machine-learning.md", "/es/python.md", "/es/web.md", "/eu/python.md", "/general/programming.md"}},
	{Query: "python data science", Category: "language-specific", Description: "Python data analysis ecosystem",
		Relevant: []string{"/python/data-science.md", "/es/python.md", "/eu/python.md"}},
	{Query: "python web frameworks", Category: "language-specific", Description: "Python web development",
		Relevant: []string{"/python/web-frameworks.md", "/es/web.md"}},
	{Query: "python machine learning", Category: "language-specific", Description: "Python ML and AI",
		Relevant: []string{"/python/machine-learning.md", "/python/data-science.md"}},

	// ── cross-language ───────────────────────────────────────────
	{Query: "programming language", Category: "cross-language", Description: "Should match language introductions across English, Spanish, and Basque",
		Relevant: []string{"/go/intro.md", "/rust/book.md", "/general/programming.md", "/es/programacion.md", "/es/rust.md", "/eu/programazioa.md"}},
	{Query: "programación", Lang: "es", Category: "cross-language", Description: "Spanish query should match Spanish and Basque docs, and English intros via shared stem",
		Relevant: []string{"/es/programacion.md", "/es/rust.md", "/es/python.md", "/es/concurrencia.md", "/es/web.md", "/go/intro.md", "/eu/programazioa.md"}},
	{Query: "concurrencia", Lang: "es", Category: "cross-language", Description: "Spanish concurrency query matching Go and Rust concurrency docs",
		Relevant: []string{"/es/concurrencia.md", "/es/programacion.md", "/go/concurrency.md", "/go/channels.md", "/rust/async.md"}},
	{Query: "error handling", Category: "cross-language", Description: "Concept that exists in Go, Rust, and Python",
		Relevant: []string{"/go/error-handling.md", "/rust/error-handling.md", "/python/testing.md", "/general/security.md"}},
	{Query: "testing", Category: "cross-language", Description: "Testing practices across languages",
		Relevant: []string{"/go/best-practices.md", "/python/testing.md", "/rust/error-handling.md", "/general/testing.md"}},
	{Query: "async", Category: "cross-language", Description: "Asynchronous programming in Rust and Python",
		Relevant: []string{"/rust/async.md", "/python/asyncio.md", "/go/concurrency.md"}},
	{Query: "performance", Category: "cross-language", Description: "Performance optimization across languages",
		Relevant: []string{"/go/performance.md", "/general/programming.md", "/python/machine-learning.md"}},
	{Query: "memory safety", Category: "cross-language", Description: "Memory safety concepts, primarily Rust",
		Relevant: []string{"/rust/book.md", "/rust/ownership.md", "/general/security.md", "/rust/unsafe.md"}},
	{Query: "concurrencia", Category: "cross-language", Description: "Basque/Spanish concurrency without lang filter",
		Relevant: []string{"/go/concurrency.md", "/go/channels.md", "/rust/async.md", "/es/concurrencia.md", "/es/programacion.md", "/eu/programazioa.md"}},

	// ── lang-filtered ────────────────────────────────────────────
	{Query: "programación", Lang: "es", Category: "lang-filtered", Description: "Spanish query with Spanish filter should only return Spanish docs",
		Relevant: []string{"/es/programacion.md", "/es/rust.md", "/es/python.md", "/es/concurrencia.md", "/es/web.md"},
		Negative: []string{"/go/intro.md", "/eu/programazioa.md"}},
	{Query: "go", Lang: "en", Category: "lang-filtered", Description: "English filter should exclude Spanish and Basque docs",
		Relevant: []string{"/go/intro.md", "/go/modules.md", "/go/concurrency.md", "/go/best-practices.md", "/go/standard-library.md", "/go/generics.md", "/go/channels.md", "/go/error-handling.md", "/go/interfaces.md", "/go/performance.md", "/general/programming.md"},
		Negative: []string{"/es/programacion.md", "/eu/programazioa.md"}},
	{Query: "rust", Lang: "eu", Category: "lang-filtered", Description: "Basque filter should only return Basque rust doc",
		Relevant: []string{"/eu/rust.md"},
		Negative: []string{"/rust/book.md", "/es/rust.md"}},

	// ── semantic ─────────────────────────────────────────────────
	{Query: "devops", Category: "semantic", Description: "Infrastructure and deployment practices",
		Relevant: []string{"/general/devops.md"}},
	{Query: "security", Category: "semantic", Description: "Application security fundamentals",
		Relevant: []string{"/general/security.md", "/rust/book.md", "/rust/ownership.md"}},
	{Query: "database", Category: "semantic", Description: "Database design and SQL",
		Relevant: []string{"/general/databases.md"}},
	{Query: "algorithms", Category: "semantic", Description: "Computer science algorithms",
		Relevant: []string{"/general/algorithms.md"}},
	{Query: "cloud computing", Category: "semantic", Description: "Cloud platforms and serverless",
		Relevant: []string{"/general/cloud.md"}},
	{Query: "design patterns", Category: "semantic", Description: "Software design patterns",
		Relevant: []string{"/general/design-patterns.md"}},

	// ── negative ─────────────────────────────────────────────────
	{Query: "rust", Category: "negative", Description: "Rust query should not return Python or edge-case docs",
		Relevant: []string{"/rust/book.md", "/rust/async.md", "/rust/traits.md", "/rust/ownership.md", "/rust/error-handling.md", "/es/rust.md", "/eu/rust.md", "/general/programming.md"},
		Negative: []string{"/python/asyncio.md", "/python/data-science.md", "/edge/empty.md", "/edge/stopwords.md"}},
	{Query: "database", Category: "negative", Description: "Database query should not return security or empty docs",
		Relevant: []string{"/general/databases.md"},
		Negative: []string{"/general/security.md", "/edge/empty.md"}},
	{Query: "go", Category: "negative", Description: "Go query should not return numeric or stopword-only docs",
		Relevant: []string{"/go/intro.md", "/go/modules.md", "/go/concurrency.md", "/go/best-practices.md", "/go/standard-library.md", "/es/programacion.md", "/eu/programazioa.md", "/general/programming.md"},
		Negative: []string{"/edge/empty.md", "/edge/numbers.md", "/edge/stopwords.md"}},
	{Query: "python decorators", Category: "negative", Description: "Python decorators should not return Rust macros doc",
		Relevant: []string{"/python/decorators.md"},
		Negative: []string{"/rust/macros.md"}},
	{Query: "pandas", Category: "negative", Description: "Pandas query should not return general programming docs",
		Relevant: []string{"/python/data-science.md", "/es/python.md", "/eu/python.md"},
		Negative: []string{"/general/programming.md", "/edge/numbers.md"}},

	// ── edge-case ────────────────────────────────────────────────
	{Query: "", Category: "edge-case", Description: "Empty query should return no results",
		Relevant: []string{}},
	{Query: "short", Category: "edge-case", Description: "Very short query matching a single-word document",
		Relevant: []string{"/edge/short.md"}},
	{Query: "unicode emoji", Category: "edge-case", Description: "Query with special characters",
		Relevant: []string{"/edge/unicode.md"}},
	{Query: "123", Category: "edge-case", Description: "Numeric query should match numeric document",
		Relevant: []string{"/edge/numbers.md"}},
	{Query: "stop words only the and of", Category: "edge-case", Description: "Query containing only stop words",
		Relevant: []string{"/edge/stopwords.md"}},

	// ── variants ─────────────────────────────────────────────────
	{Query: "go modules", Category: "variants", Description: "Base query for variant expansion",
		Relevant: []string{"/go/modules.md", "/go/best-practices.md", "/go/intro.md"},
		Variants: []string{"golang modules", "go dependency management"}},
	{Query: "rust ownership", Category: "variants", Description: "Base query for variant expansion",
		Relevant: []string{"/rust/ownership.md", "/rust/book.md", "/rust/lifetimes.md"},
		Variants: []string{"rust borrow checker", "rust memory ownership"}},
	{Query: "python data science", Category: "variants", Description: "Base query for variant expansion",
		Relevant: []string{"/python/data-science.md", "/es/python.md", "/eu/python.md"},
		Variants: []string{"python pandas", "python data analysis"}},
}

// pathSearchFunc adapts a search.Engine to eval.SearchFunc by extracting
// result paths. Shared across the eval tests and benchmark to keep the
// search/eval boundary in one place.
func pathSearchFunc(engine *Engine) eval.SearchFunc {
	return func(ctx context.Context, query, lang string, limit int) ([]string, error) {
		results, err := engine.Search(ctx, query, lang, limit, Filter{Lang: lang})
		if err != nil {
			return nil, err
		}
		paths := make([]string, len(results))
		for i, r := range results {
			paths[i] = r.Path
		}
		return paths, nil
	}
}

// indexCorpus indexes evalCorpus into database using embedFn.
func indexCorpus(tb testing.TB, database *db.DB, embedFn embed.Func) {
	tb.Helper()
	ix := index.NewIndexer(database)
	ctx := context.Background()
	for i := range evalCorpus {
		doc := evalCorpus[i]
		vec, err := embedFn(ctx, doc.Title+" "+doc.StemmedText)
		if err != nil {
			tb.Fatalf("embed %s: %v", doc.Path, err)
		}
		doc.Embedding = vec
		if err := ix.Index(ctx, doc); err != nil {
			tb.Fatalf("index %s: %v", doc.Path, err)
		}
	}
}

func setupEvalDB(t *testing.T) (*db.DB, embed.Func) {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "eval.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	embedFn := embed.NewMock()
	indexCorpus(t, database, embedFn)
	return database, embedFn
}

func TestRetrievalEvaluation(t *testing.T) {
	database, embedFn := setupEvalDB(t)
	engine := NewEngine(database, embedFn)

	qrels := eval.ExpandVariants(evalQrels)

	runner := eval.NewRunner(pathSearchFunc(engine))
	runner.Cutoffs = []int{1, 3, 5, 10}
	runner.Limit = 10

	ctx := context.Background()
	report, err := runner.RunAll(ctx, qrels)
	if err != nil {
		t.Fatalf("evaluation failed: %v", err)
	}

	// Verify aggregate metrics are within sensible bounds.
	if report.MeanMAP < 0.1 {
		t.Errorf("MAP = %f, expected >= 0.1 for this corpus", report.MeanMAP)
	}
	if report.MeanMRR < 0.1 {
		t.Errorf("MRR = %f, expected >= 0.1 for this corpus", report.MeanMRR)
	}

	// Verify no negative constraints are violated globally.
	var globalFailures []string
	for _, q := range report.Queries {
		for _, fr := range q.FailureReasons {
			if strings.Contains(fr, "negative doc") {
				globalFailures = append(globalFailures, q.Query+": "+fr)
			}
		}
	}
	if len(globalFailures) > 0 {
		t.Errorf("negative constraint violations:\n%s", strings.Join(globalFailures, "\n"))
	}

	// Store full report as human-readable text golden file.
	var sb strings.Builder
	opts := eval.DefaultTextOptions()
	opts.ShowDetail = false
	opts.ShowCategories = true
	eval.WriteText(&sb, report, opts)
	testutil.VerifyApprovedString(t, sb.String(), "retrieval_evaluation")
}

func TestRetrievalEvaluation_Debug(t *testing.T) {
	database, embedFn := setupEvalDB(t)
	engine := NewEngine(database, embedFn)
	runner := eval.NewRunner(pathSearchFunc(engine))
	runner.Cutoffs = []int{1, 3, 5, 10}
	runner.Limit = 10

	qrels := eval.ExpandVariants(evalQrels)

	ctx := context.Background()
	for _, qrel := range qrels {
		t.Run(qrel.Query, func(t *testing.T) {
			res, err := runner.Run(ctx, qrel)
			if err != nil {
				t.Fatalf("run: %v", err)
			}
			if len(res.FailureReasons) > 0 {
				t.Logf("query %q failures:", qrel.Query)
				for _, fr := range res.FailureReasons {
					t.Logf("  - %s", fr)
				}
			}
			if len(res.RankedPaths) > 0 {
				t.Logf("  ranking:  %v", res.RankedPaths)
			}
			if !hasOverlap(res.RankedPaths, qrel.Relevant) && len(qrel.Relevant) > 0 {
				t.Logf("  relevant: %v", qrel.Relevant)
			}
		})
	}
}

// hasOverlap reports whether ranked and relevant share at least one path.
func hasOverlap(ranked, relevant []string) bool {
	if len(ranked) == 0 || len(relevant) == 0 {
		return false
	}
	rel := make(map[string]struct{}, len(relevant))
	for _, p := range relevant {
		rel[p] = struct{}{}
	}
	for _, p := range ranked {
		if _, ok := rel[p]; ok {
			return true
		}
	}
	return false
}

// BenchmarkRetrievalEvaluation measures end-to-end search+eval throughput.
func BenchmarkRetrievalEvaluation(b *testing.B) {
	database, err := db.Open(filepath.Join(b.TempDir(), "bench.db"))
	if err != nil {
		b.Fatalf("open db: %v", err)
	}
	defer database.Close()

	embedFn := embed.NewMock()
	indexCorpus(b, database, embedFn)

	qrels := eval.ExpandVariants(evalQrels)
	runner := eval.NewRunner(pathSearchFunc(NewEngine(database, embedFn)))
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = runner.RunAll(ctx, qrels)
	}
}

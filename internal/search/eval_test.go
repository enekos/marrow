package search

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"marrow/internal/db"
	"marrow/internal/embed"
	"marrow/internal/eval"
	"marrow/internal/index"
	"marrow/internal/testutil"
)

// evalCorpus is a deterministic document set for retrieval evaluation.
// It covers Go, Rust, Python, Spanish, Basque, and edge cases to test
// cross-language, cross-topic, and boundary-condition retrieval quality.
var evalCorpus = []index.Document{
	// Go documents
	{Path: "/go/intro.md", Hash: "1", Title: "Go Introduction", Lang: "en", Source: "eval", StemmedText: "go introduct languag program code syntax", Embedding: nil},
	{Path: "/go/modules.md", Hash: "2", Title: "Go Modules Guide", Lang: "en", Source: "eval", StemmedText: "go modul guid depend manag version vendor", Embedding: nil},
	{Path: "/go/concurrency.md", Hash: "3", Title: "Go Concurrency Patterns", Lang: "en", Source: "eval", StemmedText: "go concurrenc pattern goroutin channel sync context worker pool", Embedding: nil},
	{Path: "/go/best-practices.md", Hash: "4", Title: "Go Best Practices", Lang: "en", Source: "eval", StemmedText: "go best practic structur code review test benchmark table driven subtest mock interface", Embedding: nil},
	{Path: "/go/standard-library.md", Hash: "5", Title: "Go Standard Library Overview", Lang: "en", Source: "eval", StemmedText: "go standard librari overview net http io fmt context sync", Embedding: nil},

	// Rust documents
	{Path: "/rust/book.md", Hash: "6", Title: "The Rust Programming Language", Lang: "en", Source: "eval", StemmedText: "rust program languag memori safeti ownership borrow checker", Embedding: nil},
	{Path: "/rust/async.md", Hash: "7", Title: "Rust Async Programming", Lang: "en", Source: "eval", StemmedText: "rust async await program futur tokio stream select runtime", Embedding: nil},
	{Path: "/rust/traits.md", Hash: "8", Title: "Rust Traits and Generics", Lang: "en", Source: "eval", StemmedText: "rust trait generic bound default implement associ type iterator", Embedding: nil},
	{Path: "/rust/ownership.md", Hash: "9", Title: "Understanding Ownership", Lang: "en", Source: "eval", StemmedText: "rust ownership move semant borrow refer mutabl scope drop", Embedding: nil},
	{Path: "/rust/error-handling.md", Hash: "10", Title: "Error Handling in Rust", Lang: "en", Source: "eval", StemmedText: "rust error handl result option panic unwrap expect match", Embedding: nil},

	// Python documents
	{Path: "/python/asyncio.md", Hash: "11", Title: "Python AsyncIO Complete Guide", Lang: "en", Source: "eval", StemmedText: "python asyncio async await coroutine task gather event loop concurrent", Embedding: nil},
	{Path: "/python/data-science.md", Hash: "12", Title: "Python Data Science with Pandas", Lang: "en", Source: "eval", StemmedText: "python data scienc pandas numpy dataframe read csv select groupby merge aggreg", Embedding: nil},
	{Path: "/python/web-frameworks.md", Hash: "13", Title: "Python Web Frameworks", Lang: "en", Source: "eval", StemmedText: "python web framework flask django fastapi rest api server request respons", Embedding: nil},
	{Path: "/python/testing.md", Hash: "14", Title: "Python Testing Strategies", Lang: "en", Source: "eval", StemmedText: "python test strategi pytest unittest mock fixtur parametriz coverag", Embedding: nil},

	// Spanish documents
	{Path: "/es/programacion.md", Hash: "15", Title: "Programación en Go", Lang: "es", Source: "eval", StemmedText: "program go lenguaj codigo practica goroutine canal concurrencia", Embedding: nil},
	{Path: "/es/rust.md", Hash: "16", Title: "Rust para Principiantes", Lang: "es", Source: "eval", StemmedText: "rust principi lenguaj program seguridad memoria propiedad", Embedding: nil},
	{Path: "/es/python.md", Hash: "17", Title: "Python para Ciencia de Datos", Lang: "es", Source: "eval", StemmedText: "python ciencia dato pandas numpy analisis dataframe", Embedding: nil},

	// Basque documents
	{Path: "/eu/programazioa.md", Hash: "18", Title: "Programazioa Go-n", Lang: "eu", Source: "eval", StemmedText: "program go lengoaia kodea praktika goroutine kanal concurrencia", Embedding: nil},

	// General / cross-topic documents
	{Path: "/general/programming.md", Hash: "19", Title: "Programming Languages Comparison", Lang: "en", Source: "eval", StemmedText: "program languag comparison go rust python perform concurrenci learn curv", Embedding: nil},
	{Path: "/general/devops.md", Hash: "20", Title: "DevOps Best Practices", Lang: "en", Source: "eval", StemmedText: "devop best practic ci cd deploy infrastructur monitor docker kubernet terraform", Embedding: nil},
	{Path: "/general/databases.md", Hash: "21", Title: "Database Design Patterns", Lang: "en", Source: "eval", StemmedText: "databas design pattern sql postgr mysql index normal transact acid", Embedding: nil},
	{Path: "/general/security.md", Hash: "22", Title: "Application Security Fundamentals", Lang: "en", Source: "eval", StemmedText: "applic secur fundament authent author oauth jwt encrypt tls https", Embedding: nil},

	// Edge cases
	{Path: "/edge/empty.md", Hash: "23", Title: "Empty Document", Lang: "en", Source: "eval", StemmedText: "", Embedding: nil},
	{Path: "/edge/short.md", Hash: "24", Title: "Tiny Doc", Lang: "en", Source: "eval", StemmedText: "short", Embedding: nil},
	{Path: "/edge/unicode.md", Hash: "25", Title: "Unicode and Special Characters", Lang: "en", Source: "eval", StemmedText: "unicod special charact emoji cjk arab mathemat symbol script mix", Embedding: nil},
	{Path: "/edge/code-heavy.md", Hash: "26", Title: "Code Heavy Document", Lang: "en", Source: "eval", StemmedText: "code heavi document go rust python exampl snippet worker thread spawn", Embedding: nil},
}

// evalQrels defines ground-truth relevance judgments for the corpus.
var evalQrels = []eval.QRel{
	{Query: "go", Relevant: []string{"/go/intro.md", "/go/modules.md", "/go/concurrency.md", "/go/best-practices.md", "/go/standard-library.md", "/es/programacion.md", "/general/programming.md", "/eu/programazioa.md"}},
	{Query: "go modules", Relevant: []string{"/go/modules.md", "/go/best-practices.md", "/go/intro.md"}},
	{Query: "programming language", Relevant: []string{"/go/intro.md", "/rust/book.md", "/general/programming.md", "/es/programacion.md", "/es/rust.md", "/eu/programazioa.md"}},
	{Query: "programación", Lang: "es", Relevant: []string{"/es/programacion.md", "/es/rust.md", "/es/python.md", "/go/intro.md", "/eu/programazioa.md"}},
	{Query: "concurrency", Relevant: []string{"/go/concurrency.md", "/rust/async.md", "/python/asyncio.md", "/es/programacion.md"}},
	{Query: "async", Relevant: []string{"/rust/async.md", "/python/asyncio.md", "/go/concurrency.md"}},
	{Query: "memory safety", Relevant: []string{"/rust/book.md", "/rust/ownership.md", "/general/security.md"}},
	{Query: "rust", Relevant: []string{"/rust/book.md", "/rust/async.md", "/rust/traits.md", "/rust/ownership.md", "/rust/error-handling.md", "/es/rust.md", "/general/programming.md"}},
	{Query: "python", Relevant: []string{"/python/asyncio.md", "/python/data-science.md", "/python/web-frameworks.md", "/python/testing.md", "/es/python.md", "/general/programming.md"}},
	{Query: "data science", Relevant: []string{"/python/data-science.md", "/es/python.md"}},
	{Query: "testing", Relevant: []string{"/go/best-practices.md", "/python/testing.md", "/rust/error-handling.md"}},
	{Query: "devops", Relevant: []string{"/general/devops.md"}},
	{Query: "security", Relevant: []string{"/general/security.md", "/rust/book.md", "/rust/ownership.md"}},
	{Query: "database", Relevant: []string{"/general/databases.md"}},
	{Query: "error handling", Relevant: []string{"/rust/error-handling.md", "/general/security.md", "/python/testing.md"}},
	{Query: "ownership", Relevant: []string{"/rust/ownership.md", "/rust/book.md", "/rust/traits.md"}},
	{Query: "traits", Relevant: []string{"/rust/traits.md"}},
	{Query: "goroutine", Relevant: []string{"/go/concurrency.md", "/go/best-practices.md", "/es/programacion.md", "/eu/programazioa.md"}},
	{Query: "pandas", Relevant: []string{"/python/data-science.md", "/es/python.md"}},
	{Query: "", Relevant: []string{}}, // edge case: empty query
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
	ix := index.NewIndexer(database)
	ctx := context.Background()

	for i := range evalCorpus {
		doc := evalCorpus[i]
		// Generate deterministic embedding from the document text.
		text := doc.Title + " " + doc.StemmedText
		vec, err := embedFn(ctx, text)
		if err != nil {
			t.Fatalf("embed %s: %v", doc.Path, err)
		}
		doc.Embedding = vec
		if err := ix.Index(ctx, doc); err != nil {
			t.Fatalf("index %s: %v", doc.Path, err)
		}
	}

	return database, embedFn
}

func TestRetrievalEvaluation(t *testing.T) {
	database, embedFn := setupEvalDB(t)
	engine := NewEngine(database, embedFn)

	searchFn := func(ctx context.Context, query, lang string, limit int) ([]string, error) {
		results, err := engine.Search(ctx, query, lang, limit, Filter{})
		if err != nil {
			return nil, err
		}
		paths := make([]string, len(results))
		for i, r := range results {
			paths[i] = r.Path
		}
		return paths, nil
	}
	runner := eval.NewRunner(searchFn)
	runner.Cutoffs = []int{1, 3, 5, 10}
	runner.Limit = 10

	ctx := context.Background()
	report, err := runner.RunAll(ctx, evalQrels)
	if err != nil {
		t.Fatalf("evaluation failed: %v", err)
	}

	// Verify aggregate metrics are within sensible bounds.
	if report.MAP < 0.1 {
		t.Errorf("MAP = %f, expected >= 0.1 for this corpus", report.MAP)
	}
	if report.MRR < 0.1 {
		t.Errorf("MRR = %f, expected >= 0.1 for this corpus", report.MRR)
	}

	// Store full report as golden file for regression detection.
	slim := toSlimReport(report)
	b, err := json.MarshalIndent(slim, "", "  ")
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	testutil.VerifyApproved(t, b, "retrieval_evaluation")
}

// slimReport strips volatile fields (RankedPaths) for stable golden files.
type slimReport struct {
	Queries       []slimQuery      `json:"queries"`
	MeanPrecision map[int]float64  `json:"mean_precision"`
	MeanRecall    map[int]float64  `json:"mean_recall"`
	MeanNDCG      map[int]float64  `json:"mean_ndcg"`
	MeanF1        map[int]float64  `json:"mean_f1"`
	MRR           float64          `json:"mrr"`
	MAP           float64          `json:"map"`
}

type slimQuery struct {
	Query     string          `json:"query"`
	Precision map[int]float64 `json:"precision"`
	Recall    map[int]float64 `json:"recall"`
	NDCG      map[int]float64 `json:"ndcg"`
	F1        map[int]float64 `json:"f1"`
	MRR       float64         `json:"mrr"`
	AP        float64         `json:"ap"`
}

func toSlimReport(r eval.Report) slimReport {
	qs := make([]slimQuery, len(r.Queries))
	for i, q := range r.Queries {
		qs[i] = slimQuery{
			Query:     q.Query,
			Precision: q.Precision,
			Recall:    q.Recall,
			NDCG:      q.NDCG,
			F1:        q.F1,
			MRR:       q.MRR,
			AP:        q.AP,
		}
	}
	return slimReport{
		Queries:       qs,
		MeanPrecision: r.MeanPrecision,
		MeanRecall:    r.MeanRecall,
		MeanNDCG:      r.MeanNDCG,
		MeanF1:        r.MeanF1,
		MRR:           r.MRR,
		MAP:           r.MAP,
	}
}

// TestRetrievalEvaluation_PerQuery allows debugging individual queries.
func TestRetrievalEvaluation_PerQuery(t *testing.T) {
	database, embedFn := setupEvalDB(t)
	engine := NewEngine(database, embedFn)
	searchFn := func(ctx context.Context, query, lang string, limit int) ([]string, error) {
		results, err := engine.Search(ctx, query, lang, limit, Filter{})
		if err != nil {
			return nil, err
		}
		paths := make([]string, len(results))
		for i, r := range results {
			paths[i] = r.Path
		}
		return paths, nil
	}
	runner := eval.NewRunner(searchFn)
	runner.Cutoffs = []int{1, 3, 5, 10}
	runner.Limit = 10

	ctx := context.Background()
	for _, qrel := range evalQrels {
		qrel := qrel
		t.Run(qrel.Query, func(t *testing.T) {
			res, err := runner.Run(ctx, qrel)
			if err != nil {
				t.Fatalf("run: %v", err)
			}
			// At least one relevant doc should appear in top 10.
			found := false
			for _, p := range res.RankedPaths {
				for _, r := range qrel.Relevant {
					if p == r {
						found = true
						break
					}
				}
				if found {
					break
				}
			}
			if !found {
				t.Logf("query %q: no relevant doc in top 10", qrel.Query)
				t.Logf("  ranking: %v", res.RankedPaths)
				t.Logf("  relevant: %v", qrel.Relevant)
			}
		})
	}
}

// BenchmarkRetrievalEvaluation measures end-to-end search+eval latency.
func BenchmarkRetrievalEvaluation(b *testing.B) {
	dir := b.TempDir()
	database, err := db.Open(filepath.Join(dir, "bench.db"))
	if err != nil {
		b.Fatalf("open db: %v", err)
	}
	defer database.Close()

	embedFn := embed.NewMock()
	ix := index.NewIndexer(database)
	ctx := context.Background()

	for i := range evalCorpus {
		doc := evalCorpus[i]
		text := doc.Title + " " + doc.StemmedText
		vec, _ := embedFn(ctx, text)
		doc.Embedding = vec
		_ = ix.Index(ctx, doc)
	}

	engine := NewEngine(database, embedFn)
	searchFn := func(ctx context.Context, query, lang string, limit int) ([]string, error) {
		results, err := engine.Search(ctx, query, lang, limit, Filter{})
		if err != nil {
			return nil, err
		}
		paths := make([]string, len(results))
		for i, r := range results {
			paths[i] = r.Path
		}
		return paths, nil
	}
	runner := eval.NewRunner(searchFn)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = runner.RunAll(ctx, evalQrels)
	}
}

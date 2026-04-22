package local

import (
	"context"
	"os"
	"sort"
	"testing"
)

// TestRetrieval_SmallCorpus is a behavioral retrieval-quality check: for
// each query, we embed a small heterogeneous corpus and assert that the
// expected passage ranks #1 under our encoder. This catches regressions
// that parity-per-input wouldn't surface — e.g. a subtle matrix-order bug
// that shifts every embedding by the same offset keeps cosine-to-reference
// high but may still flip the top-1 under our encoder.
func TestRetrieval_SmallCorpus(t *testing.T) {
	if os.Getenv("MARROW_MINILM_DIR") == "" {
		t.Skip("set MARROW_MINILM_DIR to run retrieval test")
	}
	enc := loadEncoder(t)
	ctx := context.Background()

	corpus := []string{
		"Goroutines are lightweight threads managed by the Go runtime.",
		"Rust's ownership system guarantees memory safety without a garbage collector.",
		"Python's asyncio library provides coroutine-based concurrency.",
		"Paris is the capital city of France and home to the Eiffel Tower.",
		"The Mediterranean diet emphasizes vegetables, olive oil, and seafood.",
		"Pandas is a Python library for tabular data analysis and manipulation.",
		"Convolutional neural networks excel at image recognition tasks.",
		"PostgreSQL is a relational database with strong support for SQL standards.",
		"Docker containers bundle applications with their runtime dependencies.",
		"The Pacific Ocean is the largest and deepest of Earth's oceanic divisions.",
	}
	corpusVecs := make([][]float32, len(corpus))
	for i, c := range corpus {
		v, err := enc.Embed(ctx, c)
		if err != nil {
			t.Fatal(err)
		}
		corpusVecs[i] = v
	}

	queries := []struct {
		q        string
		wantIdx  int
		wantText string
	}{
		{"concurrency in Go", 0, corpus[0]},
		{"safe memory management in systems languages", 1, corpus[1]},
		{"async programming in Python", 2, corpus[2]},
		{"what is the capital of France", 3, corpus[3]},
		{"healthy eating", 4, corpus[4]},
		{"data frame library", 5, corpus[5]},
		{"image classification models", 6, corpus[6]},
		{"sql database engine", 7, corpus[7]},
		{"application packaging", 8, corpus[8]},
		{"largest ocean on Earth", 9, corpus[9]},
	}

	var top1Hits int
	for _, q := range queries {
		qv, err := enc.Embed(ctx, q.q)
		if err != nil {
			t.Fatal(err)
		}
		ranks := make([]rank, len(corpus))
		for i, cv := range corpusVecs {
			ranks[i] = rank{idx: i, sim: cosine(qv, cv)}
		}
		sort.Slice(ranks, func(i, j int) bool { return ranks[i].sim > ranks[j].sim })
		if ranks[0].idx == q.wantIdx {
			top1Hits++
			continue
		}
		t.Errorf("query %q: top-1 %q (%.3f), expected %q (rank %d, %.3f)",
			q.q, corpus[ranks[0].idx], ranks[0].sim,
			q.wantText, findRank(ranks, q.wantIdx),
			simAt(ranks, q.wantIdx))
	}
	t.Logf("retrieval: top-1 accuracy %d/%d = %.0f%%",
		top1Hits, len(queries), 100.0*float64(top1Hits)/float64(len(queries)))
}

type rank struct {
	idx int
	sim float32
}

func findRank(ranks []rank, idx int) int {
	for i, r := range ranks {
		if r.idx == idx {
			return i + 1
		}
	}
	return -1
}

func simAt(ranks []rank, idx int) float32 {
	for _, r := range ranks {
		if r.idx == idx {
			return r.sim
		}
	}
	return 0
}

// BenchmarkEncoder_Throughput measures single-threaded throughput on a
// corpus of realistic short passages. Useful to track perf regressions.
func BenchmarkEncoder_Throughput(b *testing.B) {
	if os.Getenv("MARROW_MINILM_DIR") == "" {
		b.Skip("set MARROW_MINILM_DIR to run encoder benchmark")
	}
	enc, err := New(os.Getenv("MARROW_MINILM_DIR"))
	if err != nil {
		b.Fatal(err)
	}
	texts := []string{
		"Marrow is a local-first hybrid search engine for Markdown.",
		"Goroutines and channels make concurrent Go code simple and safe.",
		"Retrieval-augmented generation combines dense retrieval with an LLM.",
		"Convolutional neural networks achieve high accuracy on ImageNet.",
		"The capital of France is Paris, home to the Eiffel Tower.",
	}
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := enc.Embed(ctx, texts[i%len(texts)])
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkEncoder_Parallel measures N-way throughput now that Embed is
// lock-free. The gap between -1 and -P ratios shows how well the encoder
// scales across cores for bulk indexing jobs.
func BenchmarkEncoder_Parallel(b *testing.B) {
	if os.Getenv("MARROW_MINILM_DIR") == "" {
		b.Skip("set MARROW_MINILM_DIR to run encoder benchmark")
	}
	enc, err := New(os.Getenv("MARROW_MINILM_DIR"))
	if err != nil {
		b.Fatal(err)
	}
	texts := []string{
		"Marrow is a local-first hybrid search engine for Markdown.",
		"Goroutines and channels make concurrent Go code simple and safe.",
		"Retrieval-augmented generation combines dense retrieval with an LLM.",
		"Convolutional neural networks achieve high accuracy on ImageNet.",
		"The capital of France is Paris, home to the Eiffel Tower.",
	}
	ctx := context.Background()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if _, err := enc.Embed(ctx, texts[i%len(texts)]); err != nil {
				b.Fatal(err)
			}
			i++
		}
	})
}

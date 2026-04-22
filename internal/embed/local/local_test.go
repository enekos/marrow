package local

import (
	"context"
	"sort"
	"sync"
	"testing"
)

// Behavioral tests for the local encoder that require the actual MiniLM
// weights. All tests here skip when MARROW_MINILM_DIR is unset so
// `go test ./...` stays green for contributors who have not installed the
// model.

// TestRetrieval_SmallCorpus asserts that the expected passage ranks first
// under our encoder across a heterogeneous corpus. Parity checks individual
// vectors; this checks the geometry that retrieval actually depends on.
func TestRetrieval_SmallCorpus(t *testing.T) {
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
	queries := []struct {
		query   string
		wantIdx int
	}{
		{"concurrency in Go", 0},
		{"safe memory management in systems languages", 1},
		{"async programming in Python", 2},
		{"what is the capital of France", 3},
		{"healthy eating", 4},
		{"data frame library", 5},
		{"image classification models", 6},
		{"sql database engine", 7},
		{"application packaging", 8},
		{"largest ocean on Earth", 9},
	}

	corpusVecs := make([][]float32, len(corpus))
	for i, c := range corpus {
		v, err := enc.Embed(ctx, c)
		if err != nil {
			t.Fatal(err)
		}
		corpusVecs[i] = v
	}

	hits := 0
	for _, q := range queries {
		qv, err := enc.Embed(ctx, q.query)
		if err != nil {
			t.Fatal(err)
		}
		top := topMatch(qv, corpusVecs)
		if top == q.wantIdx {
			hits++
			continue
		}
		t.Errorf("query %q: top-1 %q, expected %q",
			q.query, corpus[top], corpus[q.wantIdx])
	}
	t.Logf("retrieval: top-1 %d/%d", hits, len(queries))
}

// TestEmbed_ConcurrentSafety runs many goroutines encoding the same inputs
// and asserts bit-identical output to a single-threaded baseline. Catches
// any scratch-buffer sharing introduced while tuning perf.
func TestEmbed_ConcurrentSafety(t *testing.T) {
	enc := loadEncoder(t)
	ctx := context.Background()

	inputs := []string{
		"The quick brown fox jumps over the lazy dog.",
		"hello world",
		"Marrow is a local-first search engine.",
		"Another passage for embedding concurrency testing.",
		"Short",
	}
	want := make([][]float32, len(inputs))
	for i, s := range inputs {
		v, err := enc.Embed(ctx, s)
		if err != nil {
			t.Fatal(err)
		}
		want[i] = v
	}

	const workers, iters = 8, 20
	var wg sync.WaitGroup
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func() {
			defer wg.Done()
			for it := 0; it < iters; it++ {
				for i, s := range inputs {
					got, err := enc.Embed(ctx, s)
					if err != nil {
						t.Errorf("embed: %v", err)
						return
					}
					for k := range got {
						if got[k] != want[i][k] {
							t.Errorf("input %d idx %d: got %v want %v", i, k, got[k], want[i][k])
							return
						}
					}
				}
			}
		}()
	}
	wg.Wait()
}

// topMatch returns the index of the corpus vector with highest cosine
// similarity to q.
func topMatch(q []float32, corpus [][]float32) int {
	type scored struct {
		idx int
		sim float32
	}
	ranks := make([]scored, len(corpus))
	for i, v := range corpus {
		ranks[i] = scored{i, cosine(q, v)}
	}
	sort.Slice(ranks, func(i, j int) bool { return ranks[i].sim > ranks[j].sim })
	return ranks[0].idx
}

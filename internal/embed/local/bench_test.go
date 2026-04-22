package local

import (
	"context"
	"strings"
	"testing"
)

// BenchmarkEncode measures end-to-end tokenize + forward-pass latency at
// three representative input lengths. Requires MARROW_MINILM_DIR; skipped
// otherwise.
func BenchmarkEncode(b *testing.B) {
	enc := loadEncoder(b)
	ctx := context.Background()

	cases := []struct {
		name string
		text string
	}{
		{"short", "hello world"},
		{"medium", "The quick brown fox jumps over the lazy dog. A stitch in time saves nine. Practice makes perfect."},
		{"long", strings.Repeat("Marrow is a local-first hybrid search engine that combines lexical and vector retrieval. ", 40)},
	}
	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			if _, err := enc.Embed(ctx, c.text); err != nil {
				b.Fatal(err)
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := enc.Embed(ctx, c.text); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkEncodeParallel measures throughput under N-way concurrent
// callers. The gap between this and BenchmarkEncode shows how well the
// encoder saturates multiple cores for bulk indexing.
func BenchmarkEncodeParallel(b *testing.B) {
	enc := loadEncoder(b)
	ctx := context.Background()
	text := "Retrieval-augmented generation combines dense retrieval with a generative model."

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := enc.Embed(ctx, text); err != nil {
				b.Fatal(err)
			}
		}
	})
}

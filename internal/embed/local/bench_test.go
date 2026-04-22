package local

import (
	"context"
	"os"
	"strings"
	"testing"
)

// BenchmarkEncode measures end-to-end tokenize + forward-pass time at three
// representative input lengths. Requires MARROW_MINILM_DIR; skipped otherwise.
func BenchmarkEncode(b *testing.B) {
	modelDir := os.Getenv("MARROW_MINILM_DIR")
	if modelDir == "" {
		b.Skip("set MARROW_MINILM_DIR to run benchmark")
	}
	enc, err := New(modelDir)
	if err != nil {
		b.Fatal(err)
	}
	ctx := context.Background()

	short := "hello world"
	medium := "The quick brown fox jumps over the lazy dog. A stitch in time saves nine. Practice makes perfect."
	long := strings.Repeat("Marrow is a local-first hybrid search engine that combines lexical and vector retrieval. ", 40)

	cases := []struct {
		name string
		text string
	}{
		{"short-2-tokens", short},
		{"medium-~30-tokens", medium},
		{"long-at-cap", long},
	}
	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			// Warm up.
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

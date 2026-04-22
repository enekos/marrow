package local

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// Shared test helpers. All tests that drive the real encoder go through
// loadEncoder, which skips when MARROW_MINILM_DIR is unset so contributors
// without the model on disk can still run `go test ./...`.

type parityFixture struct {
	Canaries []struct {
		Text      string    `json:"text"`
		Embedding []float32 `json:"embedding"`
	} `json:"canaries"`
	Ranking []struct {
		Query             string    `json:"query"`
		Positive          string    `json:"positive"`
		Negative          string    `json:"negative"`
		QueryEmbedding    []float32 `json:"query_embedding"`
		PositiveEmbedding []float32 `json:"positive_embedding"`
		NegativeEmbedding []float32 `json:"negative_embedding"`
	} `json:"ranking"`
}

func loadEncoder(tb testing.TB) *Encoder {
	tb.Helper()
	modelDir := os.Getenv("MARROW_MINILM_DIR")
	if modelDir == "" {
		tb.Skip("set MARROW_MINILM_DIR to run tests against the real model")
	}
	enc, err := New(modelDir)
	if err != nil {
		tb.Fatal(err)
	}
	return enc
}

func loadFixture(tb testing.TB) parityFixture {
	tb.Helper()
	path := filepath.Join("testdata", "parity.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		tb.Skipf("parity fixture not present at %s — run scripts/gen-parity-fixture.py", path)
	}
	var fx parityFixture
	if err := json.Unmarshal(raw, &fx); err != nil {
		tb.Fatal(err)
	}
	return fx
}

func cosine(a, b []float32) float32 {
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return float32(dot / math.Sqrt(na*nb))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

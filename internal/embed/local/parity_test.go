package local

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Parity against the reference sentence-transformers/all-MiniLM-L6-v2
// implementation. Two inputs are needed:
//
//   1. A model directory containing tokenizer_vocab.txt + model.safetensors,
//      pointed to by $MARROW_MINILM_DIR.
//   2. A JSON fixture at testdata/parity.json produced by
//      scripts/gen-parity-fixture.py.
//
// Both are large and held outside the repo. When either is missing the test
// is skipped so that `go test ./...` stays green for contributors who have
// not installed the model.
func TestParity_AgainstReference(t *testing.T) {
	modelDir := os.Getenv("MARROW_MINILM_DIR")
	if modelDir == "" {
		t.Skip("set MARROW_MINILM_DIR to run parity test")
	}
	fixturePath := filepath.Join("testdata", "parity.json")
	if _, err := os.Stat(fixturePath); err != nil {
		t.Skipf("parity fixture not present at %s — run scripts/gen-parity-fixture.py", fixturePath)
	}

	raw, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Text      string    `json:"text"`
		Embedding []float32 `json:"embedding"`
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}

	enc, err := New(modelDir)
	if err != nil {
		t.Fatal(err)
	}

	for _, c := range cases {
		got, err := enc.Embed(context.Background(), c.Text)
		if err != nil {
			t.Fatalf("embed %q: %v", c.Text, err)
		}
		if len(got) != len(c.Embedding) {
			t.Fatalf("%q: dim got %d want %d", c.Text, len(got), len(c.Embedding))
		}
		sim := cosine(got, c.Embedding)
		if sim < 0.999 {
			t.Errorf("%q: cosine similarity %.6f < 0.999", c.Text, sim)
		}
	}
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
	return float32(dot / (sqrt(na) * sqrt(nb)))
}

func sqrt(x float64) float64 {
	if x == 0 {
		return 0
	}
	// Newton's method — avoids a math import just for this helper.
	guess := x / 2
	for i := 0; i < 20; i++ {
		guess = (guess + x/guess) / 2
	}
	return guess
}

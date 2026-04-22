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

func loadFixture(t *testing.T) parityFixture {
	t.Helper()
	fixturePath := filepath.Join("testdata", "parity.json")
	if _, err := os.Stat(fixturePath); err != nil {
		t.Skipf("parity fixture not present at %s — run scripts/gen-parity-fixture.py", fixturePath)
	}
	raw, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatal(err)
	}
	var fx parityFixture
	if err := json.Unmarshal(raw, &fx); err != nil {
		t.Fatal(err)
	}
	return fx
}

func loadEncoder(t *testing.T) *Encoder {
	t.Helper()
	modelDir := os.Getenv("MARROW_MINILM_DIR")
	if modelDir == "" {
		t.Skip("set MARROW_MINILM_DIR to run parity test")
	}
	enc, err := New(modelDir)
	if err != nil {
		t.Fatal(err)
	}
	return enc
}

// TestParity_AgainstReference asserts vector-level parity for every canary
// input. We require cosine ≥ 0.999 per input plus a tightening aggregate
// check (mean ≥ 0.9999) so a regression on a single item stays noisy.
func TestParity_AgainstReference(t *testing.T) {
	fx := loadFixture(t)
	enc := loadEncoder(t)
	ctx := context.Background()

	var worst float32 = 1
	var worstText string
	var sum float64
	for _, c := range fx.Canaries {
		got, err := enc.Embed(ctx, c.Text)
		if err != nil {
			t.Fatalf("embed %q: %v", c.Text, err)
		}
		if len(got) != len(c.Embedding) {
			t.Fatalf("%q: dim got %d want %d", c.Text, len(got), len(c.Embedding))
		}
		sim := cosine(got, c.Embedding)
		if sim < 0.999 {
			t.Errorf("%q: cosine similarity %.6f < 0.999", truncate(c.Text, 80), sim)
		}
		if sim < worst {
			worst = sim
			worstText = c.Text
		}
		sum += float64(sim)
	}
	mean := sum / float64(len(fx.Canaries))
	t.Logf("parity: %d inputs, mean cosine %.6f, worst %.6f on %q",
		len(fx.Canaries), mean, worst, truncate(worstText, 60))
	if mean < 0.9999 {
		t.Errorf("aggregate mean cosine %.6f < 0.9999 — real quality drift", mean)
	}
}

// TestParity_RankingPreserved asserts that query·positive > query·negative
// under our encoder, matching the reference's ordering. This catches
// regressions where absolute cosine stays high but the relative geometry of
// the embedding space drifts (which would silently harm retrieval).
func TestParity_RankingPreserved(t *testing.T) {
	fx := loadFixture(t)
	enc := loadEncoder(t)
	ctx := context.Background()

	for _, r := range fx.Ranking {
		// Sanity-check the reference ordering first so a broken fixture
		// doesn't look like a Go-encoder bug.
		refPos := cosine(r.QueryEmbedding, r.PositiveEmbedding)
		refNeg := cosine(r.QueryEmbedding, r.NegativeEmbedding)
		if refPos <= refNeg {
			t.Fatalf("reference ranking unexpectedly inverted for %q (pos=%.4f neg=%.4f)",
				r.Query, refPos, refNeg)
		}

		qv, err := enc.Embed(ctx, r.Query)
		if err != nil {
			t.Fatal(err)
		}
		pv, err := enc.Embed(ctx, r.Positive)
		if err != nil {
			t.Fatal(err)
		}
		nv, err := enc.Embed(ctx, r.Negative)
		if err != nil {
			t.Fatal(err)
		}
		goPos := cosine(qv, pv)
		goNeg := cosine(qv, nv)
		if goPos <= goNeg {
			t.Errorf("ranking flip for query %q: pos=%.4f vs neg=%.4f (reference margin %.4f)",
				truncate(r.Query, 60), goPos, goNeg, refPos-refNeg)
		}
		// Also assert the margin doesn't shrink dramatically: if the ref
		// has pos-neg margin M, our margin must be ≥ 0.7*M. This catches
		// cases where ordering is preserved but discrimination is eroded.
		refMargin := refPos - refNeg
		goMargin := goPos - goNeg
		if goMargin < 0.7*refMargin {
			t.Errorf("margin collapse on %q: ref=%.4f go=%.4f (ratio %.2f)",
				truncate(r.Query, 60), refMargin, goMargin, float64(goMargin)/float64(refMargin))
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

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

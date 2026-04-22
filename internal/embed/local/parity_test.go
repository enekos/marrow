package local

import (
	"context"
	"testing"
)

// Parity against the reference sentence-transformers/all-MiniLM-L6-v2
// implementation. Two inputs are needed:
//
//  1. A model directory containing tokenizer_vocab.txt + model.safetensors,
//     pointed to by $MARROW_MINILM_DIR.
//  2. A JSON fixture at testdata/parity.json produced by
//     scripts/gen-parity-fixture.py.
//
// Both are large and held outside the repo. When either is missing the
// tests are skipped so `go test ./...` stays green for contributors who
// have not installed the model.

// TestParity_AgainstReference asserts vector-level parity for every canary.
// We require cosine ≥ 0.999 per input plus an aggregate mean ≥ 0.9999 so a
// regression on a single item still stays loud.
func TestParity_AgainstReference(t *testing.T) {
	fx := loadFixture(t)
	enc := loadEncoder(t)
	ctx := context.Background()

	var sum float64
	var worst float32 = 1
	var worstText string
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
			t.Errorf("%q: cosine %.6f < 0.999", truncate(c.Text, 80), sim)
		}
		if sim < worst {
			worst, worstText = sim, c.Text
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
// under our encoder, matching the reference. This catches regressions where
// absolute cosine stays high but relative geometry drifts.
func TestParity_RankingPreserved(t *testing.T) {
	fx := loadFixture(t)
	enc := loadEncoder(t)
	ctx := context.Background()

	for _, r := range fx.Ranking {
		refPos := cosine(r.QueryEmbedding, r.PositiveEmbedding)
		refNeg := cosine(r.QueryEmbedding, r.NegativeEmbedding)
		if refPos <= refNeg {
			t.Fatalf("reference ranking inverted for %q (pos=%.4f neg=%.4f)",
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
		goPos, goNeg := cosine(qv, pv), cosine(qv, nv)
		if goPos <= goNeg {
			t.Errorf("ranking flip for %q: pos=%.4f neg=%.4f (ref margin %.4f)",
				truncate(r.Query, 60), goPos, goNeg, refPos-refNeg)
		}
		// Also require our margin to stay ≥ 70% of the reference's — catches
		// cases where ordering is preserved but discrimination erodes.
		refMargin, goMargin := refPos-refNeg, goPos-goNeg
		if goMargin < 0.7*refMargin {
			t.Errorf("margin collapse on %q: ref=%.4f go=%.4f",
				truncate(r.Query, 60), refMargin, goMargin)
		}
	}
}

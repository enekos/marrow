package local

import (
	"context"
	"math"
	"os"
	"testing"
)

// TestCorrectness_SemanticBehavior exercises the encoder with the real
// MiniLM weights and asserts known qualitative relationships: paraphrase
// pairs have high cosine, same-frame-different-entities have lower cosine,
// unrelated pairs are near zero. Catches regressions even when no Python
// reference is available.
func TestCorrectness_SemanticBehavior(t *testing.T) {
	modelDir := os.Getenv("MARROW_MINILM_DIR")
	if modelDir == "" {
		t.Skip("set MARROW_MINILM_DIR to run")
	}
	enc, err := New(modelDir)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	embed := func(s string) []float32 {
		v, err := enc.Embed(ctx, s)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}

	paris1 := embed("Paris is the capital of France.")
	paris2 := embed("The capital city of France is Paris.")
	berlin := embed("Berlin is the capital of Germany.")
	weather := embed("The weather in San Francisco is typically mild.")
	hello := embed("hello world")

	// All outputs should be 384-dim and normalized.
	for _, v := range [][]float32{paris1, paris2, berlin, weather, hello} {
		if len(v) != 384 {
			t.Fatalf("dim = %d want 384", len(v))
		}
		var norm float64
		for _, x := range v {
			norm += float64(x) * float64(x)
		}
		if math.Abs(norm-1) > 1e-4 {
			t.Fatalf("L2 norm = %f want ~1", norm)
		}
	}

	// Paraphrase cosine should dominate.
	parisParaphrase := cosine(paris1, paris2)
	parisVsBerlin := cosine(paris1, berlin)
	parisVsWeather := cosine(paris1, weather)
	parisVsHello := cosine(paris1, hello)

	t.Logf("paraphrase: %.4f, capital-swap: %.4f, unrelated-weather: %.4f, hello-world: %.4f",
		parisParaphrase, parisVsBerlin, parisVsWeather, parisVsHello)

	if parisParaphrase < 0.9 {
		t.Errorf("paraphrase cosine %.4f < 0.9 — model likely broken", parisParaphrase)
	}
	if parisParaphrase <= parisVsBerlin {
		t.Errorf("paraphrase (%.4f) should dominate capital-swap (%.4f)",
			parisParaphrase, parisVsBerlin)
	}
	if parisVsBerlin <= parisVsWeather {
		t.Errorf("same-frame (%.4f) should dominate unrelated (%.4f)",
			parisVsBerlin, parisVsWeather)
	}
	if parisVsHello > 0.2 {
		t.Errorf("unrelated-topic cosine %.4f > 0.2 — drift in geometry", parisVsHello)
	}
}

package embed

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"
	"math/rand/v2"
)

const EmbeddingDim = 384

// Func generates a vector for the given text.
type Func func(ctx context.Context, text string) ([]float32, error)

// BatchFunc generates vectors for many texts at once. Providers that support
// batched inference (OpenAI) override this to send one HTTP request per
// batch; providers that do not (Ollama, mock) fall back to sequential Func
// calls via FallbackBatch.
type BatchFunc func(ctx context.Context, texts []string) ([][]float32, error)

// FallbackBatch implements BatchFunc semantics by calling f once per text.
// Used by providers that lack native batching.
func FallbackBatch(f Func) BatchFunc {
	return func(ctx context.Context, texts []string) ([][]float32, error) {
		out := make([][]float32, len(texts))
		for i, t := range texts {
			v, err := f(ctx, t)
			if err != nil {
				return nil, err
			}
			out[i] = v
		}
		return out, nil
	}
}

// Validate probes the embedder with a canary string and verifies the output
// dimension matches the DB schema. Run at startup so misconfigured providers
// (e.g. nomic-embed-text → 768, text-embedding-3-small → 1536) fail loudly
// instead of producing silent INSERT errors at index time.
func Validate(ctx context.Context, f Func) error {
	if f == nil {
		return fmt.Errorf("embedder is nil")
	}
	vec, err := f(ctx, "marrow embedding validation canary")
	if err != nil {
		return fmt.Errorf("canary embedding failed: %w", err)
	}
	if len(vec) != EmbeddingDim {
		return fmt.Errorf("embedding dimension mismatch: provider returned %d, schema requires %d", len(vec), EmbeddingDim)
	}
	return nil
}

// NewMock returns a deterministic mock embedding function.
// It produces stable 384-dim normalized vectors derived from a SHA-256 hash of the input text.
// This is useful for testing and CI where no external embedding service is available.
func NewMock() Func {
	return func(_ context.Context, text string) ([]float32, error) {
		if text == "" {
			return make([]float32, EmbeddingDim), nil
		}
		h := sha256.Sum256([]byte(text))
		seed := binary.BigEndian.Uint64(h[:8])
		src := rand.New(rand.NewPCG(seed, seed+1))

		vec := make([]float32, EmbeddingDim)
		var sum float64
		for i := range vec {
			// Box-Muller transform for normal distribution
			u1 := src.Float64()
			u2 := src.Float64()
			if u1 == 0 {
				u1 = 1e-10
			}
			z := math.Sqrt(-2*math.Log(u1)) * math.Cos(2*math.Pi*u2)
			vec[i] = float32(z)
			sum += float64(vec[i]) * float64(vec[i])
		}

		// L2 normalize
		norm := math.Sqrt(sum)
		if norm > 0 {
			for i := range vec {
				vec[i] /= float32(norm)
			}
		}
		return vec, nil
	}
}


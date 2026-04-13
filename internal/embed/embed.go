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
		src := rand.New(rand.NewPCG(seed, seed))

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

// SerializeF32 returns a sqlite-vec compatible blob for a float32 slice.
func SerializeF32(vec []float32) []byte {
	buf := make([]byte, len(vec)*4)
	for i, v := range vec {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(v))
	}
	return buf
}

// DeserializeF32 parses a sqlite-vec blob back into float32 slice.
func DeserializeF32(buf []byte) ([]float32, error) {
	if len(buf)%4 != 0 {
		return nil, fmt.Errorf("invalid blob length %d", len(buf))
	}
	vec := make([]float32, len(buf)/4)
	for i := range vec {
		vec[i] = math.Float32frombits(binary.LittleEndian.Uint32(buf[i*4:]))
	}
	return vec, nil
}

//go:build amd64

package mat

import (
	"math"
	"math/rand/v2"
	"testing"
)

// TestDotAVX2Forced exercises the AVX2 kernel directly even when
// cpu.X86.HasAVX2 is false (Rosetta 2 hides AVX2 from CPUID but supports
// the instruction stream on M-series Macs). Native Linux/amd64 hosts
// already cover dotAVX2 via TestDotParity; this test makes sure the
// kernel itself works when invoked.
func TestDotAVX2Forced(t *testing.T) {
	if !hasAVX2 {
		// Toggle the dispatch flag only for this test; restore so other
		// tests observe the real CPU capability.
		hasAVX2 = true
		defer func() { hasAVX2 = false }()
	}
	sizes := []int{0, 1, 7, 8, 9, 31, 32, 33, 63, 64, 384, 385, 1535, 1536}
	rng := rand.New(rand.NewPCG(7, 9))
	for _, n := range sizes {
		a := make([]float32, n)
		b := make([]float32, n)
		for i := range a {
			a[i] = rng.Float32()*2 - 1
			b[i] = rng.Float32()*2 - 1
		}
		got := dot(a, b)
		want := dotScalar(a, b)
		tol := float32(math.Max(1e-6, float64(n)*1e-6))
		if math.Abs(float64(got-want)) > float64(tol) {
			t.Fatalf("n=%d: dotAVX2=%v dotScalar=%v diff=%v", n, got, want, got-want)
		}
	}
}

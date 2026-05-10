package mat

import (
	"math"
	"math/rand/v2"
	"testing"
)

// TestDotParity verifies the active dot implementation (SIMD when present)
// matches the scalar reference across length classes that exercise the
// 16-float main loop, the 4-float remainder loop, and the 0..3 scalar
// tail. Sizes 0/1/2/3/4/5/15/16/17/63/64/65/383/384/385 hit every branch
// in dot_arm64.s and dot_amd64.s.
func TestDotParity(t *testing.T) {
	sizes := []int{0, 1, 2, 3, 4, 5, 7, 8, 15, 16, 17, 31, 32, 63, 64, 65, 127, 128, 383, 384, 385, 1535, 1536}
	rng := rand.New(rand.NewPCG(1, 2))
	for _, n := range sizes {
		a := make([]float32, n)
		b := make([]float32, n)
		for i := range a {
			a[i] = rng.Float32()*2 - 1
			b[i] = rng.Float32()*2 - 1
		}
		got := dot(a, b)
		want := dotScalar(a, b)
		// fp32 accumulation is order-dependent, so we allow a relative
		// epsilon scaled by N (the max number of additions). 1e-5 per
		// element is well above the rounding noise of either kernel.
		tol := float32(math.Max(1e-6, float64(n)*1e-6))
		if math.Abs(float64(got-want)) > float64(tol) {
			t.Fatalf("n=%d: dot=%v dotScalar=%v diff=%v", n, got, want, got-want)
		}
	}
}

// TestDotKnownValues pins a handful of trivial cases — the parity test
// uses random inputs and a tolerance, so a few exact-equality checks
// guard against silent rounding-mode regressions in the SIMD kernels.
func TestDotKnownValues(t *testing.T) {
	cases := []struct {
		name string
		a, b []float32
		want float32
	}{
		{"empty", nil, nil, 0},
		{"single", []float32{3}, []float32{4}, 12},
		{"sub-tile", []float32{1, 2, 3}, []float32{1, 1, 1}, 6},
		{"one-tile", []float32{1, 2, 3, 4}, []float32{1, 1, 1, 1}, 10},
		{"two-tile", []float32{1, 2, 3, 4, 5, 6, 7, 8}, []float32{1, 1, 1, 1, 1, 1, 1, 1}, 36},
		{"main-loop", repeat(1, 16), repeat(1, 16), 16},
		{"main-loop-plus-one", repeat(1, 17), repeat(1, 17), 17},
	}
	for _, c := range cases {
		if got := dot(c.a, c.b); got != c.want {
			t.Errorf("%s: dot=%v want %v", c.name, got, c.want)
		}
	}
}

func repeat(v float32, n int) []float32 {
	r := make([]float32, n)
	for i := range r {
		r[i] = v
	}
	return r
}

// TestDotMismatchedLengths confirms dot returns the prefix dot product
// when slices differ in length, matching the original Go contract.
func TestDotMismatchedLengths(t *testing.T) {
	a := []float32{1, 2, 3, 4, 5}
	b := []float32{10, 20, 30}
	want := float32(1*10 + 2*20 + 3*30)
	got := dot(a, b)
	if got != want {
		t.Fatalf("got %v want %v", got, want)
	}
	// Reverse: longer b.
	got = dot(b, a)
	if got != want {
		t.Fatalf("reverse: got %v want %v", got, want)
	}
}

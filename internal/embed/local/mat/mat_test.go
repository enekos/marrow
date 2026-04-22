package mat

import (
	"math"
	"testing"
)

func approxEq(a, b, tol float32) bool {
	return math.Abs(float64(a-b)) <= float64(tol)
}

func TestMatMul_2x3_3x2(t *testing.T) {
	a := []float32{
		1, 2, 3,
		4, 5, 6,
	}
	b := []float32{
		7, 8,
		9, 10,
		11, 12,
	}
	c := make([]float32, 4)
	MatMul(a, b, c, 2, 3, 2)
	want := []float32{
		58, 64,
		139, 154,
	}
	for i := range want {
		if !approxEq(c[i], want[i], 1e-5) {
			t.Fatalf("c[%d]=%v want %v", i, c[i], want[i])
		}
	}
}

func TestMatMulTransposeB_EquivalentToMatMul(t *testing.T) {
	// B = 3x2, Bᵀ = 2x3. Store B so that MatMulTransposeB treats it as Nx K.
	a := []float32{1, 2, 3, 4, 5, 6} // 2x3
	// Same multiplication as above but provide B with rows swapped to the
	// "N x K" layout MatMulTransposeB expects. Here K=3 (cols of A),
	// and N=2 (rows of output). So B must be 2x3.
	// For MatMul the B was 3x2: [7,8; 9,10; 11,12]. Transposed: [7,9,11; 8,10,12].
	bT := []float32{
		7, 9, 11,
		8, 10, 12,
	}
	c := make([]float32, 4)
	MatMulTransposeB(a, bT, c, 2, 3, 2)
	want := []float32{58, 64, 139, 154}
	for i := range want {
		if !approxEq(c[i], want[i], 1e-5) {
			t.Fatalf("c[%d]=%v want %v", i, c[i], want[i])
		}
	}
}

func TestLayerNorm_ZeroMeanUnitVar(t *testing.T) {
	x := []float32{1, 2, 3, 4, 5, 6, 7, 8}
	gamma := []float32{1, 1, 1, 1}
	beta := []float32{0, 0, 0, 0}
	LayerNorm(x, gamma, beta, 2, 4, 1e-12)
	// Each row should now have mean ~0 and var ~1.
	for r := 0; r < 2; r++ {
		row := x[r*4 : (r+1)*4]
		var mean, sq float64
		for _, v := range row {
			mean += float64(v)
		}
		mean /= 4
		for _, v := range row {
			d := float64(v) - mean
			sq += d * d
		}
		variance := sq / 4
		if math.Abs(mean) > 1e-4 {
			t.Fatalf("row %d mean=%v", r, mean)
		}
		if math.Abs(variance-1) > 1e-3 {
			t.Fatalf("row %d var=%v", r, variance)
		}
	}
}

func TestSoftmax_SumsToOne(t *testing.T) {
	x := []float32{1, 2, 3, 4}
	SoftmaxRows(x, 1, 4)
	var sum float32
	for _, v := range x {
		sum += v
	}
	if !approxEq(sum, 1, 1e-6) {
		t.Fatalf("sum=%v", sum)
	}
	// Largest input → largest output.
	if !(x[3] > x[2] && x[2] > x[1] && x[1] > x[0]) {
		t.Fatalf("monotonicity broken: %v", x)
	}
}

func TestGELU_AtZeroIsZero(t *testing.T) {
	x := []float32{0}
	GELU(x)
	if !approxEq(x[0], 0, 1e-6) {
		t.Fatalf("gelu(0)=%v", x[0])
	}
}

func TestGELU_AtOneKnownValue(t *testing.T) {
	// GELU(1) ≈ 0.8413447
	x := []float32{1}
	GELU(x)
	if !approxEq(x[0], 0.8413447, 1e-4) {
		t.Fatalf("gelu(1)=%v", x[0])
	}
}

func TestL2Normalize(t *testing.T) {
	x := []float32{3, 4}
	L2Normalize(x)
	if !approxEq(x[0], 0.6, 1e-6) || !approxEq(x[1], 0.8, 1e-6) {
		t.Fatalf("got %v", x)
	}
}

func TestMeanPoolMasked(t *testing.T) {
	// 3 tokens, hidden=2. mask = [1,1,0] → mean of first two rows.
	x := []float32{
		1, 2,
		3, 4,
		100, 200,
	}
	mask := []int32{1, 1, 0}
	out := make([]float32, 2)
	MeanPoolMasked(x, mask, 3, 2, out)
	if !approxEq(out[0], 2, 1e-6) || !approxEq(out[1], 3, 1e-6) {
		t.Fatalf("got %v", out)
	}
}

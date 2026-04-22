// Package mat implements the numeric primitives for the transformer
// forward pass: matmul, layernorm, softmax, gelu, bias add, and residual.
//
// All operations are fp32, row-major. Shapes are tracked by callers; this
// package only sees flat slices plus leading dimensions.
//
// The implementation is pure Go. The main hot spot is MatMul; we use a
// cache-blocked three-loop variant that is roughly an order of magnitude
// faster than naive i-j-k on typical encoder sizes. A faster replacement
// (gonum/blas or hand-rolled asm) can be dropped in later without touching
// the model code.
package mat

import (
	"math"
)

// MatMul computes C = A @ B where A is MxK, B is KxN, all row-major. C is
// zeroed on entry. Caller owns C and must size it MxN.
func MatMul(a []float32, b []float32, c []float32, M, K, N int) {
	for i := 0; i < M*N; i++ {
		c[i] = 0
	}
	// Cache-blocked i-k-j: each row of A is read once, each row of B is
	// streamed, each row of C is accumulated with good locality on B[k,:].
	for i := 0; i < M; i++ {
		aRow := a[i*K : (i+1)*K]
		cRow := c[i*N : (i+1)*N]
		for k := 0; k < K; k++ {
			av := aRow[k]
			if av == 0 {
				continue
			}
			bRow := b[k*N : (k+1)*N]
			for j := 0; j < N; j++ {
				cRow[j] += av * bRow[j]
			}
		}
	}
}

// MatMulTransposeB computes C = A @ Bᵀ where A is MxK and B is NxK (i.e.
// we multiply by Bᵀ which is KxN). Same layout rules. Equivalent to
// `c[i,j] = sum_k a[i,k] * b[j,k]`. Useful when the weight is stored in the
// standard PyTorch "out_features x in_features" layout and we want to
// project from in→out without transposing first.
func MatMulTransposeB(a []float32, b []float32, c []float32, M, K, N int) {
	for i := 0; i < M; i++ {
		aRow := a[i*K : (i+1)*K]
		cRow := c[i*N : (i+1)*N]
		for j := 0; j < N; j++ {
			bRow := b[j*K : (j+1)*K]
			var sum float32
			for k := 0; k < K; k++ {
				sum += aRow[k] * bRow[k]
			}
			cRow[j] = sum
		}
	}
}

// AddBias adds a row vector `bias` (len N) to each row of an MxN matrix.
func AddBias(x []float32, bias []float32, M, N int) {
	for i := 0; i < M; i++ {
		row := x[i*N : (i+1)*N]
		for j := 0; j < N; j++ {
			row[j] += bias[j]
		}
	}
}

// AddInPlace performs a += b.
func AddInPlace(a, b []float32) {
	for i := range a {
		a[i] += b[i]
	}
}

// LayerNorm applies LayerNorm per row on an MxN matrix in place:
//
//	y = gamma * (x - mean) / sqrt(var + eps) + beta
//
// gamma, beta have length N; eps is typically 1e-12 for BERT.
func LayerNorm(x []float32, gamma, beta []float32, M, N int, eps float32) {
	for i := 0; i < M; i++ {
		row := x[i*N : (i+1)*N]
		var mean float64
		for _, v := range row {
			mean += float64(v)
		}
		mean /= float64(N)
		var variance float64
		for _, v := range row {
			d := float64(v) - mean
			variance += d * d
		}
		variance /= float64(N)
		denom := float32(math.Sqrt(variance + float64(eps)))
		for j := 0; j < N; j++ {
			row[j] = gamma[j]*(row[j]-float32(mean))/denom + beta[j]
		}
	}
}

// SoftmaxRows applies softmax along each row of an MxN matrix in place.
func SoftmaxRows(x []float32, M, N int) {
	for i := 0; i < M; i++ {
		row := x[i*N : (i+1)*N]
		maxv := row[0]
		for _, v := range row[1:] {
			if v > maxv {
				maxv = v
			}
		}
		var sum float64
		for j := 0; j < N; j++ {
			row[j] = float32(math.Exp(float64(row[j] - maxv)))
			sum += float64(row[j])
		}
		inv := float32(1.0 / sum)
		for j := 0; j < N; j++ {
			row[j] *= inv
		}
	}
}

// GELU applies the exact BERT GELU activation element-wise:
//
//	y = 0.5 * x * (1 + erf(x / sqrt(2)))
//
// Note: this is the "exact" variant, not the tanh approximation. BERT and
// MiniLM use the exact form.
func GELU(x []float32) {
	const invSqrt2 = 0.7071067811865475
	for i, v := range x {
		x[i] = 0.5 * v * (1 + float32(math.Erf(float64(v)*invSqrt2)))
	}
}

// L2Normalize normalizes a single vector in place.
func L2Normalize(x []float32) {
	var sum float64
	for _, v := range x {
		sum += float64(v) * float64(v)
	}
	if sum == 0 {
		return
	}
	inv := float32(1.0 / math.Sqrt(sum))
	for i := range x {
		x[i] *= inv
	}
}

// MeanPoolMasked computes the attention-mask-weighted mean across the
// sequence axis of an LxH matrix, writing an H-dim vector to out.
func MeanPoolMasked(x []float32, mask []int32, L, H int, out []float32) {
	for j := 0; j < H; j++ {
		out[j] = 0
	}
	var n float32
	for i := 0; i < L; i++ {
		if mask[i] == 0 {
			continue
		}
		n++
		row := x[i*H : (i+1)*H]
		for j := 0; j < H; j++ {
			out[j] += row[j]
		}
	}
	if n == 0 {
		return
	}
	for j := 0; j < H; j++ {
		out[j] /= n
	}
}

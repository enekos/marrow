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
	"runtime"
	"sync"
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
//
// Hot path: 8-column tile. For each column in the tile we keep two
// independent accumulators unrolled over K by 2, giving the CPU enough
// in-flight fp ops to saturate a wide FMA pipeline. 8 × 2 = 16 fp32
// accumulators fits comfortably in arm64 / amd64 register files.
func MatMulTransposeB(a []float32, b []float32, c []float32, M, K, N int) {
	for i := 0; i < M; i++ {
		aRow := a[i*K : (i+1)*K]
		cRow := c[i*N : (i+1)*N]
		j := 0
		for ; j+8 <= N; j += 8 {
			s0, s1, s2, s3, s4, s5, s6, s7 := dot8(
				aRow,
				b[(j+0)*K:(j+1)*K], b[(j+1)*K:(j+2)*K],
				b[(j+2)*K:(j+3)*K], b[(j+3)*K:(j+4)*K],
				b[(j+4)*K:(j+5)*K], b[(j+5)*K:(j+6)*K],
				b[(j+6)*K:(j+7)*K], b[(j+7)*K:(j+8)*K],
			)
			cRow[j], cRow[j+1], cRow[j+2], cRow[j+3] = s0, s1, s2, s3
			cRow[j+4], cRow[j+5], cRow[j+6], cRow[j+7] = s4, s5, s6, s7
		}
		for ; j < N; j++ {
			cRow[j] = dot(aRow, b[j*K:(j+1)*K])
		}
	}
}

// dot8 returns eight parallel dot products of a with b0..b7. Single
// accumulator per column plus a paired even/odd split over K: each column
// has two accumulators so one FMA can retire while the next issues.
// Sharing the a-stream across eight B rows multiplies arithmetic intensity
// and keeps the L1 cache line under pressure on the same aRow for the full
// tile, so the compiler emits a tight fused-multiply-add loop.
func dot8(a, b0, b1, b2, b3, b4, b5, b6, b7 []float32) (
	float32, float32, float32, float32, float32, float32, float32, float32,
) {
	n := len(a)
	var s0a, s0b, s1a, s1b, s2a, s2b, s3a, s3b float32
	var s4a, s4b, s5a, s5b, s6a, s6b, s7a, s7b float32
	i := 0
	for ; i+2 <= n; i += 2 {
		a0, a1 := a[i], a[i+1]
		s0a += a0 * b0[i]
		s0b += a1 * b0[i+1]
		s1a += a0 * b1[i]
		s1b += a1 * b1[i+1]
		s2a += a0 * b2[i]
		s2b += a1 * b2[i+1]
		s3a += a0 * b3[i]
		s3b += a1 * b3[i+1]
		s4a += a0 * b4[i]
		s4b += a1 * b4[i+1]
		s5a += a0 * b5[i]
		s5b += a1 * b5[i+1]
		s6a += a0 * b6[i]
		s6b += a1 * b6[i+1]
		s7a += a0 * b7[i]
		s7b += a1 * b7[i+1]
	}
	r0 := s0a + s0b
	r1 := s1a + s1b
	r2 := s2a + s2b
	r3 := s3a + s3b
	r4 := s4a + s4b
	r5 := s5a + s5b
	r6 := s6a + s6b
	r7 := s7a + s7b
	if i < n {
		ai := a[i]
		r0 += ai * b0[i]
		r1 += ai * b1[i]
		r2 += ai * b2[i]
		r3 += ai * b3[i]
		r4 += ai * b4[i]
		r5 += ai * b5[i]
		r6 += ai * b6[i]
		r7 += ai * b7[i]
	}
	return r0, r1, r2, r3, r4, r5, r6, r7
}

// dot returns the scalar dot product of a and b. Callers must ensure
// len(a) == len(b); the function reads min(len) elements.
func dot(a, b []float32) float32 {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	var s0, s1, s2, s3, s4, s5, s6, s7 float32
	i := 0
	for ; i+8 <= n; i += 8 {
		s0 += a[i] * b[i]
		s1 += a[i+1] * b[i+1]
		s2 += a[i+2] * b[i+2]
		s3 += a[i+3] * b[i+3]
		s4 += a[i+4] * b[i+4]
		s5 += a[i+5] * b[i+5]
		s6 += a[i+6] * b[i+6]
		s7 += a[i+7] * b[i+7]
	}
	sum := (s0 + s1) + (s2 + s3) + (s4 + s5) + (s6 + s7)
	for ; i < n; i++ {
		sum += a[i] * b[i]
	}
	return sum
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

// GELU applies the BERT GELU activation element-wise using the fast tanh
// approximation (same as PyTorch's approximate='tanh'). This is ~2× faster
// than math.Erf with negligible accuracy difference for embedding use.
//
//	y = 0.5 * x * (1 + tanh(sqrt(2/pi) * (x + 0.044715 * x^3)))
func GELU(x []float32) {
	const geluCoeff = 0.044715
	const sqrt2OverPi = 0.7978845608028654
	for i, v := range x {
		vv := float64(v)
		x[i] = float32(0.5 * vv * (1.0 + math.Tanh(sqrt2OverPi*(vv+geluCoeff*vv*vv*vv))))
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

// MatMulTransposeBParallel splits C's rows (the M dimension) across up to
// GOMAXPROCS workers. Only worthwhile when M is large enough that the
// goroutine dispatch cost is amortized — callers should gate on size.
// Below ~64 rows the serial path is usually faster.
func MatMulTransposeBParallel(a []float32, b []float32, c []float32, M, K, N int) {
	workers := runtime.GOMAXPROCS(0)
	if workers > M {
		workers = M
	}
	if workers <= 1 {
		MatMulTransposeB(a, b, c, M, K, N)
		return
	}
	// Contiguous row ranges: worker w gets rows [w*chunk, (w+1)*chunk).
	chunk := (M + workers - 1) / workers
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		start := w * chunk
		if start >= M {
			break
		}
		end := start + chunk
		if end > M {
			end = M
		}
		wg.Add(1)
		go func(start, end int) {
			defer wg.Done()
			MatMulTransposeB(
				a[start*K:end*K],
				b,
				c[start*N:end*N],
				end-start, K, N,
			)
		}(start, end)
	}
	wg.Wait()
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

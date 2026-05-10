// Package mat implements the numeric primitives for the transformer
// forward pass: matmul, layernorm, softmax, gelu, bias add, and residual.
//
// All operations are fp32, row-major. Shapes are tracked by callers; this
// package only sees flat slices plus leading dimensions.
//
// MatMul uses a cache-blocked three-loop variant. The fp32 dot product —
// the inner kernel of MatMulTransposeB and the BERT forward pass — is
// dispatched through `dot()`, which is implemented in arch-specific files:
// arm64 NEON FMLA (dot_arm64.s) and amd64 AVX2 FMA (dot_amd64.s), with a
// scalar Go fallback in dot.go for other architectures.
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

// dot8 returns eight parallel dot products of a with b0..b7. The hot path
// inside MatMulTransposeB. Streams aRow once into L1 and runs eight
// independent dot kernels against it — on arm64/amd64 each kernel is a
// SIMD FMA loop (see dot_{arm64,amd64}.s), so the eight calls share the
// same warm a-stream and saturate the FMA pipeline.
func dot8(a, b0, b1, b2, b3, b4, b5, b6, b7 []float32) (
	float32, float32, float32, float32, float32, float32, float32, float32,
) {
	return dot(a, b0), dot(a, b1), dot(a, b2), dot(a, b3),
		dot(a, b4), dot(a, b5), dot(a, b6), dot(a, b7)
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

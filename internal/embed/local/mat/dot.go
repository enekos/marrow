package mat

// dotScalar is the portable Go fp32 dot product. It is used directly on
// architectures without a SIMD kernel (see dot_generic.go), and as a
// fallback on amd64 when AVX2 is not detected at startup.
//
// Eight scalar accumulators expose enough independent fp ops that the
// Go compiler emits a respectable fused-multiply-add loop on most
// targets — we still keep the SIMD asm path because that loop tops out
// at ~20% of NEON/AVX2 peak FLOPS.
func dotScalar(a, b []float32) float32 {
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

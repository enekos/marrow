//go:build !arm64 && !amd64

package mat

// dot delegates to the scalar implementation on architectures without a
// SIMD kernel.
func dot(a, b []float32) float32 { return dotScalar(a, b) }

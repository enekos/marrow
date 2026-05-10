//go:build arm64

package mat

// dot dispatches to the hand-rolled NEON kernel on arm64.
func dot(a, b []float32) float32 { return dotNEON(a, b) }

// dotNEON is the NEON FMLA implementation in dot_arm64.s. It computes the
// dot product of min(len(a), len(b)) elements.
//
//go:noescape
func dotNEON(a, b []float32) float32

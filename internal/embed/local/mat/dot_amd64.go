//go:build amd64

package mat

import "golang.org/x/sys/cpu"

// hasAVX2 latches the CPU's AVX2 + FMA support at process start. Both are
// required for the dotAVX2 kernel: AVX2 supplies the YMM registers and
// gather/permute used in the reduction, FMA supplies VFMADD231PS.
//
// Every x86-64 part since Haswell (2013) has both, but a runtime check is
// still cheap insurance against unusual VPS images and emulators.
var hasAVX2 = cpu.X86.HasAVX2 && cpu.X86.HasFMA

func dot(a, b []float32) float32 {
	if hasAVX2 {
		return dotAVX2(a, b)
	}
	return dotScalar(a, b)
}

// dotAVX2 is the AVX2+FMA implementation in dot_amd64.s. It computes the
// dot product of min(len(a), len(b)) elements.
//
//go:noescape
func dotAVX2(a, b []float32) float32

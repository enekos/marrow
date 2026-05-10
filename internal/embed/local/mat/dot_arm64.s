#include "textflag.h"

// func dotNEON(a, b []float32) float32
//
// Computes the fp32 dot product of two slices using arm64 NEON FMLA. Uses
// four independent 4-wide accumulators (V16..V19) to give the FMA
// pipeline enough in-flight operations to issue every cycle, then folds
// them and performs a horizontal reduction to produce the scalar result.
//
// The hot loop processes 16 floats per iteration. A 4-float fallback
// handles the remainder, and a final scalar tail handles 0-3 elements.
// For MiniLM-L6-v2 the K dimensions (384, 1536) are multiples of 16 so
// the hot path dominates.
//
// Note: Go's plan9 arm64 assembler does NOT expose vector fp ops other
// than VFMLA / VFMLS. VADD/VADDV emit *integer* lane add — using them on
// fp32 bit patterns produces nonsense. We work around that by:
//   - Using VFMLA with a precomputed all-ones vector (V20) as the
//     multiplier to fold V16..V19 into V16 (an fp add expressed as FMA).
//   - Reducing the four lanes of V16 to a scalar by extracting each lane
//     to a general-purpose register, then to an F register via FMOVS,
//     then chaining FADDS.
TEXT ·dotNEON(SB), NOSPLIT, $0-52
	MOVD	a_base+0(FP), R0
	MOVD	a_len+8(FP), R2
	MOVD	b_base+24(FP), R1
	MOVD	b_len+32(FP), R3

	// R2 = min(len(a), len(b)). CMP R3, R2 sets flags from R2-R3.
	CMP	R3, R2
	CSEL	LT, R2, R3, R2

	// Zero the four accumulators V16..V19. NEON has no zero immediate;
	// the canonical pattern is XOR with self.
	VEOR	V16.B16, V16.B16, V16.B16
	VEOR	V17.B16, V17.B16, V17.B16
	VEOR	V18.B16, V18.B16, V18.B16
	VEOR	V19.B16, V19.B16, V19.B16

	// V20 = [1.0, 1.0, 1.0, 1.0] — used as the unit multiplier when we
	// fold the four accumulators with VFMLA. 0x3f800000 is fp32 1.0.
	MOVD	$0x3f800000, R4
	VMOV	R4, V20.S[0]
	VMOV	R4, V20.S[1]
	VMOV	R4, V20.S[2]
	VMOV	R4, V20.S[3]

loop16:
	CMP	$16, R2
	BLT	loop4_check

	// Load 16 fp32 from a and 16 from b, post-incrementing both
	// pointers by 64 bytes. The four V0..V3 / V4..V7 register pairs let
	// the issue width keep up with the four FMAs below.
	VLD1.P	64(R0), [V0.S4, V1.S4, V2.S4, V3.S4]
	VLD1.P	64(R1), [V4.S4, V5.S4, V6.S4, V7.S4]

	// VFMLA Vm, Vn, Vd: Vd += Vn * Vm (Go reverses operand order from
	// the ARM-standard FMLA Vd, Vn, Vm).
	VFMLA	V0.S4, V4.S4, V16.S4
	VFMLA	V1.S4, V5.S4, V17.S4
	VFMLA	V2.S4, V6.S4, V18.S4
	VFMLA	V3.S4, V7.S4, V19.S4

	SUB	$16, R2, R2
	B	loop16

loop4_check:
	CMP	$4, R2
	BLT	reduce
loop4:
	VLD1.P	16(R0), [V0.S4]
	VLD1.P	16(R1), [V4.S4]
	VFMLA	V0.S4, V4.S4, V16.S4
	SUB	$4, R2, R2
	CMP	$4, R2
	BGE	loop4

reduce:
	// Fold V17, V19 into V16, V18 — using VFMLA against the unit vector
	// V20 because the assembler has no fp vector add mnemonic.
	VFMLA	V20.S4, V17.S4, V16.S4
	VFMLA	V20.S4, V19.S4, V18.S4
	VFMLA	V20.S4, V18.S4, V16.S4

	// Horizontal reduce V16 (4 fp32 lanes) to a scalar in F0.
	VMOV	V16.S[0], R4
	FMOVS	R4, F0
	VMOV	V16.S[1], R4
	FMOVS	R4, F1
	FADDS	F1, F0, F0
	VMOV	V16.S[2], R4
	FMOVS	R4, F1
	FADDS	F1, F0, F0
	VMOV	V16.S[3], R4
	FMOVS	R4, F1
	FADDS	F1, F0, F0

	// Scalar tail (0..3 leftover elements) accumulated into F0.
	CBZ	R2, done
tail:
	FMOVS	(R0), F1
	FMOVS	(R1), F2
	// Fd = Fn*Fm + Fa: F0 = F1*F2 + F0.
	FMADDS	F2, F0, F1, F0
	ADD	$4, R0, R0
	ADD	$4, R1, R1
	SUB	$1, R2, R2
	CBNZ	R2, tail

done:
	FMOVS	F0, ret+48(FP)
	RET

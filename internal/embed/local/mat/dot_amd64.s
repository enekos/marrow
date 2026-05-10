#include "textflag.h"

// func dotAVX2(a, b []float32) float32
//
// Computes the fp32 dot product using AVX2 + FMA. Uses four independent
// 256-bit (8 fp32) accumulators to give the FMA pipeline enough in-flight
// operations to issue at peak throughput. The hot loop processes 32
// floats per iteration (4 YMM registers × 8 lanes); an 8-float fallback
// loop and a final scalar tail handle the remainder.
//
// For MiniLM-L6-v2 the K dimensions (384, 1536) are multiples of 32 so
// the hot path always dominates. The AVX2 / FMA feature gate lives in
// dot_amd64.go and falls back to dotScalar on older parts.
TEXT ·dotAVX2(SB), NOSPLIT, $0-52
	MOVQ	a_base+0(FP), AX
	MOVQ	a_len+8(FP), CX
	MOVQ	b_base+24(FP), BX
	MOVQ	b_len+32(FP), DX

	// CX = min(len(a), len(b)). CMPQ R2, R3 sets flags from R2-R3, so
	// GT means len(a) > len(b) and we should swap.
	CMPQ	CX, DX
	CMOVQGT	DX, CX

	// Zero the four YMM accumulators.
	VXORPS	Y4, Y4, Y4
	VXORPS	Y5, Y5, Y5
	VXORPS	Y6, Y6, Y6
	VXORPS	Y7, Y7, Y7

loop32:
	CMPQ	CX, $32
	JL	loop8_check

	VMOVUPS	(AX), Y0
	VMOVUPS	32(AX), Y1
	VMOVUPS	64(AX), Y2
	VMOVUPS	96(AX), Y3
	VMOVUPS	(BX), Y8
	VMOVUPS	32(BX), Y9
	VMOVUPS	64(BX), Y10
	VMOVUPS	96(BX), Y11

	// Y_acc += Y_a * Y_b. Go syntax: VFMADD231PS arg1, arg2, arg3 →
	// arg3 = arg2 * arg1 + arg3 (Intel order reversed).
	VFMADD231PS	Y8, Y0, Y4
	VFMADD231PS	Y9, Y1, Y5
	VFMADD231PS	Y10, Y2, Y6
	VFMADD231PS	Y11, Y3, Y7

	ADDQ	$128, AX
	ADDQ	$128, BX
	SUBQ	$32, CX
	JMP	loop32

loop8_check:
	CMPQ	CX, $8
	JL	reduce
loop8:
	VMOVUPS	(AX), Y0
	VMOVUPS	(BX), Y8
	VFMADD231PS	Y8, Y0, Y4
	ADDQ	$32, AX
	ADDQ	$32, BX
	SUBQ	$8, CX
	CMPQ	CX, $8
	JGE	loop8

reduce:
	// Combine the four YMM accumulators into Y4.
	VADDPS	Y5, Y4, Y4
	VADDPS	Y7, Y6, Y6
	VADDPS	Y6, Y4, Y4

	// Reduce Y4 (8 lanes) to X4 (4 lanes) by adding upper 128 bits to
	// lower 128 bits. VEXTRACTF128 with imm=1 picks the upper half.
	VEXTRACTF128	$1, Y4, X1
	VADDPS	X1, X4, X4

	// X4 has 4 fp32 lanes. Two horizontal adds collapse to lane 0.
	VHADDPS	X4, X4, X4
	VHADDPS	X4, X4, X4

	// Done with YMM. Zero upper halves to avoid AVX/SSE transition
	// penalties on legacy parts and to play nice with the Go runtime.
	VZEROUPPER

	// Scalar tail (0..7 leftover elements) accumulated into X4.
	TESTQ	CX, CX
	JZ	done
tail:
	MOVSS	(AX), X0
	MULSS	(BX), X0
	ADDSS	X0, X4
	ADDQ	$4, AX
	ADDQ	$4, BX
	SUBQ	$1, CX
	JNZ	tail

done:
	MOVSS	X4, ret+48(FP)
	RET

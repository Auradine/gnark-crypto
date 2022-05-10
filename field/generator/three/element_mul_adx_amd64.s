// +build amd64_adx

// Copyright 2020 ConsenSys Software Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

#include "textflag.h"
#include "funcdata.h"

// modulus q
DATA q<>+0(SB)/8, $0x45723f1f7bb6a94b
DATA q<>+8(SB)/8, $0x9882583f2efca1f6
DATA q<>+16(SB)/8, $0x0000000000000003
GLOBL q<>(SB), (RODATA+NOPTR), $24

// qInv0 q'[0]
DATA qInv0<>(SB)/8, $0xe9ad66d83377679d
GLOBL qInv0<>(SB), (RODATA+NOPTR), $8

#define REDUCE(ra0, ra1, ra2, rb0, rb1, rb2) \
	MOVQ    ra0, rb0;        \
	SUBQ    q<>(SB), ra0;    \
	MOVQ    ra1, rb1;        \
	SBBQ    q<>+8(SB), ra1;  \
	MOVQ    ra2, rb2;        \
	SBBQ    q<>+16(SB), ra2; \
	CMOVQCS rb0, ra0;        \
	CMOVQCS rb1, ra1;        \
	CMOVQCS rb2, ra2;        \

// mul(res, x, y *Element)
TEXT ·mul(SB), NOSPLIT, $0-24

	// the algorithm is described here
	// https://hackmd.io/@gnark/modular_multiplication
	// however, to benefit from the ADCX and ADOX carry chains
	// we split the inner loops in 2:
	// for i=0 to N-1
	// 		for j=0 to N-1
	// 		    (A,t[j])  := t[j] + x[j]*y[i] + A
	// 		m := t[0]*q'[0] mod W
	// 		C,_ := t[0] + m*q[0]
	// 		for j=1 to N-1
	// 		    (C,t[j-1]) := t[j] + m*q[j] + C
	// 		t[N-1] = C + A

	MOVQ x+8(FP), BX

	// x[0] -> SI
	// x[1] -> DI
	// x[2] -> R8
	MOVQ 0(BX), SI
	MOVQ 8(BX), DI
	MOVQ 16(BX), R8
	MOVQ y+16(FP), R9

	// A -> BP
	// t[0] -> R14
	// t[1] -> R13
	// t[2] -> CX
	// clear the flags
	XORQ AX, AX
	MOVQ 0(R9), DX

	// (A,t[0])  := x[0]*y[0] + A
	MULXQ SI, R14, R13

	// (A,t[1])  := x[1]*y[0] + A
	MULXQ DI, AX, CX
	ADOXQ AX, R13

	// (A,t[2])  := x[2]*y[0] + A
	MULXQ R8, AX, BP
	ADOXQ AX, CX

	// A += carries from ADCXQ and ADOXQ
	MOVQ  $0, AX
	ADOXQ AX, BP

	// m := t[0]*q'[0] mod W
	MOVQ  qInv0<>(SB), DX
	IMULQ R14, DX

	// clear the flags
	XORQ AX, AX

	// C,_ := t[0] + m*q[0]
	MULXQ q<>+0(SB), AX, R10
	ADCXQ R14, AX
	MOVQ  R10, R14

	// (C,t[0]) := t[1] + m*q[1] + C
	ADCXQ R13, R14
	MULXQ q<>+8(SB), AX, R13
	ADOXQ AX, R14

	// (C,t[1]) := t[2] + m*q[2] + C
	ADCXQ CX, R13
	MULXQ q<>+16(SB), AX, CX
	ADOXQ AX, R13

	// t[2] = C + A
	MOVQ  $0, AX
	ADCXQ AX, CX
	ADOXQ BP, CX

	// clear the flags
	XORQ AX, AX
	MOVQ 8(R9), DX

	// (A,t[0])  := t[0] + x[0]*y[1] + A
	MULXQ SI, AX, BP
	ADOXQ AX, R14

	// (A,t[1])  := t[1] + x[1]*y[1] + A
	ADCXQ BP, R13
	MULXQ DI, AX, BP
	ADOXQ AX, R13

	// (A,t[2])  := t[2] + x[2]*y[1] + A
	ADCXQ BP, CX
	MULXQ R8, AX, BP
	ADOXQ AX, CX

	// A += carries from ADCXQ and ADOXQ
	MOVQ  $0, AX
	ADCXQ AX, BP
	ADOXQ AX, BP

	// m := t[0]*q'[0] mod W
	MOVQ  qInv0<>(SB), DX
	IMULQ R14, DX

	// clear the flags
	XORQ AX, AX

	// C,_ := t[0] + m*q[0]
	MULXQ q<>+0(SB), AX, R10
	ADCXQ R14, AX
	MOVQ  R10, R14

	// (C,t[0]) := t[1] + m*q[1] + C
	ADCXQ R13, R14
	MULXQ q<>+8(SB), AX, R13
	ADOXQ AX, R14

	// (C,t[1]) := t[2] + m*q[2] + C
	ADCXQ CX, R13
	MULXQ q<>+16(SB), AX, CX
	ADOXQ AX, R13

	// t[2] = C + A
	MOVQ  $0, AX
	ADCXQ AX, CX
	ADOXQ BP, CX

	// clear the flags
	XORQ AX, AX
	MOVQ 16(R9), DX

	// (A,t[0])  := t[0] + x[0]*y[2] + A
	MULXQ SI, AX, BP
	ADOXQ AX, R14

	// (A,t[1])  := t[1] + x[1]*y[2] + A
	ADCXQ BP, R13
	MULXQ DI, AX, BP
	ADOXQ AX, R13

	// (A,t[2])  := t[2] + x[2]*y[2] + A
	ADCXQ BP, CX
	MULXQ R8, AX, BP
	ADOXQ AX, CX

	// A += carries from ADCXQ and ADOXQ
	MOVQ  $0, AX
	ADCXQ AX, BP
	ADOXQ AX, BP

	// m := t[0]*q'[0] mod W
	MOVQ  qInv0<>(SB), DX
	IMULQ R14, DX

	// clear the flags
	XORQ AX, AX

	// C,_ := t[0] + m*q[0]
	MULXQ q<>+0(SB), AX, R10
	ADCXQ R14, AX
	MOVQ  R10, R14

	// (C,t[0]) := t[1] + m*q[1] + C
	ADCXQ R13, R14
	MULXQ q<>+8(SB), AX, R13
	ADOXQ AX, R14

	// (C,t[1]) := t[2] + m*q[2] + C
	ADCXQ CX, R13
	MULXQ q<>+16(SB), AX, CX
	ADOXQ AX, R13

	// t[2] = C + A
	MOVQ  $0, AX
	ADCXQ AX, CX
	ADOXQ BP, CX

	// reduce element(R14,R13,CX) using temp registers (R11,R12,BX)
	REDUCE(R14,R13,CX,R11,R12,BX)

	MOVQ res+0(FP), AX
	MOVQ R14, 0(AX)
	MOVQ R13, 8(AX)
	MOVQ CX, 16(AX)
	RET

TEXT ·fromMont(SB), NOSPLIT, $0-8

	// the algorithm is described here
	// https://hackmd.io/@gnark/modular_multiplication
	// when y = 1 we have:
	// for i=0 to N-1
	// 		t[i] = x[i]
	// for i=0 to N-1
	// 		m := t[0]*q'[0] mod W
	// 		C,_ := t[0] + m*q[0]
	// 		for j=1 to N-1
	// 		    (C,t[j-1]) := t[j] + m*q[j] + C
	// 		t[N-1] = C
	MOVQ res+0(FP), DX
	MOVQ 0(DX), R14
	MOVQ 8(DX), R13
	MOVQ 16(DX), CX
	XORQ DX, DX

	// m := t[0]*q'[0] mod W
	MOVQ  qInv0<>(SB), DX
	IMULQ R14, DX
	XORQ  AX, AX

	// C,_ := t[0] + m*q[0]
	MULXQ q<>+0(SB), AX, BP
	ADCXQ R14, AX
	MOVQ  BP, R14

	// (C,t[0]) := t[1] + m*q[1] + C
	ADCXQ R13, R14
	MULXQ q<>+8(SB), AX, R13
	ADOXQ AX, R14

	// (C,t[1]) := t[2] + m*q[2] + C
	ADCXQ CX, R13
	MULXQ q<>+16(SB), AX, CX
	ADOXQ AX, R13
	MOVQ  $0, AX
	ADCXQ AX, CX
	ADOXQ AX, CX
	XORQ  DX, DX

	// m := t[0]*q'[0] mod W
	MOVQ  qInv0<>(SB), DX
	IMULQ R14, DX
	XORQ  AX, AX

	// C,_ := t[0] + m*q[0]
	MULXQ q<>+0(SB), AX, BP
	ADCXQ R14, AX
	MOVQ  BP, R14

	// (C,t[0]) := t[1] + m*q[1] + C
	ADCXQ R13, R14
	MULXQ q<>+8(SB), AX, R13
	ADOXQ AX, R14

	// (C,t[1]) := t[2] + m*q[2] + C
	ADCXQ CX, R13
	MULXQ q<>+16(SB), AX, CX
	ADOXQ AX, R13
	MOVQ  $0, AX
	ADCXQ AX, CX
	ADOXQ AX, CX
	XORQ  DX, DX

	// m := t[0]*q'[0] mod W
	MOVQ  qInv0<>(SB), DX
	IMULQ R14, DX
	XORQ  AX, AX

	// C,_ := t[0] + m*q[0]
	MULXQ q<>+0(SB), AX, BP
	ADCXQ R14, AX
	MOVQ  BP, R14

	// (C,t[0]) := t[1] + m*q[1] + C
	ADCXQ R13, R14
	MULXQ q<>+8(SB), AX, R13
	ADOXQ AX, R14

	// (C,t[1]) := t[2] + m*q[2] + C
	ADCXQ CX, R13
	MULXQ q<>+16(SB), AX, CX
	ADOXQ AX, R13
	MOVQ  $0, AX
	ADCXQ AX, CX
	ADOXQ AX, CX

	// reduce element(R14,R13,CX) using temp registers (BX,SI,DI)
	REDUCE(R14,R13,CX,BX,SI,DI)

	MOVQ res+0(FP), AX
	MOVQ R14, 0(AX)
	MOVQ R13, 8(AX)
	MOVQ CX, 16(AX)
	RET

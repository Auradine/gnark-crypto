// Copyright 2020 ConsenSys AG
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

package bw761

// FinalExponentiation computes the final expo x**(p**6-1)(p**2+1)(p**4 - p**2 +1)/r
func (curve *Curve) FinalExponentiation(z *PairingResult, _z ...*PairingResult) PairingResult {
	var result PairingResult
	result.Set(z)

	// if additional parameters are provided, multiply them into z
	for _, e := range _z {
		result.Mul(&result, e)
	}

	result.FinalExponentiation(&result)

	return result
}

// FinalExponentiation sets z to the final expo x**((p**6 - 1)/r), returns z
func (z *PairingResult) FinalExponentiation(x *PairingResult) *PairingResult {

	var buf PairingResult
	var result PairingResult
	result.Set(x)

	// easy part exponent: (p**3 - 1)*(p+1)
	buf.FrobeniusCube(&result)
	result.Inverse(&result)
	buf.Mul(&buf, &result)
	result.Frobenius(&buf).
		MulAssign(&buf)

	// hard part exponent: a multiple of (p**2 - p + 1)/r
	// the multiple is 3*(t**3 - t**2 + 1)
	// Appendix B of https://eprint.iacr.org/2020/351.pdf
	// sage code: https://gitlab.inria.fr/zk-curves/bw6-761/-/blob/master/sage/pairing.py#L922
	var f [8]PairingResult
	var fp [10]PairingResult

	f[0].Set(&result)
	for i := 1; i < len(f); i++ {
		f[i].Expt(&f[i-1])
	}
	for i := range f {
		fp[i].Frobenius(&f[i])
	}
	fp[8].Expt(&fp[7])
	fp[9].Expt(&fp[8])

	result.FrobeniusCube(&fp[5]).
		MulAssign(&fp[3]).
		MulAssign(&fp[6]).
		CyclotomicSquare(&result)

	var f4fp2 PairingResult
	f4fp2.Mul(&f[4], &fp[2])
	buf.Mul(&f[0], &f[1]).
		MulAssign(&f[3]).
		MulAssign(&f4fp2).
		MulAssign(&fp[8])
	buf.FrobeniusCube(&buf)
	result.MulAssign(&buf)

	result.MulAssign(&f[5]).
		MulAssign(&fp[0]).
		CyclotomicSquare(&result)

	buf.FrobeniusCube(&f[7])
	result.MulAssign(&buf)

	result.MulAssign(&fp[9]).
		CyclotomicSquare(&result)

	var f2fp4, f4fp2fp5 PairingResult
	f2fp4.Mul(&f[2], &fp[4])
	f4fp2fp5.Mul(&f4fp2, &fp[5])
	buf.Mul(&f2fp4, &f[3]).
		MulAssign(&fp[3])
	buf.FrobeniusCube(&buf)
	result.MulAssign(&buf)

	result.MulAssign(&f4fp2fp5).
		MulAssign(&f[6]).
		MulAssign(&fp[7]).
		CyclotomicSquare(&result)

	buf.Mul(&fp[0], &fp[9])
	buf.FrobeniusCube(&buf)
	result.MulAssign(&buf)
	result.MulAssign(&f[0]).
		MulAssign(&f[7]).
		MulAssign(&fp[1]).
		CyclotomicSquare(&result)

	var fp6fp8, f5fp7 PairingResult
	fp6fp8.Mul(&fp[6], &fp[8])
	f5fp7.Mul(&f[5], &fp[7])
	buf.FrobeniusCube(&fp6fp8)
	result.MulAssign(&buf)

	result.MulAssign(&f5fp7).
		MulAssign(&fp[2]).
		CyclotomicSquare(&result)

	var f3f6, f1f7 PairingResult
	f3f6.Mul(&f[3], &f[6])
	f1f7.Mul(&f[1], &f[7])

	buf.Mul(&f1f7, &f[2])
	buf.FrobeniusCube(&buf)
	result.MulAssign(&buf)

	result.MulAssign(&f3f6).
		MulAssign(&fp[9]).
		CyclotomicSquare(&result)

	buf.Mul(&f4fp2, &f5fp7).
		MulAssign(&fp6fp8)
	buf.FrobeniusCube(&buf)
	result.MulAssign(&buf)

	result.MulAssign(&f[0]).
		MulAssign(&fp[0]).
		MulAssign(&fp[3]).
		MulAssign(&fp[5]).
		CyclotomicSquare(&result)

	buf.FrobeniusCube(&f3f6)
	result.MulAssign(&buf)

	result.MulAssign(&fp[1]).
		CyclotomicSquare(&result)

	buf.Mul(&f2fp4, &f4fp2fp5).MulAssign(&fp[9])
	buf.FrobeniusCube(&buf)
	result.MulAssign(&buf)

	result.MulAssign(&f1f7).MulAssign(&f5fp7).MulAssign(&fp[0])

	z.Set(&result)
	return z
}

// MillerLoop Miller loop
// https://eprint.iacr.org/2020/351.pdf (Algorithm 5)
// sage: https://gitlab.inria.fr/zk-curves/bw6-761/-/blob/master/sage/pairing.py#L344
func (curve *Curve) MillerLoop(P G1Affine, Q G2Affine, result *PairingResult) *PairingResult {

	result.SetOne() // init result
	if P.IsInfinity() || Q.IsInfinity() {
		return result
	}

	// the line goes through QCur and QNext
	var QCur, QNext, QNextNeg G2Jac
	var QNeg G2Affine

	QNeg.Neg(&Q)        // store -Q for use in NAF loop
	QCur.FromAffine(&Q) // init QCur with Q

	var lEval lineEvalRes

	// Miller loop 1
	for i := len(curve.loopCounter1) - 2; i >= 0; i-- {

		QNext.Set(&QCur)
		QNext.DoubleAssign()
		QNextNeg.Neg(&QNext)

		result.Square(result)

		// evaluates line though Qcur,2Qcur at P
		lineEvalJac(QCur, QNextNeg, &P, &lEval)
		lEval.mulAssign(result)

		if curve.loopCounter1[i] == 1 {
			// evaluates line through 2Qcur, Q at P
			lineEvalAffine(QNext, Q, &P, &lEval)
			lEval.mulAssign(result)

			QNext.AddMixed(&Q)

		} else if curve.loopCounter1[i] == -1 {
			// evaluates line through 2Qcur, -Q at P
			lineEvalAffine(QNext, QNeg, &P, &lEval)
			lEval.mulAssign(result)

			QNext.AddMixed(&QNeg)
		}
		QCur.Set(&QNext)
	}

	var result1 PairingResult
	result1.Set(result) // store the result of Miller loop 1

	var result1Inv PairingResult
	result1Inv.Inverse(&result1) // store result1 inverse for NAF loop

	lineEvalAffine(QCur, Q, &P, &lEval)

	var result1LineEval PairingResult
	result1LineEval.Set(&result1)
	lEval.mulAssign(&result1LineEval) // store result1 * (line eval) for the end

	// Miller loop 2 uses Q1, Q1Neg instead of Q, QNeg
	var Q1, Q1Neg G2Affine
	Q1.FromJacobian(&QCur)
	Q1Neg.Neg(&Q1)

	// Miller loop 2
	for i := len(curve.loopCounter2) - 2; i >= 0; i-- {

		QNext.Set(&QCur)
		QNext.DoubleAssign()
		QNextNeg.Neg(&QNext)

		result.Square(result)

		// evaluates line though Qcur,2Qcur at P
		lineEvalJac(QCur, QNextNeg, &P, &lEval)
		lEval.mulAssign(result)

		if curve.loopCounter2[i] == 1 {
			// evaluates line through 2Qcur, Q at P
			lineEvalAffine(QNext, Q1, &P, &lEval)
			lEval.mulAssign(result)
			result.MulAssign(&result1) // extra multiple of result1

			QNext.AddMixed(&Q1)

		} else if curve.loopCounter2[i] == -1 {
			// evaluates line through 2Qcur, -Q at P
			lineEvalAffine(QNext, Q1Neg, &P, &lEval)
			lEval.mulAssign(result)
			result.MulAssign(&result1Inv) // extra multiple of result1Inv

			QNext.AddMixed(&Q1Neg)
		}
		QCur.Set(&QNext)
	}

	result.Frobenius(result)
	result.MulAssign(&result1LineEval)

	return result
}

// lineEval computes the evaluation of the line through Q, R (on the twist) at P
// Q, R are in jacobian coordinates
// The case in which Q=R=Infinity is not handled as this doesn't happen in the SNARK pairing
func lineEvalJac(Q, R G2Jac, P *G1Affine, result *lineEvalRes) {

	// converts _Q and _R to projective coords
	var _Q, _R G2Proj
	_Q.FromJacobian(&Q)
	_R.FromJacobian(&R)

	result.r1.Mul(&_Q.Y, &_R.Z)
	result.r0.Mul(&_Q.Z, &_R.X)
	result.r2.Mul(&_Q.X, &_R.Y)

	_Q.Z.Mul(&_Q.Z, &_R.Y)
	_Q.X.Mul(&_Q.X, &_R.Z)
	_Q.Y.Mul(&_Q.Y, &_R.X)

	result.r1.Sub(&result.r1, &_Q.Z)
	result.r0.Sub(&result.r0, &_Q.X)
	result.r2.Sub(&result.r2, &_Q.Y)

	result.r1.Mul(&result.r1, &P.X)
	result.r0.Mul(&result.r0, &P.Y)
}

// Same as above but R is in affine coords
func lineEvalAffine(Q G2Jac, R G2Affine, P *G1Affine, result *lineEvalRes) {

	// converts Q and R to projective coords
	var _Q G2Proj
	_Q.FromJacobian(&Q)

	result.r1.Set(&_Q.Y)
	result.r0.Mul(&_Q.Z, &R.X)
	result.r2.Mul(&_Q.X, &R.Y)

	_Q.Z.Mul(&_Q.Z, &R.Y)
	_Q.Y.Mul(&_Q.Y, &R.X)

	result.r1.Sub(&result.r1, &_Q.Z)
	result.r0.Sub(&result.r0, &_Q.X)
	result.r2.Sub(&result.r2, &_Q.Y)

	// multiply P.Z by coeffs[2] in case P is infinity
	result.r1.Mul(&result.r1, &P.X)
	result.r0.Mul(&result.r0, &P.Y)
}

type lineEvalRes struct {
	r0 G2CoordType // c0.b1
	r1 G2CoordType // c1.b1
	r2 G2CoordType // c1.b2
}

func (l *lineEvalRes) mulAssign(z *PairingResult) *PairingResult {

	var a, b, c PairingResult
	a.MulByVMinusThree(z, &l.r1)
	b.MulByVminusTwo(z, &l.r0)
	c.MulByVminusFive(z, &l.r2)
	z.Add(&a, &b).Add(z, &c)

	return z
}

const tAbsVal uint64 = 9586122913090633729

// Expt set z to x^t in PairingResult and return z
func (z *PairingResult) Expt(x *PairingResult) *PairingResult {

	// tAbsVal in binary: 1000010100001000110000000000000000000000000000000000000000000001
	// drop the low 46 bits (all 0 except the least significant bit): 100001010000100011 = 136227
	// Shortest addition chains can be found at https://wwwhomes.uni-bielefeld.de/achim/addition_chain.html

	var result, x33 PairingResult

	// a shortest addition chain for 136227
	result.Set(x)             // 0                1
	result.Square(&result)    // 1( 0)            2
	result.Square(&result)    // 2( 1)            4
	result.Square(&result)    // 3( 2)            8
	result.Square(&result)    // 4( 3)           16
	result.Square(&result)    // 5( 4)           32
	result.Mul(&result, x)    // 6( 5, 0)        33
	x33.Set(&result)          // save x33 for step 14
	result.Square(&result)    // 7( 6)           66
	result.Square(&result)    // 8( 7)          132
	result.Square(&result)    // 9( 8)          264
	result.Square(&result)    // 10( 9)          528
	result.Square(&result)    // 11(10)         1056
	result.Square(&result)    // 12(11)         2112
	result.Square(&result)    // 13(12)         4224
	result.Mul(&result, &x33) // 14(13, 6)      4257
	result.Square(&result)    // 15(14)         8514
	result.Square(&result)    // 16(15)        17028
	result.Square(&result)    // 17(16)        34056
	result.Square(&result)    // 18(17)        68112
	result.Mul(&result, x)    // 19(18, 0)     68113
	result.Square(&result)    // 20(19)       136226
	result.Mul(&result, x)    // 21(20, 0)    136227

	// the remaining 46 bits
	for i := 0; i < 46; i++ {
		result.Square(&result)
	}
	result.Mul(&result, x)

	z.Set(&result)
	return z
}

// MulByVMinusThree set z to x*(y*v**-3) and return z (Fp6(v) where v**3=u, v**6=-4, so v**-3 = u**-1 = (-4)**-1*u)
func (z *PairingResult) MulByVMinusThree(x *PairingResult, y *G2CoordType) *PairingResult {

	var fourinv G2CoordType // (-4)**-1
	fourinv.SetString("5168587788236799404547592261706743156859751684402112582135342620157217566682618802065762387467058765730648425815339960088371319340415685819512133774343976199213703824533881637779407723567697596963924775322476834632073684839301224")

	// tmp = y*(-4)**-1 * u
	var tmp E2
	tmp.A0.SetZero()
	tmp.A1.Mul(y, &fourinv)

	z.MulByE2(x, &tmp)

	return z
}

// MulByVminusTwo set z to x*(y*v**-2) and return z (Fp6(v) where v**3=u, v**6=-4, so v**-2 = (-4)**-1*u*v)
func (z *PairingResult) MulByVminusTwo(x *PairingResult, y *G2CoordType) *PairingResult {

	var fourinv G2CoordType // (-4)**-1
	fourinv.SetString("5168587788236799404547592261706743156859751684402112582135342620157217566682618802065762387467058765730648425815339960088371319340415685819512133774343976199213703824533881637779407723567697596963924775322476834632073684839301224")

	// tmp = y*(-4)**-1 * u
	var tmp E2
	tmp.A0.SetZero()
	tmp.A1.Mul(y, &fourinv)

	var a E2
	a.MulByElement(&x.B2, y)
	z.B2.Mul(&x.B1, &tmp)
	z.B1.Mul(&x.B0, &tmp)
	z.B0.Set(&a)

	return z
}

// MulByVminusFive set z to x*(y*v**-5) and return z (Fp6(v) where v**3=u, v**6=-4, so v**-5 = (-4)**-1*v)
func (z *PairingResult) MulByVminusFive(x *PairingResult, y *G2CoordType) *PairingResult {

	var fourinv G2CoordType // (-4)**-1
	fourinv.SetString("5168587788236799404547592261706743156859751684402112582135342620157217566682618802065762387467058765730648425815339960088371319340415685819512133774343976199213703824533881637779407723567697596963924775322476834632073684839301224")

	// tmp = y*(-4)**-1 * u
	var tmp E2
	tmp.A0.SetZero()
	tmp.A1.Mul(y, &fourinv)

	var a E2
	a.Mul(&x.B2, &tmp)
	z.B2.MulByElement(&x.B1, &tmp.A1)
	z.B1.MulByElement(&x.B0, &tmp.A1)
	z.B0.Set(&a)

	return z
}

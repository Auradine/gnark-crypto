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

// Code generated by gurvy/internal/generators DO NOT EDIT

package bls377

// E12 is a degree-two finite field extension of fp6:
// C0 + C1w where w^3-v is irrep in fp6

// fp2, fp12 are both quadratic field extensions
// template code is duplicated in fp2, fp12
// TODO make an abstract quadratic extension template

type E12 struct {
	C0, C1 E6
}

// Equal returns true if z equals x, fasle otherwise
// TODO can this be deleted?  Should be able to use == operator instead
func (z *E12) Equal(x *E12) bool {
	return z.C0.Equal(&x.C0) && z.C1.Equal(&x.C1)
}

// String puts E12 in string form
func (z *E12) String() string {
	return (z.C0.String() + "+(" + z.C1.String() + ")*w")
}

// SetString sets a E12 from string
func (z *E12) SetString(s0, s1, s2, s3, s4, s5, s6, s7, s8, s9, s10, s11 string) *E12 {
	z.C0.SetString(s0, s1, s2, s3, s4, s5)
	z.C1.SetString(s6, s7, s8, s9, s10, s11)
	return z
}

// Set copies x into z and returns z
func (z *E12) Set(x *E12) *E12 {
	z.C0 = x.C0
	z.C1 = x.C1
	return z
}

// SetOne sets z to 1 in Montgomery form and returns z
func (z *E12) SetOne() *E12 {
	z.C0.B0.A0.SetOne()
	z.C0.B0.A1.SetZero()
	z.C0.B1.A0.SetZero()
	z.C0.B1.A1.SetZero()
	z.C0.B2.A0.SetZero()
	z.C0.B2.A1.SetZero()
	z.C1.B0.A0.SetZero()
	z.C1.B0.A1.SetZero()
	z.C1.B1.A0.SetZero()
	z.C1.B1.A1.SetZero()
	z.C1.B2.A0.SetZero()
	z.C1.B2.A1.SetZero()
	return z
}

// ToMont converts to Mont form
// TODO can this be deleted?
func (z *E12) ToMont() *E12 {
	z.C0.ToMont()
	z.C1.ToMont()
	return z
}

// FromMont converts from Mont form
// TODO can this be deleted?
func (z *E12) FromMont() *E12 {
	z.C0.FromMont()
	z.C1.FromMont()
	return z
}

// Add set z=x+y in E12 and return z
func (z *E12) Add(x, y *E12) *E12 {
	z.C0.Add(&x.C0, &y.C0)
	z.C1.Add(&x.C1, &y.C1)
	return z
}

// Sub set z=x-y in E12 and return z
func (z *E12) Sub(x, y *E12) *E12 {
	z.C0.Sub(&x.C0, &y.C0)
	z.C1.Sub(&x.C1, &y.C1)
	return z
}

// Double sets z=2*x and returns z
func (z *E12) Double(x *E12) *E12 {
	z.C0.Double(&x.C0)
	z.C1.Double(&x.C1)
	return z
}

// SetRandom used only in tests
func (z *E12) SetRandom() *E12 {
	z.C0.B0.A0.SetRandom()
	z.C0.B0.A1.SetRandom()
	z.C0.B1.A0.SetRandom()
	z.C0.B1.A1.SetRandom()
	z.C0.B2.A0.SetRandom()
	z.C0.B2.A1.SetRandom()
	z.C1.B0.A0.SetRandom()
	z.C1.B0.A1.SetRandom()
	z.C1.B1.A0.SetRandom()
	z.C1.B1.A1.SetRandom()
	z.C1.B2.A0.SetRandom()
	z.C1.B2.A1.SetRandom()
	return z
}

// Mul set z=x*y in E12 and return z
func (z *E12) Mul(x, y *E12) *E12 {
	// Algorithm 20 from https://eprint.iacr.org/2010/354.pdf

	var t0, t1, xSum, ySum E6

	t0.Mul(&x.C0, &y.C0) // step 1
	t1.Mul(&x.C1, &y.C1) // step 2

	// finish processing input in case z==x or y
	xSum.Add(&x.C0, &x.C1)
	ySum.Add(&y.C0, &y.C1)

	// step 3
	{ // begin inline: set z.C0 to (&t1) * ((0,0),(1,0),(0,0))
		var result E6
		result.B1.Set(&(&t1).B0)
		result.B2.Set(&(&t1).B1)
		{ // begin inline: set result.B0 to (&(&t1).B2) * (0,1)
			buf := (&(&t1).B2).A0
			{ // begin inline: set &(result.B0).A0 to (&(&(&t1).B2).A1) * (5)
				buf := *(&(&(&t1).B2).A1)
				(&(result.B0).A0).Double(&buf).Double(&(result.B0).A0).AddAssign(&buf)
			} // end inline: set &(result.B0).A0 to (&(&(&t1).B2).A1) * (5)
			(result.B0).A1 = buf
		} // end inline: set result.B0 to (&(&t1).B2) * (0,1)
		z.C0.Set(&result)
	} // end inline: set z.C0 to (&t1) * ((0,0),(1,0),(0,0))
	z.C0.Add(&z.C0, &t0)

	// step 4
	z.C1.Mul(&xSum, &ySum).
		Sub(&z.C1, &t0).
		Sub(&z.C1, &t1)

	return z
}

// Square set z=x*x in E12 and return z
func (z *E12) Square(x *E12) *E12 {
	// TODO implement Algorithm 22 from https://eprint.iacr.org/2010/354.pdf
	// or the complex method from fp2
	// for now do it the dumb way
	var b0, b1 E6

	b0.Square(&x.C0)
	b1.Square(&x.C1)
	{ // begin inline: set b1 to (&b1) * ((0,0),(1,0),(0,0))
		var result E6
		result.B1.Set(&(&b1).B0)
		result.B2.Set(&(&b1).B1)
		{ // begin inline: set result.B0 to (&(&b1).B2) * (0,1)
			buf := (&(&b1).B2).A0
			{ // begin inline: set &(result.B0).A0 to (&(&(&b1).B2).A1) * (5)
				buf := *(&(&(&b1).B2).A1)
				(&(result.B0).A0).Double(&buf).Double(&(result.B0).A0).AddAssign(&buf)
			} // end inline: set &(result.B0).A0 to (&(&(&b1).B2).A1) * (5)
			(result.B0).A1 = buf
		} // end inline: set result.B0 to (&(&b1).B2) * (0,1)
		b1.Set(&result)
	} // end inline: set b1 to (&b1) * ((0,0),(1,0),(0,0))
	b1.Add(&b0, &b1)

	z.C1.Mul(&x.C0, &x.C1).Double(&z.C1)
	z.C0 = b1

	return z
}

// squares an element a+by interpreted as an Fp4 elmt, where y**2=(0,1)
func fp4Square(a, b, c, d *E2) {
	var tmp E2
	c.Square(a)
	tmp.Square(b).MulByNonResidue(&tmp)
	c.Add(c, &tmp)
	d.Mul(a, b).Double(d)
}

// CyclotomicSquare https://eprint.iacr.org/2009/565.pdf, 3.2
func (z *E12) CyclotomicSquare(x *E12) *E12 {

	var res, b, a E12
	var tmp E2

	// A
	fp4Square(&x.C0.B0, &x.C1.B1, &b.C0.B0, &b.C1.B1)
	a.C0.B0.Set(&x.C0.B0)
	a.C1.B1.Neg(&x.C1.B1)

	// B
	tmp.MulByNonResidueInv(&x.C1.B0)
	fp4Square(&x.C0.B2, &tmp, &b.C0.B1, &b.C1.B2)
	b.C0.B1.MulByNonResidue(&b.C0.B1)
	b.C1.B2.MulByNonResidue(&b.C1.B2)
	a.C0.B1.Set(&x.C0.B1)
	a.C1.B2.Neg(&x.C1.B2)

	// C
	fp4Square(&x.C0.B1, &x.C1.B2, &b.C0.B2, &b.C1.B0)
	b.C1.B0.MulByNonResidue(&b.C1.B0)
	a.C0.B2.Set(&x.C0.B2)
	a.C1.B0.Neg(&x.C1.B0)

	res.Set(&b)
	b.Sub(&b, &a).Double(&b)
	z.Add(&res, &b)

	return z
}

// Inverse set z to the inverse of x in E12 and return z
func (z *E12) Inverse(x *E12) *E12 {
	// Algorithm 23 from https://eprint.iacr.org/2010/354.pdf

	var t [2]E6

	t[0].Square(&x.C0) // step 1
	t[1].Square(&x.C1) // step 2
	{                  // step 3
		var buf E6
		{ // begin inline: set buf to (&t[1]) * ((0,0),(1,0),(0,0))
			var result E6
			result.B1.Set(&(&t[1]).B0)
			result.B2.Set(&(&t[1]).B1)
			{ // begin inline: set result.B0 to (&(&t[1]).B2) * (0,1)
				buf := (&(&t[1]).B2).A0
				{ // begin inline: set &(result.B0).A0 to (&(&(&t[1]).B2).A1) * (5)
					buf := *(&(&(&t[1]).B2).A1)
					(&(result.B0).A0).Double(&buf).Double(&(result.B0).A0).AddAssign(&buf)
				} // end inline: set &(result.B0).A0 to (&(&(&t[1]).B2).A1) * (5)
				(result.B0).A1 = buf
			} // end inline: set result.B0 to (&(&t[1]).B2) * (0,1)
			buf.Set(&result)
		} // end inline: set buf to (&t[1]) * ((0,0),(1,0),(0,0))
		t[0].Sub(&t[0], &buf)
	}
	t[1].Inverse(&t[0])               // step 4
	z.C0.Mul(&x.C0, &t[1])            // step 5
	z.C1.Mul(&x.C1, &t[1]).Neg(&z.C1) // step 6

	return z
}

// InverseUnitary inverse a unitary element
// TODO deprecate in favour of Conjugate
func (z *E12) InverseUnitary(x *E12) *E12 {
	return z.Conjugate(x)
}

// Conjugate set z to (x.C0, -x.C1) and return z
func (z *E12) Conjugate(x *E12) *E12 {
	z.Set(x)
	z.C1.Neg(&z.C1)
	return z
}

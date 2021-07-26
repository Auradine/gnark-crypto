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

// Code generated by consensys/gnark-crypto DO NOT EDIT

package fft

import (
	"fmt"
	"io"
	"math/big"
	"math/bits"
	"runtime"
	"sync"

	"github.com/consensys/gnark-crypto/ecc/bw6-633/fr"

	curve "github.com/consensys/gnark-crypto/ecc/bw6-633"

	"github.com/consensys/gnark-crypto/ecc"
)

// Domain with a power of 2 cardinality
// compute a field element of order 2x and store it in FinerGenerator
// all other values can be derived from x, GeneratorSqrt
type Domain struct {
	Cardinality             uint64
	Depth                   uint64
	PrecomputeReversedTable uint64 // uint64 so it is recognized by the decoder from gnark-crypto
	CardinalityInv          fr.Element
	Generator               fr.Element
	GeneratorInv            fr.Element
	FinerGenerator          fr.Element
	FinerGeneratorInv       fr.Element

	// the following slices are not serialized and are (re)computed through domain.preComputeTwiddles()

	// Twiddles factor for the FFT using Generator for each stage of the recursive FFT
	Twiddles [][]fr.Element

	// Twiddles factor for the FFT using GeneratorInv for each stage of the recursive FFT
	TwiddlesInv [][]fr.Element

	// we precompute these mostly to avoid the memory intensive bit reverse permutation in the groth16.Prover

	// CosetTable[i][j] = domain.Generator(i-th)Sqrt ^ j
	// CosetTable = fft.BitReverse(CosetTable)
	CosetTable         [][]fr.Element
	CosetTableReversed [][]fr.Element // optional, this is computed on demand at the creation of the domain

	// CosetTable[i][j] = domain.Generator(i-th)SqrtInv ^ j
	// CosetTableInv = fft.BitReverse(CosetTableInv)
	CosetTableInv         [][]fr.Element
	CosetTableInvReversed [][]fr.Element // optional, this is computed on demand at the creation of the domain
}

// NewDomain returns a subgroup with a power of 2 cardinality
// cardinality >= m
// If depth>0, the Domain will also store a primitive (2**depth)*m root
// of 1, with associated precomputed data. This allows to perform shifted
// FFT/FFTInv.
// If precomputeReversedCosetTable is set, the bit reversed cosetTable/cosetTableInv are precomputed.
//
// example:
// --------
//
// * NewDomain(m, 0, false) outputs a new domain to perform the fft on Z/mZ.
// * NewDomain(m, 2, false) outputs a new domain to perform fft on Z/mZ, plus a primitive
// 2**2*m=4m-th root of 1 and associated data to compute fft/fftinv on the cosets of
// (Z/4mZ)/(Z/mZ).
func NewDomain(m, depth uint64, precomputeReversedTable bool) *Domain {

	// generator of the largest 2-adic subgroup
	var rootOfUnity fr.Element

	rootOfUnity.SetString("4991787701895089137426454739366935169846548798279261157172811661565882460884369603588700158257")
	const maxOrderRoot uint64 = 20

	domain := &Domain{}
	x := ecc.NextPowerOfTwo(m)
	domain.Cardinality = uint64(x)
	domain.Depth = depth
	if precomputeReversedTable {
		domain.PrecomputeReversedTable = 1
	}

	// find generator for Z/2^(log(m))Z  and Z/2^(log(m)+cosets)Z
	logx := uint64(bits.TrailingZeros64(x))
	if logx > maxOrderRoot {
		panic(fmt.Sprintf("m (%d) is too big: the required root of unity does not exist", m))
	}
	logGen := logx + depth
	if logGen > maxOrderRoot {
		panic("log(m) + cosets is too big: the required root of unity does not exist")
	}

	expo := uint64(1 << (maxOrderRoot - logGen))
	bExpo := new(big.Int).SetUint64(expo)
	domain.FinerGenerator.Exp(rootOfUnity, bExpo)
	domain.FinerGeneratorInv.Inverse(&domain.FinerGenerator)

	// Generator = FinerGenerator^2 has order x
	expo = uint64(1 << (maxOrderRoot - logx))
	bExpo.SetUint64(expo)
	domain.Generator.Exp(rootOfUnity, bExpo) // order x
	domain.GeneratorInv.Inverse(&domain.Generator)
	domain.CardinalityInv.SetUint64(uint64(x)).Inverse(&domain.CardinalityInv)

	// twiddle factors
	domain.preComputeTwiddles()

	// store the bit reversed coset tables if needed
	if depth > 0 && precomputeReversedTable {
		domain.reverseCosetTables()
	}

	return domain
}

func (d *Domain) reverseCosetTables() {
	nbCosets := (1 << d.Depth) - 1
	d.CosetTableReversed = make([][]fr.Element, nbCosets)
	d.CosetTableInvReversed = make([][]fr.Element, nbCosets)
	for i := 0; i < nbCosets; i++ {
		d.CosetTableReversed[i] = make([]fr.Element, d.Cardinality)
		d.CosetTableInvReversed[i] = make([]fr.Element, d.Cardinality)
		copy(d.CosetTableReversed[i], d.CosetTable[i])
		copy(d.CosetTableInvReversed[i], d.CosetTableInv[i])
		BitReverse(d.CosetTableReversed[i])
		BitReverse(d.CosetTableInvReversed[i])
	}
}

func (d *Domain) preComputeTwiddles() {

	// nb fft stages
	nbStages := uint64(bits.TrailingZeros64(d.Cardinality))
	nbCosets := (1 << d.Depth) - 1

	d.Twiddles = make([][]fr.Element, nbStages)
	d.TwiddlesInv = make([][]fr.Element, nbStages)
	d.CosetTable = make([][]fr.Element, nbCosets)
	d.CosetTableInv = make([][]fr.Element, nbCosets)
	for i := 0; i < nbCosets; i++ {
		d.CosetTable[i] = make([]fr.Element, d.Cardinality)
		d.CosetTableInv[i] = make([]fr.Element, d.Cardinality)
	}

	var wg sync.WaitGroup

	// for each fft stage, we pre compute the twiddle factors
	twiddles := func(t [][]fr.Element, omega fr.Element) {
		for i := uint64(0); i < nbStages; i++ {
			t[i] = make([]fr.Element, 1+(1<<(nbStages-i-1)))
			var w fr.Element
			if i == 0 {
				w = omega
			} else {
				w = t[i-1][2]
			}
			t[i][0] = fr.One()
			t[i][1] = w
			for j := 2; j < len(t[i]); j++ {
				t[i][j].Mul(&t[i][j-1], &w)
			}
		}
		wg.Done()
	}

	expTable := func(sqrt fr.Element, t []fr.Element) {
		t[0] = fr.One()
		precomputeExpTable(sqrt, t)
		wg.Done()
	}

	if nbCosets > 0 {
		cosetGens := make([]fr.Element, nbCosets)
		cosetGensInv := make([]fr.Element, nbCosets)
		cosetGens[0].Set(&d.FinerGenerator)
		cosetGensInv[0].Set(&d.FinerGeneratorInv)
		for i := 1; i < nbCosets; i++ {
			cosetGens[i].Mul(&cosetGens[i-1], &d.FinerGenerator)
			cosetGensInv[i].Mul(&cosetGensInv[i-1], &d.FinerGeneratorInv)
		}
		wg.Add(2 + 2*nbCosets)
		go twiddles(d.Twiddles, d.Generator)
		go twiddles(d.TwiddlesInv, d.GeneratorInv)
		for i := 0; i < nbCosets-1; i++ {
			go expTable(cosetGens[i], d.CosetTable[i])
			go expTable(cosetGensInv[i], d.CosetTableInv[i])
		}
		go expTable(cosetGens[nbCosets-1], d.CosetTable[nbCosets-1])
		expTable(cosetGensInv[nbCosets-1], d.CosetTableInv[nbCosets-1])

		wg.Wait()

	} else {
		wg.Add(2)
		go twiddles(d.Twiddles, d.Generator)
		twiddles(d.TwiddlesInv, d.GeneratorInv)
		wg.Wait()
	}

}

func precomputeExpTable(w fr.Element, table []fr.Element) {
	n := len(table)

	// see if it makes sense to parallelize exp tables pre-computation
	interval := 0
	if runtime.NumCPU() >= 4 {
		interval = (n - 1) / (runtime.NumCPU() / 4)
	}

	// this ratio roughly correspond to the number of multiplication one can do in place of a Exp operation
	const ratioExpMul = 6000 / 17

	if interval < ratioExpMul {
		precomputeExpTableChunk(w, 1, table[1:])
		return
	}

	// we parallelize
	var wg sync.WaitGroup
	for i := 1; i < n; i += interval {
		start := i
		end := i + interval
		if end > n {
			end = n
		}
		wg.Add(1)
		go func() {
			precomputeExpTableChunk(w, uint64(start), table[start:end])
			wg.Done()
		}()
	}
	wg.Wait()
}

func precomputeExpTableChunk(w fr.Element, power uint64, table []fr.Element) {

	// this condition ensures that creating a domain of size 1 with cosets don't fail
	if len(table) > 0 {
		table[0].Exp(w, new(big.Int).SetUint64(power))
		for i := 1; i < len(table); i++ {
			table[i].Mul(&table[i-1], &w)
		}
	}
}

// WriteTo writes a binary representation of the domain (without the precomputed twiddle factors)
// to the provided writer
func (d *Domain) WriteTo(w io.Writer) (int64, error) {

	enc := curve.NewEncoder(w)

	toEncode := []interface{}{d.Cardinality, d.Depth, d.PrecomputeReversedTable, &d.CardinalityInv, &d.Generator, &d.GeneratorInv, &d.FinerGenerator, &d.FinerGeneratorInv}

	for _, v := range toEncode {
		if err := enc.Encode(v); err != nil {
			return enc.BytesWritten(), err
		}
	}

	return enc.BytesWritten(), nil
}

// ReadFrom attempts to decode a domain from Reader
func (d *Domain) ReadFrom(r io.Reader) (int64, error) {

	dec := curve.NewDecoder(r)

	toDecode := []interface{}{&d.Cardinality, &d.Depth, &d.PrecomputeReversedTable, &d.CardinalityInv, &d.Generator, &d.GeneratorInv, &d.FinerGenerator, &d.FinerGeneratorInv}

	for _, v := range toDecode {
		if err := dec.Decode(v); err != nil {
			return dec.BytesRead(), err
		}
	}

	d.preComputeTwiddles()

	// store the bit reversed coset tables if needed
	if d.Depth > 0 && d.PrecomputeReversedTable == 1 {
		d.reverseCosetTables()
	}

	return dec.BytesRead(), nil
}

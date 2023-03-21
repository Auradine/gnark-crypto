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

	"github.com/consensys/gnark-crypto/ecc/bn254/fr"

	curve "github.com/consensys/gnark-crypto/ecc/bn254"

	"github.com/consensys/gnark-crypto/ecc"
)

// Domain with a power of 2 cardinality
// compute a field element of order 2x and store it in FinerGenerator
// all other values can be derived from x, GeneratorSqrt
type Domain struct {
	Cardinality            uint64
	CardinalityInv         fr.Element
	Generator              fr.Element
	GeneratorInv           fr.Element
	FrMultiplicativeGen    fr.Element // generator of Fr*
	FrMultiplicativeGenInv fr.Element

	// the following slices are not serialized and are (re)computed through domain.preComputeTwiddles()

	// Twiddles factor for the FFT using Generator for each stage of the recursive FFT
	Twiddles [][]fr.Element

	// Twiddles factor for the FFT using GeneratorInv for each stage of the recursive FFT
	TwiddlesInv [][]fr.Element

	// we precompute these mostly to avoid the memory intensive bit reverse permutation in the groth16.Prover

	// CosetTable u*<1,g,..,g^(n-1)>
	CosetTable         []fr.Element
	CosetTableReversed []fr.Element // optional, this is computed on demand at the creation of the domain

	// CosetTable[i][j] = domain.Generator(i-th)SqrtInv ^ j
	CosetTableInv         []fr.Element
	CosetTableInvReversed []fr.Element // optional, this is computed on demand at the creation of the domain
}

// NewDomain returns a subgroup with a power of 2 cardinality
// cardinality >= m
// shift: when specified, it's the element by which the set of root of unity is shifted.
func NewDomain(m uint64, shift ...fr.Element) *Domain {

	domain := &Domain{}
	x := ecc.NextPowerOfTwo(m)
	domain.Cardinality = uint64(x)

	// generator of the largest 2-adic subgroup

	domain.FrMultiplicativeGen.SetUint64(5)

	if len(shift) != 0 {
		domain.FrMultiplicativeGen.Set(&shift[0])
	}
	domain.FrMultiplicativeGenInv.Inverse(&domain.FrMultiplicativeGen)

	var err error
	domain.Generator, err = Generator(m)
	if err != nil {
		panic(err)
	}
	domain.GeneratorInv.Inverse(&domain.Generator)
	domain.CardinalityInv.SetUint64(uint64(x)).Inverse(&domain.CardinalityInv)

	// twiddle factors
	domain.preComputeTwiddles()

	// store the bit reversed coset tables
	domain.reverseCosetTables()

	return domain
}

func Generator(m uint64) (fr.Element, error) {
	x := ecc.NextPowerOfTwo(m)

	var rootOfUnity fr.Element

	rootOfUnity.SetString("19103219067921713944291392827692070036145651957329286315305642004821462161904")
	const maxOrderRoot uint64 = 28

	// find generator for Z/2^(log(m))Z
	logx := uint64(bits.TrailingZeros64(x))
	if logx > maxOrderRoot {
		return fr.Element{}, fmt.Errorf("m (%d) is too big: the required root of unity does not exist", m)
	}

	// Generator = FinerGenerator^2 has order x
	expo := uint64(1 << (maxOrderRoot - logx))
	var generator fr.Element
	generator.Exp(rootOfUnity, big.NewInt(int64(expo))) // order x
	return generator, nil
}

func (d *Domain) reverseCosetTables() {
	d.CosetTableReversed = make([]fr.Element, d.Cardinality)
	d.CosetTableInvReversed = make([]fr.Element, d.Cardinality)
	copy(d.CosetTableReversed, d.CosetTable)
	copy(d.CosetTableInvReversed, d.CosetTableInv)
	BitReverse(d.CosetTableReversed)
	BitReverse(d.CosetTableInvReversed)
}

func (d *Domain) preComputeTwiddles() {

	// nb fft stages
	nbStages := uint64(bits.TrailingZeros64(d.Cardinality))

	d.Twiddles = make([][]fr.Element, nbStages)
	d.TwiddlesInv = make([][]fr.Element, nbStages)
	d.CosetTable = make([]fr.Element, d.Cardinality)
	d.CosetTableInv = make([]fr.Element, d.Cardinality)

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

	wg.Add(4)
	go twiddles(d.Twiddles, d.Generator)
	go twiddles(d.TwiddlesInv, d.GeneratorInv)
	go expTable(d.FrMultiplicativeGen, d.CosetTable)
	go expTable(d.FrMultiplicativeGenInv, d.CosetTableInv)

	wg.Wait()

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

	toEncode := []interface{}{d.Cardinality, &d.CardinalityInv, &d.Generator, &d.GeneratorInv, &d.FrMultiplicativeGen, &d.FrMultiplicativeGenInv}

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

	toDecode := []interface{}{&d.Cardinality, &d.CardinalityInv, &d.Generator, &d.GeneratorInv, &d.FrMultiplicativeGen, &d.FrMultiplicativeGenInv}

	for _, v := range toDecode {
		if err := dec.Decode(v); err != nil {
			return dec.BytesRead(), err
		}
	}

	// twiddle factors
	d.preComputeTwiddles()

	// store the bit reversed coset tables if needed
	d.reverseCosetTables()

	return dec.BytesRead(), nil
}

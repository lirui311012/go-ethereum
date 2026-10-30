// Copyright 2014 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package rlp_test

import (
	"fmt"
	"io"

	"github.com/ethereum/go-ethereum/rlp"
)

type MyCoolType struct {
	Name string
	a, b uint
}

// EncodeRLP writes x as RLP list [a, b] that omits the Name field.
func (x *MyCoolType) EncodeRLP(w io.Writer) (err error) {
	return rlp.Encode(w, []uint{x.a, x.b})
}

type MyCoolType2 MyCoolType

// EncodeRLP writes x as RLP list [Name, a, b].
func (x *MyCoolType2) EncodeRLP(w io.Writer) (err error) {
	return rlp.Encode(w, []interface{}{x.Name, x.a, x.b})
}

type MyCoolType3 MyCoolType

// EncodeRLP writes x as RLP list [Name, [a, b]].
func (x *MyCoolType3) EncodeRLP(w io.Writer) (err error) {
	return rlp.Encode(w, []interface{}{x.Name, []uint{x.a, x.b}})
}

// Done
func ExampleEncoder() {
	var t *MyCoolType // t is nil pointer to MyCoolType
	bytes, _ := rlp.EncodeToBytes(t)
	fmt.Printf("%v → %X\n", t, bytes)

	t = &MyCoolType{Name: "foobar", a: 5, b: 6}
	bytes, _ = rlp.EncodeToBytes(t)
	fmt.Printf("%v → %X\n", t, bytes)

	u := &MyCoolType2{Name: "foobar", a: 5, b: 6}
	bytes, _ = rlp.EncodeToBytes(u)
	fmt.Printf("%v → %X\n", u, bytes)

	v := &MyCoolType3{Name: "foobar", a: 5, b: 6}
	bytes, _ = rlp.EncodeToBytes(v)
	fmt.Printf("%v → %X\n", v, bytes)

	// Output:
	// <nil> → C0
	// &{foobar 5 6} → C20506
	// &{foobar 5 6} → C986666F6F6261720506
	// &{foobar 5 6} → CA86666F6F626172C20506
}

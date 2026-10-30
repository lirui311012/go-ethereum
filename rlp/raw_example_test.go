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

	"github.com/ethereum/go-ethereum/rlp"
)

func ExampleRawValue() {
	// C404C20506 is the list [4, [5, 6]].
	// RawValue keeps the second item encoded.
	enc := []byte{0xC4, 0x04, 0xC2, 0x05, 0x06}
	var head struct {
		N    uint
		Rest rlp.RawValue
	}
	if err := rlp.DecodeBytes(enc, &head); err != nil {
		panic(err)
	}
	fmt.Printf("N=%d rest=%X\n", head.N, []byte(head.Rest))

	// Split the saved list without assigning it to a struct.
	content, rest, err := rlp.SplitList(head.Rest)
	if err != nil {
		panic(err)
	}
	a, rest, err := rlp.SplitUint64(content)
	if err != nil {
		panic(err)
	}
	b, rest, err := rlp.SplitUint64(rest)
	if err != nil {
		panic(err)
	}
	fmt.Printf("split %d %d leftover=%d\n", a, b, len(rest))

	// The same inner list can be built and read back as items.
	var list rlp.RawList[uint]
	if err := list.Append(uint(5)); err != nil {
		panic(err)
	}
	if err := list.Append(uint(6)); err != nil {
		panic(err)
	}
	items, err := list.Items()
	if err != nil {
		panic(err)
	}
	fmt.Printf("built %X items=%v\n", list.Bytes(), items)

	// Output:
	// N=4 rest=C20506
	// split 5 6 leftover=0
	// built C20506 items=[5 6]
}

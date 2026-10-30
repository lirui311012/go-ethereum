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

func ExampleIterator() {
	// C404C20506 is the list [4, [5, 6]].
	input := []byte{0xC4, 0x04, 0xC2, 0x05, 0x06}
	it, err := rlp.NewListIterator(input)
	if err != nil {
		panic(err)
	}
	fmt.Println("count", it.Count())
	for it.Next() {
		fmt.Printf("offset=%d value=%X\n", it.Offset(), it.Value())
	}
	if err := it.Err(); err != nil {
		panic(err)
	}

	// The second item is itself a list. Walk it the same way.
	inner, err := rlp.NewListIterator([]byte{0xC2, 0x05, 0x06})
	if err != nil {
		panic(err)
	}
	for inner.Next() {
		fmt.Printf("inner %X\n", inner.Value())
	}
	if err := inner.Err(); err != nil {
		panic(err)
	}

	// Output:
	// count 2
	// offset=1 value=04
	// offset=2 value=C20506
	// inner 05
	// inner 06
}

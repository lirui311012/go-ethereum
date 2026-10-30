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

// coolDecoded reads the list [a, b]. Name is not in the input.
type coolDecoded struct {
	Name string
	a, b uint64
}

// DecodeRLP 从 Stream 读一个列表 [a, b]。输入 C20506 时，游标是这样走的：
//
//	C2 是短列表头，载荷 2 字节。List 把 2 压进栈。
//	05、06 各是一个小于 128 的整数，Uint64 各读走 1 字节，栈顶从 2 减到 0。
//	ListEnd 要求栈顶已经是 0，再把这一层弹掉。Name 不在输入里，保持空字符串。
func (x *coolDecoded) DecodeRLP(s *rlp.Stream) error {
	// 确认下一项是列表，并记下这一层还剩多少载荷。
	if _, err := s.List(); err != nil {
		return err
	}
	// 读列表第一项。单字节整数的值就在类型字节里，后面没有额外载荷。
	a, err := s.Uint64()
	if err != nil {
		return err
	}
	b, err := s.Uint64()
	if err != nil {
		return err
	}
	// 这一层必须刚好读完。还剩字节会返回 errNotAtEOL。
	if err := s.ListEnd(); err != nil {
		return err
	}
	x.a, x.b = a, b
	return nil
}

func ExampleDecoder() {
	// C20506 is the list [5, 6]. DecodeRLP fills the unexported fields.
	var custom coolDecoded
	if err := rlp.DecodeBytes([]byte{0xC2, 0x05, 0x06}, &custom); err != nil {
		panic(err)
	}
	fmt.Printf("custom a=%d b=%d name=%q\n", custom.a, custom.b, custom.Name)

	// C986666F6F6261720506 is the list ["foobar", 5, 6].
	// A plain struct follows the default field order.
	var plain struct {
		Name string
		A, B uint
	}
	input := []byte{0xC9, 0x86, 'f', 'o', 'o', 'b', 'a', 'r', 0x05, 0x06}
	if err := rlp.DecodeBytes(input, &plain); err != nil {
		panic(err)
	}
	fmt.Printf("plain name=%q a=%d b=%d\n", plain.Name, plain.A, plain.B)

	// Output:
	// custom a=5 b=6 name=""
	// plain name="foobar" a=5 b=6
}

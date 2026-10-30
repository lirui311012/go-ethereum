// Copyright 2022 The go-ethereum Authors
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

package rlp

import (
	"encoding/binary"
	"io"
	"math/big"
	"reflect"
	"sync"

	"github.com/ethereum/go-ethereum/common/math"
	"github.com/holiman/uint256"
)

// encBuffer 是编码过程中的暂存区。字符串内容和列表头分开存放，
// 因为列表头里的长度要等列表内容写完才知道。
// str 里是字符串及其字符串头。lheads 记下每个列表头将来要插进 str 的位置。
// copyTo 和 writeTo 输出时才把头嵌回去。
type encBuffer struct {
	str     []byte     // 字符串及其字符串头。不含列表头，列表头在输出时再插入
	lheads  []listhead // 各列表头。offset 是插入 str 的位置，listEnd 之后 size 是该列表的载荷长度
	lhsize  int        // 已经结束的列表头，编码后一共占多少字节
	sizebuf [9]byte    // 写整数和长头时的临时空间。一个头最长 9 字节
}

// encBufferPool 复用 encBuffer。reset 只把切片长度截成 0，底层数组留着下次接着用。
var encBufferPool = sync.Pool{
	New: func() interface{} { return new(encBuffer) },
}

// getEncBuffer 从池里取出并清空。调用方用完要把对象 Put 回去。
func getEncBuffer() *encBuffer {
	buf := encBufferPool.Get().(*encBuffer)
	buf.reset()
	return buf
}

// reset 丢掉已编码的内容，保留 str 和 lheads 的底层数组。
func (buf *encBuffer) reset() {
	buf.lhsize = 0
	buf.str = buf.str[:0]
	buf.lheads = buf.lheads[:0]
}

// size 是目前能确定的输出长度：str 的字节数，加上已经 listEnd 的列表头。
// 还开着的列表头不在 lhsize 里，所以不计入。
func (buf *encBuffer) size() int {
	return len(buf.str) + buf.lhsize
}

// makeBytes 分配一块最终大小的输出，再把头插进去。
func (buf *encBuffer) makeBytes() []byte {
	out := make([]byte, buf.size())
	buf.copyTo(out)
	return out
}

// copyTo 按 lheads 的顺序把最终字节拼进 dst，dst 的长度必须是 size()。
// 每个头的 offset 是它在 str 里的插入点：先拷贝上一个插入点到这里的字符串，再写这个头。
// 循环结束后，把最后一个头后面剩下的字符串拷完。
func (buf *encBuffer) copyTo(dst []byte) {
	strpos := 0
	pos := 0
	for _, head := range buf.lheads {
		// write string data before header
		n := copy(dst[pos:], buf.str[strpos:head.offset])
		pos += n
		strpos += n
		// write the header
		enc := head.encode(dst[pos:])
		pos += len(enc)
	}
	// copy string data after the last list header
	copy(dst[pos:], buf.str[strpos:])
}

// writeTo 的拼接顺序和 copyTo 相同，只是分成多次 Write，不必先拼出一整块。
func (buf *encBuffer) writeTo(w io.Writer) (err error) {
	strpos := 0
	for _, head := range buf.lheads {
		// write string data before header
		if head.offset-strpos > 0 {
			n, err := w.Write(buf.str[strpos:head.offset])
			strpos += n
			if err != nil {
				return err
			}
		}
		// write the header
		enc := head.encode(buf.sizebuf[:])
		if _, err = w.Write(enc); err != nil {
			return err
		}
	}
	if strpos < len(buf.str) {
		// write string data after the last list header
		_, err = w.Write(buf.str[strpos:])
	}
	return err
}

// Write 把 b 原样追加进 str，不再加字符串头。调用方交给它的应当已经是编好的 RLP。
func (buf *encBuffer) Write(b []byte) (int, error) {
	buf.str = append(buf.str, b...)
	return len(b), nil
}

// writeBool 按整数编码。false 是 0，写成空字符串 0x80。true 是 1，单字节 0x01。
func (buf *encBuffer) writeBool(b bool) {
	if b {
		buf.str = append(buf.str, 0x01)
	} else {
		buf.str = append(buf.str, 0x80)
	}
}

// writeUint64 把无符号整数写成规范的 RLP 字符串。
// 0 是 0x80。1 到 127 直接写这个字节。更大的数先写短头 0x80+长度，再写不带前导零的大端字节。
func (buf *encBuffer) writeUint64(i uint64) {
	if i == 0 {
		buf.str = append(buf.str, 0x80)
	} else if i < 128 {
		// fits single byte
		buf.str = append(buf.str, byte(i))
	} else {
		s := putint(buf.sizebuf[1:], i)
		buf.sizebuf[0] = 0x80 + byte(s)
		buf.str = append(buf.str, buf.sizebuf[:s+1]...)
	}
}

// writeBytes 写一条字符串。单字节且值小于 0x80 时不加头，这个字节自己就是编码。
// 其余先写字符串头，再写内容。
func (buf *encBuffer) writeBytes(b []byte) {
	if len(b) == 1 && b[0] <= 0x7F {
		// fits single byte, no string header
		buf.str = append(buf.str, b[0])
	} else {
		buf.encodeStringHeader(len(b))
		buf.str = append(buf.str, b...)
	}
}

// writeString 与 writeBytes 相同，只是入参是 string。
func (buf *encBuffer) writeString(s string) {
	buf.writeBytes([]byte(s))
}

// writeBigInt 忽略符号，按绝对值编码。不超过 64 位时走 writeUint64。
// 更长时，字节数是位长向上取整到 8 的倍数再除以 8。先写头，再按大端填入内容。
func (buf *encBuffer) writeBigInt(i *big.Int) {
	bitlen := i.BitLen()
	if bitlen <= 64 {
		buf.writeUint64(i.Uint64())
		return
	}
	// Integer is larger than 64 bits, encode from i.Bits().
	// The minimal byte length is bitlen rounded up to the next
	// multiple of 8, divided by 8.
	length := ((bitlen + 7) & -8) >> 3
	buf.encodeStringHeader(length)
	buf.str = append(buf.str, make([]byte, length)...)
	bytesBuf := buf.str[len(buf.str)-length:]
	math.ReadBits(i, bytesBuf)
}

// writeUint256 按整数编码。不超过 64 位时走 writeUint64。
// 256 位最多 32 字节，一定是短字符串。四个小端 limb 排成大端放在 33 字节缓冲区的尾部，
// 头写进有效字节的前一格（那一格原是前导零），头和内容就可以一次 append。
func (buf *encBuffer) writeUint256(z *uint256.Int) {
	bitlen := z.BitLen()
	if bitlen <= 64 {
		buf.writeUint64(z.Uint64())
		return
	}
	nBytes := byte((bitlen + 7) / 8)
	var b [33]byte
	binary.BigEndian.PutUint64(b[1:9], z[3])
	binary.BigEndian.PutUint64(b[9:17], z[2])
	binary.BigEndian.PutUint64(b[17:25], z[1])
	binary.BigEndian.PutUint64(b[25:33], z[0])
	b[32-nBytes] = 0x80 + nBytes
	buf.str = append(buf.str, b[32-nBytes:]...)
}

// list 开始一个列表，先不写头。返回值是这个头在 lheads 里的下标，listEnd 要用它。
// offset 记下当前 str 的长度，也就是头将来的插入点。
// size 先暂存此刻的 lhsize。listEnd 靠这份快照把「本列表开始之前就已经存在的头」减掉。
func (buf *encBuffer) list() int {
	buf.lheads = append(buf.lheads, listhead{offset: len(buf.str), size: buf.lhsize})
	return len(buf.lheads) - 1
}

// listEnd 结算下标 index 对应的列表。里面嵌套的列表这时都已经 listEnd 过。
// 载荷长度 = 当前总长度 − 开始时 str 的长度 − 开始时已有列表头的总长度。
// 算完后 size 不再是快照，而是这个列表的载荷字节数：含内部列表头，不含自己的头。
// 载荷不足 56 字节，头只占 1 字节；否则是 1 字节类型加上长度本身的大端字节数。
// 把头的长度加进 lhsize，外层列表结算时才能把它算进自己的载荷。
func (buf *encBuffer) listEnd(index int) {
	lh := &buf.lheads[index]
	lh.size = buf.size() - lh.offset - lh.size
	if lh.size < 56 {
		buf.lhsize++ // length encoded into kind tag
	} else {
		buf.lhsize += 1 + intsize(uint64(lh.size))
	}
}

// encode 按 val 的具体类型取出缓存的 writer，把编码追加进这块缓冲。
func (buf *encBuffer) encode(val interface{}) error {
	rval := reflect.ValueOf(val)
	writer, err := cachedWriter(rval.Type())
	if err != nil {
		return err
	}
	return writer(rval, buf)
}

// encodeStringHeader 只写字符串头，不写内容。
// 长度小于 56 时，头是 0x80+长度。更长时，头是 0xB7+长度字段的字节数，后面紧跟长度的大端表示。
// 单字节且小于 0x80 的特例不在这里，由 writeBytes 处理。
func (buf *encBuffer) encodeStringHeader(size int) {
	if size < 56 {
		buf.str = append(buf.str, 0x80+byte(size))
	} else {
		sizesize := putint(buf.sizebuf[1:], uint64(size))
		buf.sizebuf[0] = 0xB7 + byte(sizesize)
		buf.str = append(buf.str, buf.sizebuf[:sizesize+1]...)
	}
}

// encReader 是 EncodeToReader 返回的 io.Reader。
// 它不必先拼出完整编码，而是按 copyTo 的顺序每次交出一小段：一段字符串，或一个刚编出来的列表头。
// 第一次读到结尾时把 encBuffer 放回池。之后 buf 为空，再读仍然返回 EOF。
type encReader struct {
	buf    *encBuffer // the buffer we're reading from. this is nil when we're at EOF.
	lhpos  int        // index of list header that we're reading
	strpos int        // current position in string buffer
	piece  []byte     // next piece to be read
}

func (r *encReader) Read(b []byte) (n int, err error) {
	for {
		if r.piece = r.next(); r.piece == nil {
			// Put the encode buffer back into the pool at EOF when it
			// is first encountered. Subsequent calls still return EOF
			// as the error but the buffer is no longer valid.
			if r.buf != nil {
				encBufferPool.Put(r.buf)
				r.buf = nil
			}
			return n, io.EOF
		}
		nn := copy(b[n:], r.piece)
		n += nn
		if nn < len(r.piece) {
			// piece didn't fit, see you next time.
			r.piece = r.piece[nn:]
			return n, nil
		}
		r.piece = nil
	}
}

// next 返回下一段还没读走的字节，结束时返回 nil。
// 列表头编进 sizebuf，返回的切片指向这块临时空间。调用方必须先拷走，才能再次调用 next。
// 顺序是当前头之前的字符串，然后这个头；头都出完之后，返回 str 里剩下的部分。
func (r *encReader) next() []byte {
	switch {
	case r.buf == nil:
		return nil

	case r.piece != nil:
		// There is still data available for reading.
		return r.piece

	case r.lhpos < len(r.buf.lheads):
		// We're before the last list header.
		head := r.buf.lheads[r.lhpos]
		sizebefore := head.offset - r.strpos
		if sizebefore > 0 {
			// String data before header.
			p := r.buf.str[r.strpos:head.offset]
			r.strpos += sizebefore
			return p
		}
		r.lhpos++
		return head.encode(r.buf.sizebuf[:])

	case r.strpos < len(r.buf.str):
		// String data at the end, after all list headers.
		p := r.buf.str[r.strpos:]
		r.strpos = len(r.buf.str)
		return p

	default:
		return nil
	}
}

// encBufferFromWriter 认出这个 Writer 其实就是正在用的编码缓冲。
// Encode 和 Reset 用它避免再套一层：认出来就直接往同一块 encBuffer 里写。
func encBufferFromWriter(w io.Writer) *encBuffer {
	switch w := w.(type) {
	case EncoderBuffer:
		return w.buf
	case *EncoderBuffer:
		return w.buf
	case *encBuffer:
		return w
	default:
		return nil
	}
}

// EncoderBuffer is a buffer for incremental encoding.
//
// The zero value is NOT ready for use. To get a usable buffer,
// create it using NewEncoderBuffer or call Reset.
//
// buf 是真正攒字节的地方。dst 是 Flush 要写去的 io.Writer。
// ownBuffer 为真表示 buf 是自己从池里拿的，Flush 时要还回去。
// 派生缓冲和别人共享 buf，ownBuffer 为假，dst 也是 nil：字节已经写进外层，Flush 不再输出。
type EncoderBuffer struct {
	buf *encBuffer
	dst io.Writer

	ownBuffer bool
}

// NewEncoderBuffer creates an encoder buffer.
//
// dst 可以是普通 Writer，也可以是另一块 EncoderBuffer。后者会变成派生缓冲，写入直接进外层的 buf。
// dst 为 nil 时只在内部攒字节，用 ToBytes 取出结果。
func NewEncoderBuffer(dst io.Writer) EncoderBuffer {
	var w EncoderBuffer
	w.Reset(dst)
	return w
}

// Reset truncates the buffer and sets the output destination.
//
// dst 本身如果已经是编码缓冲，这块 EncoderBuffer 不新开内存，而是共用外层的 *encBuffer。
// 此时结构体里的 dst 留空，ownBuffer 为假。写入已经落在外层缓冲里。
// 若再把外层记成输出目标，Flush 会把同一段字节 writeTo 一遍，内容就重复了。
// ownBuffer 为假也避免内层 Flush 把共享缓冲放回池。派生缓冲不能再次 Reset，那会清掉外层正在写的内容。
//
// dst 是普通 Writer 或 nil 时，从池里取一块自己的缓冲，ownBuffer 为真，并把 dst 记下来。
// Flush 时才把编好的字节写到这个 Writer。
func (w *EncoderBuffer) Reset(dst io.Writer) {
	if w.buf != nil && !w.ownBuffer {
		panic("can't Reset derived EncoderBuffer")
	}

	// dst 已经是编码缓冲：共用它的 *encBuffer。
	// dst 字段留空，ownBuffer 保持 false，Flush 时不再写出，也不归还缓冲。
	if dst != nil {
		if outer := encBufferFromWriter(dst); outer != nil {
			*w = EncoderBuffer{outer, nil, false}
			return
		}
	}

	// Get a fresh buffer.
	if w.buf == nil {
		w.buf = encBufferPool.Get().(*encBuffer)
		w.ownBuffer = true
	}
	w.buf.reset()
	w.dst = dst
}

// Flush writes encoded RLP data to the output writer. This can only be called once.
// If you want to re-use the buffer after Flush, you must call Reset.
//
// dst 为 nil 时没有外部写出，派生缓冲和 NewEncoderBuffer(nil) 都是这样。
// 只有 ownBuffer 为真才把 buf 放回池。派生缓冲的 buf 属于外层。
func (w *EncoderBuffer) Flush() error {
	var err error
	if w.dst != nil {
		err = w.buf.writeTo(w.dst)
	}
	// Release the internal buffer.
	if w.ownBuffer {
		encBufferPool.Put(w.buf)
	}
	*w = EncoderBuffer{}
	return err
}

// ToBytes returns the encoded bytes.
func (w *EncoderBuffer) ToBytes() []byte {
	return w.buf.makeBytes()
}

// AppendToBytes appends the encoded bytes to dst.
func (w *EncoderBuffer) AppendToBytes(dst []byte) []byte {
	size := w.buf.size()
	out := append(dst, make([]byte, size)...)
	w.buf.copyTo(out[len(dst):])
	return out
}

// Size returns the total size of the content that was encoded up to this point.
// Note this does not count the size of any lists which are still 'open' (i.e. for
// which ListEnd has not been called yet).
func (w EncoderBuffer) Size() int {
	if w.buf == nil {
		return 0
	}
	return w.buf.size()
}

// Write appends b directly to the encoder output.
func (w EncoderBuffer) Write(b []byte) (int, error) {
	return w.buf.Write(b)
}

// WriteBool writes b as the integer 0 (false) or 1 (true).
func (w EncoderBuffer) WriteBool(b bool) {
	w.buf.writeBool(b)
}

// WriteUint64 encodes an unsigned integer.
func (w EncoderBuffer) WriteUint64(i uint64) {
	w.buf.writeUint64(i)
}

// WriteBigInt encodes a big.Int as an RLP string.
// Note: Unlike with Encode, the sign of i is ignored.
func (w EncoderBuffer) WriteBigInt(i *big.Int) {
	w.buf.writeBigInt(i)
}

// WriteUint256 encodes uint256.Int as an RLP string.
func (w EncoderBuffer) WriteUint256(i *uint256.Int) {
	w.buf.writeUint256(i)
}

// WriteBytes encodes b as an RLP string.
func (w EncoderBuffer) WriteBytes(b []byte) {
	w.buf.writeBytes(b)
}

// WriteString encodes s as an RLP string.
func (w EncoderBuffer) WriteString(s string) {
	w.buf.writeString(s)
}

// List starts a list. It returns an internal index. Call ListEnd with
// this index after encoding the content to finish the list.
func (w EncoderBuffer) List() int {
	return w.buf.list()
}

// ListEnd finishes the given list.
func (w EncoderBuffer) ListEnd(index int) {
	w.buf.listEnd(index)
}

package main

import (
	"encoding/binary"
	"encoding/hex"
)

// Writer 是教学用的字节缓冲，不是 RLP。
// uint64 固定写成 8 字节大端。string 和 []byte 先写 4 字节长度，再写内容。
// 生成的方法和反射方法都往这里写，两边的字节才能逐字节对照。
type Writer struct {
	buf []byte
}

func (w *Writer) Uint64(n uint64) {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], n)
	w.buf = append(w.buf, b[:]...)
}

func (w *Writer) String(s string) {
	w.Bytes([]byte(s))
}

func (w *Writer) Bytes(p []byte) {
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], uint32(len(p)))
	w.buf = append(w.buf, b[:]...)
	w.buf = append(w.buf, p...)
}

func (w *Writer) Dump() []byte { return w.buf }

func (w *Writer) Hex() string { return hex.EncodeToString(w.buf) }

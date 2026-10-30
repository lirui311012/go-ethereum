// Package cpuwork holds the function a CPU profile should attribute time to.
package cpuwork

// iterations 把同一小段计算重复约一百万次。短时间压测时，这段循环会盖过 JSON 和套接字的开销。
const iterations = 1 << 20

// Mix 用 FNV-1a 把 s 折成一个 uint64。
// 同样的两步重复 iterations 次，是为了在 CPU 剖面上占住时间。
// go:noinline 禁止把函数内联进调用方，剖面里才能看到 Mix 这个名字。
//
//go:noinline
func Mix(s string) uint64 {
	b := []byte(s)
	// 空字符串不能对长度取余，换成一个字节，循环仍能跑。
	if len(b) == 0 {
		b = []byte{0}
	}
	// FNV-1a 64 位的初始值。
	var h uint64 = 14695981039346656037
	for i := 0; i < iterations; i++ {
		// 取正文里的一个字节异或进 h，再乘 FNV 质数。正文用完后从头再取。
		h ^= uint64(b[i%len(b)])
		h *= 1099511628211
	}
	return h
}

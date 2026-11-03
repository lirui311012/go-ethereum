package mpt

// 键在内存里按半字节走，每个半字节是 0 到 15。
// 末尾的 16 是终止符，只存在于内存，写进节点时改由首字节的标志位表示。
const nibbleTerm = 16

// keybytesToHex 把原始键拆成半字节，并在末尾加上终止符。
// 终止符让「do」和「dog」可以同时有值：两条路径不会只是一长一短。
func keybytesToHex(key []byte) []byte {
	nibbles := make([]byte, len(key)*2+1)
	for i, b := range key {
		nibbles[i*2] = b >> 4
		nibbles[i*2+1] = b & 0x0f
	}
	nibbles[len(nibbles)-1] = nibbleTerm
	return nibbles
}

// hexToCompact 是黄皮书里的 hex-prefix 编码。
// 首字节的高半字节只有两个标志：低位表示半字节个数是否为奇数，次低位表示这是不是叶子。
// 偶数个半字节时，低半字节是 0，后面每个字节省两个半字节。
// 奇数个时，第一个半字节就放在首字节的低半字节里。
func hexToCompact(hex []byte) []byte {
	term := byte(0)
	if hasTerm(hex) {
		term = 1
		hex = hex[:len(hex)-1]
	}
	buf := make([]byte, len(hex)/2+1)
	buf[0] = term << 5
	if len(hex)&1 == 1 {
		buf[0] |= 1 << 4
		buf[0] |= hex[0]
		hex = hex[1:]
	}
	for bi, ni := 0, 0; ni < len(hex); bi, ni = bi+1, ni+2 {
		buf[1+bi] = hex[ni]<<4 | hex[ni+1]
	}
	return buf
}

// compactToHex 把节点里的紧凑键还原成带终止符的半字节。
// 偶数长度时，首字节的低半字节必须是 0。不是这种唯一写法时返回 false。
func compactToHex(compact []byte) ([]byte, bool) {
	if len(compact) == 0 {
		return nil, false
	}
	flag := compact[0] >> 4
	if flag > 3 {
		return nil, false
	}
	if flag&1 == 0 && compact[0]&0x0f != 0 {
		return nil, false
	}
	base := keybytesToHex(compact)
	if flag < 2 {
		base = base[:len(base)-1]
	}
	chop := 2 - int(flag&1)
	if chop > len(base) {
		return nil, false
	}
	return base[chop:], true
}

func hasTerm(hex []byte) bool {
	return len(hex) > 0 && hex[len(hex)-1] == nibbleTerm
}

func prefixLen(a, b []byte) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	i := 0
	for i < n && a[i] == b[i] {
		i++
	}
	return i
}

func clone(b []byte) []byte {
	if len(b) == 0 {
		return nil
	}
	out := make([]byte, len(b))
	copy(out, b)
	return out
}

func concat(a, b []byte) []byte {
	out := make([]byte, len(a)+len(b))
	copy(out, a)
	copy(out[len(a):], b)
	return out
}

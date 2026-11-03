package mpt

import "encoding/binary"

// 这里只实现 Merkle Patricia Trie 用到的 RLP：字节串和列表。
// 编码规则和以太坊一致，这样节点字节才能得到同样的 Keccak-256。
//
//	单字节且小于 0x80：原样一个字节。
//	0 到 55 字节的串：0x80+长度，再接载荷。空串是 0x80。
//	更长的串：0xb7+长度所占字节数，再接长度，再接载荷。
//	列表用 0xc0 / 0xf7，算法和串相同。

func rlpString(b []byte) []byte {
	if len(b) == 1 && b[0] < 0x80 {
		return []byte{b[0]}
	}
	return rlpPrefix(0x80, b)
}

func rlpList(items ...[]byte) []byte {
	n := 0
	for _, item := range items {
		n += len(item)
	}
	payload := make([]byte, 0, n)
	for _, item := range items {
		payload = append(payload, item...)
	}
	return rlpPrefix(0xc0, payload)
}

func rlpPrefix(offset byte, payload []byte) []byte {
	if len(payload) <= 55 {
		out := make([]byte, 1+len(payload))
		out[0] = offset + byte(len(payload))
		copy(out[1:], payload)
		return out
	}
	var lenBuf [8]byte
	binary.BigEndian.PutUint64(lenBuf[:], uint64(len(payload)))
	i := 0
	for lenBuf[i] == 0 {
		i++
	}
	size := lenBuf[i:]
	out := make([]byte, 1+len(size)+len(payload))
	out[0] = offset + 55 + byte(len(size))
	copy(out[1:], size)
	copy(out[1+len(size):], payload)
	return out
}

// rlpItem 是解码后的一截 RLP。list 为真时看 items，否则看 str。
type rlpItem struct {
	list  bool
	str   []byte
	items []rlpItem
}

// decodeRLP 解码恰好一个值。后面还有多余字节，或长度头不规范，都算失败。
// 证明里的节点必须是这种唯一写法，否则同一份内容可以编出另一个哈希。
func decodeRLP(b []byte) (rlpItem, bool) {
	item, rest, ok := decodeItem(b)
	if !ok || len(rest) != 0 {
		return rlpItem{}, false
	}
	return item, true
}

func decodeItem(b []byte) (rlpItem, []byte, bool) {
	if len(b) == 0 {
		return rlpItem{}, nil, false
	}
	prefix := b[0]
	switch {
	case prefix < 0x80:
		return rlpItem{str: []byte{prefix}}, b[1:], true
	case prefix <= 0xb7:
		n := int(prefix - 0x80)
		if len(b) < 1+n {
			return rlpItem{}, nil, false
		}
		// 一个小于 0x80 的字节必须写成它自己，不能再套 0x81。
		if n == 1 && b[1] < 0x80 {
			return rlpItem{}, nil, false
		}
		return rlpItem{str: clone(b[1 : 1+n])}, b[1+n:], true
	case prefix <= 0xbf:
		size, rest, ok := readLongPayload(b, 0xb7)
		if !ok || size <= 55 {
			return rlpItem{}, nil, false
		}
		return rlpItem{str: clone(rest[:size])}, rest[size:], true
	case prefix <= 0xf7:
		n := int(prefix - 0xc0)
		if len(b) < 1+n {
			return rlpItem{}, nil, false
		}
		items, ok := decodeList(b[1 : 1+n])
		if !ok {
			return rlpItem{}, nil, false
		}
		return rlpItem{list: true, items: items}, b[1+n:], true
	default:
		size, rest, ok := readLongPayload(b, 0xf7)
		if !ok || size <= 55 {
			return rlpItem{}, nil, false
		}
		items, ok := decodeList(rest[:size])
		if !ok {
			return rlpItem{}, nil, false
		}
		return rlpItem{list: true, items: items}, rest[size:], true
	}
}

// readLongPayload 读长串或长列表的载荷。长度本身不能有前导零。
func readLongPayload(b []byte, shortPrefix byte) (int, []byte, bool) {
	ll := int(b[0] - shortPrefix)
	if ll <= 0 || ll > 8 || len(b) < 1+ll || b[1] == 0 {
		return 0, nil, false
	}
	var n uint64
	for _, c := range b[1 : 1+ll] {
		n = n<<8 | uint64(c)
	}
	if n > uint64(len(b)-(1+ll)) {
		return 0, nil, false
	}
	return int(n), b[1+ll:], true
}

func decodeList(payload []byte) ([]rlpItem, bool) {
	var items []rlpItem
	for len(payload) > 0 {
		item, rest, ok := decodeItem(payload)
		if !ok {
			return nil, false
		}
		items = append(items, item)
		payload = rest
	}
	return items, true
}

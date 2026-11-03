// Package mpt 是一棵内存里的 Merkle Patricia Trie，节点编码和以太坊一致。
//
// 键拆成半字节。没有分叉的路径收成叶子或扩展，分叉处是 16 个孩子槽再加一个值槽。
// 每个节点先做 RLP。短于 32 字节的节点直接嵌进父节点，否则父节点只留 Keccak-256。
// 根无论多短都再哈希一次。空树没有节点，根是 Keccak-256(0x80)。
//
// 调用方拿到的根可以用 Prove 带出来的节点单独验证，不必持有整棵树。
// 零值是一棵空树，可以直接调用方法。本实现不做磁盘缓存，也不保证并发安全。
package mpt

import (
	"bytes"
	"encoding/hex"
	"errors"

	"golang.org/x/crypto/sha3"
)

// Hash 是 Keccak-256 的 32 字节结果。
type Hash [32]byte

// EmptyRoot 是空树的根。它是 Keccak-256(0x80)，0x80 是 RLP 的空字节串。
var EmptyRoot = Keccak256([]byte{0x80})

// ErrBadProof 表示证明接不到给定的根，或节点编码不合法。
var ErrBadProof = errors.New("mpt: bad proof")

// Keccak256 计算以太坊使用的 Keccak-256。它和 NIST 的 SHA3-256 不是同一个函数。
func Keccak256(data []byte) Hash {
	h := sha3.NewLegacyKeccak256()
	h.Write(data)
	var out Hash
	copy(out[:], h.Sum(nil))
	return out
}

func (h Hash) String() string {
	return hex.EncodeToString(h[:])
}

// node 是树上的一个节点。valueNode 只出现在叶子末尾或分支的第 17 槽。
type node interface{ isNode() }

type shortNode struct {
	key []byte
	val node
}

type fullNode struct {
	children [17]node
}

type valueNode []byte

func (*shortNode) isNode() {}
func (*fullNode) isNode()  {}
func (valueNode) isNode()  {}

// Trie 是一棵 Merkle Patricia Trie。
type Trie struct {
	root node
}

// New 返回一棵空树。
func New() *Trie {
	return &Trie{}
}

// Put 写入或覆盖 key。value 长度为 0 时删掉这个键，和以太坊一样。
// 写入的是值的副本，之后修改传入的切片不会影响树。
func (t *Trie) Put(key, value []byte) {
	hexKey := keybytesToHex(key)
	if len(value) == 0 {
		_, t.root = deleteNode(t.root, hexKey)
		return
	}
	t.root = insertNode(t.root, hexKey, valueNode(clone(value)))
}

// Get 读取 key。键不存在时 ok 为 false。返回的是值的副本。
func (t *Trie) Get(key []byte) ([]byte, bool) {
	v, ok := getNode(t.root, keybytesToHex(key))
	if !ok {
		return nil, false
	}
	return clone(v), true
}

// Delete 删掉 key。键不存在时返回 false，树保持原样。
// 删掉之后，只剩一个孩子的分支会收成叶子或扩展。
func (t *Trie) Delete(key []byte) bool {
	removed, root := deleteNode(t.root, keybytesToHex(key))
	if !removed {
		return false
	}
	t.root = root
	return true
}

// Len 返回键的个数。
func (t *Trie) Len() int {
	return countValues(t.root)
}

// Hash 返回根哈希。空树返回 EmptyRoot。
func (t *Trie) Hash() Hash {
	if t.root == nil {
		return EmptyRoot
	}
	return Keccak256(encodeNode(t.root))
}

// Proof 是从根走到某个键的节点编码。
// 短于 32 字节的非根节点已经嵌在父节点里，不单独出现。
type Proof struct {
	Nodes [][]byte
}

// Prove 收集 key 的证明。键不存在时，节点停在路径分叉或走空的那一层，用来证明缺席。
// 空树没有节点，证明是空的。
func (t *Trie) Prove(key []byte) Proof {
	hexKey := keybytesToHex(key)
	var nodes []node
	tn := t.root
	for len(hexKey) > 0 && tn != nil {
		switch n := tn.(type) {
		case *shortNode:
			nodes = append(nodes, n)
			if !bytes.HasPrefix(hexKey, n.key) {
				tn = nil
				continue
			}
			tn = n.val
			hexKey = hexKey[len(n.key):]
		case *fullNode:
			nodes = append(nodes, n)
			tn = n.children[hexKey[0]]
			hexKey = hexKey[1:]
		default:
			tn = nil
		}
	}
	var blobs [][]byte
	for i, n := range nodes {
		enc := encodeNode(n)
		if i == 0 || len(enc) >= 32 {
			blobs = append(blobs, enc)
		}
	}
	return Proof{Nodes: blobs}
}

// Verify 用根哈希重放证明。
// found 为 true 时 value 是这个键的值。
// 证明能说明键不存在时 found 为 false，err 为 nil。
// 节点对不上根、中间缺了节点，或编码不合法时返回 ErrBadProof。
func (p Proof) Verify(root Hash, key []byte) ([]byte, bool, error) {
	if root == EmptyRoot && len(p.Nodes) == 0 {
		return nil, false, nil
	}
	byHash := make(map[Hash][]byte, len(p.Nodes))
	for _, blob := range p.Nodes {
		if len(blob) == 0 {
			return nil, false, ErrBadProof
		}
		byHash[Keccak256(blob)] = blob
	}
	blob, ok := byHash[root]
	if !ok {
		return nil, false, ErrBadProof
	}
	item, ok := decodeRLP(blob)
	if !ok {
		return nil, false, ErrBadProof
	}
	value, found, ok := walkProof(byHash, item, keybytesToHex(key))
	if !ok {
		return nil, false, ErrBadProof
	}
	return value, found, nil
}

func insertNode(n node, key []byte, value node) node {
	if len(key) == 0 {
		return value
	}
	switch n := n.(type) {
	case nil:
		return &shortNode{key: clone(key), val: value}
	case *shortNode:
		match := prefixLen(key, n.key)
		if match == len(n.key) {
			n.val = insertNode(n.val, key[match:], value)
			return n
		}
		// 两条路径在 match 处分开。共享的前缀留下，分叉点改成分支。
		branch := &fullNode{}
		branch.children[n.key[match]] = insertNode(nil, n.key[match+1:], n.val)
		branch.children[key[match]] = insertNode(nil, key[match+1:], value)
		if match == 0 {
			return branch
		}
		return &shortNode{key: clone(key[:match]), val: branch}
	case *fullNode:
		n.children[key[0]] = insertNode(n.children[key[0]], key[1:], value)
		return n
	default:
		panic("mpt: insert into a value")
	}
}

func getNode(n node, key []byte) ([]byte, bool) {
	switch n := n.(type) {
	case nil:
		return nil, false
	case valueNode:
		if len(key) != 0 {
			return nil, false
		}
		return []byte(n), true
	case *shortNode:
		if !bytes.HasPrefix(key, n.key) {
			return nil, false
		}
		return getNode(n.val, key[len(n.key):])
	case *fullNode:
		if len(key) == 0 {
			return nil, false
		}
		return getNode(n.children[key[0]], key[1:])
	default:
		return nil, false
	}
}

// deleteNode 返回是否真的删掉了一个值，以及删完后的节点。
// 分支若只剩一个槽，就收成 shortNode，避免树上留下没有分叉的分支。
func deleteNode(n node, key []byte) (bool, node) {
	switch n := n.(type) {
	case nil:
		return false, nil
	case valueNode:
		return true, nil
	case *shortNode:
		match := prefixLen(key, n.key)
		if match < len(n.key) {
			return false, n
		}
		if match == len(key) {
			return true, nil
		}
		removed, child := deleteNode(n.val, key[len(n.key):])
		if !removed {
			return false, n
		}
		if child == nil {
			return true, nil
		}
		if s, ok := child.(*shortNode); ok {
			return true, &shortNode{key: concat(n.key, s.key), val: s.val}
		}
		return true, &shortNode{key: n.key, val: child}
	case *fullNode:
		if len(key) == 0 {
			panic("mpt: branch has no nibble left")
		}
		removed, child := deleteNode(n.children[key[0]], key[1:])
		if !removed {
			return false, n
		}
		n.children[key[0]] = child
		// 换上来的孩子还在，原来至少两个槽，现在也至少两个。
		if child != nil {
			return true, n
		}
		pos := -1
		for i, c := range n.children {
			if c == nil {
				continue
			}
			if pos != -1 {
				return true, n
			}
			pos = i
		}
		if pos < 0 {
			return true, nil
		}
		if pos != nibbleTerm {
			if s, ok := n.children[pos].(*shortNode); ok {
				return true, &shortNode{key: concat([]byte{byte(pos)}, s.key), val: s.val}
			}
		}
		return true, &shortNode{key: []byte{byte(pos)}, val: n.children[pos]}
	default:
		panic("mpt: delete unknown node")
	}
}

func countValues(n node) int {
	switch n := n.(type) {
	case nil:
		return 0
	case valueNode:
		return 1
	case *shortNode:
		return countValues(n.val)
	case *fullNode:
		total := 0
		for _, c := range n.children {
			total += countValues(c)
		}
		return total
	default:
		return 0
	}
}

// encodeNode 返回节点的 RLP。孩子短于 32 字节时嵌进去，否则放 Keccak-256。
func encodeNode(n node) []byte {
	switch n := n.(type) {
	case *shortNode:
		return rlpList(rlpString(hexToCompact(n.key)), childRef(n.val))
	case *fullNode:
		items := make([][]byte, 17)
		for i := range n.children {
			items[i] = childRef(n.children[i])
		}
		return rlpList(items...)
	default:
		panic("mpt: encode value or empty node")
	}
}

// childRef 是父节点里指向孩子的那一项。
// 空槽是 0x80。值是 RLP 字节串。节点编码短于 32 字节时原样嵌进去，否则是 32 字节哈希的 RLP。
func childRef(n node) []byte {
	if n == nil {
		return []byte{0x80}
	}
	if v, ok := n.(valueNode); ok {
		return rlpString(v)
	}
	enc := encodeNode(n)
	if len(enc) < 32 {
		return enc
	}
	sum := Keccak256(enc)
	return rlpString(sum[:])
}

func walkProof(byHash map[Hash][]byte, item rlpItem, key []byte) ([]byte, bool, bool) {
	if !item.list {
		return nil, false, false
	}
	switch len(item.items) {
	case 2:
		hexKey, ok := compactToHex(item.items[0].str)
		if !ok || item.items[0].list {
			return nil, false, false
		}
		if hasTerm(hexKey) {
			// 第二项必须是值。键对不上时，这片叶子就是缺席证明。
			if item.items[1].list {
				return nil, false, false
			}
			if !bytes.Equal(hexKey, key) {
				return nil, false, true
			}
			return clone(item.items[1].str), true, true
		}
		if !bytes.HasPrefix(key, hexKey) {
			return nil, false, true
		}
		next, ok := loadRef(byHash, item.items[1])
		if !ok {
			return nil, false, false
		}
		return walkProof(byHash, next, key[len(hexKey):])
	case 17:
		if len(key) == 0 {
			return nil, false, false
		}
		if key[0] == nibbleTerm {
			slot := item.items[16]
			if slot.list {
				return nil, false, false
			}
			if len(slot.str) == 0 {
				return nil, false, true
			}
			return clone(slot.str), true, true
		}
		next, ok := followBranch(byHash, item.items[key[0]])
		if !ok {
			return nil, false, false
		}
		if next == nil {
			return nil, false, true
		}
		return walkProof(byHash, *next, key[1:])
	default:
		return nil, false, false
	}
}

// followBranch 处理分支的一个槽。空槽表示键不存在，返回 nil, true。
// 编码不合法时返回 false。
func followBranch(byHash map[Hash][]byte, slot rlpItem) (*rlpItem, bool) {
	if slot.list {
		return &slot, true
	}
	if len(slot.str) == 0 {
		return nil, true
	}
	if len(slot.str) != 32 {
		return nil, false
	}
	item, ok := loadHash(byHash, Hash(slot.str))
	if !ok {
		return nil, false
	}
	return &item, true
}

func loadRef(byHash map[Hash][]byte, slot rlpItem) (rlpItem, bool) {
	if slot.list {
		return slot, true
	}
	if len(slot.str) != 32 {
		return rlpItem{}, false
	}
	return loadHash(byHash, Hash(slot.str))
}

func loadHash(byHash map[Hash][]byte, sum Hash) (rlpItem, bool) {
	blob, ok := byHash[sum]
	if !ok {
		return rlpItem{}, false
	}
	return decodeRLP(blob)
}

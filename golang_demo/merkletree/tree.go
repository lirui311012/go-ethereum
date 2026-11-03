// Package merkletree 是一棵二叉 Merkle 树。
//
// 叶子和内部节点用不同前缀分开哈希，避免有人把中间节点当成叶子：
//
//	叶子：SHA-256(0x00 || data)
//	内部：SHA-256(0x01 || left || right)
//
// 以 a、b 为例，真实父节点是 SHA-256(0x01 || H(a) || H(b))。
// H(a) 和 H(b) 各 32 字节，拼起来是 64 字节。
// 如果两种节点用同一种哈希，把这 64 字节当作一片新叶子再哈希，结果正好等于父节点。
// 攻击者再拿父节点上面通往根的兄弟节点，就能让验证者相信这 64 字节是树上的叶子。
// 这里叶子固定从 0x00 起算，同一段 64 字节得到 SHA-256(0x00 || H(a) || H(b))，和父节点对不上。
// Verify 只按叶子规则从原始数据起算，再沿证明往上拼接。
//
// 某一层节点数是奇数时，末尾那个节点与自己配对，再进入上一层。
// 只剩一个节点时它就是根，不再和自己配对。
package merkletree

import (
	"crypto/sha256"
	"errors"
)

// ErrEmpty 表示没有叶子，构不成树。
var ErrEmpty = errors.New("merkletree: no leaves")

// ErrIndex 表示证明下标超出叶子范围。
var ErrIndex = errors.New("merkletree: index out of range")

// Hash 是 SHA-256 的 32 字节结果。
type Hash [32]byte

// LeafHash 计算一片原始数据的叶子哈希。
func LeafHash(data []byte) Hash {
	h := sha256.New()
	h.Write([]byte{0x00})
	h.Write(data)
	var out Hash
	copy(out[:], h.Sum(nil))
	return out
}

// Tree 保存从叶子到根的每一层。layers[0] 是叶子，最后一层只有根。
type Tree struct {
	// 外层 `layers` 是一个动态切片，它的每个元素是 `[]Hash` 类型
	// 内层每个 `[]Hash` 里的元素，是 `[32]byte` 的单个哈希值
	layers [][]Hash
}

// Build 用一组原始数据做叶子。空切片返回 ErrEmpty。
// 某一片数据可以是空字节，它仍然是一片合法叶子。
func Build(chunks [][]byte) (*Tree, error) {
	if len(chunks) == 0 {
		return nil, ErrEmpty
	}
	leaves := make([]Hash, len(chunks))
	for i, chunk := range chunks {
		leaves[i] = LeafHash(chunk)
	}
	// layers[0] 是叶子层。循环看的是最后一层：节点数大于 1 就两两配对，新的一层追加到末尾。
	// 三片叶子会变成 [3 个叶子, 2 个父节点, 1 个根]。只剩 1 个时停，这一层就是根。
	// 只有一片叶子时循环不进，根就是这片叶子的哈希。
	layers := [][]Hash{leaves}
	for len(layers[len(layers)-1]) > 1 {
		layers = append(layers, pair(layers[len(layers)-1]))
	}
	return &Tree{layers: layers}, nil
}

// Root 返回树根。
func (t *Tree) Root() Hash {
	top := t.layers[len(t.layers)-1]
	return top[0]
}

// Step 是证明路径上的一个兄弟节点。
// OnRight 为真时，兄弟在当前节点右边，父节点是 SHA-256(0x01 || 当前 || 兄弟)。
type Step struct {
	Sibling Hash
	OnRight bool
}

// Proof 是某片叶子到根的兄弟节点序列。Index 是这片叶子的下标。
type Proof struct {
	Index int
	Steps []Step
}

// Prove 为第 i 片叶子生成包含证明。
func (t *Tree) Prove(i int) (Proof, error) {
	if i < 0 || i >= len(t.layers[0]) {
		return Proof{}, ErrIndex
	}
	proof := Proof{Index: i}
	idx := i
	for level := 0; level < len(t.layers)-1; level++ {
		sibling, onRight := siblingAt(t.layers[level], idx)
		proof.Steps = append(proof.Steps, Step{Sibling: sibling, OnRight: onRight})
		idx /= 2
	}
	return proof, nil
}

// Verify 用证明检查 data 是否属于这棵以 root 为根的树。
func (p Proof) Verify(root Hash, data []byte) bool {
	cur := LeafHash(data)
	for _, step := range p.Steps {
		if step.OnRight {
			cur = parent(cur, step.Sibling)
		} else {
			cur = parent(step.Sibling, cur)
		}
	}
	return cur == root
}

// pair 把一层节点两两合上去。奇数个时，最后一个与自己配对。
func pair(level []Hash) []Hash {
	n := len(level)
	width := n
	if width%2 == 1 {
		width++
	}
	out := make([]Hash, width/2)
	for i := 0; i < width; i += 2 {
		left := level[i]
		right := left
		if i+1 < n {
			right = level[i+1]
		}
		out[i/2] = parent(left, right)
	}
	return out
}

// siblingAt 返回 idx 在这一层的兄弟，以及兄弟是否在右边。
// 奇数层的最后一个节点没有真实兄弟，兄弟就是它自己。
func siblingAt(level []Hash, idx int) (Hash, bool) {
	if idx%2 == 0 {
		if idx+1 < len(level) {
			return level[idx+1], true
		}
		return level[idx], true
	}
	return level[idx-1], false
}

func parent(left, right Hash) Hash {
	h := sha256.New()
	h.Write([]byte{0x01})
	h.Write(left[:])
	h.Write(right[:])
	var out Hash
	copy(out[:], h.Sum(nil))
	return out
}

// Package patricia 实现压缩前缀树（Patricia tree）。
//
// 普通字典树里一条边只有一个码点。连续没有分叉的路径在这里收成一条边，
// 边上放整段前缀。只有词尾和分叉才会停下新建节点。
// 零值是一棵空树，可以直接调用方法。
package patricia

// Tree 是一棵压缩前缀树。
type Tree struct {
	root  node
	count int
}

// node 的一条边以某个码点开头，边上是一段前缀。
// 根的 prefix 为空。其余节点的 prefix 不为空。
// end 为真表示有词在这个节点结束。
// 根以外，不是词尾的节点至少有两个孩子；否则这条边会和唯一的孩子接成一段。
type node struct {
	prefix   []rune
	children map[rune]*node
	end      bool
}

// New 返回一棵空树。
func New() *Tree {
	return &Tree{}
}

// Insert 放入 word。重复放入同一个词不改变词数。
func (t *Tree) Insert(word string) {
	t.insert(&t.root, []rune(word))
}

// Search 报告 word 是不是一个完整的词。
// 必须走完每一整段边，并停在标了词尾的节点上。停在边的中间不算。
func (t *Tree) Search(word string) bool {
	n := t.walk([]rune(word))
	return n != nil && n.end
}

// StartsWith 报告是否有词以 prefix 开头。
// 前缀可以停在一条边的中间。空前缀停在根上，空树也返回 true。
func (t *Tree) StartsWith(prefix string) bool {
	n := &t.root
	key := []rune(prefix)
	for len(key) > 0 {
		child := n.children[key[0]]
		if child == nil {
			return false
		}
		if hasPrefix(child.prefix, key) {
			return true
		}
		if !hasPrefix(key, child.prefix) {
			return false
		}
		key = key[len(child.prefix):]
		n = child
	}
	return true
}

// Delete 删掉 word。词不在树上时返回 false，树保持原样。
// 删掉之后，空边会被剪掉，只通向一个孩子的非词尾边会和那个孩子接起来。
func (t *Tree) Delete(word string) bool {
	if !t.remove(&t.root, []rune(word)) {
		return false
	}
	t.count--
	return true
}

// Len 返回树上的词数。同一个词只计一次。
func (t *Tree) Len() int {
	return t.count
}

// Edges 返回边的条数。空树是 0。
// 一个没有分叉的词只有一条边，边上是整段词。
func (t *Tree) Edges() int {
	return countEdges(&t.root)
}

// insert 把 word 放进以 n 为根的子树。word 是还没走完的后缀。
func (t *Tree) insert(n *node, word []rune) {
	if len(word) == 0 {
		if n.end {
			return
		}
		n.end = true
		t.count++
		return
	}
	r := word[0]
	child := n.children[r]
	if child == nil {
		if n.children == nil {
			n.children = make(map[rune]*node)
		}
		n.children[r] = &node{prefix: cloneRunes(word), end: true}
		t.count++
		return
	}
	common := commonPrefix(child.prefix, word)
	if common == len(child.prefix) {
		t.insert(child, word[common:])
		return
	}
	// 这条边和 word 只共享一段。把共享的部分拆成新节点。
	split := &node{
		prefix:   cloneRunes(child.prefix[:common]),
		children: make(map[rune]*node, 2),
	}
	child.prefix = cloneRunes(child.prefix[common:])
	split.children[child.prefix[0]] = child
	n.children[r] = split

	rest := word[common:]
	if len(rest) == 0 {
		split.end = true
		t.count++
		return
	}
	split.children[rest[0]] = &node{prefix: cloneRunes(rest), end: true}
	t.count++
}

// walk 沿 key 走完每一整段边。对不上，或者 key 在边的中间用完，返回 nil。
// key 为空时返回根。
func (t *Tree) walk(key []rune) *node {
	n := &t.root
	for len(key) > 0 {
		child := n.children[key[0]]
		if child == nil || !hasPrefix(key, child.prefix) {
			return nil
		}
		key = key[len(child.prefix):]
		n = child
	}
	return n
}

// remove 沿 word 往下走。走到词尾且这里确实是一个词时清掉词尾，并在返回途中收边。
func (t *Tree) remove(n *node, word []rune) bool {
	if len(word) == 0 {
		if !n.end {
			return false
		}
		n.end = false
		return true
	}
	child := n.children[word[0]]
	if child == nil || !hasPrefix(word, child.prefix) {
		return false
	}
	if !t.remove(child, word[len(child.prefix):]) {
		return false
	}
	t.compress(n, word[0])
	return true
}

// compress 在孩子不再是词尾时收边。
// 没有孩子就摘掉这条边。只剩一个孩子就把两段前缀接成一条边。
// 再往上不必循环：合并之后，父节点若也变成这种形状，会由它自己的父节点接着收。
func (t *Tree) compress(parent *node, key rune) {
	child := parent.children[key]
	if !child.end && len(child.children) == 0 {
		delete(parent.children, key)
		if len(parent.children) == 0 {
			parent.children = nil
		}
		return
	}
	if !child.end && len(child.children) == 1 {
		var grand *node
		for _, g := range child.children {
			grand = g
		}
		merged := make([]rune, 0, len(child.prefix)+len(grand.prefix))
		merged = append(merged, child.prefix...)
		merged = append(merged, grand.prefix...)
		grand.prefix = merged
		parent.children[key] = grand
	}
}

func countEdges(n *node) int {
	total := len(n.children)
	for _, child := range n.children {
		total += countEdges(child)
	}
	return total
}

func commonPrefix(a, b []rune) int {
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

func hasPrefix(s, prefix []rune) bool {
	if len(prefix) > len(s) {
		return false
	}
	for i, r := range prefix {
		if s[i] != r {
			return false
		}
	}
	return true
}

func cloneRunes(s []rune) []rune {
	if len(s) == 0 {
		return nil
	}
	out := make([]rune, len(s))
	copy(out, s)
	return out
}

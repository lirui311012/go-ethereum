// Package trie 实现前缀字典树。
//
// 每个节点按下一个 Unicode 码点分叉。从根走到标了词尾的节点，就是一个完整的词。
// 共享前缀的词共用同一段路径。零值是一棵空树，可以直接调用方法。
package trie

// Trie 是一棵字典树。
type Trie struct {
	root  node
	count int
}

// node 的一条边是一个码点。end 为真表示有词在这里结束。
type node struct {
	children map[rune]*node
	end      bool
}

// New 返回一棵空树。
func New() *Trie {
	return &Trie{}
}

// Insert 放入 word。重复放入同一个词不改变词数。
func (t *Trie) Insert(word string) {
	n := &t.root
	for _, r := range word {
		// Go 里对 nil map 取值得到该类型的零值，这里就是 nil，不会 panic
		child := n.children[r]
		if child == nil {
			if n.children == nil {
				n.children = make(map[rune]*node)
			}
			child = &node{}
			n.children[r] = child
		}
		n = child
	}
	if n.end {
		return
	}
	n.end = true
	t.count++
}

// Search 报告 word 是不是一个完整的词。
func (t *Trie) Search(word string) bool {
	n := t.walk(word)
	return n != nil && n.end
}

// StartsWith 报告是否有词以 prefix 开头。
// 空前缀停在根上，空树也返回 true。
func (t *Trie) StartsWith(prefix string) bool {
	return t.walk(prefix) != nil
}

// Delete 删掉 word。词不在树上时返回 false，树保持原样。
// 不再通向任何词的分支会在返回途中被剪掉。
func (t *Trie) Delete(word string) bool {
	if !t.remove(&t.root, []rune(word), 0) {
		return false
	}
	t.count--
	return true
}

// Len 返回树上的词数。同一个词只计一次。
func (t *Trie) Len() int {
	return t.count
}

// walk 沿 s 的每个码点向下走。走不通时返回 nil。s 为空时返回根。
func (t *Trie) walk(s string) *node {
	n := &t.root
	for _, r := range s {
		next := n.children[r]
		if next == nil {
			return nil
		}
		n = next
	}
	return n
}

// remove 沿 word 往下走。走到词尾且这里确实是一个词时清掉词尾标记，并剪掉空分支。
func (t *Trie) remove(n *node, word []rune, i int) bool {
	if i == len(word) {
		if !n.end {
			return false
		}
		n.end = false
		return true
	}
	r := word[i]
	child := n.children[r]
	if child == nil {
		return false
	}
	if !t.remove(child, word, i+1) {
		return false
	}
	if !child.end && len(child.children) == 0 {
		delete(n.children, r)
		if len(n.children) == 0 {
			n.children = nil
		}
	}
	return true
}

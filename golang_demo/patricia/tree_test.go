package patricia

import (
	"sort"
	"strings"
	"testing"
)

func TestInsertAndSearch(t *testing.T) {
	words := []string{"app", "apple", "apply", "apt"}
	tr := New()
	for _, w := range words {
		tr.Insert(w)
		assertShape(t, tr)
	}
	if tr.Len() != len(words) {
		t.Fatalf("Len() = %d, want %d", tr.Len(), len(words))
	}
	for _, w := range words {
		if !tr.Search(w) {
			t.Errorf("Search(%q) = false, want true", w)
		}
	}
	for _, w := range []string{"ap", "appl", "applepie", "ban", ""} {
		if tr.Search(w) {
			t.Errorf("Search(%q) = true, want false", w)
		}
	}
	// apply 和 apple 在最后一个码点分开，所以 le 再拆成 l、e、y。
	if got, want := dump(tr), "ap(p*(l(e*()y*()))t*())"; got != want {
		t.Fatalf("dump = %s, want %s", got, want)
	}
}

func TestLongWordIsOneEdge(t *testing.T) {
	word := strings.Repeat("ab", 50)
	tr := New()
	tr.Insert(word)
	if tr.Edges() != 1 {
		t.Fatalf("Edges() = %d, want 1", tr.Edges())
	}
	if got, want := dump(tr), word+"*()"; got != want {
		t.Fatalf("dump = %s, want %s", got, want)
	}
	runes := []rune(word)
	prefix := string(runes[:len(runes)-1])
	if tr.Search(prefix) {
		t.Fatal("Search stopped in the middle of the only edge")
	}
	if !tr.StartsWith(prefix) {
		t.Fatal("StartsWith missed a prefix of the only edge")
	}
}

func TestSplitShorterWord(t *testing.T) {
	tr := New()
	tr.Insert("apple")
	tr.Insert("app")
	assertShape(t, tr)
	if got, want := dump(tr), "app*(le*())"; got != want {
		t.Fatalf("dump = %s, want %s", got, want)
	}
	if !tr.Search("app") || !tr.Search("apple") || tr.Search("ap") {
		t.Fatalf("Search app=%v apple=%v ap=%v", tr.Search("app"), tr.Search("apple"), tr.Search("ap"))
	}
}

func TestSplitDivergent(t *testing.T) {
	tr := New()
	tr.Insert("apple")
	tr.Insert("apricot")
	assertShape(t, tr)
	if got, want := dump(tr), "ap(ple*()ricot*())"; got != want {
		t.Fatalf("dump = %s, want %s", got, want)
	}
	if tr.Edges() != 3 {
		t.Fatalf("Edges() = %d, want 3", tr.Edges())
	}
}

func TestStartsWith(t *testing.T) {
	tr := New()
	if !tr.StartsWith("") {
		t.Fatal(`StartsWith("") on an empty tree = false, want true`)
	}
	tr.Insert("apple")
	tr.Insert("apply")
	tr.Insert("banana")

	tests := []struct {
		prefix string
		want   bool
	}{
		{prefix: "", want: true},
		{prefix: "a", want: true},
		{prefix: "app", want: true},
		{prefix: "appl", want: true},
		{prefix: "apple", want: true},
		{prefix: "applepie", want: false},
		{prefix: "apply", want: true},
		{prefix: "b", want: true},
		{prefix: "ban", want: true},
		{prefix: "c", want: false},
		{prefix: "apq", want: false},
		{prefix: "applex", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.prefix, func(t *testing.T) {
			if got := tr.StartsWith(tt.prefix); got != tt.want {
				t.Fatalf("StartsWith(%q) = %v, want %v", tt.prefix, got, tt.want)
			}
		})
	}
}

func TestClassicWords(t *testing.T) {
	words := []string{"test", "slow", "water", "slower", "tester", "team", "toast"}
	want := "slow*(er*())t(e(am*()st*(er*()))oast*())water*()"

	forward := New()
	for _, w := range words {
		forward.Insert(w)
		assertShape(t, forward)
	}
	if got := dump(forward); got != want {
		t.Fatalf("forward dump = %s, want %s", got, want)
	}

	backward := New()
	for i := len(words) - 1; i >= 0; i-- {
		backward.Insert(words[i])
		assertShape(t, backward)
	}
	if got := dump(backward); got != want {
		t.Fatalf("backward dump = %s, want %s", got, want)
	}
	if forward.Len() != len(words) || forward.Edges() != 9 {
		t.Fatalf("Len=%d Edges=%d, want 7 words and 9 edges", forward.Len(), forward.Edges())
	}
}

func TestInsertOrderSameShape(t *testing.T) {
	words := []string{"team", "test", "tester", "toast", "slow", "slower", "water"}
	base := New()
	for _, w := range words {
		base.Insert(w)
	}
	want := dump(base)

	rotated := append(append([]string{}, words[3:]...), words[:3]...)
	other := New()
	for _, w := range rotated {
		other.Insert(w)
	}
	if got := dump(other); got != want {
		t.Fatalf("dump = %s, want %s", got, want)
	}
}

func TestSharedPrefixDelete(t *testing.T) {
	tr := New()
	tr.Insert("app")
	tr.Insert("apple")
	tr.Insert("apt")
	if got, want := dump(tr), "ap(p*(le*())t*())"; got != want {
		t.Fatalf("before delete dump = %s, want %s", got, want)
	}

	if !tr.Delete("apple") {
		t.Fatal("Delete(apple) = false, want true")
	}
	assertShape(t, tr)
	if got, want := dump(tr), "ap(p*()t*())"; got != want {
		t.Fatalf("after Delete(apple) dump = %s, want %s", got, want)
	}
	if tr.Search("apple") || !tr.Search("app") || !tr.Search("apt") {
		t.Fatalf("Search apple=%v app=%v apt=%v", tr.Search("apple"), tr.Search("app"), tr.Search("apt"))
	}
	if tr.StartsWith("apple") {
		t.Error(`StartsWith("apple") = true after that word was deleted`)
	}
	if !tr.StartsWith("app") {
		t.Error(`StartsWith("app") = false while "app" is still a word`)
	}

	if !tr.Delete("app") {
		t.Fatal("Delete(app) = false, want true")
	}
	assertShape(t, tr)
	// app 的词尾去掉之后，p 和 t 不再分叉，两条边接回 apt。
	if got, want := dump(tr), "apt*()"; got != want {
		t.Fatalf("after Delete(app) dump = %s, want %s", got, want)
	}
	if tr.Search("app") || !tr.Search("apt") || tr.StartsWith("app") {
		t.Fatalf("Search app=%v apt=%v StartsWith(app)=%v", tr.Search("app"), tr.Search("apt"), tr.StartsWith("app"))
	}
	if tr.Len() != 1 || tr.Edges() != 1 {
		t.Fatalf("Len=%d Edges=%d, want 1 and 1", tr.Len(), tr.Edges())
	}
}

func TestDeleteMergesSingleBranch(t *testing.T) {
	tr := New()
	tr.Insert("apple")
	tr.Insert("apply")
	tr.Insert("apt")
	if got, want := dump(tr), "ap(pl(e*()y*())t*())"; got != want {
		t.Fatalf("dump = %s, want %s", got, want)
	}

	if !tr.Delete("apply") {
		t.Fatal("Delete(apply) = false, want true")
	}
	assertShape(t, tr)
	if got, want := dump(tr), "ap(ple*()t*())"; got != want {
		t.Fatalf("after Delete(apply) dump = %s, want %s", got, want)
	}

	if !tr.Delete("apt") {
		t.Fatal("Delete(apt) = false, want true")
	}
	assertShape(t, tr)
	if got, want := dump(tr), "apple*()"; got != want {
		t.Fatalf("after Delete(apt) dump = %s, want %s", got, want)
	}
	if !tr.Search("apple") || tr.Len() != 1 || tr.Edges() != 1 {
		t.Fatalf("Search=%v Len=%d Edges=%d", tr.Search("apple"), tr.Len(), tr.Edges())
	}
}

func TestDeletePrefixKeepsExtension(t *testing.T) {
	tr := New()
	tr.Insert("app")
	tr.Insert("apple")
	if !tr.Delete("app") {
		t.Fatal(`Delete("app") = false, want true`)
	}
	assertShape(t, tr)
	if got, want := dump(tr), "apple*()"; got != want {
		t.Fatalf("dump = %s, want %s", got, want)
	}
	if tr.Search("app") || !tr.Search("apple") || !tr.StartsWith("app") {
		t.Fatalf("Search app=%v apple=%v StartsWith(app)=%v", tr.Search("app"), tr.Search("apple"), tr.StartsWith("app"))
	}
}

func TestDeleteMissingDoesNotChangeTree(t *testing.T) {
	tr := New()
	tr.Insert("apple")
	tr.Insert("apply")
	before := dump(tr)

	for _, w := range []string{"golang", "", "app", "appl", "applepie", "apq"} {
		if tr.Delete(w) {
			t.Fatalf("Delete(%q) = true, want false", w)
		}
		if got := dump(tr); got != before || tr.Len() != 2 {
			t.Fatalf("Delete(%q) changed the tree: dump %s Len %d", w, got, tr.Len())
		}
	}
}

func TestEmptyWord(t *testing.T) {
	tr := New()
	if tr.Search("") || tr.Edges() != 0 {
		t.Fatalf(`before insert: Search=%v Edges=%d`, tr.Search(""), tr.Edges())
	}
	tr.Insert("")
	assertShape(t, tr)
	if !tr.Search("") || tr.Len() != 1 || tr.Edges() != 0 || dump(tr) != "*" {
		t.Fatalf(`after Insert(""): Search=%v Len=%d Edges=%d dump=%s`, tr.Search(""), tr.Len(), tr.Edges(), dump(tr))
	}
	tr.Insert("a")
	if got, want := dump(tr), "*a*()"; got != want {
		t.Fatalf("dump = %s, want %s", got, want)
	}
	if !tr.Delete("") {
		t.Fatal(`Delete("") = false, want true`)
	}
	assertShape(t, tr)
	if tr.Search("") || !tr.Search("a") || dump(tr) != "a*()" {
		t.Fatalf(`after Delete(""): Search("")=%v Search("a")=%v dump=%s`, tr.Search(""), tr.Search("a"), dump(tr))
	}
}

func TestDuplicateInsert(t *testing.T) {
	tr := New()
	tr.Insert("eth")
	tr.Insert("eth")
	assertShape(t, tr)
	if tr.Len() != 1 || tr.Edges() != 1 || dump(tr) != "eth*()" {
		t.Fatalf("Len=%d Edges=%d dump=%s", tr.Len(), tr.Edges(), dump(tr))
	}
	if !tr.Delete("eth") {
		t.Fatal(`Delete("eth") = false, want true`)
	}
	if tr.Search("eth") || tr.Len() != 0 || tr.Edges() != 0 || dump(tr) != "" {
		t.Fatalf("after delete: Search=%v Len=%d Edges=%d dump=%q", tr.Search("eth"), tr.Len(), tr.Edges(), dump(tr))
	}
	if tr.Delete("eth") {
		t.Fatal(`second Delete("eth") = true, want false`)
	}
}

func TestReinsertAfterMerge(t *testing.T) {
	tr := New()
	tr.Insert("apple")
	tr.Insert("apply")
	if !tr.Delete("apply") {
		t.Fatal(`Delete("apply") = false, want true`)
	}
	if dump(tr) != "apple*()" {
		t.Fatalf("dump = %s, want apple*()", dump(tr))
	}
	tr.Insert("apply")
	assertShape(t, tr)
	if got, want := dump(tr), "appl(e*()y*())"; got != want {
		t.Fatalf("dump = %s, want %s", got, want)
	}
}

func TestUnicodeRunes(t *testing.T) {
	tr := New()
	tr.Insert("你好")
	tr.Insert("你好世界")
	tr.Insert("世界")
	assertShape(t, tr)

	if got, want := dump(tr), "世界*()你好*(世界*())"; got != want {
		t.Fatalf("dump = %s, want %s", got, want)
	}
	for _, w := range []string{"你好", "你好世界", "世界"} {
		if !tr.Search(w) {
			t.Errorf("Search(%q) = false, want true", w)
		}
	}
	if tr.Search("你") {
		t.Error(`Search("你") = true, want false`)
	}
	if !tr.StartsWith("你") || !tr.StartsWith("你好世") {
		t.Fatalf("StartsWith 你=%v 你好世=%v", tr.StartsWith("你"), tr.StartsWith("你好世"))
	}
	if !tr.Delete("你好") {
		t.Fatal(`Delete("你好") = false, want true`)
	}
	assertShape(t, tr)
	// 你好不再是词尾，它和后面的「世界」接成一条边。
	if got, want := dump(tr), "世界*()你好世界*()"; got != want {
		t.Fatalf("dump = %s, want %s", got, want)
	}
	if tr.Search("你好") || !tr.Search("你好世界") || !tr.StartsWith("你好") {
		t.Fatalf("after delete: 你好=%v 你好世界=%v prefix=%v", tr.Search("你好"), tr.Search("你好世界"), tr.StartsWith("你好"))
	}
}

func TestZeroValue(t *testing.T) {
	var tr Tree
	if tr.Search("a") || tr.Len() != 0 || tr.Edges() != 0 {
		t.Fatalf("zero value: Search=%v Len=%d Edges=%d", tr.Search("a"), tr.Len(), tr.Edges())
	}
	if !tr.StartsWith("") {
		t.Fatal(`zero value StartsWith("") = false, want true`)
	}
	tr.Insert("a")
	assertShape(t, &tr)
	if !tr.Search("a") || tr.Len() != 1 || tr.Edges() != 1 {
		t.Fatalf("insert on zero value: Search=%v Len=%d Edges=%d", tr.Search("a"), tr.Len(), tr.Edges())
	}
}

func TestDeleteOnlyWordClearsPrefix(t *testing.T) {
	tr := New()
	tr.Insert("cat")
	if !tr.Delete("cat") {
		t.Fatal(`Delete("cat") = false, want true`)
	}
	for _, p := range []string{"c", "ca", "cat"} {
		if tr.StartsWith(p) {
			t.Errorf("StartsWith(%q) = true after the only word was deleted", p)
		}
	}
	if !tr.StartsWith("") || tr.Len() != 0 || tr.Edges() != 0 || tr.Search("cat") || dump(tr) != "" {
		t.Fatalf("empty tree: StartsWith empty=%v Len=%d Edges=%d Search=%v dump=%q", tr.StartsWith(""), tr.Len(), tr.Edges(), tr.Search("cat"), dump(tr))
	}
}

func TestCaseSensitive(t *testing.T) {
	tr := New()
	tr.Insert("Go")
	if tr.Search("go") || tr.Search("GO") || !tr.Search("Go") {
		t.Fatalf("Search go=%v GO=%v Go=%v", tr.Search("go"), tr.Search("GO"), tr.Search("Go"))
	}
	if tr.StartsWith("g") {
		t.Error(`StartsWith("g") = true, want false`)
	}
	if got, want := dump(tr), "Go*()"; got != want {
		t.Fatalf("dump = %s, want %s", got, want)
	}
}

func TestDeleteBranchKeepsSibling(t *testing.T) {
	tr := New()
	for _, w := range []string{"test", "tester", "team", "toast"} {
		tr.Insert(w)
	}
	if !tr.Delete("tester") {
		t.Fatal(`Delete("tester") = false, want true`)
	}
	assertShape(t, tr)
	if got, want := dump(tr), "t(e(am*()st*())oast*())"; got != want {
		t.Fatalf("dump = %s, want %s", got, want)
	}
	if tr.Search("tester") || !tr.Search("test") || !tr.Search("team") || !tr.Search("toast") {
		t.Fatalf("Search tester=%v test=%v team=%v toast=%v", tr.Search("tester"), tr.Search("test"), tr.Search("team"), tr.Search("toast"))
	}
}

func FuzzInsertSearchDelete(f *testing.F) {
	f.Add("app", "apple", "apt")
	f.Add("apple", "app", "apply")
	f.Add("", "a", "ab")
	f.Add("a", "", "b")
	f.Add("你好", "你好世界", "世界")
	f.Add("Go", "go", "G")
	f.Add("cat", "cat", "catalog")
	f.Add("test", "team", "toast")
	f.Add("slow", "slower", "slight")

	f.Fuzz(func(t *testing.T, a, b, c string) {
		// 树按码点存放。非法 UTF-8 会先变成 U+FFFD，预言也用同一规则。
		raw := []string{a, b, c}
		oracle := map[string]struct{}{}
		tr := New()
		for _, w := range raw {
			tr.Insert(w)
			oracle[norm(w)] = struct{}{}
			assertShape(t, tr)
		}
		if tr.Len() != len(oracle) {
			t.Fatalf("Len() = %d, want %d", tr.Len(), len(oracle))
		}
		checkOracle(t, tr, oracle)

		other := New()
		for i := len(raw) - 1; i >= 0; i-- {
			other.Insert(raw[i])
		}
		if dump(tr) != dump(other) {
			t.Fatalf("insert order changed the shape:\n%s\n%s", dump(tr), dump(other))
		}

		key := norm(a)
		if _, ok := oracle[key]; ok {
			if !tr.Delete(a) {
				t.Fatalf("Delete(%q) = false", a)
			}
			delete(oracle, key)
		} else if tr.Delete(a) {
			t.Fatalf("Delete(%q) = true after it was already removed", a)
		}
		assertShape(t, tr)
		checkOracle(t, tr, oracle)

		if tr.Len() == 0 && (tr.Edges() != 0 || dump(tr) != "") {
			t.Fatalf("empty tree still has structure: Edges=%d dump=%q", tr.Edges(), dump(tr))
		}
	})
}

func norm(s string) string {
	return string([]rune(s))
}

func checkOracle(t *testing.T, tr *Tree, oracle map[string]struct{}) {
	t.Helper()
	for w := range oracle {
		runes := []rune(w)
		for i := 0; i <= len(runes); i++ {
			p := string(runes[:i])
			if !tr.StartsWith(p) {
				t.Fatalf("StartsWith(%q) = false", p)
			}
			_, isWord := oracle[p]
			if tr.Search(p) != isWord {
				t.Fatalf("Search(%q) = %v, want %v", p, tr.Search(p), isWord)
			}
		}
		extra := w + "\x00"
		if !startsWithAny(oracle, extra) && tr.StartsWith(extra) {
			t.Fatalf("StartsWith(%q) = true, want false", extra)
		}
		if _, isWord := oracle[extra]; !isWord && tr.Search(extra) {
			t.Fatalf("Search(%q) = true, want false", extra)
		}
	}
}

func startsWithAny(words map[string]struct{}, prefix string) bool {
	for w := range words {
		if strings.HasPrefix(w, prefix) {
			return true
		}
	}
	return false
}

// dump 把树写成稳定的字符串。词尾标 *，孩子按码点排序，括号里是下一条边。
// apple 和 apply 是 appl(e*()y*())。空词是 *。空树是空字符串。
func dump(tr *Tree) string {
	var b strings.Builder
	if tr.root.end {
		b.WriteByte('*')
	}
	writeChildren(&b, &tr.root)
	return b.String()
}

func writeChildren(b *strings.Builder, n *node) {
	keys := make([]rune, 0, len(n.children))
	for r := range n.children {
		keys = append(keys, r)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	for _, r := range keys {
		child := n.children[r]
		b.WriteString(string(child.prefix))
		if child.end {
			b.WriteByte('*')
		}
		b.WriteByte('(')
		writeChildren(b, child)
		b.WriteByte(')')
	}
}

// assertShape 检查压缩不变量：边不为空，键等于前缀的首码点，
// 非词尾节点至少两个孩子，边数不超过每个词最多拆出的两条。
func assertShape(t *testing.T, tr *Tree) {
	t.Helper()
	if len(tr.root.prefix) != 0 {
		t.Fatal("root prefix is not empty")
	}
	seen := map[*node]bool{}
	var walk func(n *node, root bool)
	walk = func(n *node, root bool) {
		t.Helper()
		if seen[n] {
			t.Fatal("cycle in tree")
		}
		seen[n] = true
		if !root {
			if len(n.prefix) == 0 {
				t.Fatal("edge prefix is empty")
			}
			if !n.end && len(n.children) < 2 {
				t.Fatalf("node %q is not compressed: end=%v children=%d", string(n.prefix), n.end, len(n.children))
			}
		}
		if len(n.children) == 0 && n.children != nil {
			t.Fatal("children map is empty but not nil")
		}
		for r, child := range n.children {
			if child == nil {
				t.Fatal("nil child")
			}
			if child.prefix[0] != r {
				t.Fatalf("edge key %q does not match prefix %q", string(r), string(child.prefix))
			}
			walk(child, false)
		}
	}
	walk(&tr.root, true)
	if words := len(collect(tr)); words != tr.Len() {
		t.Fatalf("collected %d words, Len() = %d", words, tr.Len())
	}
	if tr.Edges() > 2*tr.Len() {
		t.Fatalf("Edges() = %d, Len() = %d", tr.Edges(), tr.Len())
	}
}

func collect(tr *Tree) map[string]struct{} {
	out := map[string]struct{}{}
	var walk func(n *node, prefix []rune)
	walk = func(n *node, prefix []rune) {
		buf := append(append([]rune{}, prefix...), n.prefix...)
		if n.end {
			out[string(buf)] = struct{}{}
		}
		for _, child := range n.children {
			walk(child, buf)
		}
	}
	walk(&tr.root, nil)
	return out
}

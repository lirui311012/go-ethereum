package trie

import "testing"

func TestInsertAndSearch(t *testing.T) {
	words := []string{"app", "apple", "apply", "apt"}
	tr := New()
	for _, w := range words {
		tr.Insert(w)
	}
	if tr.Len() != len(words) {
		t.Fatalf("Len() = %d, want %d", tr.Len(), len(words))
	}
	for _, w := range words {
		if !tr.Search(w) {
			t.Errorf("Search(%q) = false, want true", w)
		}
	}
	for _, w := range []string{"ap", "applepie", "ban", "appl", ""} {
		if tr.Search(w) {
			t.Errorf("Search(%q) = true, want false", w)
		}
	}
}

func TestStartsWith(t *testing.T) {
	tr := New()
	if !tr.StartsWith("") {
		t.Fatal(`StartsWith("") on an empty trie = false, want true`)
	}
	tr.Insert("apple")
	tr.Insert("banana")

	tests := []struct {
		prefix string
		want   bool
	}{
		{prefix: "", want: true},
		{prefix: "a", want: true},
		{prefix: "app", want: true},
		{prefix: "apple", want: true},
		{prefix: "applepie", want: false},
		{prefix: "b", want: true},
		{prefix: "ban", want: true},
		{prefix: "c", want: false},
		{prefix: "apq", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.prefix, func(t *testing.T) {
			if got := tr.StartsWith(tt.prefix); got != tt.want {
				t.Fatalf("StartsWith(%q) = %v, want %v", tt.prefix, got, tt.want)
			}
		})
	}
}

func TestSharedPrefixDelete(t *testing.T) {
	tr := New()
	tr.Insert("app")
	tr.Insert("apple")
	tr.Insert("apt")

	if !tr.Delete("apple") {
		t.Fatal("Delete(apple) = false, want true")
	}
	if tr.Search("apple") {
		t.Error(`Search("apple") = true after delete`)
	}
	if !tr.Search("app") || !tr.Search("apt") {
		t.Error("deleting apple removed app or apt")
	}
	if tr.StartsWith("apple") {
		t.Error(`StartsWith("apple") = true after that word was deleted`)
	}
	if !tr.StartsWith("app") {
		t.Error(`StartsWith("app") = false while "app" is still a word`)
	}
	if tr.Len() != 2 {
		t.Fatalf("Len() = %d, want 2", tr.Len())
	}

	if !tr.Delete("app") {
		t.Fatal("Delete(app) = false, want true")
	}
	if tr.Search("app") {
		t.Error(`Search("app") = true after delete`)
	}
	if !tr.Search("apt") || !tr.StartsWith("ap") {
		t.Error("deleting the shorter word app removed apt")
	}
	if tr.StartsWith("app") {
		t.Error(`StartsWith("app") = true after app and apple were deleted`)
	}
	if tr.Len() != 1 {
		t.Fatalf("Len() = %d, want 1", tr.Len())
	}
}

func TestDeleteMissingDoesNotChangeTree(t *testing.T) {
	tr := New()
	tr.Insert("go")

	for _, w := range []string{"golang", "", "g", "goo"} {
		if tr.Delete(w) {
			t.Fatalf("Delete(%q) = true, want false", w)
		}
	}
	if !tr.Search("go") || tr.Len() != 1 {
		t.Fatalf("tree changed: Search(go)=%v Len=%d", tr.Search("go"), tr.Len())
	}
}

func TestEmptyWord(t *testing.T) {
	tr := New()
	if tr.Search("") {
		t.Fatal(`Search("") = true before insert`)
	}
	tr.Insert("")
	if !tr.Search("") || tr.Len() != 1 {
		t.Fatalf(`after Insert(""): Search=%v Len=%d`, tr.Search(""), tr.Len())
	}
	tr.Insert("a")
	if !tr.Delete("") {
		t.Fatal(`Delete("") = false, want true`)
	}
	if tr.Search("") || !tr.Search("a") || tr.Len() != 1 {
		t.Fatalf(`after Delete(""): Search("")=%v Search("a")=%v Len=%d`, tr.Search(""), tr.Search("a"), tr.Len())
	}
}

func TestDuplicateInsert(t *testing.T) {
	tr := New()
	tr.Insert("eth")
	tr.Insert("eth")
	if tr.Len() != 1 {
		t.Fatalf("Len() = %d, want 1", tr.Len())
	}
	if !tr.Delete("eth") {
		t.Fatal(`Delete("eth") = false, want true`)
	}
	if tr.Search("eth") || tr.Len() != 0 {
		t.Fatalf("after delete: Search=%v Len=%d", tr.Search("eth"), tr.Len())
	}
	if tr.Delete("eth") {
		t.Fatal(`second Delete("eth") = true, want false`)
	}
}

func TestUnicodeRunes(t *testing.T) {
	tr := New()
	tr.Insert("你好")
	tr.Insert("你好世界")
	tr.Insert("世界")

	for _, w := range []string{"你好", "你好世界", "世界"} {
		if !tr.Search(w) {
			t.Errorf("Search(%q) = false, want true", w)
		}
	}
	if tr.Search("你") {
		t.Error(`Search("你") = true, want false`)
	}
	if !tr.StartsWith("你") {
		t.Error(`StartsWith("你") = false, want true`)
	}
	if !tr.Delete("你好") {
		t.Fatal(`Delete("你好") = false, want true`)
	}
	if tr.Search("你好") || !tr.Search("你好世界") || !tr.StartsWith("你好") {
		t.Fatalf(`after delete: 你好=%v 你好世界=%v prefix=%v`, tr.Search("你好"), tr.Search("你好世界"), tr.StartsWith("你好"))
	}
	if tr.Len() != 2 {
		t.Fatalf("Len() = %d, want 2", tr.Len())
	}
}

func TestZeroValue(t *testing.T) {
	var tr Trie
	if tr.Search("a") || tr.Len() != 0 {
		t.Fatalf("zero value: Search=%v Len=%d", tr.Search("a"), tr.Len())
	}
	if !tr.StartsWith("") {
		t.Fatal(`zero value StartsWith("") = false, want true`)
	}
	tr.Insert("a")
	if !tr.Search("a") || tr.Len() != 1 {
		t.Fatalf("insert on zero value: Search=%v Len=%d", tr.Search("a"), tr.Len())
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
	if !tr.StartsWith("") || tr.Len() != 0 || tr.Search("cat") {
		t.Fatalf("empty tree: StartsWith empty=%v Len=%d Search=%v", tr.StartsWith(""), tr.Len(), tr.Search("cat"))
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
}

func FuzzInsertSearchDelete(f *testing.F) {
	f.Add("app", "apple")
	f.Add("apple", "app")
	f.Add("", "a")
	f.Add("a", "")
	f.Add("你好", "你好世界")
	f.Add("Go", "go")
	f.Add("cat", "cat")

	f.Fuzz(func(t *testing.T, a, b string) {
		tr := New()
		tr.Insert(a)
		tr.Insert(b)

		wantLen := 2
		if a == b {
			wantLen = 1
		}
		if tr.Len() != wantLen {
			t.Fatalf("Len() = %d, want %d for %q and %q", tr.Len(), wantLen, a, b)
		}
		if !tr.Search(a) || !tr.Search(b) {
			t.Fatalf("missing word: Search(%q)=%v Search(%q)=%v", a, tr.Search(a), b, tr.Search(b))
		}
		if !tr.StartsWith(a) || !tr.StartsWith(b) {
			t.Fatal("inserted word is not a prefix of itself")
		}

		if !tr.Delete(a) {
			t.Fatalf("Delete(%q) = false", a)
		}
		if a != b {
			if tr.Search(a) {
				t.Fatalf("Search(%q) = true after delete", a)
			}
			if !tr.Search(b) || !tr.StartsWith(b) {
				t.Fatalf("deleting %q removed %q", a, b)
			}
			if tr.Len() != 1 {
				t.Fatalf("Len() = %d, want 1", tr.Len())
			}
			return
		}
		if tr.Search(a) || tr.Len() != 0 {
			t.Fatalf("duplicate %q still present: Search=%v Len=%d", a, tr.Search(a), tr.Len())
		}
	})
}

package mpt

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"testing"
)

func TestEmptyRoot(t *testing.T) {
	const want = "56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421"
	var tr Trie
	if got := tr.Hash().String(); got != want {
		t.Fatalf("zero trie = %s, want %s", got, want)
	}
	if got := New().Hash().String(); got != want {
		t.Fatalf("New trie = %s, want %s", got, want)
	}
	if got := Keccak256([]byte{0x80}).String(); got != want {
		t.Fatalf("keccak(0x80) = %s, want %s", got, want)
	}
}

func TestHexPrefix(t *testing.T) {
	tests := []struct {
		hex     []byte
		compact []byte
	}{
		{hex: []byte{}, compact: []byte{0x00}},
		{hex: []byte{16}, compact: []byte{0x20}},
		{hex: []byte{1, 2, 3, 4, 5}, compact: []byte{0x11, 0x23, 0x45}},
		{hex: []byte{0, 1, 2, 3, 4, 5}, compact: []byte{0x00, 0x01, 0x23, 0x45}},
		{hex: []byte{15, 1, 12, 11, 8, 16}, compact: []byte{0x3f, 0x1c, 0xb8}},
		{hex: []byte{0, 15, 1, 12, 11, 8, 16}, compact: []byte{0x20, 0x0f, 0x1c, 0xb8}},
	}
	for _, tt := range tests {
		if got := hexToCompact(tt.hex); !bytes.Equal(got, tt.compact) {
			t.Errorf("hexToCompact(%x) = %x, want %x", tt.hex, got, tt.compact)
		}
		got, ok := compactToHex(tt.compact)
		if !ok || !bytes.Equal(got, tt.hex) {
			t.Errorf("compactToHex(%x) = %x, %v, want %x", tt.compact, got, ok, tt.hex)
		}
	}
	if _, ok := compactToHex([]byte{0x01}); ok {
		t.Fatal("even extension with a low nibble must be rejected")
	}
	if _, ok := compactToHex(nil); ok {
		t.Fatal("empty compact key must be rejected")
	}
}

func TestSingleLeafEncoding(t *testing.T) {
	tests := []struct {
		key, value string
		enc        string
	}{
		{key: "a", value: "b", enc: "c482206162"},
		{key: "do", value: "verb", enc: "c98320646f8476657262"},
	}
	for _, tt := range tests {
		tr := New()
		tr.Put([]byte(tt.key), []byte(tt.value))
		proof := tr.Prove([]byte(tt.key))
		if len(proof.Nodes) != 1 {
			t.Fatalf("%s: proof nodes = %d, want 1", tt.key, len(proof.Nodes))
		}
		got := hex.EncodeToString(proof.Nodes[0])
		if got != tt.enc {
			t.Fatalf("%s encoding = %s, want %s", tt.key, got, tt.enc)
		}
		wantHash := Keccak256(mustHex(t, tt.enc))
		if tr.Hash() != wantHash {
			t.Fatalf("%s root = %s, want %s", tt.key, tr.Hash(), wantHash)
		}
		value, found, err := proof.Verify(tr.Hash(), []byte(tt.key))
		if err != nil || !found || string(value) != tt.value {
			t.Fatalf("Verify(%s) = %q, %v, %v", tt.key, value, found, err)
		}
	}
}

func TestPutGetAndIsolation(t *testing.T) {
	tr := New()
	raw := []byte("puppy")
	tr.Put([]byte("dog"), raw)
	raw[0] = 'X'
	got, ok := tr.Get([]byte("dog"))
	if !ok || string(got) != "puppy" {
		t.Fatalf("Get = %q, %v", got, ok)
	}
	got[0] = 'Y'
	got, ok = tr.Get([]byte("dog"))
	if !ok || string(got) != "puppy" {
		t.Fatalf("stored value changed to %q", got)
	}
	if _, ok := tr.Get([]byte("do")); ok {
		t.Fatal("prefix key must not be found")
	}
	if tr.Len() != 1 {
		t.Fatalf("Len = %d, want 1", tr.Len())
	}
}

func TestClassicShape(t *testing.T) {
	tr := putAll(nil, classicPairs())
	const want = `ext[6](branch(4:ext[6 f](branch(6:ext[7](branch(6:leaf[5 T]("coin") T:"puppy")) T:"verb")) 8:leaf[6 f 7 2 7 3 6 5 T]("stallion")))`
	if got := dump(tr.root); got != want {
		t.Fatalf("dump = %s", got)
	}
	rev := classicPairs()
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}
	other := putAll(nil, rev)
	if other.Hash() != tr.Hash() {
		t.Fatalf("order changed root %s vs %s", tr.Hash(), other.Hash())
	}
	if dump(other.root) != want {
		t.Fatalf("reverse dump = %s", dump(other.root))
	}
	for _, kv := range classicPairs() {
		got, ok := tr.Get([]byte(kv[0]))
		if !ok || string(got) != kv[1] {
			t.Errorf("Get(%s) = %q, %v", kv[0], got, ok)
		}
	}
}

func TestDeleteCollapses(t *testing.T) {
	tr := putAll(nil, classicPairs())
	root := tr.Hash()
	if !tr.Delete([]byte("doge")) {
		t.Fatal("Delete(doge) = false")
	}
	const afterDoge = `ext[6](branch(4:ext[6 f](branch(6:leaf[7 T]("puppy") T:"verb")) 8:leaf[6 f 7 2 7 3 6 5 T]("stallion")))`
	if got := dump(tr.root); got != afterDoge {
		t.Fatalf("after doge = %s", got)
	}
	if tr.Delete([]byte("doge")) {
		t.Fatal("second Delete(doge) removed something")
	}
	if tr.Hash() == root {
		t.Fatal("deleting doge left the root unchanged")
	}
	if !tr.Delete([]byte("dog")) {
		t.Fatal("Delete(dog) = false")
	}
	if !tr.Delete([]byte("horse")) {
		t.Fatal("Delete(horse) = false")
	}
	only := New()
	only.Put([]byte("do"), []byte("verb"))
	if tr.Hash() != only.Hash() {
		t.Fatalf("collapsed = %s, fresh = %s", tr.Hash(), only.Hash())
	}
	if tr.Delete([]byte("missing")) {
		t.Fatal("Delete(missing) = true")
	}
	tr.Put([]byte("do"), nil)
	if tr.Len() != 0 || tr.Hash() != EmptyRoot {
		t.Fatalf("empty value Put left len=%d root=%s", tr.Len(), tr.Hash())
	}
}

func TestOverwrite(t *testing.T) {
	tr := New()
	tr.Put([]byte("do"), []byte("verb"))
	first := tr.Hash()
	tr.Put([]byte("do"), []byte("noun"))
	if tr.Len() != 1 {
		t.Fatalf("Len = %d", tr.Len())
	}
	got, _ := tr.Get([]byte("do"))
	if string(got) != "noun" || tr.Hash() == first {
		t.Fatalf("overwrite Get=%q root changed=%v", got, tr.Hash() != first)
	}
}

func TestProofInclusionAndAbsence(t *testing.T) {
	tr := putAll(nil, classicPairs())
	root := tr.Hash()
	for _, kv := range classicPairs() {
		proof := tr.Prove([]byte(kv[0]))
		if len(proof.Nodes) == 0 {
			t.Fatalf("Prove(%s) is empty", kv[0])
		}
		value, found, err := proof.Verify(root, []byte(kv[0]))
		if err != nil || !found || string(value) != kv[1] {
			t.Fatalf("Verify(%s) = %q, %v, %v", kv[0], value, found, err)
		}
	}
	for _, key := range []string{"d", "dot", "dogeo", "cat", "horsefeathers"} {
		proof := tr.Prove([]byte(key))
		value, found, err := proof.Verify(root, []byte(key))
		if err != nil || found || value != nil {
			t.Fatalf("absence %s = %q, %v, %v", key, value, found, err)
		}
	}
	// dot 走到存放 verb 的那个分支才发现槽是空的，所以这份证明里已经有 do。
	// 它能读出 verb，不能读出别的值。cat 在更早的空槽停下，子树不在证明里。
	dot := tr.Prove([]byte("dot"))
	value, found, err := dot.Verify(root, []byte("do"))
	if err != nil || !found || string(value) != "verb" {
		t.Fatalf("dot proof for do = %q, %v, %v", value, found, err)
	}
	if _, _, err := tr.Prove([]byte("cat")).Verify(root, []byte("do")); err == nil {
		t.Fatal("cat proof verified do")
	}
	var empty Trie
	value, found, err = empty.Prove([]byte("do")).Verify(EmptyRoot, []byte("do"))
	if err != nil || found || value != nil {
		t.Fatalf("empty proof = %q, %v, %v", value, found, err)
	}
	if _, _, err := (Proof{}).Verify(root, []byte("do")); err == nil {
		t.Fatal("empty proof verified a non-empty root")
	}
}

func TestProofTamper(t *testing.T) {
	tr := New()
	tr.Put([]byte("do"), []byte("verb"))
	tr.Put([]byte("dog"), []byte("puppy"))
	root := tr.Hash()
	proof := tr.Prove([]byte("dog"))
	if len(proof.Nodes) == 0 {
		t.Fatal("missing proof")
	}
	proof.Nodes[len(proof.Nodes)-1][0] ^= 0x01
	if _, _, err := proof.Verify(root, []byte("dog")); err == nil {
		t.Fatal("tampered proof verified")
	}
}

func TestRandomMatchesRebuild(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	type pair struct{ k, v []byte }
	seen := map[string][]byte{}
	var pairs []pair
	for len(pairs) < 40 {
		k := randomBytes(rng, rng.Intn(6))
		if _, ok := seen[string(k)]; ok {
			continue
		}
		v := randomBytes(rng, 1+rng.Intn(40))
		seen[string(k)] = v
		pairs = append(pairs, pair{k, v})
	}
	tr := New()
	for _, p := range pairs {
		tr.Put(p.k, p.v)
	}
	fresh := New()
	for i := len(pairs) - 1; i >= 0; i-- {
		fresh.Put(pairs[i].k, pairs[i].v)
	}
	if tr.Hash() != fresh.Hash() {
		t.Fatalf("random order root %s vs %s", tr.Hash(), fresh.Hash())
	}
	if tr.Len() != len(pairs) {
		t.Fatalf("Len = %d, keys = %d", tr.Len(), len(pairs))
	}
	for _, p := range pairs {
		got, ok := tr.Get(p.k)
		if !ok || !bytes.Equal(got, p.v) {
			t.Fatalf("Get(%x) = %x, want %x", p.k, got, p.v)
		}
		value, found, err := tr.Prove(p.k).Verify(tr.Hash(), p.k)
		if err != nil || !found || !bytes.Equal(value, p.v) {
			t.Fatalf("proof %x: %x, %v, %v", p.k, value, found, err)
		}
	}
	missing := []byte{0xff, 0xee}
	if _, ok := tr.Get(missing); ok {
		t.Fatal("unexpected missing key")
	}
	if _, found, err := tr.Prove(missing).Verify(tr.Hash(), missing); err != nil || found {
		t.Fatalf("absence proof err=%v found=%v", err, found)
	}
}

func TestLongValueIsHashedNotInlined(t *testing.T) {
	tr := New()
	tr.Put([]byte{0x00}, bytes.Repeat([]byte{0xab}, 32))
	tr.Put([]byte{0x10}, bytes.Repeat([]byte{0xcd}, 32))
	proof := tr.Prove([]byte{0x10})
	if len(proof.Nodes) < 2 {
		t.Fatalf("proof nodes = %d, want at least 2", len(proof.Nodes))
	}
	value, found, err := proof.Verify(tr.Hash(), []byte{0x10})
	if err != nil || !found || len(value) != 32 {
		t.Fatalf("Verify = %d bytes, found=%v err=%v", len(value), found, err)
	}
}

func classicPairs() [][2]string {
	return [][2]string{
		{"do", "verb"},
		{"dog", "puppy"},
		{"doge", "coin"},
		{"horse", "stallion"},
	}
}

func putAll(tr *Trie, pairs [][2]string) *Trie {
	if tr == nil {
		tr = New()
	}
	for _, kv := range pairs {
		tr.Put([]byte(kv[0]), []byte(kv[1]))
	}
	return tr
}

func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func randomBytes(rng *rand.Rand, n int) []byte {
	if n == 0 {
		return nil
	}
	b := make([]byte, n)
	rng.Read(b)
	return b
}

func dump(n node) string {
	switch n := n.(type) {
	case nil:
		return "_"
	case valueNode:
		return fmt.Sprintf("%q", []byte(n))
	case *shortNode:
		kind := "ext"
		if hasTerm(n.key) {
			kind = "leaf"
		}
		return kind + "[" + formatNibbles(n.key) + "](" + dump(n.val) + ")"
	case *fullNode:
		var b strings.Builder
		b.WriteString("branch(")
		sep := ""
		for i, c := range n.children {
			if c == nil {
				continue
			}
			b.WriteString(sep)
			sep = " "
			b.WriteString(formatIndex(i))
			b.WriteByte(':')
			b.WriteString(dump(c))
		}
		b.WriteByte(')')
		return b.String()
	default:
		return "?"
	}
}

func formatIndex(i int) string {
	if i == nibbleTerm {
		return "T"
	}
	return strconv.FormatInt(int64(i), 16)
}

func formatNibbles(k []byte) string {
	parts := make([]string, len(k))
	for i, x := range k {
		if x == nibbleTerm {
			parts[i] = "T"
		} else {
			parts[i] = strconv.FormatInt(int64(x), 16)
		}
	}
	return strings.Join(parts, " ")
}

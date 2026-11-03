package merkletree

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"testing"
)

// 测试里单独写出哈希规则，避免期望值和实现共用同一段代码。
// 叶子：SHA-256(0x00 || data)
// 内部：SHA-256(0x01 || left || right)
func specLeaf(data []byte) Hash {
	sum := sha256.Sum256(append([]byte{0x00}, data...))
	return Hash(sum)
}

func specParent(left, right Hash) Hash {
	buf := make([]byte, 1+len(left)+len(right))
	buf[0] = 0x01
	copy(buf[1:], left[:])
	copy(buf[1+len(left):], right[:])
	return Hash(sha256.Sum256(buf))
}

func TestBuildEmpty(t *testing.T) {
	for _, chunks := range [][][]byte{nil, {}} {
		_, err := Build(chunks)
		if !errors.Is(err, ErrEmpty) {
			t.Fatalf("Build(%v) error = %v, want %v", chunks, err, ErrEmpty)
		}
	}
}

func TestLeafHashMatchesSpec(t *testing.T) {
	data := []byte("tx-1")
	if got := LeafHash(data); got != specLeaf(data) {
		t.Fatalf("LeafHash = %x, want %x", got, specLeaf(data))
	}
}

func TestSingleLeafRoot(t *testing.T) {
	data := []byte("only")
	tree, err := Build([][]byte{data})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := tree.Root(), specLeaf(data); got != want {
		t.Fatalf("root = %x, want %x", got, want)
	}
	proof, err := tree.Prove(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(proof.Steps) != 0 {
		t.Fatalf("single leaf proof steps = %d, want 0", len(proof.Steps))
	}
	if !proof.Verify(tree.Root(), data) {
		t.Fatal("single leaf proof did not verify")
	}
}

func TestTwoLeafRoot(t *testing.T) {
	a, b := []byte("a"), []byte("b")
	tree, err := Build([][]byte{a, b})
	if err != nil {
		t.Fatal(err)
	}
	want := specParent(specLeaf(a), specLeaf(b))
	if got := tree.Root(); got != want {
		t.Fatalf("root = %x, want %x", got, want)
	}
}

func TestOddLeafDuplicatesLast(t *testing.T) {
	a, b, c := []byte("a"), []byte("b"), []byte("c")
	tree, err := Build([][]byte{a, b, c})
	if err != nil {
		t.Fatal(err)
	}
	left := specParent(specLeaf(a), specLeaf(b))
	right := specParent(specLeaf(c), specLeaf(c))
	want := specParent(left, right)
	if got := tree.Root(); got != want {
		t.Fatalf("root = %x, want %x", got, want)
	}
}

func TestRootOrderSensitive(t *testing.T) {
	left, err := Build([][]byte{[]byte("a"), []byte("b")})
	if err != nil {
		t.Fatal(err)
	}
	right, err := Build([][]byte{[]byte("b"), []byte("a")})
	if err != nil {
		t.Fatal(err)
	}
	if left.Root() == right.Root() {
		t.Fatal("swapping leaves kept the same root")
	}
}

func TestProofsVerifyAllIndices(t *testing.T) {
	sets := [][][]byte{
		{[]byte("a")},
		{[]byte("a"), []byte("b")},
		{[]byte("a"), []byte("b"), []byte("c")},
		{[]byte("a"), []byte("b"), []byte("c"), []byte("d")},
		{[]byte("a"), []byte("b"), []byte("c"), []byte("d"), []byte("e")},
	}
	for _, chunks := range sets {
		tree, err := Build(chunks)
		if err != nil {
			t.Fatal(err)
		}
		for i, chunk := range chunks {
			proof, err := tree.Prove(i)
			if err != nil {
				t.Fatalf("Prove(%d) len=%d: %v", i, len(chunks), err)
			}
			if proof.Index != i {
				t.Fatalf("proof index = %d, want %d", proof.Index, i)
			}
			if !proof.Verify(tree.Root(), chunk) {
				t.Fatalf("proof %d/%d did not verify", i, len(chunks))
			}
			// 同一条证明不能拿去证明别的叶子。
			other := chunks[(i+1)%len(chunks)]
			if bytes.Equal(other, chunk) {
				continue
			}
			if proof.Verify(tree.Root(), other) {
				t.Fatalf("proof %d verified the wrong leaf", i)
			}
		}
	}
}

func TestVerifyRejectsTamperedData(t *testing.T) {
	tree, err := Build([][]byte{[]byte("alpha"), []byte("beta")})
	if err != nil {
		t.Fatal(err)
	}
	proof, err := tree.Prove(0)
	if err != nil {
		t.Fatal(err)
	}
	if proof.Verify(tree.Root(), []byte("alpha-tampered")) {
		t.Fatal("tampered leaf verified")
	}
}

func TestVerifyRejectsTamperedSibling(t *testing.T) {
	tree, err := Build([][]byte{[]byte("a"), []byte("b"), []byte("c"), []byte("d")})
	if err != nil {
		t.Fatal(err)
	}
	proof, err := tree.Prove(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(proof.Steps) == 0 {
		t.Fatal("expected sibling steps")
	}
	proof.Steps[0].Sibling[0] ^= 0xff
	if proof.Verify(tree.Root(), []byte("a")) {
		t.Fatal("tampered sibling verified")
	}
}

func TestProveOutOfRange(t *testing.T) {
	tree, err := Build([][]byte{[]byte("a"), []byte("b")})
	if err != nil {
		t.Fatal(err)
	}
	for _, i := range []int{-1, 2} {
		_, err := tree.Prove(i)
		if !errors.Is(err, ErrIndex) {
			t.Fatalf("Prove(%d) error = %v, want %v", i, err, ErrIndex)
		}
	}
}

func TestProofDoesNotVerifyAgainstOtherRoot(t *testing.T) {
	one, err := Build([][]byte{[]byte("a"), []byte("b")})
	if err != nil {
		t.Fatal(err)
	}
	two, err := Build([][]byte{[]byte("a"), []byte("c")})
	if err != nil {
		t.Fatal(err)
	}
	proof, err := one.Prove(0)
	if err != nil {
		t.Fatal(err)
	}
	if proof.Verify(two.Root(), []byte("a")) {
		t.Fatal("proof verified against a different root")
	}
}

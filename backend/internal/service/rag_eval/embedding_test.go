package rageval

import (
	"testing"
)

// TestDeterministicEmbedding pins the local-dev standin embedding: same text
// in, same vector out; different text gives a different vector; the vector
// has the requested dimension and unit L2 norm.
func TestDeterministicEmbedding(t *testing.T) {
	const dims = 1536

	a := DeterministicEmbedding("哑铃全身训练", dims)
	b := DeterministicEmbedding("哑铃全身训练", dims)
	if len(a) != dims {
		t.Fatalf("embedding dims = %d, want %d", len(a), dims)
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("embedding must be deterministic, dim %d differs", i)
		}
	}

	c := DeterministicEmbedding("营养早餐碗", dims)
	same := true
	for i := range a {
		if a[i] != c[i] {
			same = false
			break
		}
	}
	if same {
		t.Fatal("different texts must produce different embeddings")
	}

	var norm float64
	for _, v := range a {
		norm += float64(v) * float64(v)
	}
	if norm < 0.9999 || norm > 1.0001 {
		t.Errorf("embedding L2 norm = %v, want ~1", norm)
	}
}

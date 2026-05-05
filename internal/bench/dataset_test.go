package bench

import "testing"

func TestBuildDatasetDeterministic(t *testing.T) {
	a, err := BuildDataset("small", 7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b, err := BuildDataset("small", 7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(a) != len(b) {
		t.Fatalf("different sizes: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("record %d mismatch: %#v vs %#v", i, a[i], b[i])
		}
	}
}

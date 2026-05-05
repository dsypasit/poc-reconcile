package bench

import "testing"

func TestSampleTargetSizeBounds(t *testing.T) {
	if got := sampleTargetSize(0); got != 0 {
		t.Fatalf("expected 0, got %d", got)
	}
	if got := sampleTargetSize(10); got != 10 {
		t.Fatalf("expected capped-to-total 10, got %d", got)
	}
	if got := sampleTargetSize(1000); got != 100 {
		t.Fatalf("expected 100 for 1000 records, got %d", got)
	}
	if got := sampleTargetSize(100000); got != 200 {
		t.Fatalf("expected max cap 200, got %d", got)
	}
}

func TestStratifiedSampleKeepsDomainCoverage(t *testing.T) {
	records := []Record{
		{Key: "alpha:000001", Value: "a1"},
		{Key: "alpha:000002", Value: "a2"},
		{Key: "beta:000001", Value: "b1"},
		{Key: "beta:000002", Value: "b2"},
		{Key: "gamma:000001", Value: "g1"},
		{Key: "gamma:000002", Value: "g2"},
	}
	sample := stratifiedSample(records, 6, 1)
	if len(sample) != 6 {
		t.Fatalf("expected 6 records, got %d", len(sample))
	}

	counts := map[string]int{}
	for _, rec := range sample {
		counts[keyDomain(rec.Key)]++
	}
	if counts["alpha"] == 0 || counts["beta"] == 0 || counts["gamma"] == 0 {
		t.Fatalf("missing expected domain coverage: %#v", counts)
	}
}

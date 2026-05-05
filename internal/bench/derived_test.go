package bench

import "testing"

func TestFormatRPS(t *testing.T) {
	if got := formatRPS(1000, 0); got != "0" {
		t.Fatalf("expected 0 on zero duration, got %s", got)
	}
	if got := formatRPS(1000, 1000); got != "1000.00" {
		t.Fatalf("unexpected rps: %s", got)
	}
}

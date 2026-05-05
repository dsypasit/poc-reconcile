package bench

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestBuildCanonicalRankableRunsSortsDeterministically(t *testing.T) {
	tmp := t.TempDir()
	runs := []RunMetrics{
		{RunID: "r-low", Approach: "approach-a", Tier: "small", DurationMS: 100, Counts: map[string]int{"records_written": 100}, Errors: map[string]int{"ingest": 0, "cutover": 0, "cleanup": 0}, Metadata: map[string]string{"overall_throughput_rps": "1000.00", "rank_eligible": "true"}},
		{RunID: "r-high", Approach: "approach-b", Tier: "small", DurationMS: 100, Counts: map[string]int{"records_written": 200}, Errors: map[string]int{"ingest": 0, "cutover": 0, "cleanup": 0}, Metadata: map[string]string{"overall_throughput_rps": "2000.00", "rank_eligible": "true"}},
		{RunID: "r-invalid", Approach: "approach-c", Tier: "small", DurationMS: 10, Counts: map[string]int{"records_written": 999}, Errors: map[string]int{"ingest": 0, "cutover": 0, "cleanup": 0}, Metadata: map[string]string{"overall_throughput_rps": "9999.00", "rank_eligible": "false"}},
	}

	for _, r := range runs[:2] {
		env := OTelExportEnvelope{Values: map[string]int64{"duration_ms": 100, "records_written": int64(r.Counts["records_written"])}}
		b, err := json.Marshal(env)
		if err != nil {
			t.Fatal(err)
		}
		runDir := filepath.Join(tmp, r.RunID)
		if err := os.MkdirAll(runDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(runDir, "otel-metrics.json"), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	ranked, _ := BuildCanonicalRankableRuns(tmp, runs)
	if len(ranked) != 2 {
		t.Fatalf("expected 2 rankable runs, got %d", len(ranked))
	}
	if ranked[0].RunID != "r-high" || ranked[1].RunID != "r-low" {
		t.Fatalf("unexpected ranking order: %s, %s", ranked[0].RunID, ranked[1].RunID)
	}
}

func TestBuildCanonicalRankableRunsSurfacesMismatch(t *testing.T) {
	tmp := t.TempDir()
	runs := []RunMetrics{
		{RunID: "r1", Approach: "approach-a", Tier: "small", DurationMS: 500, Counts: map[string]int{"records_written": 100}, Errors: map[string]int{"ingest": 0, "cutover": 0, "cleanup": 0}, Metadata: map[string]string{"overall_throughput_rps": "50.00", "rank_eligible": "true"}},
	}
	env := OTelExportEnvelope{Values: map[string]int64{"duration_ms": 100, "records_written": 100}}
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	runDir := filepath.Join(tmp, "r1")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "otel-metrics.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}

	ranked, failures := BuildCanonicalRankableRuns(tmp, runs)
	if len(ranked) != 1 {
		t.Fatalf("expected 1 rankable run, got %d", len(ranked))
	}
	if ranked[0].DurationMS != 100 {
		t.Fatalf("expected canonical duration from otel, got %d", ranked[0].DurationMS)
	}
	if len(failures) == 0 {
		t.Fatalf("expected quality failure for local-vs-otel mismatch")
	}
}

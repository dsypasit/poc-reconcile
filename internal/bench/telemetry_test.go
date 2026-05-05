package bench

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestExportOTELIfEnabled(t *testing.T) {
	tmp := t.TempDir()
	cfg := Config{
		Approach:     "approach-a",
		Tier:         "small",
		Seed:         42,
		KvrocksAddr:  "127.0.0.1:6666",
		Prefix:       "bench:test:",
		RunID:        "run-otel",
		OutDir:       tmp,
		OTELEnabled:  true,
		OTELEndpoint: "test-endpoint",
	}
	m := RunMetrics{
		RunID:       cfg.RunID,
		Approach:    cfg.Approach,
		Tier:        cfg.Tier,
		KvrocksAddr: cfg.KvrocksAddr,
		Seed:        cfg.Seed,
		StartedAt:   time.Now().UTC(),
		CompletedAt: time.Now().UTC(),
		DurationMS:  100,
		IngestMS:    30,
		CutoverMS:   20,
		CleanupMS:   50,
		Errors:      map[string]int{"ingest": 0, "cutover": 0, "cleanup": 0},
		Counts:      map[string]int{"records_written": 10, "records_deleted": 2},
	}
	if err := ExportOTELIfEnabled(tmp, cfg, m); err != nil {
		t.Fatalf("export otel: %v", err)
	}
	path := filepath.Join(tmp, cfg.RunID, "otel-metrics.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected otel artifact: %v", err)
	}
}

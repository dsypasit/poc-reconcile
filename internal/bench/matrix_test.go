package bench

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteMatrixArtifacts(t *testing.T) {
	tmp := t.TempDir()
	runs := []RunMetrics{
		{
			RunID:       "r1",
			Approach:    "approach-a",
			Tier:        "small",
			StartedAt:   time.Now().UTC(),
			CompletedAt: time.Now().UTC(),
			DurationMS:  100,
			IngestMS:    50,
			CutoverMS:   10,
			CleanupMS:   40,
			Errors:      map[string]int{"ingest": 0, "cutover": 0, "cleanup": 0},
			Counts:      map[string]int{"records_written": 1000, "records_deleted": 0},
			Metadata:    map[string]string{"ingest_throughput_rps": "20000.00", "overall_throughput_rps": "10000.00", "validation_passed": "true"},
		},
	}
	outDir, err := WriteMatrixArtifacts(tmp, "matrix-test", runs, nil)
	if err != nil {
		t.Fatalf("write matrix artifacts: %v", err)
	}
	for _, name := range []string{"matrix.json", "matrix.csv", "report.md"} {
		if _, err := os.Stat(filepath.Join(outDir, name)); err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
	}
}

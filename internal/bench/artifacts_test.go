package bench

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteArtifacts(t *testing.T) {
	tmp := t.TempDir()
	m := RunMetrics{
		RunID:       "run-1",
		Approach:    "approach-a",
		Tier:        "small",
		KvrocksAddr: "127.0.0.1:6666",
		Seed:        42,
		StartedAt:   time.Now().UTC(),
		CompletedAt: time.Now().UTC(),
		DurationMS:  100,
		IngestMS:    50,
		CutoverMS:   10,
		CleanupMS:   40,
		Errors:      map[string]int{"ingest": 0, "cutover": 0, "cleanup": 0},
		Counts:      map[string]int{"records_written": 1000, "records_deleted": 5},
		Metadata:    map[string]string{"strategy": "SCAN+UNLINK"},
	}

	runDir, err := WriteArtifacts(tmp, m)
	if err != nil {
		t.Fatalf("write artifacts: %v", err)
	}

	jsonPath := filepath.Join(runDir, "metrics.json")
	b, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("read json: %v", err)
	}
	var decoded RunMetrics
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	if decoded.RunID != m.RunID || decoded.Approach != m.Approach {
		t.Fatalf("unexpected json content: %#v", decoded)
	}

	csvPath := filepath.Join(runDir, "metrics.csv")
	f, err := os.Open(csvPath)
	if err != nil {
		t.Fatalf("open csv: %v", err)
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatalf("read csv: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[1][0] != m.RunID {
		t.Fatalf("unexpected run id in csv: %q", rows[1][0])
	}
}

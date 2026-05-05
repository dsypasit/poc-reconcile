package bench

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDatasetHashDeterministic(t *testing.T) {
	recordsA, err := BuildDataset("small", 42)
	if err != nil {
		t.Fatalf("build dataset A: %v", err)
	}
	recordsB, err := BuildDataset("small", 42)
	if err != nil {
		t.Fatalf("build dataset B: %v", err)
	}
	if gotA, gotB := datasetHash(recordsA), datasetHash(recordsB); gotA != gotB {
		t.Fatalf("expected identical hashes, got %s != %s", gotA, gotB)
	}
}

func TestWriteReproducibilityArtifacts(t *testing.T) {
	tmp := t.TempDir()
	records, err := BuildDataset("small", 7)
	if err != nil {
		t.Fatalf("build dataset: %v", err)
	}
	cfg := Config{
		Approach:    "approach-a",
		Tier:        "small",
		Seed:        7,
		KvrocksAddr: "127.0.0.1:6666",
		Prefix:      "bench:test:",
		RunID:       "run-123",
		OutDir:      tmp,
	}

	manifest, err := WriteReproducibilityArtifacts(tmp, cfg, records)
	if err != nil {
		t.Fatalf("write reproducibility artifacts: %v", err)
	}
	if manifest.RecordCount != len(records) {
		t.Fatalf("unexpected record count: %d", manifest.RecordCount)
	}
	if manifest.DatasetHash == "" {
		t.Fatalf("expected dataset hash")
	}

	configPath := filepath.Join(tmp, cfg.RunID, "config.json")
	manifestPath := filepath.Join(tmp, cfg.RunID, "manifest.json")
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("missing config artifact: %v", err)
	}
	b, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest artifact: %v", err)
	}
	var decoded ManifestSnapshot
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("decode manifest artifact: %v", err)
	}
	if decoded.RunID != cfg.RunID {
		t.Fatalf("unexpected manifest run id: %s", decoded.RunID)
	}
}

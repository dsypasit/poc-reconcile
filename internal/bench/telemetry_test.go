package bench

import (
	"encoding/json"
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
		MatrixID:     "matrix-test",
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
		DurationMS:  -100,
		IngestMS:    30,
		CutoverMS:   20,
		CleanupMS:   50,
		Errors:      map[string]int{"ingest": 0, "cutover": -1, "cleanup": 0},
		Counts:      map[string]int{"records_written": 10, "records_deleted": -2},
	}
	if err := ExportOTELIfEnabled(tmp, cfg, m); err != nil {
		t.Fatalf("export otel: %v", err)
	}
	path := filepath.Join(tmp, cfg.RunID, "otel-metrics.json")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected otel artifact: %v", err)
	}

	var got OTelExportEnvelope
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("decode otel artifact: %v", err)
	}
	if got.Labels["run_id"] != cfg.RunID || got.Labels["approach"] != cfg.Approach || got.Labels["tier"] != cfg.Tier {
		t.Fatalf("missing required run labels: %#v", got.Labels)
	}
	if got.Labels["seed"] != "42" || got.Labels["endpoint"] != cfg.OTELEndpoint || got.Labels["matrix_id"] != cfg.MatrixID {
		t.Fatalf("missing required seed/endpoint/matrix labels: %#v", got.Labels)
	}
	for k, v := range got.Values {
		if v < 0 {
			t.Fatalf("metric %s must be non-negative: %d", k, v)
		}
	}
}

func TestApplyProducerValidityGate(t *testing.T) {
	cfg := Config{
		Approach:     "approach-a",
		Tier:         "small",
		Seed:         42,
		MatrixID:     "matrix-test",
		RunID:        "run-pass",
		OTELEnabled:  true,
		OTELEndpoint: "otlp-local",
	}
	m := RunMetrics{
		RunID:      cfg.RunID,
		Approach:   cfg.Approach,
		Tier:       cfg.Tier,
		Seed:       cfg.Seed,
		DurationMS: 1,
		IngestMS:   1,
		CutoverMS:  1,
		CleanupMS:  1,
		Errors:     map[string]int{"ingest": 0, "cutover": 0, "cleanup": 0},
		Counts:     map[string]int{"records_written": 1, "records_deleted": 0},
		Metadata:   map[string]string{"otel_export_status": "exported"},
	}
	if err := ApplyProducerValidityGate(cfg, &m, nil); err != nil {
		t.Fatalf("expected valid producer gate, got %v", err)
	}
	if m.Metadata["producer_validity_passed"] != "true" || m.Metadata["rank_eligible"] != "true" {
		t.Fatalf("expected valid/rankable metadata, got %#v", m.Metadata)
	}
}

func TestApplyProducerValidityGateRejectsDisabledOTEL(t *testing.T) {
	cfg := Config{
		Approach:     "approach-a",
		Tier:         "small",
		Seed:         42,
		RunID:        "run-fail",
		OTELEnabled:  false,
		OTELEndpoint: "otlp-local",
	}
	m := RunMetrics{
		RunID:      cfg.RunID,
		Approach:   cfg.Approach,
		Tier:       cfg.Tier,
		Seed:       cfg.Seed,
		DurationMS: 1,
		IngestMS:   1,
		CutoverMS:  1,
		CleanupMS:  1,
		Errors:     map[string]int{"ingest": 0, "cutover": 0, "cleanup": 0},
		Counts:     map[string]int{"records_written": 1, "records_deleted": 0},
		Metadata:   map[string]string{"otel_export_status": "disabled"},
	}
	if err := ApplyProducerValidityGate(cfg, &m, nil); err == nil {
		t.Fatalf("expected producer validity gate failure")
	}
	if m.Metadata["producer_validity_passed"] != "false" || m.Metadata["rank_eligible"] != "false" {
		t.Fatalf("expected invalid/non-rankable metadata, got %#v", m.Metadata)
	}
}

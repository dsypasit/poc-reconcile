package bench

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type OTelExportEnvelope struct {
	RunID       string            `json:"run_id"`
	Approach    string            `json:"approach"`
	Tier        string            `json:"tier"`
	ExportedAt  time.Time         `json:"exported_at"`
	Endpoint    string            `json:"endpoint"`
	MetricNames map[string]string `json:"metric_names"`
	Values      map[string]int64  `json:"values"`
}

func MetricsNameMap() map[string]string {
	return map[string]string{
		"duration_ms":     "bench.run.duration_ms",
		"ingest_ms":       "bench.phase.ingest_ms",
		"cutover_ms":      "bench.phase.cutover_ms",
		"cleanup_ms":      "bench.phase.cleanup_ms",
		"ingest_errors":   "bench.errors.ingest_total",
		"cutover_errors":  "bench.errors.cutover_total",
		"cleanup_errors":  "bench.errors.cleanup_total",
		"records_written": "bench.records.written_total",
		"records_deleted": "bench.records.deleted_total",
	}
}

func ExportOTELIfEnabled(baseDir string, cfg Config, m RunMetrics) error {
	if !cfg.OTELEnabled {
		return nil
	}
	runDir := filepath.Join(baseDir, cfg.RunID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return fmt.Errorf("create run dir: %w", err)
	}

	envelope := OTelExportEnvelope{
		RunID:       m.RunID,
		Approach:    m.Approach,
		Tier:        m.Tier,
		ExportedAt:  time.Now().UTC(),
		Endpoint:    cfg.OTELEndpoint,
		MetricNames: MetricsNameMap(),
		Values: map[string]int64{
			"duration_ms":     m.DurationMS,
			"ingest_ms":       m.IngestMS,
			"cutover_ms":      m.CutoverMS,
			"cleanup_ms":      m.CleanupMS,
			"ingest_errors":   int64(m.Errors["ingest"]),
			"cutover_errors":  int64(m.Errors["cutover"]),
			"cleanup_errors":  int64(m.Errors["cleanup"]),
			"records_written": int64(m.Counts["records_written"]),
			"records_deleted": int64(m.Counts["records_deleted"]),
		},
	}
	payload, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal otel export: %w", err)
	}
	path := filepath.Join(runDir, "otel-metrics.json")
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		return fmt.Errorf("write otel export: %w", err)
	}
	return nil
}

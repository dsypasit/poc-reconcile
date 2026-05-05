package bench

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type OTelExportEnvelope struct {
	RunID       string            `json:"run_id"`
	Approach    string            `json:"approach"`
	Tier        string            `json:"tier"`
	Seed        int64             `json:"seed"`
	ExportedAt  time.Time         `json:"exported_at"`
	Endpoint    string            `json:"endpoint"`
	Labels      map[string]string `json:"labels"`
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
		Seed:        m.Seed,
		ExportedAt:  time.Now().UTC(),
		Endpoint:    cfg.OTELEndpoint,
		Labels:      requiredOTELLabels(cfg, m),
		MetricNames: MetricsNameMap(),
		Values: map[string]int64{
			"duration_ms":     nonNegativeInt64(m.DurationMS),
			"ingest_ms":       nonNegativeInt64(m.IngestMS),
			"cutover_ms":      nonNegativeInt64(m.CutoverMS),
			"cleanup_ms":      nonNegativeInt64(m.CleanupMS),
			"ingest_errors":   nonNegativeInt64(int64(m.Errors["ingest"])),
			"cutover_errors":  nonNegativeInt64(int64(m.Errors["cutover"])),
			"cleanup_errors":  nonNegativeInt64(int64(m.Errors["cleanup"])),
			"records_written": nonNegativeInt64(int64(m.Counts["records_written"])),
			"records_deleted": nonNegativeInt64(int64(m.Counts["records_deleted"])),
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

func requiredOTELLabels(cfg Config, m RunMetrics) map[string]string {
	labels := map[string]string{
		"run_id":   m.RunID,
		"approach": m.Approach,
		"tier":     m.Tier,
		"seed":     strconv.FormatInt(m.Seed, 10),
		"endpoint": cfg.OTELEndpoint,
	}
	if cfg.MatrixID != "" {
		labels["matrix_id"] = cfg.MatrixID
	}
	return labels
}

func nonNegativeInt64(v int64) int64 {
	if v < 0 {
		return 0
	}
	return v
}

package b

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/prasit-sri/new-cda-plan/internal/bench"
)

const (
	defaultDataTTLSeconds = 3600
)

func Run(ctx context.Context, cfg bench.Config) (bench.RunMetrics, error) {
	if err := cfg.Validate(); err != nil {
		return bench.RunMetrics{}, err
	}

	started := time.Now().UTC()
	metrics := bench.RunMetrics{
		RunID:       cfg.RunID,
		Approach:    cfg.Approach,
		Tier:        cfg.Tier,
		KvrocksAddr: cfg.KvrocksAddr,
		Seed:        cfg.Seed,
		StartedAt:   started,
		Errors:      map[string]int{"ingest": 0, "cutover": 0, "cleanup": 0},
		Counts:      map[string]int{"records_written": 0, "records_deleted": 0},
		Metadata: map[string]string{
			"strategy":              "SETEX+Version",
			"prefix":                cfg.Prefix,
			"data_ttl_seconds":      fmt.Sprintf("%d", defaultDataTTLSeconds),
			"ttl_overlap_observed":  "true",
			"local_metrics_enabled": "true",
			"otel_enabled":          strconv.FormatBool(cfg.OTELEnabled),
		},
	}

	records, err := bench.BuildDataset(cfg.Tier, cfg.Seed)
	if err != nil {
		return metrics, err
	}
	manifest, err := bench.WriteReproducibilityArtifacts(cfg.OutDir, cfg, records)
	if err != nil {
		return metrics, err
	}
	metrics.Metadata["dataset_hash"] = manifest.DatasetHash
	metrics.Metadata["manifest_file"] = "manifest.json"
	metrics.Metadata["config_file"] = "config.json"

	store := bench.NewKvrocks(cfg.KvrocksAddr)
	defer store.Close()
	if err := store.Ping(ctx); err != nil {
		return metrics, err
	}

	version := cfg.RunID
	versionedPrefix := cfg.Prefix + version + ":"
	dataTTL := time.Duration(defaultDataTTLSeconds) * time.Second
	ingestStart := time.Now()
	for _, rec := range records {
		fullKey := versionedPrefix + rec.Key
		if err := store.SetEX(ctx, fullKey, rec.Value, dataTTL); err != nil {
			metrics.Errors["ingest"]++
			continue
		}
		metrics.Counts["records_written"]++
	}
	metrics.IngestMS = time.Since(ingestStart).Milliseconds()

	validationStart := time.Now()
	validation, err := bench.ValidateCorrectness(ctx, store, records, func(rec bench.Record) string {
		return versionedPrefix + rec.Key
	}, versionedPrefix)
	if err != nil {
		metrics.Errors["cutover"]++
	}
	metrics.Metadata["validation_record_count_expected"] = strconv.Itoa(validation.ExpectedCount)
	metrics.Metadata["validation_record_count_actual"] = strconv.Itoa(validation.ActualCount)
	metrics.Metadata["validation_sample_size"] = strconv.Itoa(validation.SampleSize)
	metrics.Metadata["validation_sample_mismatches"] = strconv.Itoa(validation.SampleMismatches)
	metrics.Metadata["validation_passed"] = strconv.FormatBool(validation.Passed)
	metrics.Metadata["validation_ms"] = strconv.FormatInt(time.Since(validationStart).Milliseconds(), 10)
	if len(validation.FailureReasons) > 0 {
		metrics.Metadata["validation_failure_reasons"] = strings.Join(validation.FailureReasons, ",")
	}
	if err != nil || !validation.Passed {
		metrics.CompletedAt = time.Now().UTC()
		metrics.DurationMS = metrics.CompletedAt.Sub(started).Milliseconds()
		if err := bench.ExportOTELIfEnabled(cfg.OutDir, cfg, metrics); err != nil {
			metrics.Metadata["otel_export_status"] = "failed"
		} else if cfg.OTELEnabled {
			metrics.Metadata["otel_export_status"] = "exported"
		} else {
			metrics.Metadata["otel_export_status"] = "disabled"
		}
		if _, werr := bench.WriteArtifacts(cfg.OutDir, metrics); werr != nil {
			return metrics, fmt.Errorf("write artifacts: %w", werr)
		}
		if err != nil {
			return metrics, fmt.Errorf("validation gate error: %w", err)
		}
		return metrics, fmt.Errorf("validation gate failed: %s", metrics.Metadata["validation_failure_reasons"])
	}

	cutoverStart := time.Now()
	pointerKey := cfg.Prefix + "active_version"
	if err := store.Set(ctx, pointerKey, version); err != nil {
		metrics.Errors["cutover"]++
	}
	metrics.CutoverMS = time.Since(cutoverStart).Milliseconds()

	cleanupStart := time.Now()
	metrics.CleanupMS = time.Since(cleanupStart).Milliseconds()

	metrics.CompletedAt = time.Now().UTC()
	metrics.DurationMS = metrics.CompletedAt.Sub(started).Milliseconds()
	if err := bench.ExportOTELIfEnabled(cfg.OutDir, cfg, metrics); err != nil {
		metrics.Metadata["otel_export_status"] = "failed"
	} else if cfg.OTELEnabled {
		metrics.Metadata["otel_export_status"] = "exported"
	} else {
		metrics.Metadata["otel_export_status"] = "disabled"
	}

	if _, err := bench.WriteArtifacts(cfg.OutDir, metrics); err != nil {
		return metrics, fmt.Errorf("write artifacts: %w", err)
	}
	return metrics, nil
}

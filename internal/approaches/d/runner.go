package d

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/prasit-sri/new-cda-plan/internal/bench"
)

const (
	defaultCleanupBatchSize = 200
	defaultCleanupPauseMS   = 20
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
			"strategy":               "VersionPrefix+ThrottledUNLINK",
			"prefix":                 cfg.Prefix,
			"cleanup_batch_size":     strconv.Itoa(defaultCleanupBatchSize),
			"cleanup_pause_ms":       strconv.Itoa(defaultCleanupPauseMS),
			"cleanup_backoff_events": "0",
		},
	}

	records, err := bench.BuildDataset(cfg.Tier, cfg.Seed)
	if err != nil {
		return metrics, err
	}

	store := bench.NewKvrocks(cfg.KvrocksAddr)
	defer store.Close()
	if err := store.Ping(ctx); err != nil {
		return metrics, err
	}

	activeVersion := ""
	currentVersion, err := store.Get(ctx, cfg.Prefix+"active_version")
	if err != nil {
		metrics.Errors["cutover"]++
	} else {
		activeVersion = currentVersion
	}
	metrics.Metadata["active_version_before"] = activeVersion

	newVersion := cfg.RunID
	newPrefix := cfg.Prefix + newVersion + ":"

	ingestStart := time.Now()
	for _, rec := range records {
		fullKey := newPrefix + rec.Key
		if err := store.Set(ctx, fullKey, rec.Value); err != nil {
			metrics.Errors["ingest"]++
			continue
		}
		metrics.Counts["records_written"]++
	}
	metrics.IngestMS = time.Since(ingestStart).Milliseconds()

	validationStart := time.Now()
	validation, err := bench.ValidateCorrectness(ctx, store, records, func(rec bench.Record) string {
		return newPrefix + rec.Key
	}, newPrefix)
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
		if _, werr := bench.WriteArtifacts(cfg.OutDir, metrics); werr != nil {
			return metrics, fmt.Errorf("write artifacts: %w", werr)
		}
		if err != nil {
			return metrics, fmt.Errorf("validation gate error: %w", err)
		}
		return metrics, fmt.Errorf("validation gate failed: %s", metrics.Metadata["validation_failure_reasons"])
	}

	cutoverStart := time.Now()
	if err := store.Set(ctx, cfg.Prefix+"active_version", newVersion); err != nil {
		metrics.Errors["cutover"]++
	}
	metrics.CutoverMS = time.Since(cutoverStart).Milliseconds()

	cleanupStart := time.Now()
	if activeVersion != "" && activeVersion != newVersion {
		oldPrefix := cfg.Prefix + activeVersion + ":"
		scanned, err := store.ScanPrefix(ctx, oldPrefix)
		if err != nil {
			metrics.Errors["cleanup"]++
		} else {
			cleanupInterrupted := false
			for i := 0; i < len(scanned); i += defaultCleanupBatchSize {
				end := i + defaultCleanupBatchSize
				if end > len(scanned) {
					end = len(scanned)
				}
				if err := store.Unlink(ctx, scanned[i:end]...); err != nil {
					metrics.Errors["cleanup"]++
					continue
				}
				metrics.Counts["records_deleted"] += end - i
				if end < len(scanned) {
					metrics.Metadata["cleanup_backoff_events"] = strconv.Itoa(atoiSafe(metrics.Metadata["cleanup_backoff_events"]) + 1)
					select {
					case <-ctx.Done():
						metrics.Errors["cleanup"]++
						cleanupInterrupted = true
					case <-time.After(time.Duration(defaultCleanupPauseMS) * time.Millisecond):
					}
					if cleanupInterrupted {
						break
					}
				}
			}
		}
	}
	metrics.CleanupMS = time.Since(cleanupStart).Milliseconds()

	metrics.Metadata["active_version_after"] = newVersion
	metrics.Metadata["cleanup_batches"] = strconv.Itoa(cleanupBatches(metrics.Counts["records_deleted"], defaultCleanupBatchSize))
	metrics.Metadata["cleanup_deleted"] = strconv.Itoa(metrics.Counts["records_deleted"])

	metrics.CompletedAt = time.Now().UTC()
	metrics.DurationMS = metrics.CompletedAt.Sub(started).Milliseconds()

	if _, err := bench.WriteArtifacts(cfg.OutDir, metrics); err != nil {
		return metrics, fmt.Errorf("write artifacts: %w", err)
	}
	return metrics, nil
}

func cleanupBatches(deleted, batchSize int) int {
	if deleted <= 0 || batchSize <= 0 {
		return 0
	}
	batches := deleted / batchSize
	if deleted%batchSize != 0 {
		batches++
	}
	return batches
}

func atoiSafe(v string) int {
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0
	}
	return n
}

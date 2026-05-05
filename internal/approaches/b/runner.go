package b

import (
	"context"
	"fmt"
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
			"strategy":             "SETEX+Version",
			"prefix":               cfg.Prefix,
			"data_ttl_seconds":     fmt.Sprintf("%d", defaultDataTTLSeconds),
			"ttl_overlap_observed": "true",
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

	if _, err := bench.WriteArtifacts(cfg.OutDir, metrics); err != nil {
		return metrics, fmt.Errorf("write artifacts: %w", err)
	}
	return metrics, nil
}

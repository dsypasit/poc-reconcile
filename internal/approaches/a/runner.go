package a

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/prasit-sri/new-cda-plan/internal/bench"
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
		Metadata:    map[string]string{"strategy": "SCAN+UNLINK", "prefix": cfg.Prefix},
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

	newKeys := make(map[string]struct{}, len(records))
	ingestStart := time.Now()
	for _, rec := range records {
		fullKey := cfg.Prefix + rec.Key
		if err := store.Set(ctx, fullKey, rec.Value); err != nil {
			metrics.Errors["ingest"]++
			continue
		}
		metrics.Counts["records_written"]++
		newKeys[fullKey] = struct{}{}
	}
	metrics.IngestMS = time.Since(ingestStart).Milliseconds()

	cutoverStart := time.Now()
	if err := store.Set(ctx, cfg.Prefix+"_cutover_marker", cfg.RunID); err != nil {
		metrics.Errors["cutover"]++
	}
	metrics.CutoverMS = time.Since(cutoverStart).Milliseconds()

	cleanupStart := time.Now()
	scanned, err := store.ScanPrefix(ctx, cfg.Prefix)
	if err != nil {
		metrics.Errors["cleanup"]++
	} else {
		for _, key := range scanned {
			if strings.HasSuffix(key, "_cutover_marker") {
				continue
			}
			if _, ok := newKeys[key]; ok {
				continue
			}
			if err := store.Unlink(ctx, key); err != nil {
				metrics.Errors["cleanup"]++
				continue
			}
			metrics.Counts["records_deleted"]++
		}
	}
	metrics.CleanupMS = time.Since(cleanupStart).Milliseconds()

	metrics.CompletedAt = time.Now().UTC()
	metrics.DurationMS = metrics.CompletedAt.Sub(started).Milliseconds()

	if _, err := bench.WriteArtifacts(cfg.OutDir, metrics); err != nil {
		return metrics, fmt.Errorf("write artifacts: %w", err)
	}
	return metrics, nil
}

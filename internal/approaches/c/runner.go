package c

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/prasit-sri/new-cda-plan/internal/bench"
)

const (
	activeSlotKeySuffix = "active_slot"
	slotBlue            = "blue"
	slotGreen           = "green"
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
			"strategy": "BlueGreenPrefixSwap",
			"prefix":   cfg.Prefix,
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

	activeSlot := slotBlue
	currentSlot, err := store.Get(ctx, cfg.Prefix+activeSlotKeySuffix)
	if err != nil {
		metrics.Errors["cutover"]++
	} else if currentSlot == slotBlue || currentSlot == slotGreen {
		activeSlot = currentSlot
	}

	inactiveSlot := slotGreen
	if activeSlot == slotGreen {
		inactiveSlot = slotBlue
	}
	metrics.Metadata["active_slot_before"] = activeSlot
	metrics.Metadata["inactive_slot_loaded"] = inactiveSlot

	newPrefix := cfg.Prefix + inactiveSlot + ":"
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

	cutoverStart := time.Now()
	if err := store.Set(ctx, cfg.Prefix+activeSlotKeySuffix, inactiveSlot); err != nil {
		metrics.Errors["cutover"]++
	}
	if err := store.Set(ctx, cfg.Prefix+"validation_handoff", cfg.RunID); err != nil {
		metrics.Errors["cutover"]++
	}
	metrics.CutoverMS = time.Since(cutoverStart).Milliseconds()

	oldPrefix := cfg.Prefix + activeSlot + ":"
	cleanupStart := time.Now()
	scanned, err := store.ScanPrefix(ctx, oldPrefix)
	if err != nil {
		metrics.Errors["cleanup"]++
	} else {
		for _, key := range scanned {
			if err := store.Unlink(ctx, key); err != nil {
				metrics.Errors["cleanup"]++
				continue
			}
			metrics.Counts["records_deleted"]++
		}
	}
	metrics.CleanupMS = time.Since(cleanupStart).Milliseconds()
	metrics.Metadata["old_slot_cleaned"] = activeSlot
	metrics.Metadata["old_slot_cleanup_deleted"] = strconv.Itoa(metrics.Counts["records_deleted"])

	metrics.CompletedAt = time.Now().UTC()
	metrics.DurationMS = metrics.CompletedAt.Sub(started).Milliseconds()

	if _, err := bench.WriteArtifacts(cfg.OutDir, metrics); err != nil {
		return metrics, fmt.Errorf("write artifacts: %w", err)
	}
	return metrics, nil
}

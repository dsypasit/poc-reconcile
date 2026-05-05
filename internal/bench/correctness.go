package bench

import (
	"context"
	"fmt"
	"strings"
)

type CorrectnessResult struct {
	ExpectedCount    int
	ActualCount      int
	SampleSize       int
	SampleMismatches int
	Passed           bool
	FailureReasons   []string
}

func ValidateCorrectness(ctx context.Context, store *Kvrocks, records []Record, keyForRecord func(Record) string, scanPrefix string) (CorrectnessResult, error) {
	expectedCount := len(records)
	scanned, err := store.ScanPrefix(ctx, scanPrefix)
	if err != nil {
		return CorrectnessResult{}, fmt.Errorf("scan for parity: %w", err)
	}
	actualCount := len(scanned)

	samples := stratifiedSample(records, sampleTargetSize(expectedCount), 3)
	mismatches := 0
	for _, rec := range samples {
		got, err := store.Get(ctx, keyForRecord(rec))
		if err != nil || got != rec.Value {
			mismatches++
		}
	}

	result := CorrectnessResult{
		ExpectedCount:    expectedCount,
		ActualCount:      actualCount,
		SampleSize:       len(samples),
		SampleMismatches: mismatches,
		Passed:           expectedCount == actualCount && mismatches == 0,
	}
	if expectedCount != actualCount {
		result.FailureReasons = append(result.FailureReasons, "record_count_parity_failed")
	}
	if mismatches != 0 {
		result.FailureReasons = append(result.FailureReasons, "stratified_sampling_failed")
	}
	return result, nil
}

func sampleTargetSize(total int) int {
	if total <= 0 {
		return 0
	}
	target := total / 10
	if target < 30 {
		target = 30
	}
	if target > 200 {
		target = 200
	}
	if target > total {
		target = total
	}
	return target
}

func stratifiedSample(records []Record, targetSize int, perDomainFloor int) []Record {
	if targetSize <= 0 || len(records) == 0 {
		return nil
	}
	if targetSize > len(records) {
		targetSize = len(records)
	}

	domainBuckets := make(map[string][]Record)
	domainOrder := make([]string, 0, 8)
	for _, rec := range records {
		domain := keyDomain(rec.Key)
		if _, ok := domainBuckets[domain]; !ok {
			domainOrder = append(domainOrder, domain)
		}
		domainBuckets[domain] = append(domainBuckets[domain], rec)
	}

	selected := make([]Record, 0, targetSize)
	consumed := make(map[string]int, len(domainBuckets))

	for _, domain := range domainOrder {
		bucket := domainBuckets[domain]
		take := perDomainFloor
		if take > len(bucket) {
			take = len(bucket)
		}
		for i := 0; i < take && len(selected) < targetSize; i++ {
			selected = append(selected, bucket[i])
			consumed[domain]++
		}
	}

	for len(selected) < targetSize {
		progress := false
		for _, domain := range domainOrder {
			idx := consumed[domain]
			bucket := domainBuckets[domain]
			if idx >= len(bucket) {
				continue
			}
			selected = append(selected, bucket[idx])
			consumed[domain]++
			progress = true
			if len(selected) >= targetSize {
				break
			}
		}
		if !progress {
			break
		}
	}

	return selected
}

func keyDomain(key string) string {
	parts := strings.SplitN(key, ":", 2)
	if len(parts) == 0 || parts[0] == "" {
		return "unknown"
	}
	return parts[0]
}

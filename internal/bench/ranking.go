package bench

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
)

func BuildCanonicalRankableRuns(baseDir string, runs []RunMetrics) ([]RunMetrics, []string) {
	rankable := make([]RunMetrics, 0, len(runs))
	qualityFailures := make([]string, 0)

	for i := range runs {
		m := runs[i]
		if m.Metadata["rank_eligible"] != "true" {
			continue
		}
		env, err := readOTelEnvelope(baseDir, m.RunID)
		if err != nil {
			m.Metadata["rank_eligible"] = "false"
			m.Metadata["consumer_validity_passed"] = "false"
			m.Metadata["consumer_rejection_category"] = ConsumerCategoryOTELExportNotSuccessful
			m.Metadata["consumer_rejection_reasons"] = "otel_artifact_unreadable"
			qualityFailures = append(qualityFailures, fmt.Sprintf("run_id=%s quality_failure=otel_artifact_unreadable error=%v", m.RunID, err))
			runs[i] = m
			continue
		}

		canonicalDuration := env.Values["duration_ms"]
		canonicalOverall := deriveRPSInt64(env.Values["records_written"], canonicalDuration)
		localOverall := parseFloat(m.Metadata["overall_throughput_rps"])
		if m.DurationMS != canonicalDuration || !floatApproxEqual(localOverall, canonicalOverall, 0.01) {
			qualityFailures = append(qualityFailures, fmt.Sprintf("run_id=%s quality_failure=otel_local_mismatch local_duration=%d canonical_duration=%d local_overall_rps=%.2f canonical_overall_rps=%.2f", m.RunID, m.DurationMS, canonicalDuration, localOverall, canonicalOverall))
			m.Metadata["quality_failure"] = "otel_local_mismatch"
		}

		m.DurationMS = canonicalDuration
		m.Metadata["overall_throughput_rps"] = fmt.Sprintf("%.2f", canonicalOverall)
		m.Metadata["canonical_source"] = "otel"
		rankable = append(rankable, m)
		runs[i] = m
	}

	sort.Slice(rankable, func(i, j int) bool {
		a := rankable[i]
		b := rankable[j]
		if a.Tier != b.Tier {
			return a.Tier < b.Tier
		}
		if a.DurationMS != b.DurationMS {
			return a.DurationMS < b.DurationMS
		}
		aOverall := parseFloat(a.Metadata["overall_throughput_rps"])
		bOverall := parseFloat(b.Metadata["overall_throughput_rps"])
		if aOverall != bOverall {
			return aOverall > bOverall
		}
		if a.Approach != b.Approach {
			return a.Approach < b.Approach
		}
		return a.RunID < b.RunID
	})

	return rankable, qualityFailures
}

func readOTelEnvelope(baseDir, runID string) (OTelExportEnvelope, error) {
	var env OTelExportEnvelope
	path := filepath.Join(baseDir, runID, "otel-metrics.json")
	b, err := os.ReadFile(path)
	if err != nil {
		return env, err
	}
	if err := json.Unmarshal(b, &env); err != nil {
		return env, err
	}
	return env, nil
}

func deriveRPSInt64(recordsWritten, durationMS int64) float64 {
	if recordsWritten <= 0 || durationMS <= 0 {
		return 0
	}
	return (float64(recordsWritten) * 1000.0) / float64(durationMS)
}

func parseFloat(v string) float64 {
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0
	}
	return f
}

func floatApproxEqual(a, b, eps float64) bool {
	if a > b {
		return a-b <= eps
	}
	return b-a <= eps
}

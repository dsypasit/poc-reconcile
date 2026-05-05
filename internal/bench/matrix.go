package bench

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type MatrixSummary struct {
	MatrixID    string       `json:"matrix_id"`
	GeneratedAt time.Time    `json:"generated_at"`
	Runs        []RunMetrics `json:"runs"`
	Failures    []string     `json:"failures"`
}

func WriteMatrixArtifacts(baseDir, matrixID string, runs []RunMetrics, failures []string) (string, error) {
	outDir := filepath.Join(baseDir, matrixID)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", fmt.Errorf("create matrix dir: %w", err)
	}

	for i := range runs {
		RevalidateRunForConsumer(&runs[i])
	}
	rankedRuns, qualityFailures := BuildCanonicalRankableRuns(baseDir, runs)
	failures = append(failures, qualityFailures...)

	summary := MatrixSummary{
		MatrixID:    matrixID,
		GeneratedAt: time.Now().UTC(),
		Runs:        runs,
		Failures:    failures,
	}
	jsonPayload, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal matrix json: %w", err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "matrix.json"), jsonPayload, 0o644); err != nil {
		return "", fmt.Errorf("write matrix json: %w", err)
	}

	if err := writeMatrixCSV(filepath.Join(outDir, "matrix.csv"), runs); err != nil {
		return "", err
	}
	if err := writeMatrixReport(filepath.Join(outDir, "report.md"), summary, rankedRuns); err != nil {
		return "", err
	}

	return outDir, nil
}

func writeMatrixCSV(path string, runs []RunMetrics) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create matrix csv: %w", err)
	}
	defer f.Close()
	w := csv.NewWriter(f)
	headers := []string{"run_id", "approach", "tier", "duration_ms", "ingest_ms", "cutover_ms", "cleanup_ms", "ingest_errors", "cutover_errors", "cleanup_errors", "records_written", "records_deleted", "ingest_throughput_rps", "overall_throughput_rps", "validation_passed", "producer_validity_passed", "producer_validity_reasons", "rank_eligible"}
	if err := w.Write(headers); err != nil {
		return fmt.Errorf("write matrix csv header: %w", err)
	}
	for _, m := range runs {
		row := []string{
			m.RunID,
			m.Approach,
			m.Tier,
			strconv.FormatInt(m.DurationMS, 10),
			strconv.FormatInt(m.IngestMS, 10),
			strconv.FormatInt(m.CutoverMS, 10),
			strconv.FormatInt(m.CleanupMS, 10),
			strconv.Itoa(m.Errors["ingest"]),
			strconv.Itoa(m.Errors["cutover"]),
			strconv.Itoa(m.Errors["cleanup"]),
			strconv.Itoa(m.Counts["records_written"]),
			strconv.Itoa(m.Counts["records_deleted"]),
			m.Metadata["ingest_throughput_rps"],
			m.Metadata["overall_throughput_rps"],
			m.Metadata["validation_passed"],
			m.Metadata["producer_validity_passed"],
			m.Metadata["producer_validity_reasons"],
			m.Metadata["rank_eligible"],
		}
		if err := w.Write(row); err != nil {
			return fmt.Errorf("write matrix csv row: %w", err)
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return fmt.Errorf("flush matrix csv: %w", err)
	}
	return nil
}

func writeMatrixReport(path string, summary MatrixSummary, rankedRuns []RunMetrics) error {
	var b strings.Builder
	b.WriteString("# Benchmark Comparison Report\n\n")
	b.WriteString(fmt.Sprintf("- Matrix ID: `%s`\n", summary.MatrixID))
	b.WriteString(fmt.Sprintf("- Generated at: `%s`\n", summary.GeneratedAt.Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("- Runs: `%d`\n", len(summary.Runs)))
	b.WriteString(fmt.Sprintf("- Failures: `%d`\n\n", len(summary.Failures)))

	if len(summary.Failures) > 0 {
		b.WriteString("## Failures\n")
		for _, f := range summary.Failures {
			b.WriteString(fmt.Sprintf("- %s\n", f))
		}
		b.WriteString("\n")
	}

	b.WriteString("## Ranking by Duration\n")
	rank := 0
	for _, m := range rankedRuns {
		rank++
		b.WriteString(fmt.Sprintf("%d. `%s %s` duration=%dms overall_rps=%s validation=%s seed=%d\n",
			rank,
			m.Approach,
			m.Tier,
			m.DurationMS,
			m.Metadata["overall_throughput_rps"],
			m.Metadata["validation_passed"],
			m.Seed,
		))
	}

	return os.WriteFile(path, []byte(b.String()), 0o644)
}

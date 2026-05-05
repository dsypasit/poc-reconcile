package bench

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

func WriteArtifacts(baseDir string, m RunMetrics) (string, error) {
	runDir := filepath.Join(baseDir, m.RunID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return "", fmt.Errorf("create run dir: %w", err)
	}

	jsonPath := filepath.Join(runDir, "metrics.json")
	payload, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal json: %w", err)
	}
	if err := os.WriteFile(jsonPath, payload, 0o644); err != nil {
		return "", fmt.Errorf("write json: %w", err)
	}

	csvPath := filepath.Join(runDir, "metrics.csv")
	csvFile, err := os.Create(csvPath)
	if err != nil {
		return "", fmt.Errorf("create csv: %w", err)
	}
	defer csvFile.Close()

	w := csv.NewWriter(csvFile)
	headers := []string{"run_id", "approach", "tier", "kvrocks_addr", "seed", "ingest_ms", "cutover_ms", "cleanup_ms", "duration_ms", "ingest_errors", "cutover_errors", "cleanup_errors", "records_written", "records_deleted"}
	if err := w.Write(headers); err != nil {
		return "", fmt.Errorf("write csv header: %w", err)
	}
	row := []string{
		m.RunID,
		m.Approach,
		m.Tier,
		m.KvrocksAddr,
		strconv.FormatInt(m.Seed, 10),
		strconv.FormatInt(m.IngestMS, 10),
		strconv.FormatInt(m.CutoverMS, 10),
		strconv.FormatInt(m.CleanupMS, 10),
		strconv.FormatInt(m.DurationMS, 10),
		strconv.Itoa(m.Errors["ingest"]),
		strconv.Itoa(m.Errors["cutover"]),
		strconv.Itoa(m.Errors["cleanup"]),
		strconv.Itoa(m.Counts["records_written"]),
		strconv.Itoa(m.Counts["records_deleted"]),
	}
	if err := w.Write(row); err != nil {
		return "", fmt.Errorf("write csv row: %w", err)
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return "", fmt.Errorf("flush csv: %w", err)
	}
	return runDir, nil
}

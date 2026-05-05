package bench

import "fmt"

func PopulateDerivedMetrics(m *RunMetrics) {
	if m.Metadata == nil {
		m.Metadata = map[string]string{}
	}
	m.Metadata["records_written"] = fmt.Sprintf("%d", m.Counts["records_written"])
	m.Metadata["records_deleted"] = fmt.Sprintf("%d", m.Counts["records_deleted"])
	m.Metadata["ingest_throughput_rps"] = formatRPS(m.Counts["records_written"], m.IngestMS)
	m.Metadata["overall_throughput_rps"] = formatRPS(m.Counts["records_written"], m.DurationMS)
}

func formatRPS(count int, durationMS int64) string {
	if durationMS <= 0 {
		return "0"
	}
	rps := float64(count) * 1000.0 / float64(durationMS)
	return fmt.Sprintf("%.2f", rps)
}

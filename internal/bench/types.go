package bench

import "time"

type Record struct {
	Key   string
	Value string
}

type RunMetrics struct {
	RunID       string            `json:"run_id"`
	Approach    string            `json:"approach"`
	Tier        string            `json:"tier"`
	KvrocksAddr string            `json:"kvrocks_addr"`
	Seed        int64             `json:"seed"`
	StartedAt   time.Time         `json:"started_at"`
	CompletedAt time.Time         `json:"completed_at"`
	DurationMS  int64             `json:"duration_ms"`
	IngestMS    int64             `json:"ingest_ms"`
	CutoverMS   int64             `json:"cutover_ms"`
	CleanupMS   int64             `json:"cleanup_ms"`
	Errors      map[string]int    `json:"errors"`
	Counts      map[string]int    `json:"counts"`
	Metadata    map[string]string `json:"metadata"`
}

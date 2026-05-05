package bench

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ConfigSnapshot struct {
	Approach    string `json:"approach"`
	Tier        string `json:"tier"`
	Seed        int64  `json:"seed"`
	KvrocksAddr string `json:"kvrocks_addr"`
	Prefix      string `json:"prefix"`
	RunID       string `json:"run_id"`
}

type ManifestSnapshot struct {
	RunID          string         `json:"run_id"`
	Approach       string         `json:"approach"`
	Tier           string         `json:"tier"`
	Seed           int64          `json:"seed"`
	RecordCount    int            `json:"record_count"`
	DatasetHash    string         `json:"dataset_hash"`
	DomainCounts   map[string]int `json:"domain_counts"`
	ConfigFile     string         `json:"config_file"`
	GeneratedAtUTC time.Time      `json:"generated_at_utc"`
}

func WriteReproducibilityArtifacts(baseDir string, cfg Config, records []Record) (ManifestSnapshot, error) {
	runDir := filepath.Join(baseDir, cfg.RunID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return ManifestSnapshot{}, fmt.Errorf("create run dir: %w", err)
	}

	config := ConfigSnapshot{
		Approach:    cfg.Approach,
		Tier:        cfg.Tier,
		Seed:        cfg.Seed,
		KvrocksAddr: cfg.KvrocksAddr,
		Prefix:      cfg.Prefix,
		RunID:       cfg.RunID,
	}
	configPath := filepath.Join(runDir, "config.json")
	configBytes, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return ManifestSnapshot{}, fmt.Errorf("marshal config snapshot: %w", err)
	}
	if err := os.WriteFile(configPath, configBytes, 0o644); err != nil {
		return ManifestSnapshot{}, fmt.Errorf("write config snapshot: %w", err)
	}

	domainCounts := make(map[string]int)
	for _, rec := range records {
		domainCounts[keyDomain(rec.Key)]++
	}

	manifest := ManifestSnapshot{
		RunID:          cfg.RunID,
		Approach:       cfg.Approach,
		Tier:           cfg.Tier,
		Seed:           cfg.Seed,
		RecordCount:    len(records),
		DatasetHash:    datasetHash(records),
		DomainCounts:   domainCounts,
		ConfigFile:     "config.json",
		GeneratedAtUTC: time.Now().UTC(),
	}
	manifestPath := filepath.Join(runDir, "manifest.json")
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return ManifestSnapshot{}, fmt.Errorf("marshal manifest snapshot: %w", err)
	}
	if err := os.WriteFile(manifestPath, manifestBytes, 0o644); err != nil {
		return ManifestSnapshot{}, fmt.Errorf("write manifest snapshot: %w", err)
	}

	return manifest, nil
}

func datasetHash(records []Record) string {
	h := sha256.New()
	for _, rec := range records {
		h.Write([]byte(rec.Key))
		h.Write([]byte{'\t'})
		h.Write([]byte(rec.Value))
		h.Write([]byte{'\n'})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func formatDomainCounts(domainCounts map[string]int) string {
	parts := make([]string, 0, len(domainCounts))
	for _, domain := range []string{"alpha", "beta", "gamma", "unknown"} {
		if count, ok := domainCounts[domain]; ok {
			parts = append(parts, fmt.Sprintf("%s=%d", domain, count))
		}
	}
	return strings.Join(parts, ",")
}

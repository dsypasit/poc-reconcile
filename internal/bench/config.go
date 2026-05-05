package bench

import (
	"fmt"
	"time"
)

type Config struct {
	Approach     string
	Tier         string
	Seed         int64
	KvrocksAddr  string
	Prefix       string
	RunID        string
	OutDir       string
	OTELEnabled  bool
	OTELEndpoint string
}

func (c Config) Validate() error {
	if c.Approach == "" {
		return fmt.Errorf("approach is required")
	}
	if c.Tier == "" {
		return fmt.Errorf("tier is required")
	}
	if c.KvrocksAddr == "" {
		return fmt.Errorf("kvrocks_addr is required")
	}
	if c.Prefix == "" {
		return fmt.Errorf("prefix is required")
	}
	if c.RunID == "" {
		return fmt.Errorf("run_id is required")
	}
	if c.OutDir == "" {
		return fmt.Errorf("out_dir is required")
	}
	return nil
}

func NewRunID(now time.Time) string {
	return now.UTC().Format("20060102T150405Z")
}

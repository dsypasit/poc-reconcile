package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/prasit-sri/new-cda-plan/internal/approaches/d"
	"github.com/prasit-sri/new-cda-plan/internal/bench"
)

func main() {
	var (
		tier         = flag.String("tier", "small", "dataset tier: small|medium")
		seed         = flag.Int64("seed", 42, "dataset seed")
		kvAddr       = flag.String("kvrocks-addr", "127.0.0.1:6666", "kvrocks address")
		prefix       = flag.String("prefix", "bench:d:", "key prefix")
		artifacts    = flag.String("artifacts-dir", "artifacts", "artifacts base directory")
		otelEnabled  = flag.Bool("otel-enabled", false, "enable optional OTEL export")
		otelEndpoint = flag.String("otel-endpoint", "local-artifact", "OTEL exporter endpoint label")
		timeoutSecs  = flag.Int("timeout-seconds", 120, "run timeout seconds")
	)
	flag.Parse()

	runID := bench.NewRunID(time.Now())
	cfg := bench.Config{
		Approach:     "approach-d",
		Tier:         *tier,
		Seed:         *seed,
		KvrocksAddr:  *kvAddr,
		Prefix:       *prefix,
		RunID:        runID,
		OutDir:       *artifacts,
		OTELEnabled:  *otelEnabled,
		OTELEndpoint: *otelEndpoint,
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*timeoutSecs)*time.Second)
	defer cancel()

	metrics, err := d.Run(ctx, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "run failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("run_id=%s approach=%s tier=%s ingest_ms=%d cutover_ms=%d cleanup_ms=%d duration_ms=%d errors=%v metadata=%v\n",
		metrics.RunID,
		metrics.Approach,
		metrics.Tier,
		metrics.IngestMS,
		metrics.CutoverMS,
		metrics.CleanupMS,
		metrics.DurationMS,
		metrics.Errors,
		metrics.Metadata,
	)
}

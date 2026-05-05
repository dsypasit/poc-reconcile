package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/prasit-sri/new-cda-plan/internal/approaches/a"
	"github.com/prasit-sri/new-cda-plan/internal/approaches/b"
	"github.com/prasit-sri/new-cda-plan/internal/approaches/c"
	"github.com/prasit-sri/new-cda-plan/internal/approaches/d"
	"github.com/prasit-sri/new-cda-plan/internal/bench"
)

func main() {
	var (
		kvAddr      = flag.String("kvrocks-addr", "127.0.0.1:6666", "kvrocks address")
		artifacts   = flag.String("artifacts-dir", "artifacts", "artifacts base directory")
		seed        = flag.Int64("seed", 42, "dataset seed")
		timeoutSecs = flag.Int("timeout-seconds", 300, "per-run timeout seconds")
	)
	flag.Parse()

	matrixID := "matrix-" + bench.NewRunID(time.Now())
	runs := make([]bench.RunMetrics, 0, 8)
	failures := make([]string, 0, 8)

	type scenario struct {
		approach string
		tier     string
		prefix   string
		run      func(context.Context, bench.Config) (bench.RunMetrics, error)
	}
	scenarios := []scenario{
		{approach: "approach-a", tier: "small", prefix: "bench:a:matrix:", run: a.Run},
		{approach: "approach-b", tier: "small", prefix: "bench:b:matrix:", run: b.Run},
		{approach: "approach-c", tier: "small", prefix: "bench:c:matrix:", run: c.Run},
		{approach: "approach-d", tier: "small", prefix: "bench:d:matrix:", run: d.Run},
		{approach: "approach-a", tier: "medium", prefix: "bench:a:matrix:", run: a.Run},
		{approach: "approach-b", tier: "medium", prefix: "bench:b:matrix:", run: b.Run},
		{approach: "approach-c", tier: "medium", prefix: "bench:c:matrix:", run: c.Run},
		{approach: "approach-d", tier: "medium", prefix: "bench:d:matrix:", run: d.Run},
	}

	for i, s := range scenarios {
		runID := fmt.Sprintf("%s-%02d", bench.NewRunID(time.Now()), i+1)
		cfg := bench.Config{
			Approach:    s.approach,
			Tier:        s.tier,
			Seed:        *seed,
			KvrocksAddr: *kvAddr,
			Prefix:      fmt.Sprintf("%s%s:%s:", s.prefix, matrixID, s.tier),
			RunID:       runID,
			OutDir:      *artifacts,
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*timeoutSecs)*time.Second)
		metrics, err := s.run(ctx, cfg)
		cancel()
		runs = append(runs, metrics)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s %s run_id=%s error=%v", s.approach, s.tier, runID, err))
		}
		fmt.Printf("run_id=%s approach=%s tier=%s duration_ms=%d validation=%s\n",
			metrics.RunID, metrics.Approach, metrics.Tier, metrics.DurationMS, metrics.Metadata["validation_passed"])
	}

	outDir, err := bench.WriteMatrixArtifacts(*artifacts, matrixID, runs, failures)
	if err != nil {
		fmt.Fprintf(os.Stderr, "write matrix artifacts failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("matrix_id=%s artifacts=%s failures=%d\n", matrixID, outDir, len(failures))
	if len(failures) > 0 {
		os.Exit(1)
	}
}

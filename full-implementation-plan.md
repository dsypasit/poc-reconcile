# Full Implementation Plan: Kvrocks GCS Ingestion POCs

## Objective
Implement four comparable Go POCs for monthly full-refresh ingestion algorithms and benchmark them under a controlled environment before selecting a final production strategy.

Algorithms to implement:
- Approach A: SCAN + UNLINK
- Approach B: SETEX + Version
- Approach C: Blue/Green Swap
- Approach D: Version Prefix + Throttled UNLINK

## Agreed Constraints and Decisions
- Keep separate project folders from repo root:
  - `poc/approach-a`
  - `poc/approach-b`
  - `poc/approach-c`
  - `poc/approach-d`
- Use shared reusable code in `internal/common` for fair comparison and lower maintenance.
- Single Kvrocks instance baseline in Docker Compose first (cluster mode deferred).
- Use local mock loader for manifest/objects in v1 (real GCS pluggable later).
- Dataset model: mixed payload sizes across key domains (`hot`, `cold`, `sparse`).
- Dataset tiers:
  - `small = 100k`
  - `medium = 1M`
  - `large = 5M`
- Unified correctness gates for all approaches:
  - record-count parity
  - stratified sampling correctness
  - mismatch threshold = `0`
- Rollback handled as separate scenario profile, not baseline.
- Approach B default TTL = `72h`, configurable by `--ttl`.
- Approach A uses phased/paced reconciliation knobs.
- Approach C uses logical slots via prefixes (single Kvrocks).
- Common cutover abstraction:
  - A: reconciliation completion marker
  - B/C/D: atomic pointer flip
- Global default workers = `16` with minimal per-approach extra knobs.
- Run outputs must include structured `JSON + CSV` plus generated markdown summary.
- OTEL optional in v1 via `--otel`; local metrics always on.
- Single `docker-compose.yml` with profiles:
  - `kvrocks`
  - optional `otel`
  - optional `runner`
- Reproducibility required:
  - deterministic seed
  - persisted manifest and run artifacts in `artifacts/<run_id>`
- Completion bar:
  - all 4 binaries run end-to-end on `small` and `medium`
  - correctness gates pass
  - JSON/CSV emitted
  - one comparative markdown report generated from actual outputs

## Repository Structure Plan

```text
.
├── docker-compose.yml
├── cmd/
│   └── bench/
│       └── main.go
├── internal/
│   └── common/
│       ├── config/
│       ├── dataset/
│       ├── kv/
│       ├── validation/
│       ├── metrics/
│       ├── cutover/
│       └── artifacts/
├── poc/
│   ├── approach-a/
│   │   └── main.go
│   ├── approach-b/
│   │   └── main.go
│   ├── approach-c/
│   │   └── main.go
│   └── approach-d/
│       └── main.go
└── artifacts/
```

## Execution Phases

## Phase 1: Scaffolding
Deliverables:
- Go module initialization (if absent) and package layout.
- Empty runnable binaries for all four approaches.
- Shared folder skeleton in `internal/common`.
- `artifacts/` base directory and naming convention.

Acceptance:
- `go run` for each binary prints startup/config and exits cleanly.

## Phase 2: Shared Core Implementation (`internal/common`)
Deliverables:
- Config system (flags + env + defaults) for:
  - dataset tier/size
  - worker count
  - scenario (`baseline`, `rollback`)
  - approach-specific knobs
  - output paths
  - OTEL toggle
- Dataset/manifest mock generator and loader:
  - deterministic seed
  - domain distribution (`hot`, `cold`, `sparse`)
  - variable payload sizes
  - persisted manifest artifact
- Kvrocks client wrapper:
  - connection management
  - pipelined writes
  - scan helpers
  - unlink helpers
- Validation module:
  - record-count parity check
  - stratified sampling with random tail
  - mismatch threshold enforcement
- Metrics module:
  - local runtime metrics always on
  - OTEL exporter wiring optional by flag
- Artifact writer:
  - per-run JSON summary
  - per-run CSV metrics/events
  - standardized run metadata block
- Common interfaces:
  - `IngestionStrategy`
  - `CutoverController`
  - `CleanupController`

Acceptance:
- Unit tests for core deterministic dataset generation and validation logic.
- One smoke command can run shared pipeline components without strategy-specific logic.

## Phase 3: Approach Implementations

### Approach A (`poc/approach-a`)
Implement:
- Ingest/update active namespace.
- Phased SCAN + stale detection + UNLINK.
- Tunables:
  - `scan_count`
  - `unlink_batch`
  - `sleep_ms`
- Completion marker cutover semantics.

Acceptance:
- End-to-end baseline run with artifacts and validation results.

### Approach B (`poc/approach-b`)
Implement:
- Versioned keys with `SETEX`.
- Pointer-based cutover.
- TTL default 72h with override.
- Cleanup modeled as TTL lifecycle observability.

Acceptance:
- End-to-end run with overlap metrics and validation results.

### Approach C (`poc/approach-c`)
Implement:
- Logical blue/green slots using prefixes.
- Validate inactive slot.
- Atomic swap pointer.
- Old slot cleanup routine.

Acceptance:
- End-to-end run with swap timing and cleanup stats.

### Approach D (`poc/approach-d`)
Implement:
- Version-prefixed namespace ingest.
- Validate then atomic pointer flip.
- Async throttled UNLINK cleanup with checkpointing.

Acceptance:
- End-to-end run with cleanup pacing/backoff metrics.

## Phase 4: Docker Compose Environment
Deliverables:
- Single `docker-compose.yml` with profiles:
  - `kvrocks`
  - `otel` (optional)
  - `runner` (optional command container)
- Stable network aliases and default ports.
- Optional volume mounts for artifacts.

Acceptance:
- Kvrocks starts cleanly.
- POC binaries can connect from host and/or runner profile.

## Phase 5: Bench Orchestrator (`cmd/bench`)
Deliverables:
- Matrix executor for:
  - approaches: A/B/C/D
  - tiers: small/medium/large (large optional for first completion bar)
  - scenarios: baseline (+ rollback optional run)
- Deterministic run-id and seed handling.
- Parallel/sequential control (default sequential for fairness).
- Consolidated results generator:
  - aggregate JSON
  - aggregate CSV
  - markdown comparison report

Acceptance:
- One command runs full required matrix for small+medium and produces final report.

## Phase 6: Baseline Benchmark Runs
Required completion runs:
- All four approaches on `small` baseline.
- All four approaches on `medium` baseline.

Optional after completion bar:
- `large` tier
- rollback scenario matrix
- OTEL-enabled reruns

Deliverables:
- Raw artifacts per run under `artifacts/<run_id>`.
- Consolidated comparison markdown generated from real outputs.

## Metrics and Report Fields
Each run should capture at least:
- approach, tier, scenario, seed, run_id
- ingest duration
- validation duration
- cutover duration
- cleanup duration (or cleanup initiation for TTL-driven)
- throughput (keys/sec)
- retry/error counts
- peak/avg latency from client perspective
- keyspace overlap estimate during run
- correctness gate pass/fail and mismatch count

Comparison report should include:
- ranking by duration and throughput
- resource behavior observations
- maintainability/operability notes from implementation complexity
- failure-mode notes observed during runs
- reproducibility details (seed, config snapshot)

## Risk Register and Mitigations
- Risk: fairness drift across approaches.
  - Mitigation: centralize shared logic and identical validation gates.
- Risk: benchmark noise from host load.
  - Mitigation: fixed environment, repeated runs, median reporting.
- Risk: TTL behavior obscures immediate cleanup metrics (Approach B).
  - Mitigation: explicitly separate pipeline-time vs lifecycle-time metrics.
- Risk: SCAN pressure destabilizes single instance.
  - Mitigation: tunable pacing and bounded batch execution.
- Risk: implementation delays from OTEL setup.
  - Mitigation: OTEL optional; local metrics mandatory.

## Definition of Done
Implementation is complete when:
1. Four approach binaries exist and run end-to-end.
2. Shared infrastructure is reused across all approaches.
3. Docker Compose baseline environment is functional.
4. `cmd/bench` executes required matrix.
5. Artifacts are emitted as JSON and CSV for each run.
6. Comparative markdown report is generated from actual run outputs.
7. Required `small` and `medium` runs pass correctness gates.

## Post-Implementation Next Steps
- Add real GCS adapter behind loader interface.
- Add cluster-mode benchmark profile.
- Add repeated-trial statistical summary (p50/p95 across runs).
- Add CI job for smoke benchmark on `small` tier.

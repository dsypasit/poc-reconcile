# Kvrocks benchmark runner

This repository runs benchmark slices for approaches A, B, C, and D against
Kvrocks, and writes run artifacts to `artifacts/`.

## Prerequisites

You need the following tools installed:

- Go 1.24+
- Docker and Docker Compose
- `rtk` (used by existing project commands)

## Start the benchmark environment

Start baseline Kvrocks:

```bash
docker compose up -d kvrocks
```

Optional: start OTEL collector + Prometheus + Grafana stack:

```bash
docker compose --profile otel-stack up -d kvrocks otel-collector prometheus grafana
```

Endpoints:

- OTLP gRPC ingest: `localhost:4317`
- OTLP HTTP ingest: `localhost:4318`
- Prometheus query: `http://localhost:9090`
- Grafana UI: `http://localhost:3300` (default login `admin` / `admin`)
- Provisioned dashboard: `Kvrocks Benchmark Dashboard V1` (folder `Benchmarks`)

## Run the full benchmark matrix (recommended)

This single command runs A/B/C/D on `small` and `medium`, then writes
consolidated outputs.

```bash
rtk go run ./cmd/bench \
  -kvrocks-addr 127.0.0.1:6666 \
  -artifacts-dir artifacts \
  -timeout-seconds 300
```

`cmd/bench` runs 8 scenarios in sequence:

- `approach-a`, `approach-b`, `approach-c`, `approach-d` on `small`
- `approach-a`, `approach-b`, `approach-c`, `approach-d` on `medium`

### `cmd/bench` flags

You can tune the run with these flags:

- `-kvrocks-addr` (default `127.0.0.1:6666`): Kvrocks address.
- `-artifacts-dir` (default `artifacts`): base directory for outputs.
- `-seed` (default `42`): dataset seed for reproducibility.
- `-timeout-seconds` (default `300`): per-scenario timeout.
- `-otel-enabled` (default `true`): require OTEL export for valid/rankable
  runs.
- `-otel-endpoint` (default `otlp-local`): OTEL destination.
  - `otlp-local` maps to `http://127.0.0.1:4318/v1/metrics`.
  - You can also pass a full HTTP endpoint, for example
    `http://otel-collector:4318/v1/metrics`.

Example with explicit OTEL endpoint:

```bash
rtk go run ./cmd/bench \
  -kvrocks-addr 127.0.0.1:6666 \
  -artifacts-dir artifacts \
  -timeout-seconds 300 \
  -otel-enabled=true \
  -otel-endpoint http://127.0.0.1:4318/v1/metrics
```

Matrix outputs are written to:

- `artifacts/matrix-<timestamp>/matrix.json`
- `artifacts/matrix-<timestamp>/matrix.csv`
- `artifacts/matrix-<timestamp>/report.md`

Per-scenario outputs are written to `artifacts/<run_id>/`, including:

- `metrics.json`
- `metrics.csv`
- `config.json`
- `manifest.json`
- `otel-metrics.json`

### Verify OTEL ingest after `cmd/bench`

After the run, verify OTEL path in two places:

1. Confirm export status in matrix output:

```bash
rtk rg -n "otel_export_status|otel_label_endpoint" artifacts/matrix-*/matrix.json
```

2. Confirm metrics exist in Prometheus:

```bash
curl -fsS "http://127.0.0.1:9090/api/v1/query" \
  --get --data-urlencode 'query=bench.run.duration_ms'
```

## Run a single approach

Example for Approach D on `small`:

```bash
rtk go run ./cmd/approach-d \
  -tier small \
  -seed 42 \
  -kvrocks-addr 127.0.0.1:6666 \
  -prefix bench:d:manual: \
  -artifacts-dir artifacts \
  -timeout-seconds 120
```

You can replace `approach-d` with `approach-a`, `approach-b`, or
`approach-c`, and set `-tier medium` as needed.

## OTEL-required runs (default)

OTEL export is enabled by default for all commands and is required for valid,
rank-eligible runs.

```bash
rtk go run ./cmd/approach-c \
  -tier small \
  -kvrocks-addr 127.0.0.1:6666 \
  -otel-endpoint=otlp-local
```

Validity and ranking behavior:

- Producer hard gate requires `-otel-enabled=true` and successful export.
- Consumer re-validation quarantines invalid runs with rejection categories.
- Ranking uses canonical OTEL values (`duration_ms`, derived overall throughput)
  and surfaces OTEL-vs-local mismatches as quality failures.

Local JSON/CSV artifacts remain persisted as secondary audit evidence.

## OTEL stack smoke check

Run a basic end-to-end smoke path (collector ingest -> Prometheus query):

```bash
./tests/otel_stack_smoke.sh
```

## Per-run artifact files

Each run writes under `artifacts/<run_id>/`:

- `metrics.json`
- `metrics.csv`
- `config.json`
- `manifest.json`
- `otel-metrics.json` (canonical telemetry snapshot for ranking)

## Stop the environment

```bash
docker compose down
```

## Next steps

1. Run `rtk go test ./...` after code changes.
2. Use `artifacts/matrix-*/report.md` to compare approaches.

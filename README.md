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

## Run the full benchmark matrix (recommended)

This single command runs A/B/C/D on `small` and `medium`, then writes
consolidated outputs.

```bash
rtk go run ./cmd/bench \
  -kvrocks-addr 127.0.0.1:6666 \
  -artifacts-dir artifacts \
  -timeout-seconds 300
```

Matrix outputs are written to:

- `artifacts/matrix-<timestamp>/matrix.json`
- `artifacts/matrix-<timestamp>/matrix.csv`
- `artifacts/matrix-<timestamp>/report.md`

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

## Optional OTEL export

Local JSON/CSV metrics are always emitted. OTEL export is optional.

```bash
rtk go run ./cmd/approach-c \
  -tier small \
  -kvrocks-addr 127.0.0.1:6666 \
  -otel-enabled=true \
  -otel-endpoint=otlp-local
```

When enabled, each run also writes `otel-metrics.json` in that run's artifact
folder.

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
- `otel-metrics.json` (only when OTEL is enabled)

## Stop the environment

```bash
docker compose down
```

## Next steps

1. Run `rtk go test ./...` after code changes.
2. Use `artifacts/matrix-*/report.md` to compare approaches.

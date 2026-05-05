#!/usr/bin/env bash
set -euo pipefail

RUN_ID="smoke-$(date +%s)"
ENDPOINT="otlp-local"

wait_for_url() {
  local url="$1"
  local timeout_seconds="${2:-60}"
  local started
  started=$(date +%s)

  until curl -fsS "$url" >/dev/null; do
    if (( $(date +%s) - started > timeout_seconds )); then
      echo "timed out waiting for $url" >&2
      return 1
    fi
    sleep 2
  done
}

echo "Starting kvrocks + OTEL stack profile..."
docker compose --profile otel-stack up -d kvrocks otel-collector prometheus grafana >/dev/null

echo "Waiting for Prometheus readiness endpoint..."
wait_for_url "http://127.0.0.1:9090/-/ready" 120

echo "Sending one OTLP metric via collector HTTP endpoint..."
NOW_NANO=$(($(date +%s) * 1000000000))
cat <<JSON | curl -fsS -X POST "http://127.0.0.1:4318/v1/metrics" \
  -H "Content-Type: application/json" \
  --data-binary @- >/dev/null
{
  "resourceMetrics": [
    {
      "resource": {
        "attributes": [
          {"key": "service.name", "value": {"stringValue": "kvrocks-bench-smoke"}}
        ]
      },
      "scopeMetrics": [
        {
          "scope": {"name": "kvrocks-bench-smoke"},
          "metrics": [
            {
              "name": "benchmark_records_total",
              "description": "Smoke metric for OTEL stack bootstrap",
              "sum": {
                "aggregationTemporality": 2,
                "isMonotonic": true,
                "dataPoints": [
                  {
                    "startTimeUnixNano": "$NOW_NANO",
                    "timeUnixNano": "$NOW_NANO",
                    "asInt": "1",
                    "attributes": [
                      {"key": "run_id", "value": {"stringValue": "$RUN_ID"}},
                      {"key": "approach", "value": {"stringValue": "smoke"}},
                      {"key": "tier", "value": {"stringValue": "small"}},
                      {"key": "seed", "value": {"stringValue": "42"}},
                      {"key": "endpoint", "value": {"stringValue": "$ENDPOINT"}},
                      {"key": "matrix_id", "value": {"stringValue": "smoke-matrix"}}
                    ]
                  }
                ]
              }
            }
          ]
        }
      ]
    }
  ]
}
JSON

echo "Polling Prometheus until the metric is queryable..."
QUERY='benchmark_records_total{run_id="'$RUN_ID'",approach="smoke",tier="small",seed="42",endpoint="'$ENDPOINT'",matrix_id="smoke-matrix"}'
for _ in $(seq 1 30); do
  if curl -fsS "http://127.0.0.1:9090/api/v1/query" \
    --get --data-urlencode "query=$QUERY" | rtk rg -q '"status":"success"'; then
    if curl -fsS "http://127.0.0.1:9090/api/v1/query" \
      --get --data-urlencode "query=$QUERY" | rtk rg -q '"result":\[\{"metric"'; then
      echo "Smoke check passed: metric is queryable in Prometheus"
      exit 0
    fi
  fi
  sleep 2
done

echo "Smoke check failed: metric not found in Prometheus" >&2
exit 1

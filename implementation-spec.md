# Implementation Spec: GCS -> Kvrocks Monthly Ingestion

## Scope
This spec operationalizes the selected strategy: **Version Prefix + Throttled UNLINK** for monthly full-refresh ingestion.

It defines:
- State machine and run lifecycle
- Key schema and versioning rules
- Validation and cutover gates
- Cleanup policy with auto-backoff
- Concurrency, idempotency, and race prevention
- OTEL telemetry and minimum alerts
- Rollback policy
- First production rollout guardrails

## Finalized Decisions
- Rollback window: `72h`
- Completeness gate: `record count + key sampling correctness`
- Sampling mismatch threshold: `0`
- Read-path pointer resolution: cached pointer, `10s` poll refresh
- Cleanup throttle:
  - batch size: `500-1000` keys
  - sleep: `50-100ms` between batches
  - adaptive backoff by Kvrocks SLO signals
- Backoff triggers:
  - backoff when `p95 latency > 2x baseline` for 3 windows, or `error rate > 1%` in 1 minute
  - pause cleanup when `p99 latency > 4x baseline` or `error rate > 3%`
- Ingest concurrency: fixed worker pool, start at `16`
- Idempotency: deterministic version ID per release, idempotent writes
- Race prevention: distributed lock with lease/heartbeat + CAS cutover
- Validation failure policy: mark invalid, no cutover, preserve artifacts, alert
- Run-state truth: durable control record + OTEL telemetry
- Version retention cap: `active + 2 old`; block cutover if cap exceeded (unless manual override)
- Cutover mode: full global only
- Sampling strategy: stratified by key domain/prefix + random tail
- Sample size: `max(10,000, 0.1% of dataset)`, cap `100,000`, minimum floor per domain
- Pointer refresh: polling + optional admin-triggered refresh hook
- Rollback trigger: auto only for hard integrity regressions; else operator approved
- Observability: OTEL-based dashboards/alerts required before production
- Rollout safety: 1 full staging dry run with prod-scale shape incl. rollback + cleanup backoff simulation
- Version format: `YYYYMM-<manifest_short_hash>` with source manifest URI stored

## Architecture
1. Pull monthly dataset manifest and data from GCS.
2. Generate deterministic version ID.
3. Acquire ingestion lock.
4. Ingest all keys into version-prefixed namespace.
5. Run validation gates.
6. If valid, CAS flip active pointer.
7. Keep previous version for 72h rollback window.
8. Start throttled asynchronous cleanup of old version.
9. Enforce retained-version cap and alert on lag.

## State Machine

### States
- `RUN_CREATED`
- `LOCK_ACQUIRED`
- `INGESTING`
- `INGEST_COMPLETE`
- `VALIDATING`
- `VALIDATION_FAILED`
- `VALIDATION_PASSED`
- `CUTOVER_PENDING`
- `CUTOVER_DONE`
- `ROLLBACK_DONE` (optional)
- `CLEANUP_PENDING`
- `CLEANUP_RUNNING`
- `CLEANUP_PAUSED`
- `CLEANUP_DONE`
- `RUN_ABORTED`

### Transitions
- `RUN_CREATED -> LOCK_ACQUIRED` when distributed lock lease succeeds
- `LOCK_ACQUIRED -> INGESTING` when workers start
- `INGESTING -> INGEST_COMPLETE` when all manifest items processed
- `INGEST_COMPLETE -> VALIDATING` always
- `VALIDATING -> VALIDATION_FAILED` on any failed gate
- `VALIDATING -> VALIDATION_PASSED` if all gates pass
- `VALIDATION_PASSED -> CUTOVER_PENDING` when retention cap permits cutover
- `CUTOVER_PENDING -> CUTOVER_DONE` when CAS pointer flip succeeds
- `CUTOVER_DONE -> CLEANUP_PENDING` immediately
- `CLEANUP_PENDING -> CLEANUP_RUNNING` when rollback hold rules allow
- `CLEANUP_RUNNING -> CLEANUP_PAUSED` when pause threshold triggers
- `CLEANUP_PAUSED -> CLEANUP_RUNNING` when health recovers
- `CLEANUP_RUNNING -> CLEANUP_DONE` when old version deleted
- `VALIDATION_FAILED -> RUN_ABORTED` unless operator retries
- `CUTOVER_DONE -> ROLLBACK_DONE` on rollback event inside window

## Key Schema

### Data Keys
- Pattern: `<dataset_prefix>:<version_id>:<domain>:<entity_key>`
- Example: `tdaa:202605-a1b2c3d4:user:12345`

### Control Keys
- Active pointer key:
  - `tdaa:control:active_version`
  - value: `202605-a1b2c3d4`
- Lock key:
  - `tdaa:control:ingest_lock`
  - value includes `run_id`, `owner`, `lease_expiry`
- Run metadata key:
  - `tdaa:control:runs:<run_id>`
- Version metadata key:
  - `tdaa:control:versions:<version_id>`
- Cleanup checkpoint key:
  - `tdaa:control:cleanup:<version_id>`

## Durable Control Record
Persist this structure per run (JSON/document):
- `run_id`
- `version_id`
- `manifest_uri`
- `started_at`, `updated_at`, `ended_at`
- `state`
- `ingest_counts`: expected/processed/failed/retried
- `validation`:
  - `record_count_expected`
  - `record_count_actual`
  - `sample_size_total`
  - `sample_mismatches`
  - `sampling_breakdown_by_domain`
- `cutover`:
  - `previous_version`
  - `new_version`
  - `cutover_at`
  - `cas_expected`
  - `cas_result`
- `cleanup`:
  - `target_version`
  - `status`
  - `last_scan_cursor`
  - `deleted_keys`
  - `throttle_profile`
  - `pause_reason`
- `rollback`:
  - `eligible_until`
  - `rolled_back`
  - `reason`
- `operator_actions` (approvals/overrides)

## Version ID Rules
- Format: `YYYYMM-<manifest_short_hash>`
- `YYYYMM` comes from dataset release period, not ingestion runtime clock.
- `<manifest_short_hash>` is immutable from source manifest content.
- Same manifest must always produce same `version_id`.

## Validation Gates
Both required:
1. Record count parity.
2. Key sampling correctness.

### Record Count Gate
- Compare manifest expected record count with ingested count in target version namespace.
- Any mismatch => fail validation.

### Key Sampling Gate
- Strategy: stratified by domain/prefix + random tail sample.
- Size: `max(10,000, 0.1% of dataset)`, cap `100,000`.
- Domain floor: each domain gets minimum sample floor (configure explicitly, e.g. `>=200`).
- Threshold: `0` mismatches allowed.
- Any mismatch => fail validation.

## Cutover Protocol
1. Ensure run state is `VALIDATION_PASSED`.
2. Ensure retained-version cap will not be violated after cutover.
3. Execute CAS pointer update:
   - Condition: current active pointer == expected previous version.
   - Update: pointer -> candidate version.
4. On CAS failure:
   - do not retry blindly
   - refresh state, detect race, re-evaluate run validity
5. Mark `CUTOVER_DONE` and open rollback window for 72h.

## Reader Behavior
- Services resolve `active_version` via cached pointer.
- Refresh interval: every `10s`.
- Optional admin endpoint/hook to force immediate cache refresh during controlled cutover.
- On pointer fetch/cache error, apply short retry with jitter and fail-safe behavior defined by service SLA.

## Ingest Concurrency and Retry
- Worker pool: fixed, start at `16` workers (tunable).
- Global rate limit on write commands.
- Per-item retry with bounded attempts and jittered backoff.
- Idempotent writes into deterministic version namespace.
- Partial retries must not create duplicate semantic records.

## Locking and Race Prevention
- Use a distributed lock with lease and heartbeat renewal.
- If heartbeat fails and lease expires, run must stop writes and mark degraded state.
- Only lock holder may progress to cutover.
- CAS pointer update prevents out-of-order flips.

## Cleanup Design

### Start Conditions
- Cutover completed.
- Rollback hold and policy allow cleanup of target old version.

### Throttle Defaults
- Batch `UNLINK`: `500-1000` keys
- Sleep between batches: `50-100ms`
- Batch and sleep are runtime tunables.

### Backoff Logic
- Enter backoff when:
  - `p95 latency > 2x baseline` for 3 consecutive windows, or
  - `error_rate > 1%` in 1-minute window
- Pause cleanup entirely when:
  - `p99 latency > 4x baseline`, or
  - `error_rate > 3%`
- Resume gradually after recovery for sustained healthy windows.

### Checkpointing
- Persist cleanup cursor/checkpoint frequently.
- Cleanup worker must be restart-safe and resume from checkpoint.

## Retention and Cap Policy
- Keep versions: `active + up to 2 old`.
- If next cutover would exceed cap:
  - block cutover
  - page/operator alert
  - allow explicit override with audit trail

## Rollback Policy
- Rollback window: 72 hours from cutover.
- Auto rollback allowed only for hard integrity regressions.
- Non-integrity incidents require operator approval.
- Rollback uses same CAS safety to point active version back.
- After rollback, cleanup of failed version must be re-evaluated, not automatic.

## OTEL Specification
Use OTEL for metrics, logs, and traces. Durable control record remains source of truth.

### Traces
- Root span: `monthly_ingest_run`
- Child spans:
  - `fetch_manifest`
  - `ingest_batch`
  - `validate_record_count`
  - `validate_sampling`
  - `cutover_cas`
  - `cleanup_unlink_batch`
  - `rollback`

Span attributes (minimum):
- `run_id`
- `version_id`
- `manifest_uri`
- `state`
- `domain`
- `worker_id`
- `kvrocks_endpoint`

### Metrics (minimum names)
- `ingest_records_expected` (gauge)
- `ingest_records_processed_total` (counter)
- `ingest_records_failed_total` (counter)
- `ingest_retry_total` (counter)
- `validation_record_count_match` (gauge: 0/1)
- `validation_sample_size` (gauge)
- `validation_sample_mismatch_total` (counter)
- `cutover_cas_success_total` (counter)
- `cutover_cas_failure_total` (counter)
- `active_version_info` (gauge with version label)
- `cleanup_backlog_keys` (gauge)
- `cleanup_unlink_total` (counter)
- `cleanup_pause_total` (counter)
- `retained_versions_count` (gauge)
- `rollback_total` (counter)
- `kvrocks_command_latency_ms` (histogram)
- `kvrocks_error_rate` (gauge)

### Logs
Structured logs keyed by:
- `run_id`, `version_id`, `state`, `event`, `severity`, `reason`, `operator_id` (if manual action)

## Alerting Minimum
Create OTEL-driven alerts for:
- validation failed
- cutover CAS failure spike
- cleanup paused > threshold duration
- retained version cap near/exceeded
- kvrocks latency and error thresholds crossed
- rollback executed

## Operational Runbook
1. Start monthly run with manifest URI.
2. Verify lock acquired and workers healthy.
3. Monitor ingest progress and retries.
4. Confirm validation pass.
5. Execute and verify CAS cutover.
6. Confirm readers see new active version.
7. Observe cleanup with backoff/pause conditions.
8. Keep rollback readiness for 72h.
9. Close run after cleanup complete or documented deferred cleanup.

## First Production Rollout Gate
Before first production cutover, complete one full staging dry run with production-scale data shape including:
- full ingest
- validation fail-path drill
- successful cutover
- simulated integrity-triggered rollback
- cleanup backoff and pause/resume simulation

## Non-Goals
- Partial per-tenant/partition cutover (explicitly out of scope).
- TTL-based stale data retirement.
- In-place SCAN reconciliation of active namespace.

## Open Implementation Parameters (set in config)
- Domain sample floor per prefix
- Exact retry attempt limits and backoff curve
- Baseline window definitions for p95/p99 comparisons
- Cleanup scheduler cadence
- Manual override authz policy

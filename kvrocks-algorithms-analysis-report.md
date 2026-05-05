# Neutral Analysis Report: Kvrocks GCS Ingestion Algorithms

## Scope
This report analyzes all four algorithms from `kvrocks-gcs-ingestion-approaches.md` and **intentionally ignores the original recommendation**. It focuses on implementation-relevant tradeoffs for building POCs of every approach.

Algorithms covered:
- Approach A: SCAN + UNLINK
- Approach B: SETEX + Version
- Approach C: Blue/Green Swap
- Approach D: Version Prefix + Throttled UNLINK

## Evaluation Dimensions
- Time complexity (ingest, cutover, cleanup)
- Space complexity / storage amplification
- Maintainability
- Operational complexity
- Failure recovery behavior
- Rollback behavior
- Stale-data control strength
- Runtime resource pressure (CPU, I/O, memory)
- Observability requirements

## Common Assumptions
- Monthly dataset is full refresh, not delta.
- N = total active keys in old dataset.
- M = total keys in new monthly dataset (typically M ~= N).
- Kvrocks serves reads during ingest lifecycle.
- UNLINK is asynchronous key deletion but still generates cleanup work.

## Approach A: SCAN + UNLINK

### Mechanism
- Write/update new dataset into active namespace.
- SCAN active keyspace.
- UNLINK keys absent from new dataset.

### Complexity
- Ingest write: O(M)
- Reconciliation scan: O(N)
- Stale deletion issuance: O(K), where K = stale keys
- End-to-end monthly work: O(M + N + K)

### Space/Resource Profile
- Lowest overlap storage among approaches (near 1x steady-state).
- Heavy scan pressure on keyspace traversal.
- Cleanup cost coupled to ingest window.
- Potential latency interference from scan + delete activity.

### Maintainability / Operability
- Harder reasoning model due to in-place mutation.
- Cutover and cleanup are not cleanly separated.
- Mixed-state failure scenarios are more likely and harder to debug.

### Failure/Rollback
- Weak rollback after stale deletion starts.
- Interrupted run may leave partial reconciliation state.
- Recovery often requires additional reconciliation pass.

### Best-Fit Conditions
- Strong storage constraints.
- Team accepts higher reconciliation complexity and weaker rollback.

## Approach B: SETEX + Version

### Mechanism
- Write new versioned keys with TTL.
- Switch readers via version pointer.
- Old version expires automatically.

### Complexity
- Ingest write: O(M)
- Cutover pointer update: O(1)
- Cleanup issuance by pipeline: O(1) (deferred to TTL subsystem)
- End-to-end operational effort: O(M) from pipeline perspective; expiration cost shifts to background lifecycle.

### Space/Resource Profile
- Temporary overlap can approach ~2x during TTL window.
- Cleanup timing is implicit and delayed.
- Memory amplification depends on access patterns and metadata churn.

### Maintainability / Operability
- Simple ingestion workflow.
- Lower direct cleanup orchestration burden.
- Less explicit retirement control; behavior depends on TTL policy dynamics.

### Failure/Rollback
- Strong rollback until old version expires.
- If TTL too short, rollback safety window shrinks.
- If TTL too long, stale-data footprint and cost persist longer.

### Best-Fit Conditions
- Simplicity prioritized.
- Temporary data overlap acceptable.
- Explicit immediate retirement not required.

## Approach C: Blue/Green Swap

### Mechanism
- Build full dataset in inactive slot/environment.
- Validate isolated slot.
- Atomic reader switch to new slot.
- Cleanup old slot later.

### Complexity
- Ingest write: O(M)
- Cutover swap: O(1)
- Cleanup old slot: O(N_old)
- End-to-end monthly work: O(M + N_old)

### Space/Resource Profile
- Overlap commonly near ~2x during coexistence.
- Strong runtime isolation between serving and loading phases.
- Cleanup still required; can be scheduled separately.

### Maintainability / Operability
- Clear lifecycle boundaries (build/validate/swap/cleanup).
- More infrastructure surface area (slot lifecycle, environment orchestration).
- Operational model is explicit but broader.

### Failure/Rollback
- Strong rollback via pointer/slot switch-back while old slot retained.
- Failure before cutover has minimal serving-path impact.
- Failure after cutover still recoverable if old slot preserved.

### Best-Fit Conditions
- Isolation and rollback clarity valued over storage efficiency.
- Team can support additional environment/slot orchestration.

## Approach D: Version Prefix + Throttled UNLINK

### Mechanism
- Write new version into version-prefixed namespace.
- Validate.
- Atomic active-version pointer flip.
- Async throttled UNLINK cleanup of previous version.

### Complexity
- Ingest write: O(M)
- Cutover pointer update: O(1)
- Cleanup old version: O(N_old)
- End-to-end monthly work: O(M + N_old), with cleanup decoupled from cutover.

### Space/Resource Profile
- Temporary overlap up to ~2x during retention/cleanup window.
- Better control of cleanup load via batching/backoff.
- More predictable impact than aggressive immediate reconciliation.

### Maintainability / Operability
- Clear separation of ingest, cutover, rollback window, and cleanup.
- Requires disciplined pointer and cleanup checkpoint management.
- Moderate operational complexity with good controllability.

### Failure/Rollback
- Strong rollback until old version deleted.
- Partial cleanup failure is recoverable with checkpointed resume.
- Cutover safety depends on pointer correctness and validation gate quality.

### Best-Fit Conditions
- Need explicit cutover control and bounded cleanup behavior.
- Accept temporary overlap with managed deletion.

## Cross-Algorithm Comparison Matrix

| Dimension | A: SCAN+UNLINK | B: SETEX+Version | C: Blue/Green | D: Version Prefix+Throttled UNLINK |
| --- | --- | --- | --- | --- |
| Ingest Complexity | O(M) + reconciliation | O(M) | O(M) | O(M) |
| Cutover Complexity | In-place (no clean O(1) switch) | O(1) pointer | O(1) swap | O(1) pointer |
| Cleanup Complexity | O(K) in active path | Deferred (TTL-managed) | O(N_old) | O(N_old) throttled |
| Storage Amplification | Lowest | Medium-High during TTL | High during overlap | Medium-High during overlap |
| Maintainability | Low-Moderate | Moderate | High | High |
| Rollback Strength | Low after delete starts | High until expiry | High until old slot removed | High until old version removed |
| Failure Recovery | Hardest (mixed active state risk) | Moderate-High | High | High |
| Cleanup Control | Explicit but coupled/heavy | Weak-explicit (TTL-driven) | Explicit | Explicit + throttled |
| Operational Complexity | Moderate but brittle | Low-Moderate | Moderate-High | Moderate |
| Stale-Data Governance | Strong only after full reconcile | TTL-window dependent | Strong post-switch + cleanup | Strong post-switch + cleanup |

## Implementation Implications for POCs
To compare fairly, each POC should expose identical controls:
- Dataset generator and manifest loader
- Ingest worker count
- Cutover mechanism abstraction
- Validation hooks (record count + key sampling)
- Rollback command
- Cleanup mode and rate controls
- Metrics export (latency, error rate, throughput, backlog)

POC-specific must-haves:
- A: Efficient stale-key detection strategy and scan pacing controls
- B: TTL policy parameters and expiry observability
- C: Slot lifecycle manager and swap guard checks
- D: Version pointer CAS and cleanup checkpoint resume

## Key Risks to Validate Empirically in POCs
- A: Scan-induced latency impact during production-like load
- B: TTL lag variance and stale footprint duration
- C: Operational overhead of slot/environment lifecycle
- D: Cleanup backlog growth and backoff stability under load

## Conclusion (Recommendation intentionally omitted)
All four approaches are implementable. They differ primarily in where complexity is paid:
- A pays complexity in reconciliation and recovery.
- B pays in overlap/retirement determinism.
- C pays in infrastructure/operational breadth.
- D pays in pointer/cleanup control machinery.

Final selection should be based on benchmarked results from equal-footing POCs rather than architecture-only preference.

# PRD: Kvrocks GCS Ingestion POC Benchmark Suite

Status: needs-triage

## Problem Statement

The team needs to choose a safe, maintainable, and resource-efficient ingestion strategy for monthly full-refresh data loads from GCS into Kvrocks.

Current design discussion exists, but there is no implementation-backed comparison. Without comparable proof-of-concept implementations and benchmark outputs, strategy selection relies on architecture opinions rather than measured evidence.

The team specifically needs all four candidate algorithms implemented and evaluated under the same conditions, with reproducible artifacts, consistent correctness gates, and objective comparison output.

## Solution

Build a benchmark-oriented Go POC suite that implements four ingestion algorithms with a shared core runtime to ensure fair comparisons, while keeping separate executable project folders for each approach.

Run all approaches against the same Kvrocks baseline environment, same dataset tiers, same correctness rules, and same artifact schema. Provide a one-command orchestration flow that executes the benchmark matrix and generates machine-readable and human-readable reports.

This solution gives engineering and stakeholders a repeatable decision framework for algorithm selection and later production hardening.

## User Stories

1. As a platform engineer, I want all four ingestion algorithms implemented in runnable POCs, so that I can compare them fairly.
2. As a platform engineer, I want each algorithm to have its own executable entrypoint, so that I can run and debug strategies independently.
3. As a backend engineer, I want shared ingestion plumbing across POCs, so that differences reflect algorithm behavior, not inconsistent infrastructure code.
4. As a data engineer, I want deterministic mock dataset generation, so that repeated runs are reproducible.
5. As a data engineer, I want mixed key domains and payload sizes, so that benchmarks reflect realistic workload variation.
6. As a reliability engineer, I want correctness validation gates to be identical across approaches, so that pass/fail outcomes are directly comparable.
7. As a reliability engineer, I want record-count parity validation, so that missing or extra records are detected before acceptance.
8. As a reliability engineer, I want stratified key sampling validation with zero mismatch tolerance, so that silent data corruption is rejected.
9. As an operator, I want all runs to emit structured JSON artifacts, so that I can automate analysis and archival.
10. As an operator, I want all runs to emit CSV artifacts, so that I can inspect trends quickly in spreadsheet or BI tools.
11. As an engineering manager, I want a generated markdown comparison report, so that tradeoffs are easy to review in planning meetings.
12. As a performance engineer, I want benchmark runs across small and medium tiers at minimum, so that we can validate behavior before scaling up.
13. As a performance engineer, I want optional large-tier runs, so that I can stress-test scale boundaries.
14. As a developer, I want a single command to run the full benchmark matrix, so that comparison is low-friction and less error-prone.
15. As a developer, I want one shared Docker Compose environment, so that all approaches run under equivalent infrastructure conditions.
16. As an SRE, I want local metrics always available, so that diagnostics work even without telemetry stack setup.
17. As an SRE, I want OTEL integration to be optional by flag, so that we can adopt observability incrementally.
18. As a maintainer, I want approach-specific tuning knobs, so that each algorithm can be evaluated without modifying source code.
19. As a maintainer, I want a global worker baseline default, so that cross-approach comparisons stay aligned.
20. As a QA engineer, I want rollback exercised as a separate scenario, so that baseline performance results remain clean.
21. As a QA engineer, I want reproducible run IDs and saved manifests, so that failed runs can be recreated exactly.
22. As an architect, I want a common cutover abstraction across algorithms, so that cutover semantics can be compared consistently.
23. As an architect, I want clear separation between ingestion, validation, cutover, and cleanup phases, so that operational behavior is understandable.
24. As a product owner, I want benchmark evidence rather than recommendation bias, so that strategy selection is auditable.
25. As an incident responder, I want error and retry counts in outputs, so that failure behavior can be evaluated explicitly.
26. As a cost owner, I want overlap and cleanup behavior visible in results, so that storage and operational impact can be estimated.
27. As a senior engineer, I want deep reusable modules for core pipeline components, so that future productionization is faster.
28. As a team lead, I want completion criteria defined up front, so that delivery is objectively verifiable.
29. As a team lead, I want known risks captured with mitigations, so that benchmark interpretation is realistic.
30. As a future implementer, I want a direct path to real GCS integration after POC, so that transition to production inputs is straightforward.
31. As a future implementer, I want a path to cluster-mode benchmarking, so that architecture decisions can evolve with scaling requirements.
32. As a compliance-minded stakeholder, I want persisted artifacts per run, so that benchmark history can be reviewed later.
33. As a release manager, I want smoke-level automation support, so that regressions can be caught in CI over time.
34. As a developer, I want each approach to support scenario flags, so that targeted experiments can run quickly.
35. As a developer, I want bounded approach-specific cleanup controls, so that test runs do not destabilize the test environment unexpectedly.
36. As an evaluator, I want standardized ranking dimensions in reports, so that no approach is favored by custom metrics.

## Implementation Decisions

- Implement four strategy runners corresponding to SCAN+UNLINK, SETEX+Version, Blue/Green swap, and Version Prefix + Throttled UNLINK.
- Use a monorepo layout with separate approach folders and a shared deep-module core.
- Build deep modules for config loading, dataset generation/loading, Kvrocks access, validation, cutover orchestration, metrics emission, and artifact persistence.
- Keep a deterministic local manifest/object source in v1 through a pluggable loader contract that can later support real GCS.
- Enforce identical validation contract for all approaches: record-count parity and stratified sampling with zero mismatches.
- Standardize baseline concurrency defaults and expose runtime tuning flags for controlled experiments.
- Keep rollback evaluation as an explicit scenario mode separate from baseline runs.
- For SETEX strategy, default TTL to 72 hours and expose an override.
- For SCAN+UNLINK strategy, execute reconciliation in bounded phases with pacing controls rather than unrestricted scanning.
- Model Blue/Green using logical slots in one Kvrocks deployment to preserve fair baseline conditions.
- Use a shared cutover controller abstraction so pointer-based and marker-based cutovers can be compared through a uniform benchmark flow.
- Emit per-run structured output as JSON and CSV plus generated markdown summary.
- Provide a benchmark orchestrator command that runs the matrix and composes comparison outputs.
- Run against one Docker Compose baseline with optional telemetry profile and optional runner profile.
- Use deterministic seed and persisted run metadata/manifest under artifact directories.
- Mark implementation complete only when all four approaches pass end-to-end execution on required tiers with artifact generation and comparative report output.

## Testing Decisions

- Good tests validate external behavior and contracts, not internal implementation details.
- Core shared modules should receive the strongest automated tests because they are reused by all strategies and define benchmark fairness.
- Highest-priority test targets:
  - dataset determinism and domain/payload distribution behavior
  - validation contract behavior for record-count parity and stratified sampling mismatch handling
  - artifact serialization contract (JSON/CSV schema stability)
  - cutover abstraction contract behavior across pointer-based and marker-based flows
  - strategy smoke tests proving each runner completes baseline flow with expected lifecycle states
- Strategy-specific tests should focus on externally observable outcomes:
  - reconciliation outcomes and pacing effects for SCAN+UNLINK
  - TTL contract behavior for SETEX+Version
  - slot swap correctness for Blue/Green
  - cleanup throttling/checkpoint resume semantics for Version Prefix + Throttled UNLINK
- Integration smoke tests should run against Compose Kvrocks baseline to verify real command-path compatibility.
- Comparative report generation should be tested with fixture artifacts to ensure deterministic aggregation and ranking output.

## Out of Scope

- Real GCS production integration in this phase.
- Kvrocks cluster-mode benchmarking in this phase.
- Production rollout, migration, or live cutover automation.
- Partial per-tenant cutover models.
- Final architecture recommendation based only on static design analysis.
- Advanced statistical benchmarking framework beyond required baseline matrix.

## Further Notes

- This PRD intentionally emphasizes unbiased implementation and measurement over design-preference recommendations.
- The benchmark suite is expected to become a reusable foundation for future production hardening work.
- OTEL support is included as opt-in to balance delivery speed and observability maturity.
- The generated comparison report should be treated as decision input and reviewed alongside operational constraints.

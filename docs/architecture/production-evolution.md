# Production release gates

Updated 2026-11-09. These gates define the deployment sequence and required evidence.

## 1. Initial deployment

Start with a modular monolith, one coordinator, and one managed model on a local host. Separate the data protocol, resource ledger, and backend implementation so platform, engine, client, and deployment changes remain localized. Correct cancellation evidence and accounting are prerequisites for adding distributed infrastructure.

## 2. Stages

| Stage | Capability | Release evidence | Status |
| --- | --- | --- | --- |
| A: Architecture foundation | Dependency boundaries, domain values, ports, machine-readable contracts, replacement and negative tests | `make check`, `make contracts`, requirements-to-evidence review | Foundation gates passed |
| B: Usable local service | T0-T8 HTTP, scheduling, native engine, Python client | Pinned artifacts, real cancellation/unloading, application comparisons | Control prototype exists; T6/T8 acceptance pending |
| C: Reliable single host | Ownership journal, restart reconciliation, key rotation, shutdown, config generations, operational telemetry | Crash/replay/corruption/disconnect injection; long-running and rollback records | Later release |
| D: Controlled remote workers | Authenticated encrypted RPC, worker-local ownership, deduplication, fencing, durable coordination | No capacity overcommit under partitions, duplicate commands, expired leases, or stale owners | Requires a multi-host use case |
| E: Production cluster and team operations | Capacity planning, tenant quotas, HA, rolling upgrades, backup/recovery, incident response | Workload-specific SLOs, recovery objectives, and operational ownership | Outside the current implementation |

Each stage must pass before the next deployment claim. Add storage or messaging through dedicated ports and migration tasks while retaining domain rules and the public HTTP contract.

## 3. Initial reliable-host acceptance criteria

These are candidate gates to validate on the target hardware and workload:

- Run a fixed configuration for at least 4 hours and 1000 valid calls. Report successes, refusals, cancellations, and incomplete results. Include resource refusals in user-visible availability.
- Exercise at least 100 controlled startup/shutdown and cancellation cycles. Confirm owned-process reclamation within budget. Test forced termination and crash recovery separately.
- Target less than 10 ms of added p95 control-path overhead, excluding model execution, loading, and queue time. Record workload and measurement boundaries.
- Retain occupancy when completion is unknown. Retired instances, duplicate commands, journal corruption, and failed writes must not cause excess allocation.
- Exclude keys and raw chat content from logs, diagnostics, and metrics. Bound output buffers, event retention, and label cardinality under sustained load.

CPU/GPU throughput and capacity budgets depend on the host and model. Maintain a report for each supported configuration.

## 4. Reliability and persistence

The in-memory queue does not survive restart. A disconnected response cannot automatically resume. Task lookup or resumable output requires an explicit protocol, retention policy, and durable state; request IDs alone provide no idempotency guarantee.

Before effects occur, journal the minimum operation intent and owner. Record completion evidence and results afterward. Keep raw prompts and responses out of the journal. Corrupt or missing state requires reconciliation before another model is loaded. Introduce a storage port when persistence is required.

Leases and fencing can reject new commands from stale owners, but cannot establish that old GPU work stopped. Partitions, worker loss, and lease expiry require stop evidence or conservative isolation before capacity reuse.

## 5. Security and configuration

Local mode uses loopback and one Bearer token. Nonlocal listeners require TLS, authentication, separate operational access rules, request limits, and integration tests.

Pin backend and model sources by version and digest. Download to temporary files, verify integrity, and publish atomically. Reject corrupt caches and digest mismatches. Pass startup parameters as argument arrays; keep request content, model IDs, and paths out of shell command construction.

Version configuration and model profiles. Bind accepted requests to their model, configuration, and policy generations. Hot updates validate before publishing an immutable snapshot. Changes to instance parameters drain the old instance and create a new incarnation while preserving in-flight context limits.

## 6. Observability

Measure pressure, queue time, load time, and execution time separately. Put request IDs and error details in bounded logs or traces. Metric labels use controlled sets with cardinality limits. See [Prometheus instrumentation guidance](https://prometheus.io/docs/practices/instrumentation/#do-not-overuse-labels) and [OpenTelemetry memory guidance](https://opentelemetry.io/docs/specs/otel/metrics/supplementary-guidelines/#memory-management).

Telemetry output failures must not block resource reclamation. Loss-tolerant events use bounded queues with drop counters. Authoritative evidence belongs to the coordinator and journal, with delivery guarantees appropriate to resource accounting.

## 7. Compatibility and upgrades

Use the machine-readable HTTP contract for validation and version comparison. Optional response fields can be added compatibly. Field removal, error/stream termination changes, and authentication changes require versioning and migration notes. Python clients accept new fields and treat unknown error codes as failures.

Record API versions, backend protocol versions, model revisions, and configuration versions separately. Release traceable builds, dependencies, and model manifests. Verify old-config loading, draining, and rollback before upgrading. Rolling back must retain unknown-occupancy records.

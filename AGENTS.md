# Project context

Read `README.md`, `CONTEXT.md`, `docs/architecture/overview.md`, `docs/architecture/runtime-contracts.md`, the design in `docs/superpowers/specs/`, and `ROADMAP.md` before changing behavior. Accepted decisions live in `docs/adr/`. The user has chosen the recommended baseline and asked for low coupling, extensibility and a production evolution path; do not reopen recorded choices or confuse architecture foundation tests with a running inference service.

## Product scope

- Serve a personal computer while preserving everyday application responsiveness.
- Develop the first measured path on macOS arm64 with 8 GiB shared memory.
- Prefer one Go control process and an external native inference engine.
- Keep model identity, supported context and routing destination explicit.
- The first real caller is the user's own script or SDK. Validate the HTTP contract and one selected client path; do not make a chat UI or a new standalone SDK a prerequisite for the first release.
- Python is the confirmed language for the first real caller. Keep total deadlines separate from HTTP read timeouts, close streams on cancellation, and never treat unexpected EOF or partial output as successful completion.
- Validate the complete lifecycle with a pinned small model before evaluating the user's target model. The current candidate metadata is in `docs/research/small-model-baseline.json`; metadata verification is not a successful inference run or a memory-capacity guarantee.
- Keep research and user-facing documentation in Chinese; use English for code identifiers.

## Resource and lifecycle invariants

- On unified-memory hardware, never add RAM and GPU memory as independent capacity.
- Represent unavailable or stale metrics explicitly; never turn them into zero usage.
- Missing or stale required memory-pressure observations block new generation admission and new model loads, including when a model is already loaded. Preserve in-flight generation under its existing deadline; missing observations alone are not critical-pressure evidence. Optional GPU-utilization gaps do not trigger this guard.
- Allow healthy inference to use idle compute capacity within memory safety constraints. High total CPU utilization caused by inference itself must not independently trigger throttling; distinguish owned inference load from other workload estimates and expose attribution uncertainty.
- Account for in-flight requests and concurrent model-load reservations before admission.
- Propagate client cancellation upstream. Release execution reservations exactly once after engine completion or owned-process exit is confirmed; an HTTP disconnect alone is not proof of released compute.
- Use normal draining for configuration changes and idle unloading. Under confirmed critical memory pressure, follow ADR-0001: daily applications take priority and bounded emergency reclamation may interrupt generation and terminate an owned engine.
- Once emergency cancellation has started, finish reclamation and report interruption even if pressure subsequently improves. Never silently retry an interrupted generation.
- Only terminate child processes owned by this application; an attached external server is not an owned process.
- Bound queue length and wait time. Expose reason codes for resource decisions.
- A generation request may wait for the required model's cold start and then execute on the same HTTP request. Track ordinary queue wait, shared model-load wait, execution time and the total deadline separately; new waiters or state transitions must not reset budgets.
- Share one bounded load attempt across waiters. Cancelling one waiter must not cancel a load needed by others; if no demand remains, cancel an unnecessary load without releasing its memory reservation before confirmed cleanup.
- Default to local execution. Remote routing requires explicit configuration and a request policy that permits it.

## Implementation and verification

Enforce `architecture/dependencies.json`. Domain values do not import transport, serialization, process or platform APIs. Application code depends on consumer-owned ports, never concrete adapters. Policy is pure and receives time explicitly. Concrete implementations are connected only at the composition root; inbound handlers never acquire engine lifecycle or reservation authority.

Keep inbound use cases in `internal/application/api` separate from outbound contracts in `internal/application/ports`. HTTP adapters may import the inbound API and domain values, not application implementations, outbound ports or other adapters.

Separate periodic `ResourceSampler` from cached `ResourceReader`. HTTP diagnostics must not trigger OS probes or make admission decisions. Preserve output completion versus resource-release completion, and ignore evidence for retired instance incarnations.

Service readiness is separate from instantaneous generation capacity (ADR-0004). An unloaded model or a temporary observation guard does not itself make the application unready; it still cannot bypass request-level resource protection.

Run `make setup-tools` once, then `make check` for relevant source or contract changes. These checks cover dependency boundaries, Go tests/race/vet and OpenAPI fixtures. Lock new tool dependencies with hashes; keep them outside the Go runtime dependency graph. CI configuration is not evidence that remote CI has run.

When implementation begins, prioritize meaningful checks for admission races, cancellation, missing metrics, draining, queue expiry and streaming. Simulated scheduler tests and real model inference measurements must be reported separately.

Do not commit keys, model weights, machine serial numbers, platform UUIDs, raw prompts or responses. Keep model caches and local runtime state outside tracked source.

## Upstream research

Use official repositories, official documentation and official pricing pages. Add the check date and a source URL when updating observations. `sources.json` records Git blob hashes for selected files; these are file-content fingerprints, not repository commit hashes or dependency pins.

Before copying or distributing upstream material, inspect the actual file-level license. LiteLLM's root license excludes material under `enterprise/` if present; its root MIT terms must not be assumed to cover every commercial component. Model weights and inference backends require their own license checks.

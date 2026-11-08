# Project context

Read `README.md`, the current design in `docs/superpowers/specs/`, and `ROADMAP.md` before changing behavior. This repository starts with research and a proposed design; do not describe planned capabilities as implemented features.

## Product scope

- Serve a personal computer while preserving everyday application responsiveness.
- Develop the first measured path on macOS arm64 with 8 GiB shared memory.
- Prefer one Go control process and an external native inference engine.
- Keep model identity, supported context and routing destination explicit.
- Keep research and user-facing documentation in Chinese; use English for code identifiers.

## Resource and lifecycle invariants

- On unified-memory hardware, never add RAM and GPU memory as independent capacity.
- Represent unavailable or stale metrics explicitly; never turn them into zero usage.
- Account for in-flight requests and concurrent model-load reservations before admission.
- Cancel upstream work when the client disconnects, and release reservations exactly once.
- Drain in-flight requests before changing engine startup parameters or unloading a model.
- Only terminate child processes owned by this application; an attached external server is not an owned process.
- Bound queue length and wait time. Expose reason codes for resource decisions.
- Default to local execution. Remote routing requires explicit configuration and a request policy that permits it.

## Implementation and verification

Keep telemetry collection, policy evaluation, engine management and HTTP forwarding separate. Policy decisions should accept explicit snapshots and clocks so state transitions can be tested deterministically.

When implementation begins, prioritize meaningful checks for admission races, cancellation, missing metrics, draining, queue expiry and streaming. Simulated scheduler tests and real model inference measurements must be reported separately.

Do not commit keys, model weights, machine serial numbers, platform UUIDs, raw prompts or responses. Keep model caches and local runtime state outside tracked source.

## Upstream research

Use official repositories, official documentation and official pricing pages. Add the check date and a source URL when updating observations. `sources.json` records Git blob hashes for selected files; these are file-content fingerprints, not repository commit hashes or dependency pins.

Before copying or distributing upstream material, inspect the actual file-level license. LiteLLM's root license excludes material under `enterprise/` if present; its root MIT terms must not be assumed to cover every commercial component. Model weights and inference backends require their own license checks.

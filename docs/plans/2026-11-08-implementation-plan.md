# Initial implementation plan

Drafted 2026-11-08; updated 2026-11-09 (Asia/Shanghai). T0-T5 have implementations and recorded-path validation. Native model acceptance remains pending.

Behavior follows the [design](../superpowers/specs/2026-11-08-adaptive-ai-gateway-design.md) and [HTTP contract](../contracts/http-api.md); dependencies follow the [architecture](../architecture/overview.md).

## 1. Delivery scope

- Personal-computer use, initially macOS arm64 with 8 GiB unified memory.
- Use idle compute within memory constraints. Critical pressure prioritizes everyday applications and can interrupt managed generation.
- Python scripts as the first real client.
- One request can wait for one bounded shared cold start and then generate.
- Validate lifecycle and resource control with a pinned small model before target-model evaluation.
- Missing or stale required memory pressure pauses new generation/loading, preserves active requests, and requires reliable recovery observations.

The design uses a modular monolith with replaceable adapters and separate production release gates. Current implementation and measurement limits are in the [runtime report](../reports/2026-11-09-runtime-validation.md).

## 2. Validation baseline

The [Qwen2.5-0.5B-Instruct-GGUF](https://huggingface.co/Qwen/Qwen2.5-0.5B-Instruct-GGUF) Q4_K_M artifact is downloaded and digest-verified. [Baseline metadata](../research/small-model-baseline.json) records its revision, size, digest, and license. File length does not establish peak unified-memory use. Host resource protection prevented generation during the recorded run.

The engine uses the official b12110 arm64 artifact corresponding to the llama.cpp [v0.9.0 server documentation](https://github.com/ggml-org/llama.cpp/blob/v0.9.0/tools/server/README.md). Artifact digest and executable version are verified. Native generation, cancellation, unloading, and peak memory require suitable host conditions.

Initial probes use context 2048, one model, one execution, and at most 128 output tokens. The public cap is 512. Short inputs validate response, streaming, cancellation, and cleanup; answer quality is outside lifecycle acceptance.

## 3. Modules and state

| Location | Responsibility | Status |
| --- | --- | --- |
| `cmd/gateway/` | Startup, composition, process exit; connects concrete adapters | Implemented and exercised |
| `internal/domain/` | Observations, model/instance/execution identity, output, evidence | Values and scope checks implemented |
| `internal/application/api/` | Inbound queries and commands | Resource, generation, and status contracts implemented |
| `internal/application/ports/` | Outbound sampling/reading, rules, lifecycle, generation | Native implementation and process fixtures; model acceptance at T6 |
| `internal/application/` | Coordinator, queue, ledger, budgets | Implemented |
| `internal/policy/` | Pure rules with explicit time | Memory/external CPU rules implemented; coordinator owns recovery |
| `internal/adapter/config/` | TOML, secret sources, validated configuration | Implemented |
| `internal/adapter/llamacpp/` | Native protocol, owned process, exit evidence | Fixtures passed; model measurement at T6 |
| `internal/adapter/platform/` | Periodic sampling and snapshot publication | macOS pressure, conservative headroom, windowed CPU attribution implemented |
| `internal/adapter/http/` | Auth, DTOs, errors, SSE | Live synthetic HTTP checks passed |
| `internal/adapter/observability/` | Metadata logging | Logging implemented; full metrics output deferred |
| `internal/adapter/fake/` | Controlled snapshots, clocks, backend | Test adapters implemented |
| `internal/architecture/`, `cmd/archcheck/` | Dependency rules and negative tests | Implemented and passed |
| `api/` | Machine-readable contract and fixtures | OpenAPI 3.1.1 and validation |
| `examples/python/` | Async caller example | Synthetic path passed; native acceptance awaits T6 |

Engine lifecycle, protection state, and request state have separate storage. Public summaries such as ready, constrained, and reclaiming combine those axes without discarding the underlying states and reasons.

Ordinary reconfiguration drains; emergencies follow ADR-0001. Ownership and incarnation identify managed processes. A late result belongs to its original incarnation and cannot mutate a restarted instance.

## 4. Task dependencies

| Task | Output | Prerequisite | Acceptance check |
| --- | --- | --- | --- |
| T0: Architecture and contracts | Go ports, dependency manifest, HTTP/OpenAPI, release gates | Selected architecture and scope | Boundary and schema checks; HTTP behavior exercised in T5 |
| T1: Minimal runtime | Entry point, config, logs, liveness, bounded exit | T0 | Locatable config errors; no owned fixture process left after shutdown |
| T2: Scheduling and budgets | `application/coordinator.go`, `ledger.go`, `budgets.go`, request state, timing interfaces | Domain/port contracts and controlled backend | No overcommit; 45-second loading survives the 30-second queue budget; retired evidence cannot release current work; budgets persist |
| T3: Engine supervision | Shared loading, health, incarnations, cancellation, forced exit | T1 and controlled backend | One cancelled load waiter preserves other demand; last demand cleans up; occupancy remains until confirmed release |
| T4: macOS resources | Initial pressure, sampling, CPU attribution, observation guard | Verified platform facts and domain/ports | Correct mapping; missing required data blocks resident-model execution; active generation preserved; owned CPU alone never throttles; GPU absence tolerated |
| T5: HTTP and streaming | Validation, inbound-use-case mapping, errors, SSE | T2-T4 core checks; handlers consume only application/api | Live OpenAPI conformance; readiness/capacity separation; loading/cancellation/interruption; slow clients do not block control |
| T6: Native small model | Pinned engine/GGUF, profile, measurements | T5 synthetic path and safe loading conditions | Reproducible loading/generation/cancellation/unloading; model peak and controller overhead recorded separately |
| T7: Client acceptance | Reproducible Python example, lock, results | T5/T6 runnable | Cold start, total deadline, streaming, cancellation, parseable errors; no automatic retries; EOF cannot imply success |
| T8: Application comparison | Fixed versus resource-aware report | T6/T7 | Same model/context/application load; task time, queue time, first token, throughput, pressure |

Initial startup-failure protection allows three failed starts within 60 seconds, with 1/2-second backoffs after the first two. A failed request ends after its first attempt; later valid demand can initiate another allowed load. The third failure requires explicit recovery, initially a gateway restart. Idle unloading, user cancellation, and emergency reclamation do not count as backend failures. Calibrate these values after fixture and hardware checks.

T2/T3/T4 can be developed independently once their interfaces are fixed.

T6-T8 acceptance uses the [evaluation protocol](../evaluation/README.md), [metric definitions](../evaluation/metrics.md), [scenario matrix](../evaluation/scenarios.md) and [report template](../evaluation/report-template.md). Artifact verification, successful native execution and measured foreground benefit require separate verdicts.

### Prototype sequence

1. Establish HTTP/OpenAPI, Go ports, and import rules. Module path: `github.com/AtomsH4/adaptive-ai-gateway`.
2. Wire `cmd/gateway/`, TOML validation in `adapter/config/`, and logging in `adapter/observability/`. Keep environment reads and process operations outside business packages. Pin configuration dependencies. Synthetic mode needs no model or cloud key.
3. Use the fake backend for one slot, four waiters, one shared load, cumulative deadlines, cancellation confirmation, and late results. Model speed must not determine these tests.
4. Add read-only platform sampling and policy fixtures, then HTTP integration. Check recovery/loading/total-budget combinations for duplicate charging or deadline resets.

Run relevant behavior checks, `make check`, and client checks after each package. Synthetic HTTP and child-process evidence remain distinct from T6 measurements.

### Completion and rollback

- Accept a task when implementation, tests, and status records agree.
- Fix a failed module at its gate. Preserve layer boundaries, unknown occupancy, and required guards.
- Use immutable configuration versions and new incarnations for instance changes. Before durable recovery exists, restart does not resume old requests.
- Declare unsupported cancellation capabilities. Managed instances can be stopped with exit confirmation; attached occupancy stays unknown until evidence arrives.
- Journals, remote workers, HA, and team operations follow [separate release gates](../architecture/production-evolution.md).

## 5. Initial resource accounting

Track load reservation, model residency, and execution slots separately. An execution slot does not reserve another full copy of model memory.

Check a calibrated model high-water estimate before loading. Retain that reservation during loading, without subtracting the same observed memory decrease again or starting a second load/generation. Pressure observations continue to constrain a load in progress.

On readiness, convert the load reservation to resident state for the same model. Release execution occupancy after confirmed completion and model occupancy after confirmed unload/exit. Other applications can allocate memory after admission, so the estimate includes headroom without guaranteeing every future allocation.

If the model budget is unproven, calibrate it independently. One short request or GGUF length does not establish a memory upper bound. General automatic budgeting for unknown models is outside this single-model release.

Observation guard blocks new allocation while retaining resident state and reservations. Existing loading keeps its original deadline; generation waits for required observations to recover. Two fresh normal samples are the initial recovery criterion, alongside budgets and other guards. Previously triggered critical reclamation retains priority if later samples become unknown.

## 6. Platform mapping

A read-only query of `kern.memorystatus_vm_pressure_level` returned 2. The installed SDK's dispatch flags and Apple's [XNU conversion/sysctl implementation](https://github.com/apple-oss-distributions/xnu/blob/main/bsd/kern/kern_memorystatus_notify.c) identify exported user-space flags rather than internal enum values.

| Exported value | Normalized value |
| --- | --- |
| 1 | normal |
| 2 | warning |
| 4 | critical |
| Query failure or other value | unavailable; retain raw values for diagnostics |

This is a sample and interface check, not continuous pressure measurement. Resample on each run. The source marks this sysctl masked, so keep it inside the platform adapter and fail explicitly if unavailable or inconsistent across OS versions. SDK values and source fingerprints are in the research index.

## 7. Python client

`examples/python/` uses Python 3.12.13 and [HTTPX 0.28.1](https://pypi.org/project/httpx/0.28.1/) with a complete hash lock. `asyncio.timeout` requires Python 3.11+; the system Python 3.9.6 is outside this example's baseline.

- Use [AsyncClient and context-managed streaming](https://www.python-httpx.org/async/) to close responses. This example is the initial client, not a general SDK compatibility claim.
- Apply a 300-second outer [asyncio timeout](https://docs.python.org/3.11/library/asyncio-task.html#asyncio.timeout). Configure [network limits](https://www.python-httpx.org/advanced/timeouts/) separately: connect 5 seconds, write 10, pool wait 5, read 300. Read limits apply to waiting for a data chunk, not the entire call.
- Set the asynchronous [transport](https://www.python-httpx.org/advanced/transports/) to zero connection retries. Add no status-code or interrupted-stream retry loop.
- On deadline, cancellation, or keyboard interruption, close the stream/connection and propagate cancellation. Server-side release still requires evidence.
- Require normal termination markers. Stream errors, unexpected EOF, and network failures retain request ID/reason/partial state and fail the call. Normal output limits retain their finish reason.
- Client acceptance includes ordinary/streaming/cancel paths, simulated 45-second loading, fragment boundaries, valid endings, explicit errors, and unterminated EOF. Controlled services or clocks can shorten tests; record actual waiting separately on hardware. Implemented and pending cases are identified in the runtime report.

Test third-party SDKs separately when required. Same-request cold starts and emergency interruption retain the same contracts for every client.

## 8. Verification and measurements

`make check`, client tests, live synthetic HTTP, and real-child fixtures cover their recorded control paths. Native cancellation, unloading, peaks, and application impact remain T6-T8 work.

Use controlled clocks, fake engines, and test-only telemetry for pressure injection. Keep injection out of default administrative endpoints and avoid exhausting real host memory to test reclamation.

A native report records model revision/digest, actual engine version, input/output tokens, context, CPU/GPU path, starting pressure, cold start, queue time, first token, execution duration, and post-unload state. Complete the small-model lifecycle before increasing model size or context, changing one experimental variable at a time.

## 9. Current acceptance status

T0-T5 have implementations and recorded-path validation. T7 passed on the synthetic backend. Warning pressure and insufficient headroom blocked the recorded T6 load. Remaining work is the native lifecycle, Python native acceptance, and application comparison.

Platform compatibility, backend evidence, memory peaks, and responsiveness require their own results. A failed measurement leads to an implementation or calibration fix, without silently expanding scope into multiple hosts, tool calls, or cloud fallback.

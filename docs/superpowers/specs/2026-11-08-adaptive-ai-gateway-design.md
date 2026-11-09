# Adaptive AI Gateway: initial design

Drafted: 2026-11-08. Architecture revised: 2026-11-09 (Asia/Shanghai). Status: accepted baseline with the control layer implemented; real-model validation and parameter calibration remain pending. See the [design decisions](../../plans/2026-11-08-implementation-review.md) and [module architecture](../../architecture/overview.md).

## 1. Problem and scope

Local inference competes with browsers, editors and video calls for resources. A stable endpoint should admit model requests when capacity permits, queue or reject them with a clear reason under pressure, and release model memory when idle.

The first version covers one host, one managed model and one local engine. It accepts the supported subset of text chat requests and returns JSON or SSE output with resource-decision metadata. Users provide model weights; a downloader, chat client and desktop interface are outside the initial requirements.

The first client is a Python script. Deliver a callable interface, reproducible examples and machine-readable errors, verified through asynchronous HTTP. A third-party SDK or a new standalone SDK is not a prerequisite. The server remains a Go control process.

When the model is unloaded, the generation request waits for a bounded cold start, then continues on the same HTTP connection. Normal loading does not require polling or retrying the request.

Validate the full lifecycle with a small model before evaluating the target model. The pinned official [validation baseline](../../research/small-model-baseline.json) has been downloaded and verified. Host resource protection has blocked actual generation. Artifact size is neither a memory requirement nor a quality guarantee.

The initial scope is admission between requests, bounded queuing and lifecycle control. Multiple hosts, enterprise tenancy, training, cloud fallback and CPU/GPU placement changes during generation require separate designs.

## 2. Technical approach and components

The [upstream comparison](../../research/upstream-comparison.md) evaluates three approaches. The selected path is a Go control process with an independent native `llama-server` / Metal child process.

| Component | Responsibility | Input and output | Dependencies |
| --- | --- | --- | --- |
| Gateway | Validate HTTP, encode semantic output and propagate cancellation | Request → application command carrying a request ID | Application use cases; no direct engine ports |
| Telemetry | Collect and normalize resource observations | System state → timestamped snapshot with capability flags | Platform collectors |
| Policy | Decide whether to admit new work | Snapshot, model state and reservations → decision and reason codes | Explicit time and policy configuration |
| Application Coordinator | Own waiting, budgets, capacity and protection state | Commands and evidence → reservations, state or errors | Consumer-owned ports and explicit time |
| Engine Supervisor | Coordinate loading, health, draining and exit | Desired state → observations and completion evidence | Concrete adapters reached through application lifecycle ports |
| Event Recorder | Record decisions and runtime statistics | Structured events → bounded in-memory records and logs | Stable event structures |

Use TOML configuration, in-memory state and structured logs. PostgreSQL, Redis and NATS are unnecessary for this scope. Evaluate SQLite later if persistent history is needed. Upstream responsibility boundaries inform the design; module size follows the initial requirements.

## 3. Request and control flow

1. Validate the model identifier, body size, deadlines and declared capabilities.
2. Read the latest snapshot and assess freshness from its collection time.
3. Check resources, existing reservations, concurrency and queue capacity in one atomic admission path so concurrent requests cannot claim the same capacity.
4. Grant a reservation when the model is ready and policy permits. Otherwise, enter bounded waiting or return a reason.
5. Coordinate one shared cold-start attempt. Waiters remain on their original HTTP requests and compete for execution after loading. New waiters do not reset the load deadline; cancelling one waiter preserves a load still needed by others.
6. Forward admitted work to the specified model and propagate client cancellation upstream. Release an execution reservation exactly once after matching engine-completion or owned-process-exit evidence. An HTTP disconnect does not prove computation has stopped.
7. Record queue time, execution time, available token usage and decision reasons. The control loop updates state and handles idle unloading.

Preserve the real model ID requested by the client. A future logical alias that permits quality changes needs an explicit contract and records of the actual model used. Do not silently substitute a model or shorten the required context.

## 4. Metrics and capacity

A snapshot includes collection time, system memory pressure, estimated available memory, CPU utilization, engine health, active requests, queue depth and per-field supported / unavailable / stale status. GPU utilization is optional on the initial macOS path.

Allow inference to use idle compute capacity. Distinguish total CPU, managed-engine CPU, controller CPU and estimated other workload CPU. High total utilization caused mainly by inference does not independently trigger throttling. Use a consistent sampling window and a whole-machine 0–100% scale.

Estimate other load from whole-machine and owned-process counters over the same window. This is not a direct measurement of foreground application delay. Changes in process ownership, mismatched windows or missing data make attribution uncertain; uncertainty does not establish that other applications are idle. Retain the single-execution baseline when attribution is unreliable. Do not interrupt generation or increase concurrency based only on total CPU. Measure effects on representative everyday tasks separately.

Unified-memory hardware has one physical capacity pool. Model estimates include weights, KV cache, compute buffers and process overhead; model-file size alone is insufficient. Verify the engine's actual context capacity after loading. Insufficient capacity fails loading rather than silently narrowing the client contract. External applications may allocate memory after admission, so capacity decisions include headroom and cannot prevent every out-of-memory condition.

Treat Metal's recommended working set as performance guidance. Name global GPU metrics, engine-process metrics and framework resource statistics separately.

Missing or stale required memory-pressure observations block new generation and model loads, including execution with an already resident model. Preserve active generation under its existing deadline. Unknown pressure alone is not critical-pressure evidence. Missing optional GPU utilization does not trigger Observation Guard; uncertain CPU attribution retains the single-execution baseline.

Initial observation rules:

- A sample older than 6 seconds is `telemetry_stale`. Query failure, unsupported sources and unmapped values are `telemetry_unavailable`. Do not replace reliable pressure observations with zero, a cached normal value or a free-memory percentage.
- Existing waiters may await recovery, and new requests may join if waiting capacity remains. Shared-loading wait consumes only that load attempt's budget; other observation-recovery wait consumes cumulative ordinary queue time. If a required observation still blocks work when ordinary waiting expires, return 503 with its telemetry reason rather than 429. The total deadline has precedence, and no budget resets.
- Observation failure alone does not interrupt active generation. An existing load may finish within its original deadline, but new generation waits for recovery. If demand disappears, follow the existing cancellation and cleanup rules.
- Require two consecutive fresh normal samples, then check model budgets and ordinary load conditions before readmission. These are initial calibration values. Warning uses ordinary protection; a fresh critical sample starts emergency reclamation. Later missing data does not revoke a deadline established by valid critical evidence.
- Observation Guard yields `can_execute_now=false`. `/readyz` indicates that the application can safely handle commands; an unloaded model or temporary protection does not itself make it unready. `/healthz` indicates process liveness. See [ADR-0004](../../adr/0004-service-readiness-and-capacity.md). Resource diagnostics retain the actual blocking reason.

## 5. Control states and hysteresis

| State | New-request behavior | Lifecycle action |
| --- | --- | --- |
| initializing | Wait on the same request for shared loading within the total deadline, then compete for execution | One startup and health-check sequence; joining does not reset the load deadline |
| ready | Admit subject to concurrency, resources and reservations | Retain the current configuration |
| constrained | Bound waiting under contention or Observation Guard; issue no new execution reservation while required memory observations are invalid | Preserve active generation; do not start an additional model |
| draining | Pause new reservations | Wait for active requests before unloading or reconfiguration |
| reclaiming | Stop admission; end waiters with a resource-pressure error | Cancel generation through bounded emergency reclamation and terminate the owned engine if required |
| unloaded | Check resources before demand-driven cold start | Retain no model residency |
| failed | Return an engine error with recovery state | Use bounded backoff rather than unlimited restarts |

This table summarizes observable behavior. Store engine lifecycle, protection and request state separately so conditions such as loading while constrained remain representable. See the [implementation plan](../../plans/2026-11-08-implementation-plan.md) for modules, accounting and tasks.

The current implementation samples every 2 seconds and marks data stale after 6 seconds. Warning immediately blocks new work; two fresh normal samples permit recovery checks. Model and application measurements must determine whether to relax this conservative behavior. Reliable estimated external CPU load at or above 85% blocks new work. Unreliable attribution retains single execution, and inference's own high utilization is not a blocking reason.

Ordinary CPU contention restricts new work only when supported by reliable other-workload evidence. Recovery windows prevent repeated switching; platform validation and application comparisons determine final thresholds. Preserve active requests under ordinary contention. Scheduling priorities or other ways to yield during execution require separate platform verification. Stopping new requests alone does not establish foreground responsiveness.

Under confirmed critical memory pressure, everyday applications take priority. The reclamation protocol uses candidate timings that still require measurement:

1. On a fresh valid critical observation, stop admission, reject new execution requests and end waiters with 503 / `memory_pressure`. Enter reclaiming.
2. If no generation is active, cancel incomplete loading or unload the idle model immediately rather than waiting for the idle timeout.
3. Give active generation a configurable short grace period, initially 5 seconds. A valid non-critical observation during grace withdraws reclamation that has not started and enters the appropriate protection or recovery state. Missing or stale data is not recovery evidence.
4. If relief is not confirmed by grace expiry, cancel generation. Allow up to 3 seconds for cooperative completion, then force termination of the owned engine if completion remains unconfirmed. This budget bounds when forced exit is initiated; it does not guarantee that the OS has freed resources by that time.
5. Once cancellation is issued, finish reclamation even if pressure improves. Do not report success or silently resume the interrupted request. Generation completion still requires model unload or process exit. Release execution reservations on matching work-completion or exit evidence, residency on unload or exit evidence, and incomplete load reservations only on confirmed cleanup.
6. Report `memory_pressure_interrupted`: 503 before output begins, or the declared stream error after streaming starts. Do not automatically retry the call.
7. Keep the model unloaded until fresh observations satisfy recovery conditions. The initial proposal is 30 continuous seconds of normal memory pressure plus the other validated admission requirements. Recovery does not require a fixed low total CPU utilization.

These timings are reproducible experiment defaults, not measured host guarantees. Switching to CPU inference under high CPU load is not a general fallback; first reduce new work.

For ordinary GPU-layer, thread-count or context changes, drain active requests before restarting the owned engine. Emergency reclamation is a separate interruption path. The initial version manages its own child processes. A future attached external service does not grant process-termination authority.

## 6. Interfaces and initial configuration

HTTP behavior is implemented and verified with a synthetic backend. Real-model validation awaits safe host resource conditions.

| Endpoint | Purpose |
| --- | --- |
| `GET /healthz` | Controller liveness |
| `GET /readyz` | Whether initialization is complete and commands can be handled safely; request admission and diagnostics determine immediate capacity |
| `GET /v1/models` | Configured real model identifiers |
| `POST /v1/chat/completions` | Supported text chat requests with JSON or SSE output |
| `GET /admin/resources` | Current observations, sample age, missing fields and capacity estimates |
| `GET /admin/scheduler` | Model state, queue, reservations and recent decision reasons |

The controller defaults to `127.0.0.1:8787`; the managed engine uses a separately configured loopback port. All endpoints except `/healthz` require the same local Bearer token. The initial version has no multi-user permission layer. Default logs contain metadata and exclude prompts, responses and keys. See the [HTTP contract](../../contracts/http-api.md) for fields, status codes and stream termination.

Initial defaults are one loaded model, one active execution, at most four non-executing requests and a 300-second idle-unload timeout. Example configurations may override the idle timeout to 30 seconds. Every waiter, including the request that triggers cold start, counts toward the same bounded capacity. The body limit is 1 MiB. The default and maximum output are both 512 tokens; larger requests are rejected. Pin model and context capacity against the engine version and measured requirements rather than treating all 8 GiB as available model memory.

Use separate configurable time budgets. State transitions never restart them:

| Budget | Initial value | Accounting and boundary |
| --- | --- | --- |
| Ordinary execution queue | 30 seconds cumulative | Resource admission, draining and execution-slot waiting; excludes wait on an active shared model load |
| Shared model load | 120 seconds per attempt | Starts when loading begins; all waiters share the deadline, and later arrivals receive only the remaining time |
| Generation execution | 120 seconds | Starts at engine dispatch and includes prompt processing, first-token wait and complete output |
| Total request | 270 seconds | Starts when a valid request is accepted and spans all phases; requeueing and observation recovery never reset it |

A normal cold start longer than 30 seconds can therefore continue to generation on the same request. Ordinary waiting before and after loading is cumulative. A failed call does not automatically enter a second load attempt or retry generation. Expiry of the total or applicable phase budget ends waiting; simultaneous expiry records the total-timeout reason first.

Client disconnect removes the waiter and propagates cancellation. Preserve a shared load while another waiter needs it. When the last demand leaves, cancel the unnecessary load and clean up the owned process. Retain its load reservation until cleanup is confirmed. Engine recovery serves later requests and does not extend a failed call.

Do not send success status or custom loading SSE events during admission, loading or ordinary queuing. Preserve the ability to return an ordinary HTTP error. The response may have no body data until generation begins. Integration tests must cover the longest valid wait against the selected client's first-byte and total timeouts. The example uses a 300-second outer deadline and network timeouts that permit the server's valid waiting period. A read timeout is not a total-call deadline.

Support only the API subset covered by contract tests. Unknown parameters are rejected under the current HTTP contract. Any future passthrough requires documented rules.

The initial client acceptance path combines raw HTTP and an asynchronous Python script. The design proposed an isolated Python 3.11+ environment and HTTPX 0.28.1; the implementation pins the interpreter and dependencies. Verify JSON, streaming chunks, cancellation, queue rejection, loading waits and emergency interruption. Set total and network-phase timeouts explicitly, disable automatic retries and close streams on cancellation. Errors must be observable programmatically. Verify the selected client's error and stream handling rather than assuming arbitrary SDK compatibility.

An error or unexpected EOF after partial output fails the call. Retain an incomplete marker and the boundary of received content; do not merge it into a completed result. Reaching the output-token limit is a normal finish reason, distinct from reclamation interruption. See the [Python client plan](../../plans/2026-11-08-implementation-plan.md#7-python-client) for dependencies, timeouts and checks.

## 7. Errors, cancellation and recovery

- Invalid or unsupported requests: 400. Unknown model: 404. Oversized body: 413.
- Full waiting capacity: 429 / `queue_full`. Ordinary slot or draining wait expiry: 429 / `queue_timeout`. Provide `Retry-After` only with a reliable estimate.
- Required telemetry, memory or other-load protection that outlasts waiting: 503 with the appropriate `telemetry_unavailable`, `telemetry_stale`, `memory_pressure` or `cpu_busy` reason.
- Unavailable engine or failed loading: 503 with `engine_unavailable` or `model_load_failed`. Shared loading expiry: 504 / `model_load_timeout`.
- Total request expiry: 504 / `request_timeout`. Invalid upstream execution response: 502. Execution expiry: 504 / `execution_timeout`. After streaming starts, use the stream-error contract because the HTTP status is already committed.
- Never automatically retry a call after SSE content has been emitted. Emit the declared stream error and close the connection to avoid duplicate content.
- Propagate client cancellation promptly. If the engine ignores cancellation, follow the declared recovery strategy and affect only owned processes. Retain execution occupancy until completion is confirmed.
- Critical-pressure reclamation follows [ADR-0001](../../adr/0001-prioritize-everyday-apps-under-critical-pressure.md). It may interrupt generation and terminate the owned engine. Distinguish admission rejection from interrupted generation, with no silent retry.
- An engine crash fails the current request. Release reservations on confirmed exit evidence. Restart for later requests with bounded backoff; after the configured failure limit, remain failed until explicit recovery.

## 8. Verification and acceptance

Verify control logic with a synthetic engine and controlled time, then run separately identified end-to-end measurements with the pinned small model.

| Check | Acceptance evidence |
| --- | --- |
| Concurrent admission | Parallel requests do not over-allocate reservations or duplicate queuing and loading actions |
| Client cancellation | Remove queued work or cancel upstream promptly; an engine that ignores HTTP cancellation keeps its execution reservation until confirmed completion; release once |
| Pressure and recovery | Critical memory immediately blocks new admission; normal recovery, stale data, grace expiry and recovery after cancellation have explicit outcomes |
| Idle compute use | With normal memory and low other load, inference causing 90% total CPU does not trigger throttling; uncertain attribution does not report idle capacity |
| Ordinary application contention | Reliable high other load restricts new work; effects on active requests remain distinct from emergency reclamation, and actual yielding is measured separately |
| Emergency reclamation | Initiate termination of a cancellation-ignoring synthetic engine by the force-exit deadline; retain reservations until exit evidence; leave attached processes unaffected |
| Missing metrics | A loaded model still pauses new execution; preserve active work; recovery does not reset budgets; persistent failure ends with the correct 503 reason; missing optional GPU utilization permits operation |
| Observation Guard recovery | Test consecutive recovery samples, warning, critical and missing data after prior critical; unknown pressure alone never forces interruption |
| Draining and reconfiguration | Ordinary reconfiguration waits for active work; only the emergency path permits deliberate interruption |
| Queue and deadlines | Distinguish a full queue, ordinary wait expiry, cold-start expiry and execution expiry |
| Same-request cold start | A 45-second load without ordinary queuing can proceed to generation without hitting the 30-second queue budget |
| Shared loading and cancellation | Multiple requests trigger one load; one cancelled waiter preserves others; removing the last demand cleans up loading without early reservation release |
| Cumulative deadlines | Protection, recovery and requeueing retain used budgets; later waiters do not reset shared loading; total expiry prevents further generation for that request |
| Streaming | Preserve chunk order and cancellation, with no automatic whole-call retry after output |
| Script and client calls | Verify responses, streams, cancellation and failures through raw HTTP and the selected client; record versions, timeouts and retry settings |
| Host resources | Measure controller and engine memory separately; idle controller RSS below 100 MiB is a design target until measured |
| Application benefit | Compare fixed and resource-aware operation with the same model and context; report representative foreground-task times and inference experience without a predetermined improvement percentage |

## 9. Commercial and release direction

Validate the free local version's benefit first. Professional desktop features and team governance are later product hypotheses in the [pricing and business study](../../research/pricing-and-business.md). Billing, subscription checks and token markups are outside the initial version.

The repository contains original design and implementation, without copied upstream inference code. It uses Apache-2.0. Configuration dependencies and downloaded validation artifacts are pinned; models retain their own licenses. Commercial product features are separate from open-source license terms.

## 10. Implementation status

The scheduler, HTTP layer, platform collectors and engine adapter are implemented. Synthetic-backend and child-process fixtures passed. Real-model generation and everyday-application comparisons remain pending; see the [runtime report](../../reports/2026-11-09-runtime-validation.md). Thresholds and model peak-memory estimates still require measured calibration.

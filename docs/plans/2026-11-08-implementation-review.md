# Initial design decisions

Drafted 2026-11-08; archived 2026-11-09 (Asia/Shanghai). The seven product decisions below define the initial scope. Current implementation follows the architecture and implementation plan; later results are in the [runtime report](../reports/2026-11-09-runtime-validation.md).

References: [design](../superpowers/specs/2026-11-08-adaptive-ai-gateway-design.md), [milestones](../../ROADMAP.md), [glossary](../../CONTEXT.md).

## Decision sequence

Product trade-offs determine the public behavior; interface contracts, state, accounting, and acceptance tasks follow. Technical facts come from repository inspection, host checks, and official documentation.

| Order | Decision | Dependency | Outcome |
| --- | --- | --- | --- |
| 1 | Everyday applications versus active generation under critical pressure | Defines interruption and error semantics | Everyday applications take priority; emergency interruption is allowed |
| 2 | Observable criteria for ordinary contention | Defines CPU attribution and recovery | Use idle capacity; validate attribution and application impact |
| 3 | First real caller | Defines cold-start, error, and cancellation acceptance | Scripts/SDK-style programmatic calls |
| 4 | Cold-start waiting | Defines whether normal loading needs client polling/retries | Wait and generate on the same request |
| 5 | Model/context/budget measurement | Defines initial hardware scope and estimates | Validate a small model first; measure the budget |
| 6 | First client language | Defines timeout, cancellation, and streaming tests | Python |
| 7 | Missing required memory pressure | Defines availability under uncertain protection | Pause new generation, retain active work, recover from reliable observations |
| 8 | Admission, cancellation, draining, recovery accounting | Implements the preceding choices | Explicit concurrency and lifecycle contracts |
| 9 | Modules and acceptance gates | Depends on behavior and failures | T0-T8 plan and HTTP contract |

The initial architecture was a Go control process with a native llama.cpp backend. Managed and attached engines have different lifecycle authority. The glossary distinguishes those terms from admission, draining, and emergency reclamation.

## Risks addressed by the decisions

1. **Reclamation latency:** waiting for a long generation could preserve pressure for 120 seconds. A bounded grace period followed by cancellation/managed exit replaces that rule. The 5-second grace and 3-second cooperative-exit values require hardware validation.
2. **Owned load in total CPU:** remove the original total-CPU 85%/65% thresholds. Measure owned and external work separately, show uncertainty, and measure actual yielding effects. Stopping new work alone does not establish foreground responsiveness.
3. **Cancellation versus release:** HTTP closure does not release execution occupancy. Require engine completion or managed-process exit, validated through the adapter.
4. **Cold-start timing:** track shared loading separately from ordinary queuing, with a total deadline and explicit last-waiter cleanup. Validate client first-byte and overall limits.
5. **Memory double counting:** distinguish loading, residency, execution slots, and request caches across the same physical pool.
6. **Missing observations:** required-memory failure blocks new work even for a resident model. Define bounded waiting, 503 reasons, recovery samples, and precedence for an existing critical path. Optional GPU gaps do not apply the same guard.

## 1. Critical memory pressure

Scenario: a video call is active while a model generates a long answer and the system reports critical memory pressure.

Prioritize everyday applications and permit emergency interruption. [ADR-0001](../adr/0001-prioritize-everyday-apps-under-critical-pressure.md) records the trade-off. Draining means waiting for natural completion; emergency reclamation can interrupt work. Grace, error encoding, and recovery criteria are engineering parameters to validate.

## 2. Idle compute

Scenario: memory is normal, other applications have little load, and inference accounts for system CPU usage of 90%.

Allow inference to use the available capacity while retaining memory protection. Reduce new work when other applications need resources. Total CPU alone does not establish external contention. Process attribution, uncertainty, yielding mechanisms, and application impact require measurements.

## 3. First client

Validate programmatic calls, first over raw HTTP and then through Python. Cover cold-start waits, streaming errors, and cancellation. A chat-client integration or standalone SDK is outside the initial prerequisite set. Network libraries differ in timeout, retry, and stream-error behavior, so broader SDK compatibility requires separate tests.

## 4. Cold-start waiting

Scenario: a script sends the first request to an unloaded model, and a healthy cold start can exceed the original 30-second queue limit.

Keep the request open for loading and then generate, with deadlines and cancellation. Initial configurable budgets are 30 seconds of cumulative ordinary waiting, 120 seconds per shared load, 120 seconds of execution, and 270 seconds total.

Waiters share one load deadline. Joining and state transitions do not reset it. One cancelled waiter leaves a load running for remaining demand; the last valid demand triggers cleanup.

Required work includes explicit request phases, one load owner/deadline, bounded waiters, cancellation and critical paths, simulated loading beyond 30 seconds, late joins, and correct errors before output. Client first-byte and total timeouts require integration checks. These tasks were added to the implementation plan.

## 5. Small-model validation

The first host has 8 GiB of memory. Validate the lifecycle with a small quantized text model before working on a target application model.

At review time, the official Qwen2.5-0.5B-Instruct-GGUF Q4_K_M metadata and pinned Apache-2.0 license had been checked, but weights had not been downloaded. [Baseline metadata](../research/small-model-baseline.json) tracks subsequent verification. Memory, throughput, and unloading need measurements; answer quality is a separate concern.

The plan covers modules, shared loading, state axes, accounting, sampling, native execution, and client acceptance. Host queries and XNU source established that exported pressure values 1/2/4 differ from the kernel's internal enum.

## 6. Python client

Use Python for the first example. The review proposed an isolated Python 3.11+ environment, pinned HTTPX, an outer request deadline, and explicit stream cleanup. HTTPX read timeouts bound waiting for a chunk rather than the entire call. Implementation and later results belong to the runtime report.

## 7. Observation failure

Scenario: the model is resident and active generation is healthy, but required memory-pressure data is unavailable or stale. Optional GPU utilization is outside this condition.

Pause new generation and preserve active work. Resume only after reliable recovery observations and other admission requirements are met. Waiters that outlast the existing budget receive the corresponding telemetry 503. Unknown pressure alone does not force interruption; an earlier valid critical path retains its priority.

Ordinary waiting, shared loading, and total time remain separate budgets through recovery. The glossary names this state Observation Guard.

## Engineering baseline

The [design](../superpowers/specs/2026-11-08-adaptive-ai-gateway-design.md), [HTTP contract](../contracts/http-api.md), and [implementation plan](2026-11-08-implementation-plan.md) form the accepted baseline. Library selection and tunable durations are engineering parameters to verify. Record material architectural trade-offs in ADRs and domain definitions in the glossary; keep measured values and implementation details in their respective specifications and reports.

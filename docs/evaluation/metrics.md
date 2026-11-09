# Metric definitions

Contract: `desktop-eval-v1`. Apply these definitions to every result and imported tool summary. Field names below describe the evaluation record; they do not add fields to the public HTTP API.

## 1. Population and measurement window

Declare an arrival window `[S, E)` using the collector's monotonic clock. The measured cohort contains every valid request scheduled in that window. Warmup, invalid-request probes and deliberately cancelled functional probes are separate labeled populations. Never remove a request because it was refused, timed out or interrupted by policy.

At `E`, stop arrivals and allow measured work to reach its existing request/client deadline. Record terminal outcome for every request; unanswered work remains incomplete at the declared observation cutoff. Define `F` as the latest cohort terminal or cutoff time, and `W = max(E, F) - S`. Headline cohort rates use this same duration including drain. Report arrival duration and drain duration separately. Do not import event-window rates as cohort rates.

With no scheduled requests, inference fractions and rates are not applicable. A nonpositive window, impossible timestamp order or an undisclosed early send invalidates timing evidence. When unfinished work remains, use the declared cutoff in `F`; do not shorten the denominator to the final successful response.

Partition the cohort into mutually exclusive outcomes:

| Outcome | Definition |
| --- | --- |
| completed | A valid response reaches the normal JSON or SSE termination contract |
| refused | Admission rejects the request, including queue capacity, telemetry or resource protection |
| timed_out | A declared server or client deadline expires |
| interrupted | The system deliberately ends active generation, including emergency reclamation |
| cancelled | The caller explicitly cancels, independently of deadline expiry |
| error | Transport, protocol, engine or other failure without a more specific terminal cause |
| incomplete | No terminal outcome is established by the observation cutoff |
| not_sent | Scheduled demand was never dispatched by the harness |

The counts must sum to the cohort size. Prefer an established specific cause over generic transport EOF; otherwise preserve uncertainty and its evidence. Record partial output as an independent flag. A completed transport response may still fail the workload's fixed validity predicate. Such a request remains in completed counts but is excluded from useful completions.

`not_sent` is a harness result, not a server refusal. Intended functional cancellations stay outside performance cohorts; cancellation caused by excessive wait stays visible in the affected workload. Planned and actual request totals must both be published.

## 2. Clocks and timing

Use one client monotonic clock for all timestamps in a request's client record. Keep wall time only for correlation. Do not subtract monotonic values from different processes or hosts without a measured clock relationship and error bound.

| Timestamp | Definition |
| --- | --- |
| target | Scheduled arrival time, fixed before sending |
| sent | Client begins the request operation, including any measured client-pool wait |
| first_content | First parsed event with non-empty supported generated text |
| last_content | Last such generated-content event |
| verified_end | Normal completion is validated: full JSON, or normal SSE finish followed by `[DONE]` |
| outcome_end | Terminal failure/cancellation/timeout, or the incomplete observation cutoff |

Role-only events, empty deltas, headers, heartbeats and loading notices are not content. A failed stream can have first/last content but no verified end. Never convert last content or EOF alone into completion.

| Metric | Formula or measurement | Unit and population |
| --- | --- | --- |
| dispatch lag | `sent - target` | ms; all sent requests |
| client TTFT | `first_content - sent` | ms; requests with actual content |
| scheduled TTFT | `first_content - target` | ms; headline arrival-based latency objective |
| content latency | `last_content - sent` | ms; content-bearing requests; not proof of completion |
| completion latency | `verified_end - sent` | ms; normally completed requests |
| scheduled completion latency | `verified_end - target` | ms; headline arrival-based completion objective |
| failure latency | `outcome_end - target` | ms; each unsuccessful outcome separately |
| chunk gap | Time between successive non-empty parsed content events | ms; aggregate with chunk and request counts |
| TPOT | `(last_content - first_content) / (output_tokens - 1)` | ms/token; completed requests with reliable token counts greater than one |

Publish success and failure distributions separately. A missing TTFT after rejection is expected and does not make the known refusal indeterminate. A single content chunk can contain multiple tokens; chunk gaps are not per-token intervals. TPOT is a request-level amortized value, not an observation of every token arrival. With fewer than two tokens, TPOT is not applicable.

## 3. Useful capacity

A useful completion must pass the termination contract and the predeclared workload-validity predicate, then satisfy every configured latency objective. Record whether that predicate checks structure or semantic task quality. Structural validity does not establish answer accuracy.

```text
completion_fraction = completed / cohort_size
useful_attainment = useful_completions / cohort_size
cohort_goodput = useful_completions / W_seconds
refusal_fraction = refused / cohort_size
interruption_fraction = interrupted / cohort_size
```

A known refusal, timeout, interruption or error contributes no useful completion. If objectives are absent, required fields are missing for otherwise potentially useful completions, or the harness failed to deliver the profile, headline goodput and attainment are unavailable and the benefit verdict is INSUFFICIENT. Report undetermined counts. An optional diagnostic lower bound counts known useful requests; an upper bound additionally counts undetermined requests. Do not label either bound as measured goodput.

Obtain token counts from documented backend usage or a pinned model-compatible tokenizer and template. Record the source and whether it is measured or estimated. Unknown counts stay null. Do not replace tokens with bytes, words or SSE chunks. Token throughput, when available, uses completed output tokens divided by the same `W`; partial-output tokens are reported separately.

## 4. Foreground benefit

For each task and paired block, compute `foreground_p95_slowdown_C = p95_task_C / p95_task_A - 1`; also report B/A and C/B. Keep absolute task times and failed/dropped task counts. If A has zero/invalid time, missing completions or incomparable state, the ratio is unavailable.

Define task completion before measurement. Browser tests may use input-to-visible-result timing in a pinned local fixture; compilation or lint tests use a fixed project and cache state. Video metrics require an instrumented frame/audio deadline, not subjective observation. Keep task failures in the result so dropping difficult actions cannot improve p95. Do not average unrelated task ratios into a benefit score.

A failed or missing reference action invalidates the paired task comparison. Unexpected C task failures prevent a foreground PASS even when successful-action p95 is low. Any permissible failure budget must be declared before the experiment and evaluated separately from latency.

For matched idle inference runs, `idle_goodput_retention = goodput_C / goodput_B`. B must have positive measurable goodput with the same objectives and workload. Report ratios per block, uncertainty, and absolute counts/rates. A zero or unknown baseline produces an unavailable ratio, never 100% retention.

## 5. Resource effects and lifecycle

| Metric | Measurement and limits |
| --- | --- |
| Pressure exposure | Sampled normal/warning/critical durations plus unknown duration. Hold a sample only within its declared freshness limit; retain sampling uncertainty and gaps. This is not continuous OS ground truth. |
| Available-memory estimate | Bytes with source, quality and observation age; the current free-plus-speculative estimate is not all reclaimable memory. |
| CPU attribution | Whole-machine 0–100% total, owned and estimated-other utilization over the same window; retain reliability. High owned load alone does not indicate a bad policy. |
| Memory | Controller and engine peak/idle RSS or explicitly named OS footprint. Keep measurement types distinct. Shared CPU/GPU memory is one physical pool; never sum overlapping metrics. |
| Swap | Counter delta over a known interval with units and reset handling; starting allocated swap is not swap caused by the experiment. |
| Admission reaction | Valid critical observation accepted by the coordinator → new work blocked. Record sensing/collection delay separately where observable. |
| Reclamation | Record grace start, cancellation issued, forced-exit initiated, matching execution completion, process exit and post-exit memory observations separately. |
| Recovery | Valid recovery conditions satisfied → admission reopened; distinguish policy-required waiting from additional delay. |
| Churn | Loads, unloads, restarts, emergency interruptions and protection transitions per hour, with reason and workload demand. |
| Energy | Optional measured joules per useful completion, with baseline power and measurement boundaries; unavailable measurement is not zero energy. |

Configured cancellation/force-exit budgets bound actions, not the instant the OS returns memory. A closed response is not release evidence. Assess matching instance/execution evidence and actual process state separately. Aggregate administrative polling can bound a transition interval but cannot establish its exact time.

## 6. Control overhead and uncertainty

Measure active control processing with spans that exclude ordinary queueing, shared loading, model execution and output waiting. Without those spans, an equivalent synthetic direct/gateway experiment can measure added client latency, labeled with that scope. Subtracting unrelated p95 values does not produce p95 overhead. Prefer paired per-request differences or direct spans.

Record collector CPU/memory and sampling cadence. Report observability-enabled diagnostic runs separately if instrumentation changes the workload. Retain all per-run counts, medians, p95 values and sample sizes; calculate the protocol's uncertainty over paired blocks. Missing required fields carry a reason such as unsupported, unavailable, stale, not instrumented or not executed, rather than a numeric zero.

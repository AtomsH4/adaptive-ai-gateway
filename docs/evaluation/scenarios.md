# Evaluation scenarios

Protocol: `desktop-eval-v1`. This matrix specifies required observations. Links to existing tests identify starting evidence, not a claim that every row has full coverage or was rerun for this document.

## 1. Control and fault scenarios

Run these at L0 using controlled time, deterministic resources, a synthetic backend and owned child-process fixtures. Record each branch and evidence path separately. Native measurements do not replace these cases.

| ID | Stimulus | Required result | Existing starting point |
| --- | --- | --- | --- |
| C01 | Concurrent demand with an active execution and a full waiting queue | At most one execution and four waiters under the default profile; excess work has an explicit outcome; no duplicate load | [Coordinator tests](../../internal/application/coordinator_test.go), including parallel capacity and cold-start waiter tests |
| C02 | Shared cold start lasting 45 simulated seconds with a 30-second ordinary queue budget | The same request can continue after loading; shared-load time does not consume ordinary queue time; one load attempt | `TestSharedColdStartDoesNotSpendOrdinaryQueueBudget` in coordinator tests |
| C03 | Cancel one shared-load waiter, then remove the final remaining demand | Preserve loading needed by others; cancel an unused load; retain its reservation until cleanup evidence | Coordinator and [backend tests](../../internal/adapter/llamacpp/backend_test.go); verify each branch explicitly |
| C04 | Cancel an active HTTP caller while the engine ignores connection cancellation | End caller demand promptly, request cancellation, retain execution occupancy until matching completion or owned exit | `TestCancellationDoesNotReleaseUntilOwnedExitEvidence`; `TestHTTPDisconnectCannotProveCancelledComputeReleased` |
| C05 | Duplicate release, wrong execution, retired incarnation or reused process identity | Release at most once for matching work; old or foreign evidence cannot release current capacity or authorize termination | [Domain evidence tests](../../internal/domain/runtime_test.go) and backend fixtures |
| C06 | Unavailable, stale, future-dated or unmapped required memory data while the model is resident | Block new loading/execution, preserve active generation under its deadline, retain budgets and report the telemetry cause | [Policy tests](../../internal/policy/resourceguard_test.go), reader-failure and coordinator observation tests |
| C07 | High owned CPU, high reliable other CPU, then uncertain attribution | Owned utilization alone does not throttle; current reliable external threshold applies; uncertain attribution is not reported as idle or critical | [Load policy tests](../../internal/policy/load_test.go) and resource-guard tests |
| C08 | Fresh critical pressure with no active work, with active work, recovery during grace, then missing data or recovery after cancellation | Stop new work; apply the appropriate cleanup/grace path; unknown data does not revoke valid critical evidence; issued cancellation completes reclamation and reports interruption | `TestCriticalEscalatesDespiteMissingObservationThenRequiresRecovery`; add branch-specific evidence where absent |
| C09 | Repeated reads of one normal sample, two distinct fresh samples, warning/critical transitions and post-critical recovery | Reads do not count as new samples; recovery uses the configured fresh-sample/window rules and does not flap or reset request budgets | `TestRepeatedDiagnosticQueriesDoNotCountAsRecoverySamples` and coordinator recovery tests |
| C10 | Requeue through resource/slot/observation waits; join a late shared load; expire coincident deadlines | Ordinary waiting is cumulative; shared deadline is not reset; total deadline takes precedence; failed calls do not silently retry | Shared-load deadline and observation-budget coordinator tests; retain explicit combined-case coverage |
| C11 | Normal SSE, arbitrary transport fragmentation, partial output then error/EOF, slow reader and cancellation | Preserve event order; require normal finish plus `[DONE]`; fail incomplete streams; bounded output cannot stall control; no whole-call retry | [HTTP tests](../../internal/adapter/http/handler_test.go), [client tests](../../examples/python/test_client.py), API fixtures; coverage is branch-specific |
| C12 | An owned child and a reachable but attached/foreign service | Stop only an owned instance with matching identity; unsupported attached capabilities are rejected; no foreign process is killed | Backend foreign-owner checks, domain evidence and configuration tests |
| C13 | Engine crash, repeated failed starts, normal shutdown and ambiguous cleanup | Fail affected calls; bounded restart/backoff; reject new intake during shutdown; retain unresolved occupancy; no promise of crash-resumed requests | Backend child fixtures, coordinator and [configuration tests](../../internal/adapter/config/config_test.go); full crash-recovery journal is a later stage |
| C14 | Unloaded/busy/protected service, bad token, invalid/unsupported body and unknown model | Readiness remains distinct from execution capacity; documented auth/validation/errors; no premature HTTP success during loading/waiting | `TestHTTPAuthenticationStrictRequestAndReadiness`, contract fixtures and live checks |

For critical reclamation, retain times for observation acceptance, admission stop, grace expiry, cancellation, forced-exit initiation, execution completion and process exit. Check configured action budgets independently of actual OS memory return. A stopped HTTP stream cannot satisfy the exit requirement.

Faults must be injected through tests or an isolated fixture. Administrative resource endpoints remain read-only. Do not lower production memory guards, allocate memory until the host fails, terminate attached engines, or expose a default HTTP endpoint for pressure injection.

## 2. Native workload matrix

Use the pinned validation model first. Initial native probes use context 2048, one execution, a common requested output cap no greater than 128, and the same fixed parameters in B and C. Other profiles need their own identities and results. The public 512-token maximum does not require every initial probe to request 512 tokens.

| ID | Workload and groups | Measurements | Acceptance purpose |
| --- | --- | --- | --- |
| W01 | Warm model, otherwise idle host; B/C | Scheduled/client TTFT, completion latency, goodput, reliable token rate, controller/engine cost | Detect excessive idle admission loss and overhead; compare idle goodput retention |
| W02 | Model initially unloaded; B/C with equivalent startup boundaries | Shared/native load time, first content, validated completion, peak memory and cleanup | Establish actual cold-start lifecycle; keep results separate from W01 |
| W03 | Replayed interaction with a pinned local browser/editor fixture; A/B/C | Input-to-visible-result latency, completed/failed actions, inference goodput and pressure | Measure an interactive foreground task rather than inferring responsiveness from CPU |
| W04 | Fixed compilation/lint or other reproducible CPU task; A/B/C | Task completion latency, throughput/failures, CPU attribution, inference goodput | Measure CPU contention using fixed source, toolchain and cache state |
| W05 | Foreground demand increases, holds and recovers on a fixed schedule; A/B/C | Foreground timing, protection transitions, waiting/refusals, recovery delay, useful capacity | Evaluate adaptation and recovery; preserve the same timeline in every group |
| W06 | Seeded burst and fixed/Poisson arrival traces; B/C | Target versus actual arrivals, queue/wait distributions, every terminal outcome and cleanup | Verify bounded load behavior and offered-demand accounting; do not label lower delivered traffic as better latency |
| W07 | Demand followed by idle intervals before/after the configured unload timeout; B/C | Residency, unload confirmation, reload count, subsequent cold start and controller cost | Quantify memory release versus cold-start/churn cost using identical demand |
| W08 | Sustained mix and repeated lifecycle/cancellation cycles; B/C as applicable | Drift, leaks, outcomes, pressure, reclamation evidence and repeated-run stability | L4 reliable-host gate, separately reported from short-run benefit |

For W03/W04, require one interactive and one CPU-bound foreground fixture before a general desktop-benefit claim. A narrower report can claim only the task actually measured. Define the exact fixture/action sequence and its failure detector before running; no foreground automation is provided by this documentation.

W05 must stay within the shared native safety envelope. Model critical-memory behavior with C08 fixtures. If a native run hits the safety abort, retain its partial trace and mark the paired comparison INSUFFICIENT.

Replay a fixed schedule or a seeded trace in W06. Do not choose an arrival rate from a slow group's observed completion rate. Log any harness concurrency limit and its dispatch lag. Queue overflow is an observable outcome, not a reason to omit a request.

## 3. Workload controls

- Use public versioned inputs and predeclared validity predicates. Record input token classes and actual output counts where reliable. Preserve model identity, context and output caps across groups.
- Freeze warmup, model residency, prefix reuse, filesystem/build-cache policy and connection reuse. Cold model state does not automatically mean a cold filesystem cache; name each condition.
- Use the same foreground actions and scheduled inference demand in each paired block. Record power/thermal state and restore the declared state before the next group.
- Complete at least five balanced/randomized blocks and the protocol's sample floor before a p95 benefit verdict. Keep per-block distributions; an underpowered comparison is INSUFFICIENT.
- Keep calibration and holdout workloads separate. Optional policy ablations must preserve memory/ownership invariants and declare exactly which rule changes. Such toggles are not currently exposed as benchmark runtime options.
- Observe cancellation and cleanup after the arrival window. Preserve incomplete work and missed dispatches instead of truncating the trace to completed requests.

## 4. Engine and synthetic diagnostics

Engine prompt-processing/decoding microbenchmarks and S0/S1 synthetic overhead tests explain bottlenecks. They do not substitute for W01-W07 or establish a foreground benefit. Record native build/profile parameters and report engine, transport and control timing separately.

The current fake backend and `scripts/check_live.py` exercise HTTP behavior. They do not provide a matched synthetic direct/gateway load experiment, a foreground fixture, or an automatic resource-collection harness. Those capabilities require a later implementation.

## 5. Coverage record

For every C/W row, record verdict, test command or workload profile, source commit, artifact path, observed branches and remaining gaps. A link to a test file alone is not PASS for the entire row. Retain hardware runs separately from simulated-time tests and real-child fixtures that perform no model inference.

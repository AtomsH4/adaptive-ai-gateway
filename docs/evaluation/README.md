# Evaluation protocol

Protocol: `desktop-eval-v1`. Established 2026-11-09 (Asia/Shanghai). This protocol defines measurements and verdicts; it does not report a new benchmark or add an automated runner.

Evaluate useful inference capacity subject to correct resource ownership and acceptable foreground application impact. Use the [metric definitions](metrics.md), [scenario matrix](scenarios.md) and [report template](report-template.md) together. The [design](../superpowers/specs/2026-11-09-evaluation-design.md) and [upstream study](../research/evaluation-upstreams.md) explain the approach.

## 1. Evidence levels

| Level | Question | Required evidence | Limit of the claim |
| --- | --- | --- | --- |
| L0: Control correctness | Do admission, cancellation, budgets and ownership obey their contracts? | Deterministic tests, controlled time, protocol fixtures and real child-process fixtures | Does not prove native model performance or foreground benefit |
| L1: Gateway cost | What work does the control/HTTP layer add? | Equivalent synthetic direct and gateway paths, client timing and controller measurements | Synthetic throughput is not native inference throughput |
| L2: Native lifecycle | Does the pinned model actually load, generate, cancel and unload? | Engine/model/profile identities, valid terminal output, release evidence and host observations | Artifact verification or a correct refusal is not successful model execution |
| L3: Policy benefit | Does resource-aware operation improve the declared desktop trade-off? | Paired foreground-only, fixed-engine and resource-aware runs on the same workloads | One host/profile does not establish universal thresholds |
| L4: Reliable host | Does the result survive sustained operation and repeated lifecycle changes? | Long-running, cycle, failure and recovery evidence | Separate from L0-L3; follows the existing production release gates |

Report every level separately. A higher-level performance number cannot compensate for a failed ownership or accounting invariant.

## 2. Comparison groups

| Group | Configuration | Comparison purpose |
| --- | --- | --- |
| A: Foreground only | The foreground fixture runs without inference | Reference task latency, pressure and resource use |
| B: Fixed native engine | Pinned model and fixed serving parameters, one execution slot; no resource-aware admission | Native inference and foreground interference reference |
| C: Resource-aware gateway | Same engine/model/profile and request schedule through the gateway | Admission, queuing, reclamation, recovery and idle-unload effects |
| S0/S1: Synthetic pair | Equivalent deterministic backend output, direct and through the gateway | Isolate control/transport overhead from model work |

A has no inference denominator. Record B's native queuing behavior; queue/admission differences are part of the treatment. Do not add a client-side concurrency restriction that silently changes offered demand for one group.

For B and C, freeze engine version, model digest, context capacity, thread count, GPU layers, execution concurrency, request bodies, output caps, connection policy and arrival timestamps. Use the same foreground actions in A/B/C. Change only the policy being evaluated. Cold and warm runs are separate workloads. Preserve and report any unavoidable difference.

All native groups share an independent, recorded host-safety abort rule. Stop on confirmed critical pressure or a declared unsafe condition. Leave the gateway's own guards enabled. An aborted comparison is incomplete evidence, not a successful policy comparison. Exercise dangerous pressure and termination cases in controlled fixtures instead of exhausting real host memory.

## 3. Frozen workload profile

Complete these fields before measurement. Keep calibration runs separate from the comparison and retain a digest of the frozen profile.

| Field | Required content |
| --- | --- |
| Identity | Protocol version, profile ID/revision/digest, scenario ID, run/block ID, source commit and dirty-tree state |
| Host | OS/architecture, CPU/GPU and physical memory, power mode, starting thermal state, competing applications; exclude unique device identifiers |
| Model and engine | Model ID/revision/digest, engine release/build/digest, actual context, runtime profile and startup arguments with secrets removed |
| Input | Public fixture IDs and digests, prompt-length classes, exact output cap, structural or semantic validity predicate, token-count source |
| Traffic | Fixed or seeded arrival trace, duration, target count, actual-send logging and any client concurrency cap |
| Foreground task | Fixture version, replayed actions, task-completion definition, timing instrument and failure criterion |
| State | Warm/cold model state, filesystem/prefix cache policy, warmup and cooldown boundaries, prior idle time |
| Runtime policy | Effective queue/load/execution/request/idle budgets, memory budget, CPU threshold, recovery and cancellation settings |
| Objectives | Named latency limits with units and populations, foreground constraint, idle-capacity constraint and confidence rule |
| Safety | Common abort conditions, responsible owned-process cleanup, and post-run occupancy checks |
| Coverage | Required observations, sampling cadence, missing-data rules, artifact paths and retention policy |

Latency objectives are workload-specific. Set them before comparing B and C using a separate calibration or an explicit application requirement. The server's 270-second total deadline is an operational limit, not a default responsiveness target. Without configured and measurable objectives, goodput is unavailable.

## 4. Experiment procedure

1. Run the relevant L0 checks. Resolve failed invariants before interpreting performance.
2. Verify artifacts and effective configuration. Confirm safe host conditions without substituting synthetic values for real telemetry.
3. Freeze the profile and arrival trace. Use the declared warmup; do not import warmup calls into the measured cohort.
4. Run matched A/B/C blocks in randomized or balanced order. Restore the declared model/cache/foreground state between groups and record the order.
5. Keep sending against the same arrival schedule. Record target-to-send lag and missed dispatches. A slow response must not automatically reduce the planned request count.
6. Stop new arrivals at the fixed window boundary, then observe every measured request to normal termination or its declared deadline. Do not cancel outstanding work merely because the arrival window ended.
7. Confirm owned-process and reservation cleanup independently of response completion. Record pressure/thermal state after the run.
8. Summarize each workload and evidence level separately. Pair foreground ratios and B/C goodput ratios within each block; retain the raw counts and per-run values.
9. Compare candidate policies on holdout traces or foreground tasks that were not used for calibration. Document regressions as well as improvements.

Use cohort-with-drain accounting from the metric contract. Keep actual-send latency and scheduled-arrival latency visible. If the generator misses arrivals or exceeds the profile's predeclared send-lag tolerance, mark the comparison INSUFFICIENT and identify the harness bottleneck.

## 5. Candidate desktop targets

These values are initial project targets, not industry standards, measured results or runtime configuration changes. A release profile must version and freeze its targets before evaluation. Report against the frozen values even when a run fails.

| Criterion | Candidate target | Applicability |
| --- | --- | --- |
| Ownership/accounting/protocol violations | Zero observed violations in all required control scenarios | Hard gate; zero observations describe the tested scope, not a proof for all executions |
| Foreground p95 slowdown in C relative to A | At most 10% for each required task | Desktop benefit candidate; also report B/A and C/B |
| Foreground task failures | No unexpected failed or dropped actions | A failed reference task invalidates its comparison; any permitted failure budget must be frozen in the profile |
| Idle goodput retention, C/B | At least 90% | Same idle workload/objectives; undefined if B has no measurable positive goodput |
| Controller idle RSS | Below 100 MiB | Existing design target, measured separately from the engine |
| Added active control-path p95 | Below 10 ms | Existing production candidate; excludes queue, load, model execution and output waiting; needs spans or a justified equivalent measurement |
| Comparison repetitions | At least five complete paired blocks | Minimum evidence floor, not a guarantee of statistical precision |
| Samples for a p95 benefit comparison | At least 100 task observations per relevant group/workload in each block | Below this floor, report descriptive values and INSUFFICIENT for a p95 benefit claim |
| Inference rate sample floor | At least 100 scheduled valid requests per B/C workload across blocks | Retain every outcome; a floor alone does not guarantee useful precision |
| Stable-host endurance | At least four hours, 1000 valid calls and 100 lifecycle/cancellation cycles | L4 candidate from the existing reliable-host plan; separate from short-run L3 |

Report p50/p95 and sample counts. Label p99 with fewer than 1000 observations as exploratory. For each task, the primary foreground estimate is the mean of its paired block p95 slowdown ratios. Idle retention is the mean of paired block goodput ratios; every denominator must be valid and positive. Estimate 95% uncertainty intervals for these means over paired run blocks, preserving within-run correlation. Five blocks may still leave uncertainty too wide to decide. Record the resampling method and seed if using bootstrap; do not treat every streaming chunk as an independent run.

For a maximum target, the upper uncertainty bound must meet the limit; for a minimum target, the lower bound must meet it. An estimate whose interval crosses the target is INSUFFICIENT. A confidently violating result is FAIL. Do not hide a required task's regression in an average across unrelated workloads.

## 6. Verdict rules

| Verdict | Meaning |
| --- | --- |
| PASS | Required evidence is valid, sufficiently precise and meets the frozen criterion |
| FAIL | An invariant is violated or sufficient measurements establish a target violation |
| INSUFFICIENT | Missing data, an unsafe/aborted run, inadequate samples, confounding or uncertainty prevents a decision |
| NOT_RUN | The scenario or measurement has not been executed |
| NOT_APPLICABLE | The profile explicitly excludes a non-required capability, with a reason; never use this to waive required memory or ownership checks |

Apply data-validity and correctness gates before benefit claims. A policy that refuses all work has zero useful capacity, regardless of foreground latency. An unavailable required measurement is not zero usage or a passing measurement.

Among policies meeting the same hard and desktop constraints, prefer greater conforming goodput. At comparable goodput, prefer lower foreground slowdown and resource cost. If neither policy dominates, report the trade-off; do not invent an overall percentage improvement or combine incompatible units into a score.

No overall effectiveness PASS is available while required L2/L3 evidence is NOT_RUN or INSUFFICIENT. Keep L4 release readiness independent from a local policy-benefit verdict.

## 7. Existing evidence and collection gaps

Run these commands from the repository root for their stated scope:

```sh
make setup-tools setup-client
make check check-client
```

`scripts/check_live.py` validates an explicitly running service's HTTP contract. It is not a load generator or foreground benchmark. Existing test and runtime results belong to the [runtime report](../reports/2026-11-09-runtime-validation.md); creating this protocol does not rerun them.

| Evidence | Current source | Additional work before a complete comparison |
| --- | --- | --- |
| Control invariants | Go tests and API/client fixtures | Complete the scenario coverage map and retain run-specific results |
| Pressure, memory estimate and CPU attribution | `/admin/resources` | Periodic export with observation quality, gaps and collection overhead |
| Queue, execution and protection summaries | `/admin/scheduler` | Durable per-run sampling; its 128 recent decisions are not a complete event journal |
| Client content and terminal timing | Existing Python/HTTP paths | A collector preserving arrival traces, cancellation and stream boundaries |
| Exact per-request queue/load/control spans | Coordinator behavior and controlled fixtures | Dedicated evidence or instrumentation; polling cannot establish exact spans |
| Native release and residency | Backend completion/exit evidence | Correlated lifecycle records and OS memory observations |
| Controller/engine memory, swap and foreground latency | Separate host/task observations | Reproducible collectors and foreground fixtures |
| Native model and desktop benefit | No completed acceptance run | L2/L3 measurements under safe conditions |

GPU utilization and energy are optional when unavailable. Their absence does not waive required memory-pressure evidence. The current repository has control-path evidence but no established native desktop benefit.

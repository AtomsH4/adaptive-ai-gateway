# Evaluation report template

Template version: `desktop-eval-v1`. This file is an unexecuted form. NOT_RUN and unavailable fields are intentional; they are not zero measurements. Copy it to a dated report and populate it from retained evidence using the [protocol](README.md), [metrics](metrics.md) and [scenarios](scenarios.md).

## 1. Run identity and frozen profile

| Field | Recorded value |
| --- | --- |
| Report date/timezone and run/block IDs | Not recorded |
| Source commit and dirty-tree state | Not recorded |
| Protocol, profile ID/revision/digest and scenario IDs | Not recorded |
| Host OS/architecture, CPU/GPU and physical memory | Not recorded |
| Power/thermal state and competing workload | Not recorded |
| Engine build/digest and model ID/revision/digest | Not recorded |
| Actual context, threads, GPU layers, concurrency and output cap | Not recorded |
| Effective policy, waiting/load/execution/total/idle/recovery budgets | Not recorded |
| Client/interpreter/dependencies, retries and timeouts | Not recorded |
| Input/foreground fixture versions, hashes and validity predicate | Not recorded |
| Arrival trace/seed/count/window and dispatch-lag tolerance | Not recorded |
| Cold/warm state, prefix/filesystem/build caches, connection reuse | Not recorded |
| Warmup, drain/cutoff rules and safety abort condition | Not recorded |
| Frozen latency objectives, desktop targets and uncertainty rule | Not recorded |
| Group order, repetitions and calibration versus holdout split | Not recorded |

Exclude authentication tokens, raw private prompts/responses and unique device identifiers. Public fixture identifiers/hashes are sufficient to reproduce the workload. Give any unavailable field a reason and its effect on the verdict.

## 2. Evidence and artifacts

Use a run-specific ignored directory such as `.local/evaluation/<run-id>/`. The following names describe a future collector's record contract; the repository does not generate these files automatically. Manually assembled evidence may use other names with the same documented fields.

| Artifact | Required record content |
| --- | --- |
| `manifest.json` | Frozen profile, source/artifact digests, groups/order, clock/units, safety rule and observer overhead |
| `requests.jsonl` | Run/group/request/scenario IDs; target/sent/content/terminal times; outcome/reason; finish marker; partial flag; validity result; token source/count; missing-value reasons |
| `resources.jsonl` | Observation and receipt times; source/age/quality; pressure, available estimate and CPU attribution; optional memory/swap/energy metrics with units |
| `foreground.csv` | Run/group/block/task/action IDs; target/start/completion times, validity/failure and duration |
| `lifecycle.jsonl` | Instance/incarnation/execution identity; load/cancel/force-exit/release/exit events; timestamps, evidence source and timing uncertainty |
| `summary.json` | Population counts, windows, per-run metrics, missingness, uncertainty and each gate's verdict |

Record actual paths and hashes: Not recorded. State sampling/export gaps explicitly; the scheduler's recent-decision ring is not a complete lifecycle log.

## 3. Data validity

| Check | Verdict | Evidence or limitation |
| --- | --- | --- |
| Model/profile/input/foreground identity matches across paired groups | NOT_RUN | No run recorded |
| Arrival schedule delivered within declared lag tolerance | NOT_RUN | No run recorded |
| Cohort counts reconcile, including failures and incomplete work | NOT_RUN | No run recorded |
| Clock relationships, units and measurement windows are valid | NOT_RUN | No run recorded |
| Required observations are fresh and sufficiently complete | NOT_RUN | No run recorded |
| Cache/power/thermal state and group order are comparable | NOT_RUN | No run recorded |
| Sample count and paired-run uncertainty support the claim | NOT_RUN | No run recorded |
| Safety abort and post-run cleanup are accounted for | NOT_RUN | No run recorded |

## 4. Demand and outcomes

Create one table per workload and run/block. A has no inference population. Keep deliberate functional probes separate from performance demand.

| Count or duration | B: Fixed engine | C: Gateway |
| --- | --- | --- |
| Scheduled cohort / actually sent | Not measured | Not measured |
| Completed / passed validity predicate / useful completions | Not measured | Not measured |
| Refused, by reason | Not measured | Not measured |
| Timed out, by phase/client cause | Not measured | Not measured |
| Interrupted / caller cancelled | Not measured | Not measured |
| Error / incomplete / not sent | Not measured | Not measured |
| Partial output / undetermined objective eligibility | Not measured | Not measured |
| Arrival window / drain / total rate denominator W | Not measured | Not measured |

Sum the mutually exclusive terminal categories to the scheduled cohort size. Validity and partial-output fields are overlapping annotations, not extra terminal categories. Retain reason-code totals and explicit discrepancies.

## 5. Performance and foreground results

Report units, population size and missing count for every row. Separate cold/warm and workload classes. Keep success and failure latencies distinct.

| Metric | A: Foreground only | B: Fixed engine | C: Gateway |
| --- | --- | --- | --- |
| Foreground task p50/p95 and failed actions | Not measured | Not measured | Not measured |
| Scheduled and client TTFT p50/p95 | Not applicable | Not measured | Not measured |
| Validated completion latency p50/p95 | Not applicable | Not measured | Not measured |
| Failure latency, by outcome | Not applicable | Not measured | Not measured |
| Dispatch lag and missed arrivals | Not applicable | Not measured | Not measured |
| Chunk gaps / TPOT with count source | Not applicable | Not measured | Not measured |
| Useful attainment / cohort goodput | Not applicable | Not measured | Not measured |
| Critical/warning/unknown exposure and swap delta | Not measured | Not measured | Not measured |
| Controller and engine RSS/CPU, separately | Not applicable | Not measured | Not measured |
| Loads/unloads/restarts/protection transitions | Not applicable | Not measured | Not measured |
| Cancellation, exit and recovery intervals | Not applicable | Not measured | Not measured |

Record B/A, C/A and C/B foreground ratios, C/B idle goodput retention, per-block values and 95% uncertainty: Not measured. Record the estimator, resampling unit/count/seed and any exploratory tail percentiles. Do not obtain overhead by subtracting unrelated p95 values.

For S0/S1, provide a separate table of paired client latency, active control spans where available, controller cost and workload identity. Do not merge synthetic and native throughput.

## 6. Scenario coverage and verdicts

Attach one row per required C01-C14 and selected W01-W08 with exact branch coverage, command/profile, artifact and verdict. Required rows without evidence remain NOT_RUN or INSUFFICIENT.

| Gate | Verdict | Evidence and scope |
| --- | --- | --- |
| Data validity | NOT_RUN | No run recorded |
| L0: Control correctness | NOT_RUN | No run recorded |
| L1: Gateway cost | NOT_RUN | No run recorded |
| L2: Native lifecycle | NOT_RUN | No run recorded |
| L3: Policy benefit | NOT_RUN | No run recorded |
| L4: Reliable host | NOT_RUN | No run recorded |

Use PASS, FAIL, INSUFFICIENT, NOT_RUN or justified NOT_APPLICABLE. State the frozen target and interval for numerical decisions. Required invariant failure prevents an effectiveness PASS. Missing native or foreground evidence prevents a desktop-benefit PASS. A correct safety refusal can pass its refusal case while native execution remains unverified.

## 7. Interpretation and follow-up

Record the supported conclusion, observed trade-offs, aborts, confounders, missing measurements and next experiment. Distinguish measured values from candidate parameters and simulated results. A report limited to one host, fixture or model must name that scope. Do not claim general improvement, production readiness or semantic answer quality beyond the evidence.

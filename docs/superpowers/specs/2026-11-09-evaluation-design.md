# Evaluation system design

Date: 2026-11-09 (Asia/Shanghai). Scope: evaluation specifications, scenarios and report templates. Automated collectors and reporting programs are a later implementation task.

## Objective

Evaluate whether resource-aware operation preserves everyday application responsiveness while completing useful inference work. Keep model quality, engine speed, gateway overhead and scheduling benefit separately measurable.

The evaluation must reject unsafe behavior, expose conservative policies that refuse most work, and distinguish missing evidence from measured failure. It must not change the runtime's resource protections or declared model/context limits.

## Approaches considered

| Approach | Benefit | Limitation |
| --- | --- | --- |
| Engine microbenchmarks | Isolate prompt-processing and decoding capacity | Do not establish HTTP behavior, scheduling benefit or foreground responsiveness |
| One general load-testing tool | Provides traffic generation and client-side timing | Tool-specific denominators, terminal handling and metric names need adaptation; desktop tasks and reclamation still need evidence |
| Layered evaluation | Connects correctness, client experience, resource effects and application benefit | Requires a shared measurement contract and several evidence sources |

Use the layered approach. Borrow measurement methods from five upstream repositories without adopting their published scores or adding their packages to the runtime. The [research comparison](../../research/evaluation-upstreams.md) records pinned sources and adaptations.

## Evaluation boundaries

1. Deterministic tests establish invariants, including identity, reservations, deadlines and stream completion.
2. A synthetic backend isolates HTTP and control overhead.
3. Native runs establish actual model loading, generation, cancellation and unloading.
4. Paired experiments compare foreground-only, fixed native inference and resource-aware operation.
5. Long-running tests establish the separate reliable-host release gate.

Only the coordinator retains admission and resource-release authority. Read-only diagnostics supply observations; they do not become benchmark control endpoints. Fault injection belongs in tests or an explicitly isolated fixture.

## Measurement decisions

- Track all scheduled valid demand, actual dispatch and every terminal outcome. Refusals and interrupted work remain visible.
- Use client monotonic timing for arrival delay, first content, last content and validated completion. Server and process timings retain their own clock and uncertainty.
- Separate response completion from evidence that execution and model occupancy ended.
- Distinguish content chunks from tokens. Missing token counts disable token-derived metrics rather than falling back to chunk counts.
- Freeze workload, model, engine, policy, objectives and measurement windows before a comparison. Calibrate on separate runs.
- Give each gate a result of PASS, FAIL, INSUFFICIENT, NOT_RUN or justified NOT_APPLICABLE. There is no aggregate score that can offset an invariant violation.
- Compare useful completion rate and foreground slowdown under the same constraints. Use repeated paired runs and retain uncertainty.

## Deliverables

| Document | Responsibility |
| --- | --- |
| [Evaluation protocol](../../evaluation/README.md) | Evidence levels, comparison procedure, gates and candidate targets |
| [Metric definitions](../../evaluation/metrics.md) | Populations, clocks, formulas, units, missing values and collection limits |
| [Scenario matrix](../../evaluation/scenarios.md) | Required control cases and native workloads, with expected observations |
| [Report template](../../evaluation/report-template.md) | Reproducible manifests, counts, measurements, verdicts and limitations |
| [Upstream comparison](../../research/evaluation-upstreams.md) | Primary sources, adopted methods and differences |

Source revisions and file-content fingerprints are stored in [sources.json](../../research/sources.json). This design adds no benchmark result. Existing evidence remains in the [runtime report](../../reports/2026-11-09-runtime-validation.md).

## Future automation boundary

A later collector may read the existing administrative endpoints, exercise the supported chat subset, observe owned-process lifecycle evidence and run a foreground fixture. It must preserve this protocol's populations and missing-value rules. Until that implementation is reviewed, the templates support manually assembled evidence and existing test artifacts; they are not executable benchmark commands.

# Milestones

Updated 2026-11-09. Behavior is defined in the [design](docs/superpowers/specs/2026-11-08-adaptive-ai-gateway-design.md); module boundaries are defined in the [architecture](docs/architecture/overview.md).

| Milestone | Deliverable | Acceptance evidence | Status |
| --- | --- | --- | --- |
| M0: Research and repository | Local Git repository, upstream comparison, business models, machine baseline, initial design | Source and link checks; initial commit | Complete |
| M1: Architecture and plan | Modules, consumer-owned ports, API contract, dependency rules, production roadmap | Architecture and machine-readable contract checks | Complete; hardware capability remains subject to runtime acceptance |
| M2: Control prototype | Control process, fake engine, resource snapshots, bounded queue, decision reasons | Admission, phase budgets, shared loading, cancellation, missing metrics, reclamation | Implemented with synthetic-backend and child-process validation |
| M3: Native local execution | llama.cpp, pinned small model, Python client, streaming, idle unloading | Cold start, generation, cancellation, and draining over HTTP and Python | Artifacts verified; generation blocked by recorded memory conditions; acceptance pending |
| M4: Everyday application comparison | Inference alongside browsers, editors, and representative tasks | Fixed-configuration versus resource-aware measurements | Not started |
| M5: Further extensions | Desktop management, additional backends, or multiple devices | A separate design for a demonstrated requirement | Not started |

See the [implementation plan](docs/plans/2026-11-08-implementation-plan.md) for task dependencies. T0-T5 have implementations and validation for the recorded paths. Real-model T6/T7 and application comparison T8 remain pending; see the [runtime report](docs/reports/2026-11-09-runtime-validation.md).

Reliable single-host operation, remote workers, and production clusters have separate release gates. Provider integrations, team governance, and paid features remain outside the initial prototype.

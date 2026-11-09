# Architecture requirements and evidence

This is the T0 acceptance snapshot from 2026-11-09 (Asia/Shanghai). Runtime implementation and later results are recorded in the [runtime validation report](../reports/2026-11-09-runtime-validation.md).

T0 established module boundaries, extension contracts, an implementation plan, and a production roadmap. T1-T8 runtime acceptance was outside that snapshot.

## 1. Requirements and evidence

| Requirement | Deliverable | Verified scope |
| --- | --- | --- |
| Low coupling | [Architecture](overview.md), [dependency manifest](../../architecture/dependencies.json), [checker](../../internal/architecture/check.go) | Production imports passed. Negative cases covered forbidden network/process effects, adapter dependencies, reverse imports, external packages, unregistered modules, cycles, and non-host files |
| HTTP has no engine authority | Separate inbound [api](../../internal/application/api/resources.go) and outbound [ports](../../internal/application/ports/runtime.go) | The actual manifest rejected HTTP imports of outbound ports, application implementations, and engine adapters. This verifies imports, not all runtime behavior |
| Replaceable backends, platforms, and rules | [Extension points](overview.md#5-extension-points), narrow interfaces, false capability defaults | [Application tests](../../internal/application/assessment_test.go) injected two independent readers into the same use case through its inbound query interface. A second real backend/platform was not tested |
| Resource semantics outside transport | [Domain values](../../internal/domain/resources.go), [ResourceGuard](../../internal/policy/resourceguard.go), [state contracts](runtime-contracts.md) | Tests covered unknown/failed/stale/future readings, freshness boundaries, critical classification, and high owned CPU/missing optional GPU. Admission and reclamation were later tasks |
| Scoped cancellation and release | [Identity and evidence](../../internal/domain/runtime.go), [ADR-0003](../adr/0003-output-and-resource-completion.md) | Tests rejected old incarnations, wrong executions, cancellation requests, and empty evidence. Physical engine or OS memory release was not measured |
| Versioned external contract | [OpenAPI](../../api/openapi.json), [semantics](../contracts/http-api.md), [fixtures](../../api/fixtures/cases.json) | 17 JSON and 5 SSE cases passed, including strict requests, extensible responses, unknown error codes, and incomplete streams. No live endpoint was exercised at T0 |
| Implementation sequence | [Plan](../plans/2026-11-08-implementation-plan.md), [milestones](../../ROADMAP.md) | T0-T8 defined outputs, dependencies, and checks. Only T0 had been accepted |
| Production path | [Release gates](production-evolution.md), [ADR-0002](../adr/0002-modular-monolith-and-ports.md) | Local service, reliable host, remote worker, and cluster gates covered journals, reconciliation, partitions, SLOs, and rollback. These remained future capabilities |
| Traceable terminology and decisions | [Glossary](../../CONTEXT.md), [decisions](../plans/2026-11-08-implementation-review.md), [ADRs](../adr/0004-service-readiness-and-capacity.md), [sources](../research/sources.json) | Model/config identity and readiness/capacity were distinguished. Source dates, file fingerprints, and CI commit references were recorded separately |

## 2. Local checks at T0

Environment: macOS arm64, Go 1.26.3, isolated CPython 3.12.13 contract tools. `make check` exited with code 0.

| Command | Result | Coverage |
| --- | --- | --- |
| `go run ./cmd/archcheck` | Passed | Existing production sources complied with the manifest |
| `go test ./...` | Passed | Resource classification, evidence scope, reader replacement, architecture behavior |
| `go test -race ./...` | Passed | Exercised paths; the coordinator had not yet been implemented |
| `go vet ./...` | Passed | Static Go checks |
| `.venv-tools/bin/python scripts/check_contracts.py` | Passed: 17 JSON / 5 SSE | Schema constraints and synthetic stream termination |

A read-only script checked 83 local links/anchors across 19 Markdown files, five JSON files, and CI YAML. Four Action references matched the source index. `gofmt -l internal cmd` and `git diff --check` had no output.

Validation dependencies were pinned in the [hash lock](../../tools/requirements.txt), separate from Go runtime dependencies. [CI](../../.github/workflows/quality.yml) was configured for Linux/macOS Go checks and contract checks; this snapshot contained local results only.

## 3. Follow-up work

The architecture requirements had concrete deliverables and executable boundary checks. T1 was the next package: configuration, composition, logging, liveness, and bounded shutdown. T2-T5 would add synthetic runtime validation, followed by real-model and Python acceptance.

At T0, no engine or model had been installed, no inference or paid call had run, and dynamic GPU adaptation and production reliability were unverified. Later implementation status belongs to the runtime report.

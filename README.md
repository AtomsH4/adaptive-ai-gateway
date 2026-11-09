# Adaptive AI Gateway

A resource-aware gateway for local model inference on personal computers. It controls admission, queues requests, and unloads managed models when resources are needed by other applications.

**Status:** working control prototype. HTTP/SSE, single-model scheduling, shared cold starts, bounded queues, request budgets, a managed llama.cpp adapter, macOS memory/CPU sampling, and a Python client example are implemented. The synthetic backend has passed live HTTP checks. Engine and model artifacts are verified; real-model validation remains pending after memory protection prevented loading. See the [runtime validation report](docs/reports/2026-11-09-runtime-validation.md).

## Scope

- Initial platform: macOS arm64 with 8 GiB of unified memory.
- Initial client: Python scripts over HTTP.
- One model and one execution slot, with up to four waiting requests.
- A request can wait for a bounded cold start and then generate on the same connection.
- Healthy inference can use idle compute. High CPU usage from the engine alone does not trigger throttling.
- Critical memory pressure can interrupt generation and stop the managed engine; see [ADR-0001](docs/adr/0001-prioritize-everyday-apps-under-critical-pressure.md).
- Missing required memory-pressure data blocks new work while active requests retain their deadlines. GPU utilization is optional.
- Model identity stays explicit. Model changes, quality changes, and remote routing require configuration.

## Architecture

A Go control process manages a separate native llama.cpp process. The application owns admission and resource accounting; HTTP, platform sampling, configuration, and inference use separate adapters. Concrete implementations are connected at the composition root.

See the [architecture](docs/architecture/overview.md), [runtime contracts](docs/architecture/runtime-contracts.md), and [production roadmap](docs/architecture/production-evolution.md). Startup parameters such as GPU layers and context capacity are changed between executions. The current implementation requires a restart for configuration changes.

## Run

Requirements: Go 1.26.3, Python 3.12, and uv. macOS CPU attribution uses a cgo bridge to the installed SDK. CPU attribution is unavailable when cgo is disabled.

```sh
make setup-tools setup-client build
export AIGATEWAY_TOKEN="$(openssl rand -hex 24)"
bin/gateway -config configs/fake.toml
```

The fake backend emits synthetic text and labels its resource source `synthetic.development_only`. In another terminal with the same token:

```sh
.venv-client/bin/python examples/python/client.py --model validation-model --stream
```

To download the pinned macOS arm64 baseline and verify its size and SHA-256, then start the native configuration:

```sh
.venv-tools/bin/python scripts/fetch_baseline.py
bin/gateway -config configs/local-qwen.toml
```

The download is about 508 MB. This configuration uses Qwen2.5-0.5B, a 2048-token context, one execution slot, two CPU threads, and Metal GPU-layer parameters. The 2 GiB pre-load headroom is a calibration candidate. Loading is blocked by memory pressure, missing required telemetry, or insufficient headroom. GPU utilization, hot configuration updates, and automatic profile selection remain unavailable.

The server listens on `127.0.0.1:8787`. Every endpoint except `/healthz` requires a local Bearer token. SIGINT or SIGTERM stops intake and starts bounded cleanup of owned instances. Downloaded artifacts remain in ignored `.local/` storage.

## Verification

```sh
make setup-tools
make check
make setup-client check-client
```

The checks cover dependency rules, Go tests/race/vet, OpenAPI fixtures, and the Python client. Tool and client dependencies use separate environments and hash locks. CI runs Go checks on Linux and macOS, plus contract and client checks.

Current evidence covers adapter replacement, telemetry failures, scoped release evidence, concurrent admission, shared loading, cancellation, observation recovery, and child-process cleanup. Real-model measurements and everyday application comparisons are separate acceptance steps.

## Documentation

1. [Architecture and extension points](docs/architecture/overview.md)
2. [Upstream comparison and technical options](docs/research/upstream-comparison.md)
3. [Pricing and business models](docs/research/pricing-and-business.md)
4. [Machine baseline](docs/local-machine.md)
5. [Initial design](docs/superpowers/specs/2026-11-08-adaptive-ai-gateway-design.md)
6. [Milestones](ROADMAP.md)
7. [Source index and license fingerprints](docs/research/sources.json)
8. [Glossary](CONTEXT.md) and [design decisions](docs/plans/2026-11-08-implementation-review.md)
9. [Implementation plan](docs/plans/2026-11-08-implementation-plan.md)
10. [HTTP contract](docs/contracts/http-api.md)
11. [Architecture verification snapshot](docs/architecture/verification.md)
12. [Evaluation protocol](docs/evaluation/README.md), [metrics](docs/evaluation/metrics.md), [scenarios](docs/evaluation/scenarios.md), and [report template](docs/evaluation/report-template.md)

Research was checked on 2026-11-08; the architecture and runtime records were updated on 2026-11-09 (Asia/Shanghai). Individual sources retain their check dates. Measurements, proposed budgets, and future requirements are labeled separately.

## License

[Apache-2.0](LICENSE). See [third-party notices](THIRD_PARTY_NOTICES.md). The engine and model retain their own licenses; upstream inference implementations are not copied into this repository.

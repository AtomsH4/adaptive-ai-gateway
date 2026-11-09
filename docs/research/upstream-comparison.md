# Upstream comparison and technical options

Checked 2026-11-08 (Asia/Shanghai). This snapshot describes upstream documentation. File-level license fingerprints are in the [source index](sources.json).

## Repositories

| Project | Relevant design | Use in this project | Initial boundary |
| --- | --- | --- | --- |
| [GPUStack](https://github.com/gpustack/gpustack) | Separate controller, scheduler, worker, metrics, and gateway; filter feasible placements before scoring | Control-loop and resource-adaptation patterns | The checked README documented Linux-only workers; the full platform is not the local runtime base |
| [LocalAI](https://github.com/mudler/LocalAI) | Common model API, per-model backends, SmartRouter using loaded models, VRAM, and node state | Backend adapters, lifecycle management, idle eviction | One host and one model initially; no full distributed service chain |
| [llama.cpp](https://github.com/ggml-org/llama.cpp) | GGUF quantization, CPU/GPU inference, Metal, HTTP serving, load-time parameter fitting | Separate native `llama-server` process | `--fit` adapts loading parameters; placement changes during generation need further engine support |
| [LiteLLM](https://github.com/BerriAI/litellm) | Common API, routing, fallback, usage and cost attribution | Routing/inference separation; possible later connection to an external deployment | No requirement to support all providers or enterprise features |
| [KTransformers](https://github.com/kvcache-ai/ktransformers) | Heterogeneous CPU/GPU execution, MoE expert placement, optimized kernels | Later research into heterogeneous execution | Validate a runnable small local model first; large-memory MoE is outside the first delivery |

GPUStack's [architecture](https://github.com/gpustack/gpustack/blob/main/docs/architecture.md) separates Higress from deployment scheduling. Its [scheduler documentation](https://github.com/gpustack/gpustack/blob/main/docs/scheduler.md) distinguishes CPU and partial-offload support by backend; fallback capabilities vary.

LocalAI's [distributed scheduling](https://localai.io/docs/features/distributed-mode/#scheduling-algorithm) covers model reuse, free-VRAM selection, and idle-model eviction. These are lifecycle references; desktop foreground responsiveness still requires separate measurements.

llama.cpp's [Metal build](https://github.com/ggml-org/llama.cpp/blob/master/docs/build.md#metal-build) and [fit-params documentation](https://github.com/ggml-org/llama.cpp/blob/master/tools/fit-params/README.md) support the local engine path. Pin a release and verify its actual arguments before use.

## Technical options

The following trade-offs are project design judgments.

| Option | Benefits | Cost | Assessment |
| --- | --- | --- | --- |
| A: Go control layer + native llama.cpp | Small module scope, available local toolchain, independent policy and adapter development | Custom streaming, cancellation, reservations, queues, and supervision | Selected for desktop resource control |
| B: Extend LocalAI | Existing APIs, backends, and broader product features | Larger upstream scope to understand and maintain; desktop policy still needed | Suitable if multimodal and multi-backend coverage becomes a priority |
| C: LiteLLM + local engine + telemetry bridge | Established local/remote provider integration | Admission, model state, and reload coordination across components; custom hardware policy | Suitable for an existing multi-provider requirement |

## Forms of dynamic adaptation

1. **Load-time fitting:** choose startup parameters from model, context, and resources; reuse engine capabilities where available.
2. **Scheduling between requests:** admit, queue, and unload based on load, memory pressure, and model state. This is the initial focus.
3. **Changing placement during generation:** move weights, caches, or execution plans through engine-specific mechanisms. This remains a later research topic.

## Reuse and licenses

The project references architectures and public interfaces without copying upstream inference implementations. Apache-2.0 was the proposed project license at this research snapshot; the current license is in [LICENSE](../../LICENSE).

| Project | Root license inspected | Reuse requirements to check |
| --- | --- | --- |
| GPUStack | [Apache-2.0](https://github.com/gpustack/gpustack/blob/main/LICENSE) | Applicable license, attribution, NOTICE, and modification notices |
| LocalAI | [MIT](https://github.com/mudler/LocalAI/blob/master/LICENSE) | Copyright/license text; separate backend and model terms |
| llama.cpp | [MIT](https://github.com/ggml-org/llama.cpp/blob/master/LICENSE) | Engine distribution and model-weight distribution separately |
| LiteLLM | [Root MIT terms and scope exception](https://github.com/BerriAI/litellm/blob/main/LICENSE) | The root excludes an `enterprise/` directory if present. Fetching `enterprise/LICENSE` returned 404 at the check date, which establishes no reuse rights for commercial components |
| KTransformers | [Apache-2.0](https://github.com/kvcache-ai/ktransformers/blob/main/LICENSE) | Licenses of the actual kernels, dependencies, and models introduced |

Reuse and distribution must follow the specific files, revisions, and license texts involved.

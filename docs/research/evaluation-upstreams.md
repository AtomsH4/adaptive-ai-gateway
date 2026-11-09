# Evaluation methods from upstream repositories

Checked 2026-11-09 (Asia/Shanghai). The following are methodology references, not benchmark runs on this project. No upstream benchmark package was installed and no source implementation was copied. Repository revisions and file-content fingerprints are recorded in [sources.json](sources.json).

## Sources and adaptations

| Repository and pinned source | Method adopted | Adaptation for this project |
| --- | --- | --- |
| [GuideLLM metrics](https://github.com/vllm-project/guidellm/blob/c96aa1727995adb44d7187fba8c37146f184b13e/docs/en/guides/metrics.md) | Objective-conforming goodput, request outcomes, dispatch delay and measurement uncertainty | Use an explicit scheduled-request cohort. Retain rejected, interrupted and unfinished demand. Tool exclusions and windows must be mapped before importing a result. |
| [vLLM benchmark guide](https://github.com/vllm-project/vllm/blob/9ad002988359b6f7a703d9000f9a5f3a331e1a39/docs/benchmarking/cli.md) | Client timing, token versus streaming-event distinctions, cache controls | Define first content, last content and validated termination separately. Record cache state and tokenizer identity; do not treat one SSE event as one token. |
| [AIPerf metric reference](https://github.com/ai-dynamo/aiperf/blob/ab610add4e8876fca223197596c728e17365f381/docs/metrics-reference.md) | Separate HTTP-byte timing, content timing and inter-chunk gaps | Exclude role-only and empty events from first content. Preserve terminal validation beyond last content. Chunk timing remains available when token counts are unknown. |
| [llama.cpp benchmark guide](https://github.com/ggml-org/llama.cpp/blob/a4d4f2fd4ec85ec6a0a9ece4870416c1b6cc24f5/tools/llama-bench/README.md) | Separate prompt-processing and decoding measurements, repeated trials and parameter records | Use these as engine diagnostics. Their timing excludes tokenization and sampling, so it cannot establish end-to-end latency or desktop benefit. |
| [LiteLLM benchmark methodology](https://github.com/BerriAI/litellm-docs/blob/704f99b77f251df668a52c4487bb7d870130028a/docs/benchmarks.md) | Isolate proxy overhead with a mock model and disclose deployment conditions | Run equivalent synthetic direct and gateway paths. Report local host and enabled features; published cluster/mock-model scores are not targets for native desktop inference. |

GuideLLM's inspected definitions exclude some incomplete or undetermined requests from SLO attainment and use different windows for request and event metrics. This project's strict offered-demand accounting is an intentional adaptation. Imported summaries alone may be insufficient to reconstruct it.

Metric names are not interchangeable across tools. For example, a tool's request latency may stop at last content while this project also requires a validated terminal event. A tool's token metric may use a fallback that this protocol disallows. Preserve the original field name and formula in any adapter, then map to the [canonical metrics](../evaluation/metrics.md).

## Gaps addressed locally

The five references inform engine, gateway and client measurements. Foreground task slowdown, emergency reclamation, missing required observations, owned-process authority and shared-memory accounting are additional requirements from this project's design. Their [scenarios](../evaluation/scenarios.md) and [candidate targets](../evaluation/README.md#5-candidate-desktop-targets) are project choices, not upstream standards.

## Tool selection

Use existing Go/HTTP/Python checks for their documented scope. A future traffic adapter can use a compatible upstream tool or a small local collector, provided it preserves supported request fields, cancellation, terminal validation, token provenance and arrival schedules. Tool compatibility has to be tested before use.

Do not send parameters such as `ignore_eos`, `stream_options` or `top_p` merely because a benchmark emits them. The current [HTTP contract](../contracts/http-api.md) rejects unsupported fields. External tools must target `/v1/chat/completions`, use the local Bearer token and disable automatic retries. Record transformations rather than silently discarding parameters.

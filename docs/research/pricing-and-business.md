# Pricing and business models

Checked 2026-11-08 (Asia/Shanghai). Amounts retain their source currency. Software licensing, model API charges, hardware, electricity, and operations are separate costs.

## Upstream snapshot

| Project | Open-source self-hosting | Commercial offering documented | Price evidence and scope |
| --- | --- | --- | --- |
| GPUStack | Apache-2.0 code, subject to its terms | Enterprise HA, tenancy, governance, branding, support | The [enterprise page](https://gpustack.ai/enterprise/) directed buyers to sales; no fixed numeric price was found |
| LiteLLM | Official pricing listed **US$0** for self-hosted OSS | Enterprise governance, security, support; annual quotes by gateway request capacity, architecture, and support needs | [Pricing](https://www.litellm.ai/pricing) described gateway-scale licensing rather than token-based enterprise licensing; provider charges are separate |
| LocalAI | MIT runtime | Business-contact option | The checked [website](https://localai.io/) gave no uniform subscription price; commercial service availability or cost was not established |
| llama.cpp | MIT inference engine | No common commercial package verified in this research | [Root license](https://github.com/ggml-org/llama.cpp/blob/master/LICENSE); adjacent product prices are outside the engine's license terms |
| KTransformers | Apache-2.0 code | Repository focused on heterogeneous inference and training | [Root license](https://github.com/kvcache-ai/ktransformers/blob/main/LICENSE); no uniform service quote verified |

LiteLLM's root license distinguishes the open-source scope from a possible enterprise directory. Sales-page features and distributable source-code rights require separate checks.

## Product hypotheses

These are unvalidated product options, with no committed pricing.

- **Local edition:** free self-hosting focused on resource budgets, admission decisions, and application-impact reporting.
- **Personal paid features:** evaluate demand for installation/updates, workload profiles, reports, and desktop management. Measure retention and setup time saved before choosing a price or subscription versus perpetual licensing.
- **Team edition:** consider central policy, device management, audit, and support when multi-device demand exists. Tenancy and billing are outside the initial release.
- **Third-party APIs:** use caller-owned keys, show provider charges separately, and require explicit remote fallback configuration.

Estimate local cost from measured power, electricity tariffs, and optional hardware amortization. Show unknown cost when measurements are absent. Keep software price, compute cost, and provider charges separate in the data model and interface; local token counts alone do not establish a cloud bill.

## Validation before pricing

The product question is whether resource-aware scheduling reduces manual engine tuning while preserving useful local inference during everyday work.

Compare fixed configuration and resource-aware policy with the same model, context, and application load. Record memory pressure, queue time, generation speed, and foreground-task completion time. Use those results to assess paid features and pricing.

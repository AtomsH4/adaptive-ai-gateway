# 上游能力、架构与技术路线

核对日期：2026-11-08（Asia/Shanghai）。本表描述上游公开材料，不表示这些能力已经集成本项目。许可证文件的 Git blob 指纹见 [来源索引](sources.json)。

## 仓库比较

| 上游 | 可借鉴的模式与技术 | 对本项目的采用方式 | 首版边界 |
| --- | --- | --- | --- |
| [GPUStack](https://github.com/gpustack/gpustack) | 控制器、调度器、Worker、指标与网关职责分离；先过滤可运行位置，再评分选择 | 借鉴控制循环和资源适配思路 | 当前 README 指出 Worker 仅支持 Linux；整个平台不作为本机运行底座 |
| [LocalAI](https://github.com/mudler/LocalAI) | 统一模型 API，按模型选择推理后端；分布式 SmartRouter 按已加载模型、显存和节点状态调度 | 借鉴后端适配器、模型生命周期和空闲淘汰 | 首版缩小到单机、单模型；不引入其完整分布式服务链 |
| [llama.cpp](https://github.com/ggml-org/llama.cpp) | GGUF 量化模型、CPU/GPU 推理、Metal、HTTP 服务与加载阶段自动适配参数 | 推荐使用独立原生 `llama-server` 子进程 | `--fit` 是加载参数适配；持续生成中的资源迁移需要另外的引擎支持 |
| [LiteLLM](https://github.com/BerriAI/litellm) | API 统一、模型路由、失败回退、使用量与费用归属 | 借鉴路由与推理分层；后续可对接外部部署 | 首版无需覆盖全部供应商，也不依赖企业功能 |
| [KTransformers](https://github.com/kvcache-ai/ktransformers) | CPU/GPU 异构计算、MoE 专家放置及优化 | 保留为后续异构执行研究路径 | 先验证本机可运行的小模型；不将大内存 MoE 场景作为首个交付目标 |

GPUStack 的 [架构说明](https://github.com/gpustack/gpustack/blob/main/docs/architecture.md) 明确将 Higress 网关与部署调度分开。其 [调度说明](https://github.com/gpustack/gpustack/blob/main/docs/scheduler.md) 区分了不同后端的 CPU 执行和部分卸载路径，不能假定所有后端都具备相同回退能力。

LocalAI 的 [分布式调度说明](https://localai.io/docs/features/distributed-mode/#scheduling-algorithm) 提供模型复用、空闲显存选择和空闲模型淘汰机制；这可以作为生命周期策略参考，但不等于已经实现对桌面前台应用的感知。

llama.cpp 的 [Metal 构建说明](https://github.com/ggml-org/llama.cpp/blob/master/docs/build.md#metal-build) 和 [自动适配说明](https://github.com/ggml-org/llama.cpp/blob/master/tools/fit-params/README.md) 是本机引擎路径的直接依据。采用具体版本时需要固定版本并检查实际命令行参数。

## 三条可选路线

下表中的取舍是本项目的工程判断。

| 路线 | 优点 | 代价 | 结论 |
| --- | --- | --- | --- |
| A：Go 控制层 + 原生 llama.cpp | 模块范围可控；本机已有 Go；可独立迭代资源策略与引擎适配器 | 自己处理流式转发、取消、资源预留、排队和生命周期 | **推荐**，适合把桌面资源调度作为核心能力 |
| B：基于 LocalAI 扩展 | 可复用模型 API、多后端与较完整的产品能力 | 需要理解和跟踪较大的上游范围，桌面策略仍需开发 | 适合更快覆盖多模态和多后端的后续阶段 |
| C：LiteLLM + 本地引擎 + 指标桥接 | 接入本地与远端模型的路径成熟 | 需要跨组件协调准入、模型状态和重载；硬件反馈策略仍需开发 | 适合已经有多供应商需求的用户 |

## 分清三种“动态”

1. **加载时适配**：根据模型、上下文和可用资源选择启动配置。首版复用引擎已有能力。
2. **请求之间调度**：依据负载、内存压力和模型状态做准入、排队、空闲卸载。这是本项目首版重点。
3. **生成中改变计算放置**：涉及权重、缓存及执行计划，属于引擎层优化。后续按具体模型与后端研究，首版不承诺无缝迁移。

## 独立实现与许可

当前只参考架构与公开接口，没有导入或复制上游实现。拟议的本项目发布许可是 Apache-2.0，尚未作出正式发布承诺。

| 上游 | 已读取的根许可 | 使用时关注点 |
| --- | --- | --- |
| GPUStack | [Apache-2.0](https://github.com/gpustack/gpustack/blob/main/LICENSE) | 按实际复用内容保留许可、归属与适用的 NOTICE，修改文件应标注变更 |
| LocalAI | [MIT](https://github.com/mudler/LocalAI/blob/master/LICENSE) | 保留适用版权与许可文本；后端和模型另查 |
| llama.cpp | [MIT](https://github.com/ggml-org/llama.cpp/blob/master/LICENSE) | 引擎分发与模型权重分发分别检查 |
| LiteLLM | [根文件中的 MIT 与范围例外](https://github.com/BerriAI/litellm/blob/main/LICENSE) | 根文件对可能存在的 `enterprise/` 目录有单独许可规定；本次读取 `enterprise/LICENSE` 返回 404，不据此推断商业组件可复用 |
| KTransformers | [Apache-2.0](https://github.com/kvcache-ai/ktransformers/blob/main/LICENSE) | 实际引入的内核、依赖和模型仍需逐项核对 |

此处记录已读取文件的工程依据。正式复用与发布以具体文件、版本和其许可文本为准。

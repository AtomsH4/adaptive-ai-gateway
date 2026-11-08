# Adaptive AI Gateway

面向个人电脑的资源感知 AI 网关：在运行本地模型时，为日常应用保留资源，并解释每次排队、拒绝或模型卸载的原因。

**当前阶段：可运行的本机原型。** 已有 HTTP/SSE、单模型调度、共享冷启动、有限队列、分阶段预算、受管 llama.cpp 适配器、macOS 内存/CPU 采集和 Python 异步示例。假引擎闭环已通过实际 HTTP 验证；真实模型已下载并校验，但本机内存压力阻止加载，完整实机推理及应用对照实验尚未完成，见[运行记录](docs/reports/2026-11-09-runtime-validation.md)。

本轮重点是低耦合、可扩展性与未来生产演进。[架构主文档](docs/architecture/overview.md)定义模块与依赖，[状态和端口契约](docs/architecture/runtime-contracts.md)定义所有权与取消证据，[生产化路线](docs/architecture/production-evolution.md)列出阶段门槛。

## 已确定的场景

- 个人电脑，兼顾日常使用与本地 AI。
- 首台验证设备为 macOS / Apple Silicon / 8 GiB 内存。
- 第一个真实使用入口为用户自己的 Python 脚本；首版围绕程序调用验证，不依赖聊天客户端或桌面界面，也不承诺所有 Python SDK 都已兼容。
- 模型尚未加载时，同一个生成请求在明确时限内等待冷启动，然后继续生成；调用方可以取消。
- 首个实机目标为固定小模型的完整流程验证，随后再评估实际目标模型；测试文件的版本与校验记录见[验证模型基线](docs/research/small-model-baseline.json)。
- 在内存安全预算内允许 AI 充分利用空闲算力；其他应用需要资源时再让出，不因 AI 自身造成的全机 CPU 高占用单独触发限流。
- 日常调度优先在请求之间调整资源占用；严重内存压力下日常应用优先，允许紧急中断生成并回收受管引擎占用，见 [ADR-0001](docs/adr/0001-prioritize-everyday-apps-under-critical-pressure.md)。
- 关键内存压力观测失效时暂停新生成，保留在途请求，恢复可靠观测后再放行；可选 GPU 利用率缺失不触发这项限制。

## 推荐路线

采用 **Go 单进程网关与调度控制层 + 原生 llama.cpp / Metal 推理子进程**。复用成熟推理引擎，自己实现硬件指标归一化、资源准入、有限队列、状态转换与原因记录。初期使用配置文件和内存状态，暂不引入数据库服务。

模型层数、上下文容量等启动参数，在受管引擎没有在途请求时重新配置。后续可通过适配器接入 Ollama、LocalAI 或外部网关。

架构采用模块化单体：应用层持有准入和账本，HTTP、平台与引擎通过小接口连接，只有组合入口依赖具体实现。原生引擎与模型保存在忽略的 `.local/`，不纳入 Git；没有调用付费模型。

## 启动与调用

需要 Go 1.26.3、Python 3.12 和 uv。macOS 的 CPU 归因使用本机 SDK 的 cgo 桥接；禁用 cgo 时该指标明确不可用。

```sh
make setup-tools setup-client build
export AIGATEWAY_TOKEN="$(openssl rand -hex 24)"
bin/gateway -config configs/fake.toml
```

假引擎只输出合成文本，资源来源标为 `synthetic.development_only`。在配置了相同令牌的终端调用：

```sh
.venv-client/bin/python examples/python/client.py --model validation-model --stream
```

真实引擎的固定基线只适用于 macOS arm64；下载量约 508 MB，下载后验证长度与 SHA-256：

```sh
.venv-tools/bin/python scripts/fetch_baseline.py
bin/gateway -config configs/local-qwen.toml
```

此配置使用 Qwen2.5-0.5B、上下文 2048、单执行槽位、2 个 CPU 线程和 Metal GPU 层参数。2 GiB 的加载前余量为待标定初值；warning/critical、关键观测缺失或余量不足时不会加载。GPU 利用率仍不可用，配置变更需重启，尚无热更新或多配置自动选择。

默认监听 `127.0.0.1:8787`。`/healthz` 外的接口要求本地 Bearer 令牌；收到 SIGINT/SIGTERM 时拒绝新工作并有界清理自有实例。

## 当前可验证的内容

- 生产 Go 包的依赖方向、未批准导入和循环；检查器有反例测试。
- 更换资源读取适配器不修改应用用例；失败、过期或未知压力不会伪装成正常。
- 资源释放证据必须匹配实例代次与执行，取消请求本身不能释放容量。
- [OpenAPI 3.1.1 契约](api/openapi.json)、严格请求与可扩展响应，以及正常/中断/断流 fixture。
- 并行准入、共享加载、取消保留占用、观测失效与 critical 恢复；真实子进程夹具验证进程退出证据，区别于模型推理实测。

环境：Go 1.26.3；契约校验使用隔离的 Python 3.12 环境。首次执行：

```sh
make setup-tools
make check
make setup-client check-client
```

`setup-tools` 和 `setup-client` 使用独立环境及完整哈希锁。CI 配置 Linux/macOS Go 检查、契约及 Python 测试；发布和远端运行状态以实际结果记录，不把本地检查当作远端 CI 成功。

## 阅读顺序

1. [架构、端口与生产演进](docs/architecture/overview.md)
2. [上游能力、架构与三条技术路线](docs/research/upstream-comparison.md)
3. [公开价格、商业模式与产品定位](docs/research/pricing-and-business.md)
4. [本机环境与首轮验证边界](docs/local-machine.md)
5. [首版设计草案](docs/superpowers/specs/2026-11-08-adaptive-ai-gateway-design.md)
6. [阶段路线图](ROADMAP.md)
7. [来源索引与许可证文件指纹](docs/research/sources.json)
8. [领域术语](CONTEXT.md)与[实施细节审议](docs/plans/2026-11-08-implementation-review.md)
9. [实施计划与任务依赖](docs/plans/2026-11-08-implementation-plan.md)
10. [首版 HTTP 契约](docs/contracts/http-api.md)
11. [目标要求与验证证据](docs/architecture/verification.md)

## 首版希望验证的行为

用户继续使用浏览器、编辑器等应用；网关根据系统内存压力、CPU 负载和本地模型状态，决定是否接收新的推理请求。资源不足时使用有限队列或明确返回忙碌原因；资源恢复后再开放准入。模型空闲时允许卸载。

首版按真实模型标识提供服务；更换模型、改变质量档位和转发云端必须由显式配置决定。

## 仓库约定

初始研究日期为 **2026-11-08（Asia/Shanghai）**，架构修订日期为 **2026-11-09**。外部来源分别记录核对日期；本项目自己的设计、预算与性能目标单独标注。

本项目采用 [Apache-2.0](LICENSE)。第三方声明见 [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)；引擎和模型遵循各自的许可。本仓库没有复制上游推理实现。

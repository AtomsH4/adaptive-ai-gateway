# 目标要求与验证证据

核验日期：2026-11-09（Asia/Shanghai）。目标为用户确认的“开发架构和计划，保证低耦合、可扩展性，便于未来向生产规格发展”。本轮验收对象是架构、实施计划和验证这些边界的基础代码；完整本机推理服务属于后续 T1–T8。

## 1. 要求到证据

| 要求 | 已交付证据 | 验证与实际边界 |
| --- | --- | --- |
| 低耦合的模块边界 | [架构主文档](overview.md)、[依赖清单](../../architecture/dependencies.json)、[检查器](../../internal/architecture/check.go) | 当前生产包导入检查通过；反例覆盖网络/进程越界、应用依赖适配器、反向依赖、未批准外部包、未登记模块与循环；也扫描非本机平台文件 |
| HTTP 不绕过应用获得引擎权限 | 入站 [api](../../internal/application/api/resources.go) 与出站 [ports](../../internal/application/ports/runtime.go) 分包 | 以仓库实际清单测试 HTTP 导入出站端口、应用实现和引擎适配器均被拒绝；不把导入检查当作全部运行行为的证明 |
| 后端、平台与规则可替换 | [扩展矩阵](overview.md#5-扩展方式)、小接口与缺省为 false 的能力声明 | [只读用例测试](../../internal/application/assessment_test.go) 将两个独立读取实现注入同一用例，并通过入站查询接口调用；尚无第二个真实推理后端或平台 |
| 资源保护语义不藏在协议层 | [领域值](../../internal/domain/resources.go)、纯 [ResourceGuard](../../internal/policy/resourceguard.go)、[状态契约](runtime-contracts.md) | 测试覆盖未知、失败、过期、未来时间、临界新鲜度和 critical 分类；高自身 CPU/缺失可选 GPU 不影响内存分类。实际准入、恢复窗口和紧急回收仍待 T2–T4 |
| 取消和释放具有可扩展的所有权边界 | [实例/执行与证据](../../internal/domain/runtime.go)、[ADR-0003](../adr/0003-output-and-resource-completion.md) | 测试拒绝旧实例代次、错误执行、取消请求和空证据；这是证据范围校验，未证明真实引擎已经停止或操作系统已释放内存 |
| 对外契约能独立演进 | [OpenAPI](../../api/openapi.json)、[语义说明](../contracts/http-api.md)、[fixture](../../api/fixtures/cases.json) | 17 个 JSON 场景与 5 个 SSE 场景通过；严格请求、可扩展响应、未知错误码和不完整流可区分。未调用真实 HTTP 推理端点 |
| 有可执行的实施顺序 | [实施计划](../plans/2026-11-08-implementation-plan.md)、[路线图](../../ROADMAP.md) | T0–T8 分别定义产物、前置条件与完成检查；T0 基础完成，T1–T8 的运行能力未验收 |
| 有向生产规格发展的路径 | [生产演进门槛](production-evolution.md)、[ADR-0002](../adr/0002-modular-monolith-and-ports.md) | 依次定义可用本机、可靠单机、远程 Worker 与生产集群；journal、重启对账、网络分区、SLO 和回滚各有进入条件，均未被当前测试冒充为已实现 |
| 决策和术语可追溯 | [领域词汇](../../CONTEXT.md)、[审议记录](../plans/2026-11-08-implementation-review.md)、[ADRs](../adr/0004-service-readiness-and-capacity.md)、[来源索引](../research/sources.json) | 已确认取舍进入文档；模型身份与运行配置分开，服务就绪与即时推理容量分开；来源日期、文件指纹与 CI 提交引用分开记录 |

## 2. 本地检查记录

环境为 macOS arm64，Go 1.26.3；契约工具运行在独立的 CPython 3.12.13 环境中。已执行 `make check`，退出码为 0：

| 实际执行 | 结果 | 能证明的范围 |
| --- | --- | --- |
| `go run ./cmd/archcheck` | 通过 | 已有生产 Go 源文件符合清单；尚未实现的模块由反例及后续检查约束 |
| `go test ./...` | 通过 | 已实现的资源分类、证据范围、读取替换与架构检查行为 |
| `go test -race ./...` | 通过 | 本次被测试路径未发现数据竞争；未来协调器尚不存在 |
| `go vet ./...` | 通过 | 当前 Go 代码的静态检查 |
| `.venv-tools/bin/python scripts/check_contracts.py` | 通过，17 JSON / 5 SSE | OpenAPI 及合成 fixture 的结构、约束与结束语义 |

另以只读脚本检查了 19 个 Markdown 文件的 83 个本地链接及锚点、5 个 JSON 文件和 CI YAML，均通过；CI 的 4 处 Action 引用与来源索引中的提交一致。`gofmt -l internal cmd` 和 `git diff --check` 无输出。上述检查是本地语法与一致性核验，不是远端 CI 执行记录。

工具依赖在 [完整哈希锁文件](../../tools/requirements.txt) 中固定，不进入 Go 运行依赖。已配置 [CI](../../.github/workflows/quality.yml) 的 Linux/macOS Go 检查与独立契约检查，Action 使用提交引用；本地结果不等于远端 CI 已运行。

## 3. 完成结论与下一工作包

架构与计划目标有对应产物和当前可执行证据，本轮基础可以作为后续实现入口。它降低跨层依赖风险，并明确新增能力的位置；真实平台、后端和生产可靠性仍需逐阶段证明。

下一工作包是 T1：配置校验、组合入口、日志、存活接口及有界关闭；随后以假引擎完成 T2–T5，再进入真实小模型和 Python 闭环。尚未安装推理引擎、下载模型权重或执行付费调用，未宣称完整调度、GPU 动态适配或生产服务已经可用。

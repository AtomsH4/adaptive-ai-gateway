# 首版 HTTP 契约草案

初稿：2026-11-08；实现修订：2026-11-09（Asia/Shanghai）。版本：**0.1.0 预发布契约；HTTP 与假引擎闭环已实现并验证，真实模型验收未完成**。

机器描述以 [OpenAPI](../../api/openapi.json) 为准；Markdown 解释语义和实现门槛。接口变更同时更新两者及 fixture。

这是本项目首版支持的调用子集。行为依据见 [设计草案](../superpowers/specs/2026-11-08-adaptive-ai-gateway-design.md)，Python 验收方式见 [实施计划](../plans/2026-11-08-implementation-plan.md#7-python-调用方实施提案)。不据此宣称任意模型提供商或 SDK 的全部功能兼容。

## 1. 通用规则

- 默认地址为 `http://127.0.0.1:8787`，请求及响应使用 UTF-8。
- `/healthz` 之外的接口要求 `Authorization: Bearer <本地令牌>`。首版使用单个本地令牌，缺失或无效时返回 401 / `authentication_failed`；配置和日志不回显令牌。
- 服务器为每次 HTTP 调用生成请求 ID，通过 `X-Request-ID` 返回。它用于诊断，不是幂等键，首版不提供重复请求去重或自动恢复执行的承诺。
- 请求体上限为 1 MiB；JSON 不合法或类型错误返回 400。超过大小限制返回 413 / `request_too_large`。
- 普通响应和错误为 JSON；流式生成成功开始后使用 `text/event-stream`。排队与冷启动期间不提前发送 200 或伪造生成数据。

## 2. 接口范围

| 接口 | 首版含义 |
| --- | --- |
| `GET /healthz` | 只报告控制进程存活；不表示模型或资源已就绪 |
| `GET /readyz` | 应用已初始化、可安全处理命令时返回 200；启动、关闭或结构故障时 503；不要求模型已加载或有立即执行槽位 |
| `GET /v1/models` | 返回唯一配置的真实模型 ID；不隐式加载模型 |
| `POST /v1/chat/completions` | 普通或流式文本聊天生成 |
| `GET /admin/resources` | 返回资源观测值、来源、新鲜度与不可用原因 |
| `GET /admin/scheduler` | 返回引擎、保护、队列及执行状态的元数据 |

诊断接口在观测不完整时仍可返回 200 和明确的 unavailable / stale 字段；不能为显示完整而填充零。诊断响应不包含原始 prompt、response 或密钥。

`/admin/scheduler` 分别报告 `service_ready`、`can_execute_now`、`can_queue`、引擎与保护状态。模型未加载、忙碌或临时观测保护不单独使服务未就绪，实际请求仍受完整准入约束；设计理由见 [ADR-0004](../adr/0004-service-readiness-and-capacity.md)。

## 3. 文本生成请求

| 字段 | 类型与约束 |
| --- | --- |
| `model` | 必填字符串，必须匹配唯一配置的真实模型 ID；未知值返回 404 / `model_not_found` |
| `messages` | 必填且非空的消息列表；每条包含 `role` 与字符串 `content` |
| `messages[].role` | 首版支持 `system`、`user`、`assistant` |
| `stream` | 可选布尔值，默认 false |
| `max_tokens` | 可选整数，范围 1–512，默认 512 |
| `temperature` | 可选有限数值，范围 0–2，默认 0.7 |

未知字段明确返回 400 / `unsupported_parameter`，不静默忽略。首版不支持图像或音频内容块、工具调用、结构化输出约束或多候选生成。服务端可以为真实引擎做明确的字段适配，不能让后端默认行为改变已声明的请求含义。

包含聊天模板开销的输入加预留输出，须在已配置上下文容量内。不足时返回 400 / `context_length_exceeded`，不静默截断、换模型或缩短请求。精确检查由真实引擎适配器验证；发生在提交执行的受控路径中，计入执行预算。

## 4. 等待与成功结果

请求可以在同一连接中等待共享模型加载后继续生成。普通排队累计 30 秒、一次共享加载 120 秒、执行 120 秒、整个请求 270 秒是可配置实验初值。总时限始终生效，状态变化不重置预算。

所有未执行请求共用上限为 4 的等待容量，加载等待者也计入；首版按接受顺序处理仍有效的请求，取消项及时移除，不增加隐式优先级。

普通成功响应提供单个 choice，至少包含生成 ID、对象类型、真实模型 ID、assistant 消息与 finish reason；返回格式采用文本聊天完成的字段结构。正常 EOS 使用 `stop`，正常达到输出上限使用 `length`。只有后端提供可信计量时才返回 token usage，未知不填零。

## 5. 错误结果

错误 JSON 至少提供 `error.code`、`error.message`、`error.type`、`error.param` 与 `request_id`。代码供程序判断，message 为可读说明，非字段错误的 param 为 null。

| HTTP 状态 | error.code | 条件 |
| --- | --- | --- |
| 400 | `invalid_request`、`unsupported_parameter`、`context_length_exceeded` | 请求不符合首版契约 |
| 401 | `authentication_failed` | 本地令牌缺失或无效 |
| 404 | `model_not_found` | 模型 ID 不匹配 |
| 413 | `request_too_large` | 请求体超过上限 |
| 429 | `queue_full` | 有界等待容量已满 |
| 429 | `queue_timeout` | 普通执行槽位或排空等待预算到期 |
| 503 | `telemetry_unavailable`、`telemetry_stale` | 关键内存观测阻塞至等待预算到期 |
| 503 | `memory_pressure`、`cpu_busy` | 对应资源条件阻塞至等待预算到期；critical 内存压力可立即拒绝 |
| 503 | `memory_pressure_interrupted` | 已获准的生成被紧急回收中断，且尚未发送成功响应 |
| 503 | `engine_unavailable`、`model_load_failed` | 引擎不可用或单次加载失败 |
| 503 | `service_not_ready`、`service_stopping` | 应用未完成初始化、结构故障或正在关闭 |
| 502 | `upstream_error` | 已执行的上游协议或响应异常 |
| 504 | `model_load_timeout`、`execution_timeout`、`request_timeout` | 对应加载、执行或总时限到期 |

多个期限同时到期时，先判断请求总时限，其次共享加载时限，再判断当前阶段预算。普通等待到期的错误码依据当前实际阻塞原因；关键观测失效不能误报为 429。critical 紧急回收对新准入和等待队列的处理按设计草案立即执行。

流已经开始后，不能改变已发送的 HTTP 状态；改为发送明确的流式错误并关闭连接。错误不触发整次自动重试，也不把当前请求重新加入第二轮加载或执行。

## 6. 流式结束与取消

正常数据以 SSE `data` 字段承载 JSON 文本聊天分片，包含 choice 的增量内容。正常结束先提供非空 finish reason，再发送 `data: [DONE]` 终止标记。

流中错误使用携带 `error` 对象及 `request_id` 的 SSE data 消息，随后关闭流，不补发成功 finish reason 或 `[DONE]`。如果错误事件本身无法写出，直接关闭连接；Python 调用方依据缺少正常终止标记识别不完整结果。

仅收到部分内容、只有 EOF、或只收到终止标记却缺少合法结束信息，都不计为完整成功。流式达到 token 上限以正常 `length` 表达，不与资源压力中断混用。

客户端取消或超时关闭连接时，网关移除对应等待项或传播执行取消。已加载模型、执行槽位和加载预留何时释放，依据引擎完成或进程退出证据判断，不因 HTTP 已关闭提前释放。

## 7. 观测保护

关键内存压力不可用或过期时，不开始新的模型加载或生成，即使模型已加载。现有生成继续受原时限保护；未知本身不触发紧急强制中断。

有等待容量的请求可以等待恢复。正在等待既有共享加载时计算同一个加载期限，其他观测恢复等待计入累计普通排队预算，不能重复扣两种阶段预算；请求总时限始终生效。恢复后仍沿用已消耗的预算。新鲜 warning 不直接恢复正常准入，新鲜 critical 进入紧急回收。可选 GPU 利用率缺失不触发这项限制。

## 8. 契约验证任务

固定一个 Python 异步 HTTP 调用方，分别验证请求字段、鉴权、模型身份、过大请求、队列满、加载等待、各类超时、取消、错误事件和正常结束。流式解析覆盖跨网络分片的 UTF-8 与 SSE 事件边界，不能将网络分片直接当作一条事件。

先通过假引擎与可控时间验证上述行为，再用固定的小模型和引擎版本确认适配器实际兑现契约。两类结果分别报告，文档完成不等于接口已经实现或验证通过。

`make contracts` 验证合成 fixture；`scripts/check_live.py` 已对运行中的假引擎服务验证 5 个 GET、JSON 生成及 SSE 生成。原生子进程夹具和真实模型分别报告，见 [运行验证](../reports/2026-11-09-runtime-validation.md)。调度诊断还提供最多 128 条近期决策元数据。

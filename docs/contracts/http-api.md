# HTTP contract

Initial draft: 2026-11-08. Implementation update: 2026-11-09 (Asia/Shanghai). Version: **0.1.0 pre-release**. HTTP and the synthetic-backend path are implemented and checked; real-model acceptance remains pending.

[OpenAPI](../../api/openapi.json) is the machine-readable contract. Update the schema, this document, and fixtures together when changing the interface. The [design](../superpowers/specs/2026-11-08-adaptive-ai-gateway-design.md) defines behavior; the [Python client plan](../plans/2026-11-08-implementation-plan.md#7-python-client) defines the initial integration path. Compatibility is limited to the documented subset.

## 1. General rules

- Default address: `http://127.0.0.1:8787`. Requests and responses use UTF-8.
- Every endpoint except `/healthz` requires `Authorization: Bearer <local-token>`. Missing or invalid tokens return 401 / `authentication_failed`. Logs and configuration diagnostics exclude token values.
- The server generates an ID for each HTTP call and returns it in `X-Request-ID`. This diagnostic ID supplies no idempotency, duplicate suppression, or execution-recovery guarantee.
- Request bodies are limited to 1 MiB. Malformed JSON or invalid types return 400; oversized bodies return 413 / `request_too_large`.
- Ordinary responses and errors use JSON. Streaming generation uses `text/event-stream` after valid output begins. Queuing and cold start send no premature 200 response or fabricated generation events.

## 2. Endpoints

| Endpoint | Meaning |
| --- | --- |
| `GET /healthz` | Control-process liveness |
| `GET /readyz` | 200 when initialized and able to process commands safely; 503 during startup, shutdown, or structural failure. A loaded model or free execution slot is not required |
| `GET /v1/models` | The single configured model ID; querying does not load it |
| `POST /v1/chat/completions` | Ordinary or streaming text chat generation |
| `GET /admin/resources` | Observations, sources, freshness, and unavailable states |
| `GET /admin/scheduler` | Engine, protection, queue, and execution metadata |

Diagnostics can return 200 with unavailable or stale fields. Missing measurements remain explicit. Responses exclude raw prompts, generated content, and keys.

Scheduler diagnostics report `service_ready`, `can_execute_now`, `can_queue`, engine state, and protection state separately. Admission checks apply even when the service is ready. See [ADR-0004](../adr/0004-service-readiness-and-capacity.md).

## 3. Generation request

| Field | Type and constraint |
| --- | --- |
| `model` | Required string matching the configured model ID; otherwise 404 / `model_not_found` |
| `messages` | Required nonempty list of objects with `role` and string `content` |
| `messages[].role` | `system`, `user`, or `assistant` |
| `stream` | Optional boolean, default false |
| `max_tokens` | Optional integer, 1-512, default 512 |
| `temperature` | Optional finite number, 0-2, default 0.7 |

Unknown fields return 400 / `unsupported_parameter`. Image/audio blocks, tool calls, structured-output constraints, and multiple candidates are outside this version. Backend field mapping must preserve the declared request meaning.

Input tokens, including chat-template overhead, plus reserved output must fit the configured context. Exceeding capacity returns 400 / `context_length_exceeded`. The adapter validates the exact limit on the controlled dispatch path, within the execution budget. Requests retain their model and requested context rather than being silently truncated or redirected.

## 4. Waiting and success

A connection can wait for a shared load and then generate. Initial configurable budgets are 30 seconds of cumulative ordinary waiting, 120 seconds per shared load, 120 seconds of execution, and 270 seconds total. State changes do not reset them.

At most four unexecuted requests can wait, including cold-start waiters. Valid requests proceed in acceptance order; cancelled items are removed without implicit priorities.

An ordinary success has one choice and includes the completion ID, object type, model ID, assistant message, and finish reason. `stop` denotes normal EOS; `length` denotes the output limit. Include token usage only when the backend provides reliable counts.

## 5. Errors

Error JSON includes `error.code`, `error.message`, `error.type`, `error.param`, and `request_id`. Programs use the code; the message is descriptive. `param` is null for errors unrelated to a field.

| HTTP status | `error.code` | Condition |
| --- | --- | --- |
| 400 | `invalid_request`, `unsupported_parameter`, `context_length_exceeded` | Invalid or unsupported request |
| 401 | `authentication_failed` | Missing or invalid token |
| 404 | `model_not_found` | Model ID mismatch |
| 413 | `request_too_large` | Body limit exceeded |
| 429 | `queue_full` | Waiting capacity exhausted |
| 429 | `queue_timeout` | Ordinary slot/draining wait expired |
| 503 | `telemetry_unavailable`, `telemetry_stale` | Required memory observation blocked work until wait expiry |
| 503 | `memory_pressure`, `cpu_busy` | Resource constraint persisted until wait expiry; critical memory pressure can reject immediately |
| 503 | `memory_pressure_interrupted` | Emergency reclamation interrupted admitted work before a success response was sent |
| 503 | `engine_unavailable`, `model_load_failed` | Unavailable engine or failed load attempt |
| 503 | `service_not_ready`, `service_stopping` | Initialization, structural fault, or shutdown |
| 502 | `upstream_error` | Executed upstream protocol/response failure |
| 504 | `model_load_timeout`, `execution_timeout`, `request_timeout` | Load, execution, or total deadline expired |

If deadlines coincide, apply total request expiry first, shared-load expiry second, then the current phase budget. A waiting error reflects the actual blocker; required-telemetry failure returns 503 rather than an ordinary queue 429. Critical pressure follows the immediate queue handling in the design.

After streaming starts, send an error event and close the connection. The sent HTTP status cannot change. A failed call is neither retried nor requeued for another load or execution attempt.

## 6. Streaming and cancellation

SSE `data` fields contain JSON chat-completion chunks with incremental choice content. Normal termination sends a nonempty finish reason followed by `data: [DONE]`.

A stream error sends an `error` object and `request_id`, then closes. It sends no success finish reason or `[DONE]`. If the error itself cannot be written, close directly; the client recognizes the missing normal terminator.

Partial content, EOF alone, or `[DONE]` without a valid finish reason is incomplete. A normal token limit uses `length`, distinct from resource interruption.

Caller cancellation removes waiting demand or propagates execution cancellation. The engine's confirmed completion or process exit determines slot, load-reservation, and resident-model release; HTTP closure alone cannot release them.

## 7. Observation guard

Unavailable or stale required memory pressure blocks new loads and executions, including for a resident model. Active generation retains its original deadline. Unknown data alone does not trigger forced interruption.

Requests can wait for recovery within capacity. Existing shared-load wait uses that load's deadline; other recovery waiting consumes cumulative ordinary queue time. A phase is charged once, the total deadline always applies, and recovery preserves consumed budgets. Fresh warning does not restore normal admission; fresh critical starts reclamation. Missing optional GPU utilization does not trigger this guard.

## 8. Contract validation

The initial Python async client must cover fields, authentication, identity, body limits, queue capacity, loading, deadlines, cancellation, errors, and normal termination. Stream parsing must handle UTF-8 and SSE boundaries split across network chunks.

Use a fake backend and controlled time before testing the pinned model/engine. Record those results separately.

`make contracts` checks synthetic fixtures. `scripts/check_live.py` passed five GET endpoints, JSON generation, and SSE generation against a running synthetic service. Native-child fixtures and model acceptance are described in the [runtime report](../reports/2026-11-09-runtime-validation.md). Scheduler diagnostics retain at most 128 recent decision records.

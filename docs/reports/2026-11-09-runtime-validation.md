# Local prototype validation

Recorded 2026-11-09 (Asia/Shanghai). The control and synthetic-backend paths are implemented and tested. Real-model execution and everyday application comparisons remain pending.

## 1. Implementation and acceptance status

| Task | Implementation | Evidence and remaining work |
| --- | --- | --- |
| T1 | Go entry point, strict TOML, token environment, metadata logs, loopback server, bounded shutdown | Invalid-config cases, live server, clean SIGTERM exit; owned-child cleanup tested with a real process fixture |
| T2 | Serial coordinator, capacity ledger, bounded waiting, shared loading, cumulative budgets | Concurrent capacity, simulated 45-second loading, shared deadline, cancellation retaining occupancy, observation waits, critical recovery |
| T3 | Native process adapter, incarnations, digests, health observation, release supervision | Real child fixture covered output, cancellation without release proof, matching exit, foreign-owner rejection; the fixture runs no model |
| T4 | macOS pressure, conservative memory headroom, CPU attribution over a common window | Mach/libproc counters and tests; changed PID start identity invalidates attribution; GPU utilization unavailable |
| T5 | Six HTTP endpoints, auth, validation, JSON/SSE, cancellation, bounded output | Live synthetic service passed 5 GET + 1 JSON + 1 SSE schema/termination checks; request and error tests are separate |
| T6 | Pinned artifact downloader and native configuration | Artifact sizes/digests and executable version verified; memory protection prevented loading, so generation, peak memory, and unloading remain unverified |
| T7 | Python 3.12.13 / HTTPX 0.28.1 async example and hash lock | Ordinary/streaming synthetic calls, terminal markers, unknown errors, total timeout passed; real-model client acceptance awaits T6 |
| T8 | Fixed versus resource-aware comparison criteria | Not run; no application-responsiveness improvement has been established |

Only the coordinator changes occupancy. Blocking work runs through ports. Deadlines use the injected clock and are checked before command processing. Output is bounded by 16 events and 64 KiB per request; recent decisions are limited to 128 metadata records. Raw prompts and responses are excluded from those records.

Native cancellation closes the call and requests a stop. Disconnects, EOF, and a single idle observation do not prove cancelled computation ended. Ambiguous work retains occupancy until owned-process exit is confirmed. The backend uses a separate temporary local token and inherits only required OS environment variables.

CPU process counters are converted from Mach time to nanoseconds. A Darwin regression compares the result with system `getrusage`.

## 2. Executed checks

- `make check`: dependency rules, Go tests/race/vet, 19 JSON and 5 SSE synthetic cases.
- `make check-client`: Python termination and cancellation cases.
- `make build`: local control binary.
- `scripts/check_live.py --model validation-model`: live synthetic HTTP contract checks.
- Python ordinary and streaming calls: both returned `stop` with synthetic text.
- SIGTERM: service exit code 0; real-child fixtures separately checked exit evidence.

These local results were checked before the runtime commit. Remote CI is tracked through the repository's Actions records.

## 3. Recorded host resource gate

The official v0.9.0 `nightly-tag.txt` identified b12110. The macOS arm64 archive SHA-256 was `4f5b8de12e0b9516658ca6a5764c6ffb3d7c659c3dd538556b1413c7b4050662`; `--version` reported `0.9.0-dev (build 12110, commit cd959bae9)`. The 491400032-byte Qwen artifact matched its pinned digest. See the [baseline metadata](../research/small-model-baseline.json).

A controlled gateway run recorded:

- Fresh warning memory pressure and about 178.0 MiB of free + speculative pages, below the configured 2 GiB load headroom.
- Reliable CPU attribution for that window: about 57.93% total and 0.1093% owned usage.
- Ready service, unloaded model, zero executions; generation returned `503 / memory_pressure` after about 30.012 seconds.
- Gateway RSS of 11328 KiB (about 11.1 MiB), followed by exit code 0. Protection thresholds remained enabled.

Free + speculative pages are a conservative estimate, not all reclaimable cache. The 2 GiB setting is not a measured model peak. This run validates the refusal path under the recorded conditions; model execution and memory-capacity acceptance remain open.

## 4. Remaining acceptance work

1. With normal pressure and sufficient configured headroom, measure cold start, generation, cancellation, idle unloading, memory peaks, and context behavior. Calibrate budgets from those results.
2. Repeat the native path with the Python example, then run the T8 application comparison.
3. Track automatic profile selection, hot updates, journals, and crash recovery as separate future tasks. Current configuration changes require restart.

Source is published at [AtomsH4/adaptive-ai-gateway](https://github.com/AtomsH4/adaptive-ai-gateway). Public visibility and SSH identity were verified, and code was pushed to `main`. Use the Actions records for remote CI results.

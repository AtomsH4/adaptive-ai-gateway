# Machine baseline

Collected 2026-11-08 (Asia/Shanghai) using read-only `system_profiler`, `sysctl`, runtime-version, and command-path checks. This snapshot preceded inference benchmarks.

## Observed environment

| Item | Observation |
| --- | --- |
| OS | macOS 26.5.2, Darwin arm64 |
| Chip | Apple A18 Pro |
| CPU | 6 cores: 2 performance and 4 efficiency cores, as reported by the system |
| GPU | 5 cores with Metal support, as reported by the system |
| Physical memory | 8,589,934,592 bytes (8 GiB) |
| Go | 1.26.3, darwin/arm64 |
| Other tools | Git, uv, Python 3.9.6, Node, Homebrew |
| Inference tools | `ollama` and `llama-server` were not found in PATH; other locations were not excluded |
| Build tools | `cmake` was not found in PATH |

Serial numbers, platform UUIDs, display serial numbers, and complete raw hardware reports are excluded from the repository.

## Design implications

1. Validate a native Metal engine first. The llama.cpp [build documentation](https://github.com/ggml-org/llama.cpp/blob/master/docs/build.md#metal-build) describes default Metal support on macOS.
2. CPU and GPU share physical memory on Apple unified-memory hardware. Use one capacity pool. See Apple's [unified-memory discussion](https://developer.apple.com/videos/play/tech-talks/10580/) and [Metal working-set API](https://developer.apple.com/documentation/metal/mtldevice/recommendedmaxworkingsetsize).
3. Start lifecycle measurements with a quantized text model around 0.5B-1.5B parameters, one loaded model, one execution, and a short context. Verify license, revision, and digest independently. This is an experimental starting point; compatibility and speed require measurements.
4. Treat GPU utilization as optional. When unavailable without elevated privileges, report that state and use memory pressure, CPU information, engine state, and queue state conservatively.
5. Keep memory headroom, recommended Metal working set, and process RSS as distinct observations. The recommended working set is performance guidance, not an independent memory pool or reservation.

## Measurement record

Record model digest, engine version, context, concurrency, and maximum output. Measure idle and representative application loads, observation freshness, cold start, control/engine memory, queue time, first-token latency, and generation speed. Exercise missing metrics, queue expiry, caller cancellation, and load failure.

No engine or model had been installed at the time of this snapshot. Later downloads and runtime checks are recorded in the [runtime report](reports/2026-11-09-runtime-validation.md); real-model execution and foreground-application improvements remain unverified there.

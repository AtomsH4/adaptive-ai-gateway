# 本机环境与验证边界

采集日期：2026-11-08（Asia/Shanghai）。依据本机只读的 `system_profiler`、`sysctl`、运行时版本与命令路径检查；未做推理基准测试。

## 已观察到的环境

| 项目 | 观察结果 |
| --- | --- |
| 系统 | macOS 26.5.2，Darwin arm64 |
| 芯片 | Apple A18 Pro |
| CPU | 6 核，系统报告为 2 个性能核和 4 个能效核 |
| GPU | 系统报告 5 核，支持 Metal |
| 物理内存 | 8,589,934,592 bytes，即 8 GiB |
| Go | 1.26.3，darwin/arm64 |
| 其他开发工具 | Git、uv、Python 3.9.6、Node、Homebrew 可用 |
| 推理工具 | 当前 PATH 中未发现 `ollama` 或 `llama-server`；这不证明其他目录没有安装 |
| 构建工具 | 当前 PATH 中未发现 `cmake` |

仓库不保存机器序列号、平台 UUID、显示器序列号或原始完整硬件报告。

## 对技术路线的影响

1. 优先验证原生 Metal 引擎。llama.cpp 官方 [构建文档](https://github.com/ggml-org/llama.cpp/blob/master/docs/build.md#metal-build) 说明 macOS 默认构建 Metal 支持。
2. Apple 统一内存设备中 CPU 与 GPU 共享物理内存；不要把 8 GiB RAM 和某个 GPU 指标再相加为可用总容量。依据 Apple 的 [统一内存说明](https://developer.apple.com/videos/play/tech-talks/10580/) 与 [Metal 工作集接口](https://developer.apple.com/documentation/metal/mtldevice/recommendedmaxworkingsetsize)。
3. 首次实机验证建议选择约 0.5B–1.5B 参数的量化文本模型、单模型、单并发、较短上下文。这是保守的实验起点，不是已验证的兼容性或速度承诺；具体模型须单独确定许可、版本和校验值。
4. 首版无需管理员权限的 GPU 全局利用率采集。若指标不可得，就标为 unavailable，并以系统内存压力、CPU、引擎状态和队列情况作保守决策。
5. RAM 余量、Metal 建议工作集、进程驻留内存属于不同观测；Metal 建议工作集是性能参考，不是独立显存池或已预留容量。

## 首轮测量应记录什么

- 模型文件校验值、引擎版本、上下文配置、并发和最大输出长度。
- 系统空闲与代表性日常应用负载下的内存压力、CPU 和采样新鲜度。
- 模型冷启动时间、控制进程内存、引擎内存、排队时长、首 token 时间与生成速度。
- 指标缺失、队列超时、客户端取消和引擎加载失败时的实际行为。

本轮未安装引擎或下载模型，所以模型可运行性、端到端延迟和对前台应用的改善均尚未验证。

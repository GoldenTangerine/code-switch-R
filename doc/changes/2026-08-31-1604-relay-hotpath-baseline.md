<!--
@name: 代理热路径性能基线
@Descripttion: 测量代理请求读取、响应 Hook、负载捕获与脱敏成本并记录 B03 完成状态
@version: 1.0.0
@Author: sm
@Date: 2026-08-31 16:04:13
@LastEditTime: 2026-08-31 16:04:13
@FilePath: doc/changes/2026-08-31-1604-relay-hotpath-baseline.md
-->

# 代理热路径性能基线

- 变更时间：2026-08-31 16:04:13 CST（Asia/Shanghai）
- 涉及范围：代理热路径行为测试、Benchmark、代码与性能优化计划 B03 状态

## 变更内容

- 新增 `services/providerrelay_hotpath_benchmark_test.go`，使用生成负载和 `httptest`，不访问真实 Provider 或用户日志。
- 覆盖 1 KiB、64 KiB、1 MiB 和 8 MiB 附近的请求读取、捕获关闭、原始捕获和脱敏捕获。
- 覆盖非流式直接复制、日志 Hook、64 KiB SSE 逐行 Hook、负载截断及输出一致性。
- 新增 3 个行为测试入口和 32 个子基准，记录 `ns/op`、`B/op`、`allocs/op` 和吞吐。
- 直接测量会话标识与敏感关键词快速检查，确认正则扫描及请求体重复脱敏是捕获开启后的主要成本。
- 更新 `doc/code-performance-optimization-plan.md`：B03 标记完成，OPT-004 写入测量证据，新增 OPT-014/B12。
- 未修改 `services/providerrelay.go` 或其他生产代码，未修改捕获、脱敏、截断、日志、协议、配置或依赖。

## 关键测量

- 1 MiB 请求读取：271.933-285.656 µs/op，约 5.24 MB/op。
- 1 MiB 非流式响应：无 Hook 55.289-60.262 µs/op；日志 Hook 且捕获关闭 2.696-2.802 ms/op。
- 1 MiB 捕获：原始 419.086-451.244 ms/op；脱敏 1.346-1.419 s/op。
- 8 MiB 附近捕获：原始 3.143-3.431 s/op；脱敏 10.319-10.432 s/op。
- 1 MiB 无匹配快速检查：会话标识 135.047-157.596 ms/op；敏感关键词 74.378-76.380 ms/op。
- 64 KiB SSE：无 Hook 56.251-59.658 µs/op；捕获关闭 0.924-1.004 ms/op；原始捕获 10.368-10.536 ms/op；脱敏捕获 32.483-32.741 ms/op。
- 完整 5 轮数据、环境和适用边界见总计划 B03 执行记录。

## 验证结果

- 新增 B03 行为测试及既有负载脱敏、响应转发测试：通过。
- `go test -race ./services/ -run 'TestRelayHotPath' -count=1`：通过，耗时 74.184 秒。
- 1 MiB 及以下各基准使用默认窗口独立运行 5 次：通过。
- 8 MiB 附近基准使用 `-benchtime=1x -count=5`：通过。
- `go test ./services/... -count=1`：通过，耗时 21.701 秒。
- 一次并行运行完整回归与高内存竞态检测时完整回归失败且日志截断；随后串行独立运行均通过，最终结果以串行命令为准。
- Go 测试仅出现既有 macOS 链接目标版本警告。
- 按项目限制未运行 lint、构建或开发服务。

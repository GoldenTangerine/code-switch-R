<!--
@name: 黑名单失败路径并发基线
@Descripttion: 记录同一与不同 Provider 并发失败记录的等待、分配和队列写入数据
@version: 1.0.0
@Author: sm
@Date: 2026-09-01 02:24:42
@LastEditTime: 2026-09-01 02:24:42
@FilePath: doc/changes/2026-09-01-0224-blacklist-failure-contention-baseline.md
-->

# 黑名单失败路径并发基线

## 变更时间

2026-09-01 02:24:42 CST（Asia/Shanghai）

## 涉及范围

- `services/blacklist_failure_benchmark_test.go`
- `services/blacklistservice.go`（仅测量）
- `services/dbqueue.go`（仅测量）
- `doc/code-performance-optimization-plan.md`

## 变更内容

- 新增首次已绑定、首次未绑定、常规递增、阈值拉黑、去重命中和已拉黑六类固定语义夹具。
- 新增同/不同 Provider × 1/8/32 并发 Benchmark，报告吞吐、P50/P95、分配和队列写次数。
- 每轮断言数据库状态、运行时快照、拉黑原因和最终计数，不修改生产锁、队列、配置或 schema。
- 根据数据延期缩锁或 Provider 分片实现，保留现有全局串行语义。

## 验证结果

- 六类语义夹具、Blacklist/DBWriteQueue/调度目标回归通过。
- 5 轮正式基准中，32 路不同 Provider 的中位吞吐为 9,196 calls/s，P50 1.781 ms，P95 3.350 ms；32 路同 Provider 为 23,390 calls/s、0.739 ms、1.305 ms。
- 32 路不同 Provider 为 666,319-668,308 B/op、18,763-18,768 allocs/op；每批 32 次队列写入。
- 并发缩小基准在 `-race` 下通过。
- 排除独立登记的 OPT-022 后完整 services 回归通过；仅有既有 macOS 链接目标版本警告。

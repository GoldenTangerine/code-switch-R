<!--
@name: SQLite 双写队列正确性与性能基线
@Descripttion: 为 DBWriteQueue 增加专属测试和 Benchmark，并记录 B02 完成状态
@version: 1.0.0
@Author: sm
@Date: 2026-08-31 15:34:12
@LastEditTime: 2026-08-31 15:34:12
@FilePath: doc/changes/2026-08-31-1534-dbqueue-baseline.md
-->

# SQLite 双写队列正确性与性能基线

- 变更时间：2026-08-31 15:34:12 CST（Asia/Shanghai）
- 涉及范围：DBWriteQueue 专属测试、性能基线、代码与性能优化计划 B02 状态

## 变更内容

- 新增 `services/dbqueue_test.go`，全部使用临时 SQLite，不读取用户数据库。
- 覆盖单次写入顺序、事务组成功与原子回滚、50 条并发批量写入、上下文取消、关闭排空和关闭后拒绝写入。
- 单次与批量模式均覆盖取消和关闭分支；同步条件不依赖任意 `Sleep`。
- 新增单次、批量和双语句事务组三个 Benchmark，分别记录端到端耗时、吞吐、分配和现有 QueueStats。
- 更新 `doc/code-performance-optimization-plan.md`：B02 和 OPT-007 标记完成，记录 5 轮基准数据。
- 初始事务组基准复用任务切片时发现 `send on closed channel`，已按当前生产调用模式执行基准，并新增 OPT-013/B11 单独处理。
- 未修改 `services/dbqueue.go` 或其他生产代码，未修改队列容量、批量窗口、超时、数据库结构、配置或依赖。

## 验证结果

- `go test ./services/ -run 'TestDBWriteQueue' -count=1 -v`：通过。
- `go test ./services/ -run 'TestDBWriteQueue' -count=20`：通过。
- `go test -race ./services/ -run 'TestDBWriteQueue' -count=1`：通过，未发现竞态。
- `go test ./services/ -run '^$' -bench 'BenchmarkDBWriteQueue' -benchmem -count=5`：通过。
- 基准环境：macOS 27.0、Apple M4、darwin/arm64、Go 1.25.5、临时 SQLite WAL 单连接、队列容量 5000。
- 5 轮端到端范围：单次写入 35.034-37.756 µs/op；批量同步调用 1.000656-1.006196 ms/op；双语句事务组 42.787-48.801 µs/op。
- 分配范围：单次 657-658 B/op、13 allocs/op；批量 706-717 B/op、15 allocs/op；事务组 1491-1493 B/op、32 allocs/op。
- `go test ./services/...`：通过。
- Go 测试仅出现既有 macOS 链接目标版本警告。
- 按项目限制未运行 lint、构建或开发服务。

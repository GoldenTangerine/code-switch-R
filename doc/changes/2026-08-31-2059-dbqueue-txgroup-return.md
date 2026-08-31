<!--
@name: 事务组返回所有权边界
@Descripttion: 记录 ExecTxGroup 正常返回前等待全部任务结果关闭的竞态修复与基准
@version: 1.0.0
@Author: sm
@Date: 2026-08-31 20:59:57
@LastEditTime: 2026-08-31 20:59:57
@FilePath: doc/changes/2026-08-31-2059-dbqueue-txgroup-return.md
-->

# 事务组返回所有权边界

## 变更时间

2026-08-31 20:59:57 CST（Asia/Shanghai）

## 涉及范围

- `services/dbqueue.go`
- `services/dbqueue_test.go`
- `doc/code-performance-optimization-plan.md`

## 变更内容

- 新增同一双任务 `[]WriteTask` 顺序复用 1000 次的回归测试，并核对 2000 条写入完整落库。
- 修改前普通测试可能因调度时序通过，但 `go test -race` 稳定捕获调用方覆写 `Result` 与 worker 关闭后续结果通道的竞态。
- `ExecTxGroup` 在收到首个共享事务结果后，等待组内全部结果通道关闭再返回，保证正常返回时 worker 已完成对任务对象的访问。
- 保留调用方 `Result` 回填、事务开启、SQL 顺序、回滚、提交、错误文本、超时、容量和 QueueStats 语义。
- 30 秒超时后任务继续执行是既有契约；超时后并发复用同一切片不在本批范围。
- 未修改 ProviderStore 调用、数据库 schema、配置、公共 API 或依赖。

## 性能对比

`BenchmarkDBWriteQueue/tx_group` 各 5 轮：

- 修改前：39,046-41,192 ns/op，中位数 39,830 ns/op；1,492-1,494 B/op；32 allocs/op。
- 修改后：38,530-54,857 ns/op，中位数 39,360 ns/op；1,491-1,494 B/op；32 allocs/op。
- 区间重叠且修改后包含调度离群值，结论为无可确认性能回退，不宣称吞吐提升。

## 验证结果

- 修改前目标 `go test -race` 稳定报告数据竞态，修改后通过。
- 目标事务成功、回滚和复用测试连续 20 次通过。
- 全部 DBWriteQueue 测试的 `go test -race` 通过。
- ProviderStore 相关测试通过。
- `go test ./services/... -count=1` 通过。
- `go test . -count=1` 通过。
- `git diff --check` 通过。
- Go 测试仅有项目既有 macOS 链接目标版本警告。

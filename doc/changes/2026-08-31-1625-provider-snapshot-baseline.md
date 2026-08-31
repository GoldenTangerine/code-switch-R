<!--
@name: 供应商快照性能基线
@Descripttion: 测量 Provider 深拷贝、快照命中、指纹与稳定刷新成本并记录 B04 完成状态
@version: 1.0.0
@Author: sm
@Date: 2026-08-31 16:25:11
@LastEditTime: 2026-08-31 16:25:11
@FilePath: doc/changes/2026-08-31-1625-provider-snapshot-baseline.md
-->

# 供应商快照性能基线

- 变更时间：2026-08-31 16:25:11 CST（Asia/Shanghai）
- 涉及范围：Provider 深拷贝隔离、快照性能基线、代码与性能优化计划 B04 状态

## 变更内容

- 新增 `services/provider_snapshot_benchmark_test.go`，使用 TestMain 临时 SQLite 和生成 Provider，不读取用户配置。
- 补齐 CLIConfig、Claude Desktop 路由、并发限制、预算、额度查询和内部错误切片的深拷贝隔离测试。
- 验证当前语义指纹与存储行候选均能检测嵌套 payload 变化。
- 新增 15 个子基准，覆盖 1/10/100 个 Provider 的直接克隆、快照命中、当前语义指纹、存储行候选和稳定刷新。
- 记录每次操作的查询数、行数、语义字节、耗时和分配。
- 更新 `doc/code-performance-optimization-plan.md`：B04 标记完成，OPT-005/006 写入测量数据，新增 B13。
- 未修改 `services/providerservice.go`、`services/providerstore.go` 或其他生产代码，未修改快照、存储、轮询、配置或依赖。

## 关键测量

- 10 个 Provider：快照命中 38.624-41.152 µs/op，约 57.2 KB/op、951 allocs/op。
- 100 个 Provider：快照命中 390.187-414.853 µs/op，约 568.0 KB/op、9502 allocs/op。
- 10 个 Provider 稳定刷新：232.154-262.290 µs/op，约 164.6 KB/op、2169 allocs/op、1 query/op。
- 100 个 Provider 稳定刷新：2.128-2.274 ms/op，约 1.67-1.72 MB/op、约 21.5k allocs/op、1 query/op。
- 100 个 Provider 当前语义指纹：2.127-4.040 ms/op；存储行候选：0.479-0.611 ms/op，中位数约改善 4.37 倍。
- 完整 5 轮数据、样本结构和适用边界见总计划 B04 执行记录。

## 验证结果

- 新增与既有 Provider 快照测试：通过。
- `go test -race ./services/ -run '^TestProviderSnapshot|^TestProviderServiceSnapshot' -count=1`：通过。
- `go test ./services/ -run '^$' -bench 'BenchmarkProvider(Snapshot|Clone)' -benchmem -count=5`：通过，耗时 110.498 秒。
- `go test ./services/... -count=1`：通过，耗时 21.603 秒。
- Go 测试仅出现既有 macOS 链接目标版本警告。
- 按项目限制未运行 lint、构建或开发服务。

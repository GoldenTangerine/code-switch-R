<!--
@name: Provider 存储行指纹
@Descripttion: 记录供应商快照改用稳定存储行指纹的行为保护与性能对比
@version: 1.0.0
@Author: sm
@Date: 2026-08-31 22:07:28
@LastEditTime: 2026-08-31 22:07:28
@FilePath: doc/changes/2026-08-31-2207-provider-store-fingerprint.md
-->

# Provider 存储行指纹

## 变更时间

2026-08-31 22:07:28 CST（Asia/Shanghai）

## 涉及范围

- `services/providerstore.go`
- `services/providerservice.go`
- `services/provider_snapshot_benchmark_test.go`
- `doc/code-performance-optimization-plan.md`

## 变更内容

- SQLite 平台的快照指纹改为哈希按 `sort_index, id` 排序的完整存储行，稳定轮询不再反序列化并重新序列化全部 Provider。
- 保存端从本次实际构造并写入的行直接返回确定性指纹，移除保存后的额外查询及其外部写入竞态窗口。
- 空列表使用与数据库完全一致的哨兵行计算指纹，保留未初始化 `nil` 与已初始化空列表的区别。
- raw payload 格式或冗余公共列变化时，按旧版 Provider JSON 语义确认业务内容未变，只同步新指纹并保留原快照运行时字段。
- custom 平台继续使用文件加载和 Provider JSON 语义指纹，不套用 SQLite 行算法。
- 未修改公共 API、Wails 签名、Provider JSON、SQLite schema、排序、1 秒轮询、外部变化二次确认、Claude 副作用、配置或依赖。

## 性能对比

同机同命令各 5 轮中位数：

| Provider 数 | 稳定刷新修改前 | 稳定刷新修改后 | 结果 |
|---:|---:|---:|---:|
| 1 | 30.325 µs | 12.345 µs | 约 -59.3% |
| 10 | 226.384 µs | 61.383 µs | 约 -72.9% |
| 100 | 2,077.316 µs | 446.699 µs | 约 -78.5% |

- 1 个 Provider：16,934-16,936 B/op、238 allocs/op 降至 6,721-6,723 B/op、38 allocs/op。
- 10 个 Provider：164,594-164,853 B/op、2,169 allocs/op 降至 63,886-63,942 B/op、178 allocs/op。
- 100 个 Provider：1,672,585-1,722,478 B/op、21,524-21,526 allocs/op 降至 646,024-653,173 B/op、1,623-1,624 allocs/op。

## 验证结果

- 保存返回指纹与实际 SQLite 行及 custom 文件语义一致。
- 公共列、payload 格式、损坏 payload、顺序、ID 回填、nil/空哨兵和 raw 非语义变化测试通过。
- 现有外部快照刷新测试及 B13 目标 `go test -race` 通过。
- `BenchmarkProviderSnapshotFingerprint` 和 `BenchmarkProviderSnapshotRefresh` 修改后各完成 5 轮。
- `go test ./services/... -count=1` 通过。
- `go test . -count=1` 通过。
- Go 测试仅有项目既有 macOS 链接目标版本警告。

<!--
@name: SQLite 连接池 busy timeout
@Descripttion: 记录默认 SQLite 连接池逐连接应用 busy_timeout 的实现、测试与性能对比
@version: 1.0.0
@Author: sm
@Date: 2026-08-31 18:42:19
@LastEditTime: 2026-08-31 18:42:19
@FilePath: doc/changes/2026-08-31-1842-sqlite-connection-busy-timeout.md
-->

# SQLite 连接池 busy timeout

## 变更时间

2026-08-31 18:42:19 CST（Asia/Shanghai）

## 涉及范围

- `services/database.go`
- `services/sqlite_runtime_write_contention_test.go`
- `services/testdb_main_test.go`
- `services/session_usage_test.go`
- `doc/code-performance-optimization-plan.md`

## 变更内容

- 新增 `defaultSQLiteDSN`，通过 modernc `_pragma=busy_timeout%3d30000` 让默认连接池的每个新连接应用既有 30 秒等待策略。
- `InitDatabase` 使用该 DSN；显式 busy timeout、WAL、cache、temp store、连接池大小和初始化顺序保持不变。
- 新增同时持有 5 个连接的属性测试，逐个断言 `PRAGMA busy_timeout=30000`。
- 压力测试默认配置组直接复用生产 DSN helper，旧 DSN 仅作为 legacy 对照。
- 全局测试库恢复、Provider 隔离库和 Session Usage 隔离库同步复用生产 DSN，避免测试重绑后参数漂移。
- 未修改 Wails API、业务事务、数据库 schema、数据格式、队列参数、依赖或用户配置。

## 性能对比

修改前 5 轮直写竞争组：

- 成功数：1-201/596，中位数 4。
- `SQLITE_BUSY`：395-595/596，中位数 592。
- 总耗时中位数：4.682 ms；该值主要来自快速失败。

修改后 5 轮生产默认配置组：

- 成功数：596/596。
- `SQLITE_BUSY/LOCKED`：0/596。
- 总耗时：80.138-109.045 ms，中位数 106.120 ms。
- 日志成功 P50/P95 中位数：42.234/50.176 ms。
- Provider 改名成功 P50/P95 中位数：23.268/42.970 ms。

## 验证结果

- 新测试先确认因生产 DSN helper 缺失而失败，完成最小实现后通过。
- 5 连接属性测试连续 20 次通过。
- SQLite 竞争测试 5 轮全部通过，默认配置每轮均为 596/596 成功、0 busy/locked。
- 两个 B14 目标测试的 `go test -race` 通过。
- `go test ./services/... -count=1` 通过。
- `go test ./resources/model-pricing/... -count=1` 通过。
- 仅有项目既有 macOS 链接目标版本警告。

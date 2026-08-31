<!--
@name: SQLite 运行时写竞争基线
@Descripttion: 记录 B07 生产直写审计、临时库竞争测量及逐连接 busy timeout 候选结果
@version: 1.0.0
@Author: sm
@Date: 2026-08-31 18:21:47
@LastEditTime: 2026-08-31 18:21:47
@FilePath: doc/changes/2026-08-31-1821-sqlite-runtime-contention-baseline.md
-->

# SQLite 运行时写竞争基线

## 变更时间

2026-08-31 18:21:47 CST（Asia/Shanghai）

## 涉及范围

- `services/sqlite_runtime_write_contention_test.go`
- `doc/code-performance-optimization-plan.md`
- SQLite 默认连接池、双写队列、Provider 改名、健康历史清理和显式维护直写审计

## 变更内容

- 将生产直写分为初始化/迁移、队列降级、运行时简单或维护写、运行时复合事务、显式维护/恢复和独立数据库六类。
- 新增只使用 `t.TempDir()` 的压力测试，对比双队列、双队列加直写、逐连接 `busy_timeout` 三个场景；不读取用户数据库。
- 固定验证 500 条日志批任务、80 条单次队列任务、12 个 Provider 改名事务和 4 次健康历史清理，并断言最终行数与跨表事务原子性。
- 5 轮当前直写竞争组出现 66-595 次 `SQLITE_BUSY`，中位数 593/596；逐连接 `_pragma=busy_timeout%3d30000` 候选 5 轮均为 596/596 成功、0 次 busy。
- 新增 OPT-015/B14，后续只处理默认连接池逐连接 timeout；本批未修改生产 Go、前端、配置、依赖、schema、接口或用户行为。

## 验证结果

- `go test ./services -run '^TestSQLiteRuntimeWriteContentionBaseline$' -count=1 -v`：通过。
- `go test ./services -run '^TestSQLiteRuntimeWriteContentionBaseline$' -count=5 -v`：通过，三个场景数据一致性均满足断言。
- `go test -race ./services -run '^TestSQLiteRuntimeWriteContentionBaseline$' -count=1`：通过。
- `go test ./services/... -count=1`：通过。
- 仅有项目既有 macOS 链接目标版本警告。

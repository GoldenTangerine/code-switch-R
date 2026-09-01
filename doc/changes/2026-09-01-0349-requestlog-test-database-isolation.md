<!--
@name: 根包模拟日志测试数据库隔离
@Descripttion: 将模拟日志测试从真实用户数据库迁移到测试专属临时 SQLite
@version: 1.0.0
@Author: sm
@Date: 2026-09-01 03:49:57
@LastEditTime: 2026-09-01 03:49:57
@FilePath: doc/changes/2026-09-01-0349-requestlog-test-database-isolation.md
-->

# 根包模拟日志测试数据库隔离

## 变更时间

2026-09-01 03:49:57 CST（Asia/Shanghai）

## 涉及范围

- `requestlog_mock_test.go`
- `doc/code-performance-optimization-plan.md`

## 变更内容

- 移除测试文件连接真实 `~/.code-switch/app.db` 的包级初始化。
- 在目标测试内使用 `t.TempDir()` 创建临时 SQLite，并创建覆盖种数函数实际字段的最小 `request_log` 表。
- 移除被 xdb 安全规则拒绝且对新建空表无效的无条件删除调用。
- `SeedMockRequestLogs` 的生成范围、随机分布、写入字段和现有断言均保持不变。

## 验证结果

- `TestSeedMockRequestLogs` 连续 3 次通过，根包完整测试通过。
- `go test ./services/... -count=1` 与模型定价资源测试通过。
- 前端 71 个测试文件、561 项测试及 `vue-tsc --noEmit` 通过。
- `git diff --check` 通过；Go 仅有既有 macOS 链接目标版本警告。

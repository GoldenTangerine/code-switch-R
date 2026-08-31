<!--
@name: 日志仪表盘聚合性能基线
@Descripttion: 测量日志仪表盘重复聚合、查询计划与前端调用次数并记录 B05 完成状态
@version: 1.0.0
@Author: sm
@Date: 2026-08-31 16:58:22
@LastEditTime: 2026-08-31 16:58:22
@FilePath: doc/changes/2026-08-31-1658-log-dashboard-baseline.md
-->

# 日志仪表盘聚合性能基线

- 变更时间：2026-08-31 17:09:29 CST（Asia/Shanghai）
- 涉及范围：LogService 聚合测试与基准、日志页面调用次数测试、代码与性能优化计划 B05 状态

## 变更内容

- 新增 `services/log_dashboard_benchmark_test.go`，使用 TestMain 临时 SQLite 生成 1k/10k/100k 日志，不读取用户 `app.db`。
- 增加 Proxy、Session、All、Provider 和模型筛选下四类聚合结果等价测试。
- 增加 4 组 `EXPLAIN QUERY PLAN` 断言，确认当前时间、平台和 Provider 范围索引使用情况。
- 新增 63 个子基准，记录四个聚合入口与完整刷新冷热缓存的调用数、SQL 数、完整对象数、耗时和分配。
- 在 `useLogsPageData.test.ts` 固定选中模型时首次 7 个日志 + 3 个配置调用，以及配置缓存命中后仅新增 7 个日志调用。
- 更新 `doc/code-performance-optimization-plan.md`：B05 标记完成，OPT-009 写入实测数据，确认 B06 待处理。
- 未修改生产 LogService、SQL、索引、Wails 接口、前端调用链、配置或依赖。

## 关键测量

- 100k Proxy 当前范围匹配 40,004 行；冷缓存完整刷新构建 170,031 个完整日志对象，执行 6 个日志调用和 9 条 SQL。
- 100k Proxy 冷缓存 5 轮为 3.124-3.201 s/op，约 2.617 GB/op、约 3681 万 allocs/op；暖缓存为 2.908-3.076 s/op，SQL 从 9 条降为 8 条，但对象构建量不变。
- 100k All + Provider + 模型冷缓存为 553.552-590.596 ms/op，7 个日志调用、10 条 SQL、27,792 个完整日志对象、约 426.46 MB/op。
- 查询计划均命中现有时间或复合范围索引；All 来源另有 `session_usage_dedup` 主键 correlated subquery，本批不新增索引。
- 完整 5 轮数据、指标定义和适用边界见总计划 B05 执行记录。

## 验证结果

- `go test ./services -run 'TestLogDashboardAggregation' -count=1 -v`：通过。
- `pnpm exec vitest run src/components/Logs/composables/useLogsPageData.test.ts`：15 项测试通过。
- `go test ./services -run '^$' -bench '^BenchmarkLogDashboardAggregation$' -benchtime=1x -count=5`：63 个子基准通过，耗时 99.315 秒。
- `go test ./services/... -count=1`：通过，耗时 21.789 秒。
- `pnpm test:unit`：69 个测试文件、537 项测试全部通过。
- `git diff --check`：通过。
- Go 测试仅出现既有 macOS 链接目标版本警告；按项目限制未运行 lint、构建或开发服务。

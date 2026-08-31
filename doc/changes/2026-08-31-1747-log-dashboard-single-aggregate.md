<!--
@name: 日志仪表盘单次聚合
@Descripttion: 复用一次主范围日志生成仪表盘统计，并保持旧接口与用户行为不变
@version: 1.0.0
@Author: sm
@Date: 2026-08-31 17:47:43
@LastEditTime: 2026-08-31 17:47:43
@FilePath: doc/changes/2026-08-31-1747-log-dashboard-single-aggregate.md
-->

# 日志仪表盘单次聚合

- 变更时间：2026-08-31 17:54:09 CST（Asia/Shanghai）
- 涉及范围：LogService 日志聚合、Wails 日志服务封装、日志页面数据编排、B05/B06 性能基准、优化计划状态

## 变更内容

- 提取 Stats、Summary、Provider、Model 的共享聚合函数，四个旧 Wails 方法继续保留并复用相同逻辑。
- 新增 `LogDashboardAggregateV1` 和 `LogService.DashboardAggregateRangeV1`，主范围日志只查询、构建和定价适配一次。
- Summary 历史比较范围、Provider 性能 SQL 与 20 秒缓存、三态结果、排序和活动序列语义保持不变。
- `frontend/src/services/logs.ts` 新增类型化动态 Wails 封装；日志仪表盘以单次聚合请求替代四个独立统计请求。
- 分页、Provider refs、Provider 配置缓存、选中模型时的无模型过滤选项和 Provider 延迟加载保持独立。
- 新增新旧逐字段等价、Provider 可选聚合、Wails 参数、过期响应和精确调用次数测试。
- 扩展 B05 基准为 81 个子场景，保留旧路径并增加优化后冷热对照。
- 未修改数据库 schema、索引、配置、旧接口签名、JSON 子结构、统计口径或 UI 交互结果。

## 性能对比

- 100k Proxy 冷缓存中位数：`3.435s → 1.248s`，约降低 63.7%。
- 100k Proxy 分配：`2,617.259MB/op → 783.718MB/op`，约降低 70.1%。
- 100k Proxy 对象数：`170,031 → 50,019 fullrows/op`；调用 `6 → 3`，SQL `9 → 6`。
- 100k Session + Provider 冷缓存中位数：`422.956ms → 196.721ms`，约降低 53.5%。
- 100k All + Provider + 模型冷缓存中位数：`563.822ms → 334.434ms`，约降低 40.7%。
- 1k/10k/100k、Proxy/Session/All 的完整 5 轮数据见总计划 B06 执行记录。

## 验证结果

- 后端新旧聚合逐字段等价测试：通过。
- 前端日志 service 与 composable：20 项目标测试通过。
- `go test -race ./services -run '^TestDashboardAggregateRangeV1' -count=1`：通过。
- `pnpm exec vue-tsc --noEmit`：通过。
- `go test ./services -run '^$' -bench '^BenchmarkLogDashboardAggregation$' -benchtime=1x -count=5`：81 个子场景通过，耗时 129.264 秒。
- `go test ./services/... -count=1`：通过，耗时 22.234 秒。
- `go test ./resources/model-pricing/... -count=1`：通过。
- `pnpm test:unit`：69 个测试文件、538 项测试全部通过。
- `git diff --check`：通过。
- Go 测试仅出现既有 macOS 链接目标版本警告；按项目限制未运行 lint、构建或开发服务。

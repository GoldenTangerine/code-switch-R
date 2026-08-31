<!--
@name: 通用代理调度行为基线
@Descripttion: 为通用 Codex 代理调度增加组合行为矩阵并记录 B01 完成状态
@version: 1.0.0
@Author: sm
@Date: 2026-08-31 15:17:39
@LastEditTime: 2026-08-31 15:17:39
@FilePath: doc/changes/2026-08-31-1517-relay-scheduling-baseline.md
-->

# 通用代理调度行为基线

- 变更时间：2026-08-31 15:17:39 CST（Asia/Shanghai）
- 涉及范围：Codex 通用代理调度测试、代码与性能优化计划 B01 状态

## 变更内容

- 新增 `services/providerrelay_scheduling_matrix_test.go`，不修改任何生产代码。
- 使用同一组 4 个 Provider 和本地 `httptest` 上游固定顺序跨 Level 降级行为。
- 固定同级轮询只在当前 Level 内旋转，失败后继续下一 Level 的行为。
- 固定黑名单模式跳过已拉黑 Provider，并优先于已开启的轮询设置。
- 固定强制优先 Provider 在普通 Level 前尝试的行为。
- 固定既有会话绑定 Provider 优先，并在失败降级成功后迁移绑定的行为。
- 断言成功响应来源、最后使用 Provider、固定黑名单失败计数和会话绑定结果。
- 更新 `doc/code-performance-optimization-plan.md`：B01 和 OPT-003 标记为已完成，已完成批次更新为 1，待处理批次更新为 9。
- 未执行 B02，未修改 HTTP 路由、调度逻辑、黑名单逻辑、数据库结构、配置或依赖。

## 验证结果

- 修改前相关代理测试基线：通过。
- `go test ./services/ -run '^TestProxyHandlerSchedulingBehaviorMatrix$' -count=1 -v`：通过，5 个子测试全部通过。
- `go test -race ./services/ -run '^TestProxyHandlerSchedulingBehaviorMatrix$' -count=1`：通过，未发现竞态。
- `go test ./services/...`：通过。
- Go 测试仅出现既有的 macOS 链接目标版本警告。
- `git diff --check`：通过。
- 按项目限制未运行 lint、构建或开发服务。

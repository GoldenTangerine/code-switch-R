<!--
@name: 通用代理尝试状态收敛
@Descripttion: 记录通用代理调度失败与收尾路径的最小结构重构和行为验证
@version: 1.0.0
@Author: sm
@Date: 2026-08-31 20:48:16
@LastEditTime: 2026-08-31 20:48:16
@FilePath: doc/changes/2026-08-31-2048-relay-attempt-state.md
-->

# 通用代理尝试状态收敛

## 变更时间

2026-08-31 20:48:16 CST（Asia/Shanghai）

## 涉及范围

- `services/providerrelay.go`
- `services/providerrelay_scheduling_matrix_test.go`
- `doc/code-performance-optimization-plan.md`

## 变更内容

- 新增具体 `providerAttemptState`，统一记录最后错误、Provider、耗时和尝试次数。
- 统一 Provider 成功后的失败计数清零与最后使用记录，统一失败计数写入错误处理。
- 统一响应已开始和客户端中断的停止出口；会话分支通过同步回调在原时点恢复或释放绑定。
- 统一所有 Provider 结束后的并发限制 429 优先和最后上游原始错误透传。
- 保留黑名单、会话亲和、轮询、强制优先、重试、通知、日志和响应 JSON 的原分支语义。
- B01 行为矩阵新增“固定拉黑 + 已绑定会话”场景，验证失败计数和绑定迁移。
- 未修改 Gemini、自定义 CLI、协议转换、模型映射、请求日志 SQL、HTTP Client、公共方法、路由、配置、数据格式或依赖。

## 前后对比

- `proxyHandler`：737 行降至 620 行，减少 117 行，约 15.9%。
- `providerrelay.go`：11,447 行降至 11,402 行，净减少 45 行。
- 调度矩阵：5 个场景增至 6 个场景。
- 本批为代码质量重构，不声明运行时性能提升。

## 验证结果

- `TestProxyHandlerSchedulingBehaviorMatrix` 单轮和连续 5 轮通过。
- 调度矩阵 `go test -race` 通过。
- 6 项关键错误透传测试通过。
- 4 项会话迁移与回退测试通过。
- `go test ./services/... -count=1` 通过。
- `go test . -count=1` 通过。
- `git diff --check` 通过。
- Go 测试仅有项目既有 macOS 链接目标版本警告。

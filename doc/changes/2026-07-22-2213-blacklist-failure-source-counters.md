/**
 * @name: 黑名单失败来源与双计数优化
 * @Descripttion: 记录真实请求和健康巡检独立计数、错误来源诊断与拉黑原因展示
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-07-22 22:13:56
 * @LastEditTime: 2026-07-22 22:13:56
 * @FilePath: doc/changes/2026-07-22-2213-blacklist-failure-source-counters.md
 */

# 黑名单失败来源与双计数优化

- 变更时间：2026-07-22 22:13:56 CST（Asia/Shanghai）
- 涉及范围：供应商黑名单、后台健康巡检、代理请求日志、供应商日志、通用设置与供应商卡片

## 变更内容

- 真实请求失败与后台健康巡检改为独立连续计数和独立阈值；真实请求阈值仅统计网络错误、断流、`429` 和 `5xx`。
- 手动健康检测不再影响拉黑计数；后台巡检默认连续失败 3 次触发拉黑，可在通用设置中配置为 2-9 次。
- 真实请求成功同时清零两套计数；关闭供应商健康自动拉黑时清零该供应商的巡检计数。
- 请求日志新增 `error_source`，区分供应商响应、上游网络、上游断流、代理内部和客户端取消。
- `502` 即使关闭完整请求体采集，仍保存最多 2KB 的脱敏错误摘要；供应商日志直接展示错误来源。
- 供应商卡片展示真实请求与后台巡检的当前计数、各自阈值、拉黑触发来源和脱敏原因。
- 黑名单表新增健康计数、触发来源和原因字段；一次性清理旧版本遗留的未拉黑计数，并标记仍生效的历史黑名单。

## Code Review 修正

- 修正时间：2026-07-23 09:37:07 CST（Asia/Shanghai）
- 活跃拉黑期间的后续成功请求不再清空触发来源、原因和触发计数。
- 黑名单状态变更增加服务级读写锁保护；正常成功请求可并发读取，状态写入保持串行，避免失败计数丢失或成功清零被旧计数覆盖。
- 健康检查成功仅在存在健康失败计数时写库，避免每分钟无意义写入和全量快照刷新。
- 双阈值和拉黑时长改为一次后端调用保存；后续步骤失败时回滚基础配置，前端失败后重新加载服务端真实值。
- 新增活跃拉黑详情保留、健康成功空写入、并发健康计数和双阈值保存回归测试。

## 验证结果

- `go test ./services/...`：通过。
- `go test -race ./services -run 'TestBlacklistSuccessWaitsForRunningMutation|TestHealthCheckFailureCounterIsSerialized|TestBlacklistActiveSuccessPreservesTriggerDetails' -count=1`：通过。
- `cd frontend && pnpm test:unit`：通过，54 个测试文件、401 个测试。
- `cd frontend && pnpm exec vue-tsc --noEmit`：通过。
- `git diff --check`：通过。

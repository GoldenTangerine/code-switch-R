/**
 * @name: 供应商额度后台恢复
 * @Descripttion: 为额度耗尽供应商增加低资源占用的后端自动恢复检查
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-07-31 15:44:02
 * @LastEditTime: 2026-07-31 15:44:02
 * @FilePath: doc/changes/2026-07-31-1544-provider-quota-background-recovery.md
 */

# 供应商额度后台恢复

- 变更时间：2026-07-31 15:44:02 CST（Asia/Shanghai）
- 涉及范围：额度自动停用服务、应用设置、系统通知、通用设置页与中英文文案

## 变更内容

- 应用进程存活时，后台自动检查额度停用和临时启用的供应商，额度恢复后重新启用并清理自动化状态。
- 后台使用单协调协程、单计时器和串行查询；没有待恢复供应商时不保留运行中的计时器。
- 新增后台恢复间隔设置，默认 60 秒，可配置范围为 10 - 3600 秒。
- 新增独立的额度恢复通知开关，默认关闭；同轮多个供应商恢复时合并为一条系统通知。
- 保留主页原有额度查询间隔，后台查询失败时保持供应商状态并按固定间隔重试。
- 补充真实调度集成测试，覆盖首次完整间隔、间隔热更新后重置计时、自动恢复以及等待状态快速停止。
- 发布版本推进到 `v2.9.17`，同步发布说明与全平台构建元数据。

## 验证结果

- `go test ./services/...`：通过。
- `cd frontend && pnpm test:unit`：通过，57 个测试文件、429 项测试。
- `cd frontend && pnpm exec vue-tsc --noEmit`：通过。
- `git diff --check`：通过。
- `go test ./services/ -run 'TestProviderQuotaRecoveryScheduler(WaitsBeforeFirstCheck|ResetsIntervalAndRestoresProvider|StopsPromptlyWhileWaiting)$' -count=1 -v`：通过。
- `go test -race ./services/ -run 'TestProviderQuotaRecoveryScheduler(WaitsBeforeFirstCheck|ResetsIntervalAndRestoresProvider|StopsPromptlyWhileWaiting)$' -count=1`：通过。

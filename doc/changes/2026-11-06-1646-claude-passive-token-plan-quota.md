<!--
@name: Claude 被动采样套餐额度
@Descripttion: 记录 Token Plan 供应商新增 Claude 被动采样及代理额度缓存实现。
@version: 1.0.0
@Author: sm
@Date: 2026-11-06 16:46:17
@LastEditTime: 2026-11-06 16:46:17
@FilePath: doc/changes/2026-11-06-1646-claude-passive-token-plan-quota.md
-->

# Claude 被动采样套餐额度

- 变更时间：2026-11-06 16:46:17（Asia/Shanghai，UTC+08:00）。
- 涉及范围：供应商额度查询配置弹窗、额度配置类型、首页额度刷新与展示、中英文文案、代理转发、额度查询服务、额度自动停用服务及相关测试。
- 参考实现：sub2api `backend/internal/service/ratelimit_service.go` 的 `samplePassiveUsageFromHeaders` 和 `backend/internal/service/account_usage_service.go` 的被动用量读取。

## 变更内容

- 在“额度查询配置 → Token Plan → Token Plan 供应商”新增“Claude 被动采样”，保存为 `tokenPlanProvider: claude_passive`，查询类型为 `token_plan_claude_passive`，复用供应商 API 地址与密钥。
- 代理转发 Claude 请求时从上游响应头采集 `5h`、`7d`、`7d_oi` 窗口的 `utilization` 和 `reset`；普通响应、流式响应和 429 响应均可采样，额度读取不额外发送 HTTP 请求。
- 将使用率转换为百分制额度，显示已用、剩余比例和重置倒计时；接受 0% 使用率、超出 100% 的用量，以及秒或毫秒重置时间；忽略负数、NaN、Infinity 和无效重置时间。
- 样本按 API 地址和密钥的 SHA-256 摘要隔离，保存到 `provider_claude_quota_samples` 表；建表和写入使用全局数据库写入队列，服务重建后加载最近样本。采样表不保存明文地址或密钥。
- 部分响应头只更新对应窗口；窗口重置后不把旧用量当成新窗口用量。首次采样前和样本过期时显示中英文等待提示，保留有效的其他窗口。
- 首页刷新时直接读取本地最新样本，跳过远端缓存等待与活跃供应商限制；沿用现有 30 秒刷新定时器。此模式隐藏远端查询超时与自动轮询配置。
- 被动模式仅展示用量，不参与额度耗尽自动停用，避免自动停用后无法通过真实请求产生新样本；弹窗说明此行为。

## 验证结果

- `go test ./services/ -run 'TestClaudePassiveQuota' -count=1` 通过，覆盖窗口解析、身份隔离、异常数据、过期窗口、数据库恢复、流式及限流转发、仅展示模式。
- `go test ./services/...` 通过。
- `pnpm test:unit` 通过，76 个测试文件、594 个测试。
- `pnpm exec vue-tsc --noEmit` 通过。
- `go test -race ./services/ -run 'TestClaudePassiveQuotaConcurrentReadsAndSamples' -count=1` 通过。
- `git diff --check` 通过。
- 未运行应用构建、打包或开发启动；未连接真实 Claude 账号验证。

## 使用条件

- 需启用对应供应商的本地代理托管，并让请求经过本地代理。
- 上游需要返回 Claude 套餐用量响应头；被中转站过滤或不提供这些数据时显示等待采样。
- 数据表示最后一次采样的套餐窗口比例，无法换算为精确剩余 Token 或金额。

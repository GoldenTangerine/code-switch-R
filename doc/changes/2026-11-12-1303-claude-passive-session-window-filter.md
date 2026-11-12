<!--
@name: Claude 被动采样按会话窗口过滤
@Descripttion: 记录参考 sub2api 在新的 5 小时窗口内隐藏未重新采样的 Claude 额度窗口。
@version: 1.0.0
@Author: sm
@Date: 2026-11-12 13:03:30
@LastEditTime: 2026-11-12 13:03:30
@FilePath: doc/changes/2026-11-12-1303-claude-passive-session-window-filter.md
-->

# Claude 被动采样按会话窗口过滤

- 变更时间：2026-11-12 13:03:30（Asia/Shanghai，UTC+08:00）。
- 涉及范围：Claude 被动采样读取逻辑及相关测试。
- 参考实现：sub2api `backend/internal/service/ratelimit_service.go` 中新 5 小时窗口初始化（`needInitWindow`）时清空 `passive_usage_*` 被动采样数据，再由 `samplePassiveUsageFromHeaders` 按本次响应头写回。

## 问题

- 7d_oi（Fable 专属周窗口）响应头只随 Fable 请求返回。本项目按窗口分别保存样本，最近未使用 Fable 时仍会展示数天前的 7d F 读数；sub2api 在新的 5 小时窗口开始后清空该数据，不再展示，两边表现不一致。

## 变更内容

- `queryClaudePassiveQuota` 以最近一次 5 小时样本所属窗口的开始时间（`five_hour.ResetAt - 5h`）为界，跳过最后采样时间早于该时间的其他窗口。
- 只在读取时过滤，不删除样本：同一 5 小时窗口内的请求不影响已有窗口；新窗口内重新采样到 7d_oi 后自动恢复展示；会话窗口到期但无新请求时沿用最近一次会话的判断，不恢复更早的旧样本。
- 5 小时窗口没有重置时间时不做过滤。Codex 被动采样保持原有逻辑，与 sub2api 一致。
- Codenotch 按托盘共享快照中的额度项渲染，无需修改代码。

## 验证结果

- `go test ./services/... .` 通过；新增 `TestClaudePassiveQuotaHidesWindowsNotResampledInCurrentSession`，覆盖同一会话窗口保留、新会话窗口隐藏、会话过期不恢复旧样本以及 Fable 请求后恢复展示。
- 未运行应用构建、打包或开发启动，未连接真实 Claude 账号联调。

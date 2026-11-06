<!--
@name: 智谱 GLM Token Plan 额度查询优化
@Descripttion: 记录智谱 GLM 额度查询的站点路由、跨站回退与额度窗口解析修复。
@version: 1.0.0
@Author: sm
@Date: 2026-11-06 11:00:00
@LastEditTime: 2026-11-06 11:00:00
@FilePath: doc/changes/2026-11-06-1100-glm-token-plan-quota-query.md
-->

# 智谱 GLM Token Plan 额度查询优化

- 变更时间：2026-11-06 11:00:00（Asia/Shanghai，UTC+08:00）。
- 涉及范围：`services/providerquotaqueryservice.go`、`services/providerquotaqueryservice_test.go`。
- 参考实现：sub2api `backend/internal/service/cn_provider_quota_service.go` 中的智谱额度探测与 `parseZhipuTokenTiers`。

## 问题原因

- `resolveProviderQuotaQueryTargetBaseURL` 对 GLM 固定返回 `https://api.z.ai`。国内站与国际站账号体系独立，Base URL 为 `open.bigmodel.cn` 的供应商更换为国内站 Key 后，请求国际站查不到额度。
- 解析时以 `total <= 0` 判断条目无效：只返回 `percentage` 的条目在用量为 0% 时被丢弃，新 Key 因此提示“未返回可展示的额度窗口”。
- 只接受 `TOKENS_LIMIT`，新版套餐仅返回 `CREDIT_LIMIT` 时没有可展示的窗口。
- `unit` 缺失且名称无法识别的条目被直接丢弃；只有 `usage` 没有 `currentValue` 的条目会显示为 0%。

## 变更内容

- 站点路由：按 hostname 识别官方站点，`bigmodel.cn` 优先查询 `open.bigmodel.cn`，`z.ai` 与中转等未知域名优先查询 `api.z.ai`；本地地址保持直连，不做回退。
- 跨站回退：新增 `queryGLMQuotaWithFallback`，首选站点出现任何失败（认证失败、`success=false`、无可展示窗口、网络错误）都会尝试另一站点；两次均失败时合并两个站点的错误信息。
- 解析拆分为 `parseGLMTokenPlanLimits` 与 `resolveGLMQuotaUsage`：
  - 存在 `TOKENS_LIMIT` 时只用它，否则降级使用 `CREDIT_LIMIT`，避免两种度量互相覆盖、影响额度耗尽自动停用的判断。
  - 只有 `usage` 与 `currentValue` 同时存在且 `usage > 0` 时才按数值计算，否则按 `percentage` 折算为百分制；0% 视为有效窗口。
  - 窗口判定顺序为 `unit`（3=5 小时，6=每周）、名称标签、重置时间兜底（无重置时间的优先归入 5 小时，其余按时间升序填补空缺窗口）。
  - 输出键名保持 `five_hour` / `weekly`，前端展示无需调整。
- `success=false` 且 `msg` 为空时返回“未知错误”，避免出现空白提示。
- 团队版 GLM Coding Plan 需要额外的组织 ID / 项目 ID 请求头，本次未支持。

## 验证结果

- 新增 0% 新 Key、仅 `CREDIT_LIMIT`、`TOKENS_LIMIT` 与 `CREDIT_LIMIT` 混合、只有 `usage`、`unit` 缺失兜底、中转域名回退、国内站优先等用例。
- `go test ./services/ -run 'Quota|Tray'` 通过，`go vet ./services/` 无告警。
- 本机 5 个智谱供应商的 Base URL 均为 `open.bigmodel.cn`，与站点路由问题吻合；尚未使用真实 Key 验证官方接口。

## 发布

- 随 Code Switch R v2.11.26 发布。

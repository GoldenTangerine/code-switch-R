<!--
@name: 被动采样会话窗口重置展示
@Descripttion: 记录参考 sub2api 调整被动采样 5 小时会话窗口过期后的展示逻辑。
@version: 1.0.0
@Author: sm
@Date: 2026-11-12 11:58:24
@LastEditTime: 2026-11-12 11:58:24
@FilePath: doc/changes/2026-11-12-1158-passive-session-window-reset.md
-->

# 被动采样会话窗口重置展示

- 变更时间：2026-11-12 11:58:24（Asia/Shanghai，UTC+08:00）。
- 涉及范围：被动采样推算逻辑、额度项推算标记、首页卡片、数据总览弹窗、托盘共享快照、中英文文案、前端绑定及相关测试。
- 参考实现：sub2api `backend/internal/service/account_usage_service.go` 的 `estimateSetupTokenUsage`、`buildCodexUsageProgressFromExtra`，以及前端 `UsageProgressBar.vue` 对已过期重置时间的处理。

## 问题

- v2.11.29 对所有窗口统一按周期顺延重置时间。Claude 5 小时窗口在下一次请求时才开始计时，顺延出的倒计时可能与实际不符，容易被误解为额度会在该时间重置。

## 变更内容

- `applyPassiveQuotaWindowEstimate` 按窗口周期区分处理：
  - 周期短于 24 小时的会话窗口（Claude 5h、Codex 300 分钟主窗口）越过重置时间后，用量归 0、清空 `nextReset`，标记 `estimate: reset`。
  - 固定周期窗口（7 天等）保持按周期顺延，标记 `estimate: projected`。
  - 未知周期的 Codex 窗口仍按过期处理；`aged` 标记规则不变。
- 首页卡片对带推算标记的额度项增加 `card-quota-item--estimated` 样式，进度条变淡并停止呼吸动画；`reset` 状态在倒计时位置显示“待开始”。
- 数据总览弹窗的 `reset` 状态同样显示“待开始”；托盘显示“窗口已重置，下次请求后开始计时”。
- `ProviderQuotaDisplayItem` 透传 `estimate` 字段；新增 `quotaPassiveWindowReset`、`quotaPassiveWindowIdle` 中英文文案。

## 验证结果

- `go test ./services/... ./resources/... .` 通过，覆盖 Claude / Codex 会话窗口重置归零、跨多个周期不臆造倒计时，以及 7 天窗口按周期顺延。
- `pnpm test:unit` 通过，76 个测试文件、602 个测试。
- `pnpm exec vue-tsc --noEmit` 通过。
- 未运行应用构建、打包或开发启动，未连接真实 Claude / Codex 账号联调。

## 与 sub2api 的差异

- sub2api 对 7 天窗口过期后保留旧用量并提示等待刷新；本项目按固定周期顺延并归零，同时标注为本地推算。
- sub2api 不标记过旧样本；本项目超过 5 分钟未采样时提示读数可能不准确。

/**
 * @name: 供应商拉黑详情悬浮交互优化
 * @Descripttion: 优化首页供应商拉黑详情弹窗的悬停关闭判断和长原因滚动展示。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-31 22:25:31
 * @LastEditTime: 2026-08-31 22:25:31
 * @FilePath: doc/changes/2026-08-31-2225-provider-blacklist-popover-hover.md
 */

# 供应商拉黑详情悬浮交互优化

- 变更名称：provider-blacklist-popover-hover
- 变更时间：2026-08-31 22:25:31（Asia/Shanghai，UTC+08:00）
- 涉及范围：首页供应商卡片、弹窗悬浮交互、前端回归测试

## 变更内容

- “已拉黑”入口悬停 100ms 后打开详情，同时离开入口和弹窗 150ms 后关闭。
- 鼠标进入弹窗会取消待执行的关闭，点击打开后继续保持固定显示。
- 保留点击外部、Esc、状态切换和操作完成后的关闭行为，并在组件卸载时清理定时器。
- 拉黑原因区域最大高度限制为 160px，超出后在原因区域内滚动。
- 现有额度错误弹窗保持原来的 100ms 打开和关闭延迟。

## 验证结果

- `cd frontend && pnpm exec vitest run src/components/Main/components/ProviderCard.test.ts src/components/Main/utils/providerQuotaErrorInteraction.test.ts`：通过，2 个测试文件、22 个测试。
- `cd frontend && pnpm test:unit`：通过，71 个测试文件、552 个测试。
- `cd frontend && pnpm exec vue-tsc --noEmit`：通过。
- `git diff --check -- <本任务文件>`：通过。

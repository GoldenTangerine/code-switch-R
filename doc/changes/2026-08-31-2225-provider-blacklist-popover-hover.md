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

- “已拉黑”状态改为完整红色标签命中区，悬停 100ms 后打开详情，离开标签和弹窗 150ms 后关闭。
- 详情弹窗通过 Teleport 显示，自动选择标签上方或下方并限制在视口内；标签与弹窗之间增加透明命中区域，避免移入时闪退。
- 点击标签后固定显示详情，保留再次点击、点击外部、Esc、状态切换和操作完成后的关闭行为，并在组件卸载时清理监听与定时器。
- 详情展示供应商、拉黑等级、剩余时间、触发来源、原因、拉黑时间、解禁时间、请求失败计数和巡检失败计数，缺失值统一显示为 `—`。
- 弹窗最大高度为 360px，正文独立滚动且支持文本选择复制，底部解除拉黑和清零等级按钮保持可见。
- 现有额度错误弹窗保持原来的 100ms 打开和关闭延迟。
- 弹窗高度按所选方向的实际可用空间收缩，视口空间不足时不覆盖拉黑标签。
- 拉黑详情与额度错误详情在悬停和点击打开时保持互斥，并修正弹窗主文字主题变量。

## 验证结果

- `cd frontend && pnpm exec vitest run src/components/Main/components/ProviderCard.test.ts src/components/Main/utils/providerQuotaErrorPopover.test.ts src/components/Main/utils/providerQuotaErrorInteraction.test.ts`：通过，3 个测试文件、29 个测试。
- `cd frontend && pnpm test:unit`：通过，71 个测试文件、564 个测试。
- `cd frontend && pnpm exec vue-tsc --noEmit`：通过。
- `git diff --check -- <本任务文件>`：通过。

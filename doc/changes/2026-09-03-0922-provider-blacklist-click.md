<!--
/**
 * @name: 供应商拉黑详情点击修复
 * @Descripttion: 修复首页供应商已拉黑标签点击无法显示详情的问题。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-09-03 09:22:12
 * @LastEditTime: 2026-09-03 09:22:12
 * @FilePath: doc/changes/2026-09-03-0922-provider-blacklist-click.md
 */
-->

# 供应商拉黑详情点击修复

- 变更名称：provider-blacklist-click
- 变更时间：2026-09-03 09:22:12（Asia/Shanghai，UTC+08:00）
- 涉及范围：首页供应商卡片、拉黑详情交互、前端回归测试

## 变更内容

- 修复指标区域单项 `v-for` 将弹窗模板 ref 收集为数组的问题，确保点击后可以正常定位并显示 Teleport 详情弹窗。
- 移除“已拉黑”详情的悬停打开、延迟定时器和固定状态组合，改为仅点击打开。
- 首次点击显示详情，再次点击、点击外部、按 `Esc`、状态变化或执行详情操作后关闭。
- 保留与额度错误弹窗的互斥行为，标签悬停时显示手形光标。
- 删除仅服务于悬停跨区域的透明桥接样式。
- 新增基于 `happy-dom` 的组件交互测试，真实触发点击、外部点击和 `Esc`，并验证 Teleport 弹窗可见性。
- 将额度错误弹窗交互对象改为直接 `const` 初始化，修正监听器依赖的声明顺序，并清理拉黑标签容器的无效定位样式。

## 验证结果

- `cd frontend && pnpm exec vitest run src/components/Main/components/ProviderCard.test.ts src/components/Main/utils/providerQuotaErrorPopover.test.ts src/components/Main/utils/providerQuotaErrorInteraction.test.ts --silent`：通过，3 个测试文件、30 项测试。
- `cd frontend && pnpm exec vitest run --silent`：通过，71 个测试文件、565 项测试。
- `cd frontend && pnpm exec vue-tsc --noEmit`：通过。
- `git diff --check`：通过。

## 发布

- 应用版本推进到 `v2.11.14`，同步全平台构建元数据与发布说明。
- 推送 `v2.11.14` Tag 后触发 GitHub Actions 全平台自动打包。

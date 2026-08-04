/**
 * @name: 托盘指标数字显示优化
 * @Descripttion: 缩短托盘花费格式并优化指标列宽，减少数字截断
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-04 14:56:55
 * @LastEditTime: 2026-08-04 14:56:55
 * @FilePath: doc/changes/2026-08-04-1456-tray-metric-number-display.md
 */

# 托盘指标数字显示优化

- 变更时间：2026-08-04 14:56:55 CST（Asia/Shanghai）
- 涉及范围：托盘供应商统计、金额格式、指标布局

## 变更内容

- 托盘花费由本地化的 `US$ 数字` 格式统一为紧凑的 `$数字` 格式。
- 保留金额的本地化千位分隔符和两位小数，不影响主窗口金额显示。
- 减少指标单元格横向内边距，并为 Tokens 和花费分配更多列宽。
- 花费字号调整为 `11px`，优先完整显示较长金额，同时保留溢出提示。
- 补充零金额、常规金额和五位数金额格式回归测试。
- 发布版本推进到 `v2.9.21`，同步更新说明、应用版本常量与全平台 Wails 构建元数据。

## 验证结果

- `cd frontend && pnpm exec vitest run src/components/Tray/trayProviderStats.test.ts`：通过，11 项测试。
- `cd frontend && pnpm test:unit`：通过，60 个测试文件、452 项测试。
- `cd frontend && pnpm exec vue-tsc --noEmit`：通过。
- `wails3 task common:update:build-assets`：通过，全平台构建元数据已同步为 `2.9.21`。
- `git diff --check`：通过。
- 按项目限制未启动开发服务、未执行构建和 lint。

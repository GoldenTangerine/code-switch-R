/**
 * @name: 可搜索输入下拉显式选择
 * @Descripttion: 记录全局可搜索模型输入框仅允许显式选择候选的交互修复
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-07-20 15:17:47
 * @LastEditTime: 2026-07-20 15:17:47
 * @FilePath: doc/changes/2026-07-20-1517-searchable-input-explicit-selection.md
 */

# 可搜索输入下拉改为显式选择

## 变更时间

2026-07-20 15:17:47（Asia/Shanghai，UTC+08:00）

## 涉及范围

- 全局可搜索模型输入框
- 模型映射与 CLI 模型输入交互
- 前端单元测试

## 变更内容

- 输入内容匹配候选项时，不再于 Tab、失焦或直接按 Enter 后自动写入首个匹配项。
- 仅在鼠标左键点击候选，或使用方向键导航后按 Enter 时写入候选项。
- 保留模型映射和 CLI 模型输入原有的直接 Enter 操作。
- 输入法合成 Enter 不触发切换焦点、提交映射或应用配置。
- 失焦和完成候选选择后完整清理导航状态，避免后续 Enter 误选或无响应。
- 补齐 Home、End、PageUp、PageDown 候选导航，同时保留 Shift 与 Home、End 的文本选择行为。

## 验证结果

- `pnpm test:unit`：通过（51 个测试文件，380 个测试）。
- `pnpm exec vue-tsc --noEmit`：通过。
- `git diff --check`：通过。

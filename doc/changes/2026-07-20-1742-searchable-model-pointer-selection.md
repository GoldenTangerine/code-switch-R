/**
 * @name: 模型候选鼠标选择修复
 * @Descripttion: 记录可搜索模型输入框的鼠标与键盘显式选择修复
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-07-20 17:42:10
 * @LastEditTime: 2026-07-20 17:42:10
 * @FilePath: doc/changes/2026-07-20-1742-searchable-model-pointer-selection.md
 */

# 模型候选鼠标选择修复

## 变更时间

2026-07-20 17:42:10 CST（Asia/Shanghai）

## 涉及范围

- 全局可搜索模型输入框
- 模型映射与 CLI 模型选择
- 前端单元测试

## 变更内容

- 将候选项的鼠标选择许可提前到捕获阶段，确保 Headless UI 写入候选前已记录明确选择意图。
- 移除鼠标处理中提前消费选择许可的异步清理，修复点击候选无法回填输入框。
- 仅鼠标左键点击或键盘明确导航后按 Enter 写入候选，保留直接 Enter、Tab 和失焦时的自定义输入。
- 补充非左键不选择以及 `k3` 模糊匹配 `kimi-k3` 的回归测试。

## 验证结果

- `pnpm test:unit`：通过（51 个测试文件，381 项测试）。
- `pnpm exec vue-tsc --noEmit`：通过。
- `git diff --check`：通过。

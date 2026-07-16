/**
 * @name: 连接详情悬浮窗主题修复
 * @Descripttion: 记录连接详情悬浮窗亮色与暗色配色修复。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-07-15 22:58:41
 * @LastEditTime: 2026-07-15 22:58:41
 * @FilePath: doc/changes/2026-07-15-2258-connection-tooltip-theme.md
 */

# 连接详情悬浮窗主题修复

- 变更时间：2026-07-15 22:58:41 CST（Asia/Shanghai）
- 涉及范围：首页连接详情、模型参数悬浮窗、亮色与暗色主题

## 变更内容

- 修复悬浮窗亮色与暗色配色写反，导致暗色主题仍显示白色背景的问题。
- 亮色主题使用浅色背景和深色文字，暗色主题使用深色背景和浅色文字。
- 保留原有边框、阴影、定位、键盘聚焦和 Teleport 交互逻辑。

## 验证结果

- `pnpm --dir frontend exec vue-tsc --noEmit`：通过。
- `pnpm --dir frontend test:unit`：46 个测试文件、331 个测试通过。
- `git diff --check`：通过。

/**
 * @name: 移除 ace-tool 关联
 * @Descripttion: 记录移除 ace-tool MCP 注册及项目残留配置。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-07-15 17:32:30
 * @LastEditTime: 2026-07-15 17:32:30
 * @FilePath: doc/changes/2026-07-15-1732-remove-ace-tool.md
 */

# 移除 ace-tool 关联

- 变更时间：2026-07-15 17:32:30 CST（Asia/Shanghai）
- 涉及范围：Codex MCP 注册、Claude 项目权限、项目忽略配置、ace-tool 本地索引

## 变更内容

- 删除 Codex 全局 `ace-tool` MCP 注册。
- 删除 `.claude/settings.local.json` 中的 `mcp__ace-tool__search_context` 权限。
- 删除 `.gitignore` 中的 `.ace-tool/` 说明。
- 删除 `.ace-tool/index.json` 本地索引缓存。

## 验证结果

- `codex mcp list`：不再包含 `ace-tool`。
- 全仓搜索：不再存在 ace-tool MCP 配置或项目说明；本变更记录除外。
- JSON 校验：`.claude/settings.local.json` 格式有效。

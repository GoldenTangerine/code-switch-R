/**
 * @name: Claude Header 会话粘滞修复
 * @Descripttion: 记录 Claude Code 主 Agent、子 Agent 与嵌套子 Agent 的供应商继承修复
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-20 17:35:00
 * @LastEditTime: 2026-08-20 17:35:00
 * @FilePath: doc/changes/2026-08-20-1735-claude-header-session-affinity.md
 */

# Claude Header 会话粘滞修复

- 变更时间：2026-08-20 17:35:00 CST（Asia/Shanghai）
- 涉及范围：Claude 代理请求会话识别、Subagent 供应商偏好、Sticky Session 继承、后端回归测试

## 变更内容

- 优先读取 Claude Code 官方 Header：`x-claude-code-session-id`、`x-claude-code-agent-id`、`x-claude-code-parent-agent-id`。
- 使用 `session-id` 作为整棵 Agent 树的根，会话内的主 Agent、一级子 Agent 和嵌套子 Agent 使用稳定的父子哈希关系。
- `code-switch-r-subagent` 子进程按根会话读取主 Agent 最近成功使用的供应商，即使子进程请求体中的 `metadata` 会话标识发生变化，也不会重新选择供应商。
- 兼容主请求使用旧 `metadata.user_id`、子请求使用官方 Header 的混合客户端场景；按哈希后的 `session-id` 维护根会话别名，并限制容量与空闲 TTL。
- Sticky Session 子 Agent 优先继承直接父节点；父节点尚未建立绑定时回退继承根会话绑定。
- 未提供 Claude 官方 Header 时保留现有请求体识别逻辑，兼容旧版 Claude Code 客户端。
- Header 只参与哈希计算，不记录原始会话 ID、Agent ID 或父 Agent ID。

## 验证结果

- `go test ./services -run 'TestClaude(Header|Subagent)|TestRelaySessionIdentity|TestSessionRelation|TestClaudeHeader' -count=1`：通过。
- `git diff --check`：通过。
- 完整 `go test ./services/...`：通过；存在已有 macOS target-version linker warning，不影响测试结果。
- 混合身份路由与 Sticky 继承回归测试：通过。

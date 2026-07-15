# Claude Responses 子代理续链隔离

- 变更时间：2026-07-15 11:38:25 CST（Asia/Shanghai）
- 涉及范围：Claude 托管路由、OpenAI Responses API、`previous_response_id` 续链、后端回归测试

## 变更内容

- Responses 续链键在 Claude metadata session 基础上追加代理上下文指纹。
- 上下文指纹使用有效模型、规范化 system、工具定义和首条 user 内容生成，隔离主代理与不同子代理。
- 同一代理后续轮次保持续链键稳定，不同 Explore 子任务不再互相覆盖 response ID。
- 缺少稳定代理上下文时不启用续链，避免仅凭 session 复用错误上下文。
- 优先使用 `agent_id`、`subagent_id`、`task_id`、`invocation_id` 或 `parent_tool_use_id` 作为稳定代理身份；system 与 tools 动态变化不会切断同一代理续链。
- 子代理缺少唯一身份字段时保守停用续链并回放完整上下文，避免相同提示的并行任务共享 `previous_response_id`。
- 保持现有 Responses 文本和流式事件转换行为，不过滤 `<agent-message>` 等合法文本。

## 验证结果

- Responses 主代理、多个 Explore 上下文隔离测试：通过。
- 显式代理 ID 稳定性、相同提示隔离、无唯一 ID 子代理保守回退测试：通过。
- Responses 续链、工具调用、失效回退和输入裁剪回归测试：通过。
- `go test ./services/... -count=1`：通过。
- 新增续链隔离测试执行 `go test -race`：通过。

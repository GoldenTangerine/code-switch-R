# Codex 压缩流断流诊断

- 变更时间：2026-07-15 15:08:09 CST（Asia/Shanghai）
- 涉及范围：Codex `/responses` 转发、请求日志存储、日志页 HTTP 状态诊断

## 变更内容

- 确认 `stream closed before response.completed` 表示上游 Responses 流在协议完成事件前结束，压缩请求是触发场景，不是本地压缩过程直接导致。
- 记录最后事件、终止事件、错误分类、压缩请求与输出状态、已转发字节数及上游 HTTP 协议，不保存压缩内容、请求标识或密文。
- 上游返回 `200` 但未产生任何流字节时，在提交响应头前返回可重试错误，允许现有供应商故障转移。
- 已向客户端输出流内容后保持禁止重试，避免拼接两个供应商的响应；不伪造 `response.completed`。
- 日志页 HTTP 状态支持悬停和键盘聚焦查看流诊断，历史日志无诊断字段时保持原展示。

## 验证结果

- `go test ./services/... -count=1`：通过。
- 聚焦 `go test -race`：通过。
- `pnpm --dir frontend test:unit`：45 个测试文件、321 个测试通过。
- `pnpm --dir frontend exec vue-tsc --noEmit`：通过。
- `git diff --check`：通过。
- 按项目约束未运行 lint、build 或 dev。

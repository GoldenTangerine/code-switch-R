# Claude 代理认证与转发优化

## 变更时间

2026-07-17 11:22:19 CST（Asia/Shanghai）

## 涉及范围

- Claude 代理接管配置与恢复状态
- Claude 上游认证头转发
- 通用设置、CLI 配置预览与 WSL 自动配置

## 变更内容

- 新增 Claude 代理认证字段设置，支持 `ANTHROPIC_AUTH_TOKEN` 与 `ANTHROPIC_API_KEY`，默认保持 `ANTHROPIC_AUTH_TOKEN`。
- 代理接管时仅写入一个占位认证字段；运行中切换设置后立即原子重写，关闭代理时恢复启用前的两个认证字段。
- 代理状态升级为 v2，并兼容迁移 v1 状态中的 API Key 基线。
- Claude 请求转发前清理客户端认证头，再按供应商认证类型注入真实凭证；Codex 与 Gemini 转发行为不变。
- CLI 配置预览、高级编辑与 WSL 自动配置使用同一 Claude 代理认证字段设置。
- 启动协调仅重写已有有效接管状态，状态文件缺失时保留无状态关闭兜底，避免把代理占位值误记为原始配置。
- Claude 代理启用、关闭、认证字段重写与直连应用统一串行化，避免并发操作覆盖最终状态。
- 认证字段单选限制仅作用于代理模式；直连预览、保存和高级编辑继续保留原有 `ANTHROPIC_API_KEY` 行为。

## 验证结果

- `go test ./services/...`：通过。
- `go test ./resources/model-pricing/...`：通过。
- `cd frontend && pnpm test:unit -- appSettings.test.ts`：49 个测试文件、345 个测试通过。
- `cd frontend && pnpm exec vue-tsc --noEmit`：通过。
- `go test -race ./services -run 'TestClaudeSettingsService|TestCliConfigServiceClaude' -count=1`：通过。
- `go test ./...`：根包、资源包和 `services` 通过；既有 `cmd/updater` Windows 构建约束与 `scripts/` 多入口结构导致全仓命令无法通过。
- 未运行 lint、构建或启动命令，符合项目限制。

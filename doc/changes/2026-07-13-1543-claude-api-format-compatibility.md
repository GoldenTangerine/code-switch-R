# Claude API 格式转换兼容性补全

- 变更名称：Claude API 格式转换兼容性补全
- 变更时间：2026-07-13 15:43:17 CST
- 涉及范围：Claude 供应商的 OpenAI Chat Completions / OpenAI Responses 格式转换、SSE 转换、供应商转发重试、剪贴板降级、项目开发规则

## 变更内容

- 统一复用 Anthropic 图片、工具定义、工具结果和 usage 规范化逻辑。
- 跳过空白 Base64 图片，避免生成无效 Data URI。
- Chat 响应兼容 `reasoning`、`reasoning_content`，并按 `thinking -> text/refusal -> tool_use` 输出。
- Responses SSE 兼容 `response.reasoning_text.delta/done`。
- Anthropic `web_search_*` 工具在 Responses 请求中映射为原生 `web_search`。
- 强制选择 `web_search` 时使用原生 Responses 工具选择；Chat 路径移除无法满足的无效强制选择。
- Responses 非流式和流式 `web_search_call` 转换为配对的 `server_tool_use` 与 `web_search_tool_result`，并防止重复事件。
- Responses 非流式 Web Search 结果保留并去重 `url_citation` 引用。
- usage 增加缓存写入字段别名和显式零值优先级兼容，同时规范 Anthropic `input_tokens` 口径。
- 对上游明确拒绝的安全白名单可选参数执行最多一次兼容重试。
- `prompt_cache_key`、`previous_response_id` 与可选参数共用一次兼容重试预算。
- 按供应商身份、规范化 API URL、最终端点和 API 格式记忆不支持字段；不包含 API Key。
- 不支持字段解析在支持参数列表前停止，避免误删合法字段。
- 不支持字段记忆增加 30 分钟有效期和容量上限，上游能力变化后可恢复探测。
- API URL 能力键统一 scheme、host、默认端口和尾部斜杠。
- 浏览器环境跳过 Wails Clipboard 调用，避免开发服务器响应造成复制假成功。
- 保持现有 UI、Provider 数据结构、数据库、Wails 接口、路由、流式模式和供应商降级体系不变。
- 在项目级 `AGENTS.md` 增加每次任务必须编写变更 Markdown 的规则。

## 验证结果

- `git diff --check` 通过。
- 沙箱内执行 `GOCACHE=/tmp/code-switch-go-build GOFLAGS=-mod=readonly go test ./services/...` 通过。
- 执行 `pnpm test:unit` 通过，共 43 个测试文件、303 个测试。
- 执行 `pnpm exec vue-tsc --noEmit` 通过。
- 定向执行 Claude 兼容逻辑 `go test -race` 通过。
- 未运行 lint、Wails 构建、应用启动、发布或 Git 操作。

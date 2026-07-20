# Claude 供应商认证字段

- 变更时间：2026-07-20 17:22:06 CST（Asia/Shanghai）
- 涉及范围：Claude 供应商编辑、CLI 配置预览、直连应用、托管转发、可用性探测、模型价格获取、供应商导入

## 变更内容

- 在高级功能的 API 格式下新增 Claude 认证字段，默认使用 `ANTHROPIC_AUTH_TOKEN`，支持 `ANTHROPIC_API_KEY` 和自定义 Header。
- 复用 `connectivityAuthType` 持久化认证选择，两个标准认证字段严格互斥。
- 自定义 Header 仅允许托管路由，并在保存时校验 Header 名称。
- CLI 预览按直连或托管模式展示对应配置；全局设置名称调整为“本地代理认证字段”。
- cc-switch 配置与深链导入兼容两个标准认证字段，同时存在时优先 `ANTHROPIC_AUTH_TOKEN`。
- 统一供应商认证 Header 解析，原生 Claude 请求独立补充 `anthropic-version`，避免认证方式影响协议头。
- Claude 两个标准认证字段始终保持只读，不进入 CLI 可编辑配置或供应商 `cliConfig`。
- “保存并应用”状态实时使用当前认证选择，自定义 Header 禁止直连，切回标准字段后可恢复直连。

## 验证结果

- `go test ./services/...`：通过。
- `cd frontend && pnpm test:unit`：通过，51 个测试文件、382 个测试。
- `cd frontend && pnpm exec vue-tsc --noEmit`：通过。

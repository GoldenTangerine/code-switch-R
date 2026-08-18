# Sticky Session Affinity

- 变更时间：2026-08-18 10:48:33 CST
- 涉及范围：供应商代理路由、并发详情、首页平台开关、供应商编辑表单、应用设置

## 变更内容

- 为 Claude、Codex、Gemini 和 Custom CLI 增加按平台控制的会话粘滞开关，默认关闭。
- Sticky 开启后，可靠识别的根会话及子会话优先复用当前供应商；新会话在最高优先级 Level 内按会话绑定数与活跃请求数的容量比选择供应商。
- 增加根会话/子会话关系、TTL、容量上限、内存绑定淘汰和失败后的自动迁移逻辑。
- 迁移期间按实际供应商分别统计活跃请求，避免旧请求被错误计入新供应商负载。
- 连接详情展示会话编号与层级；点击活跃连接打开同平台供应商弹窗，可手动切换，切换从下一次请求生效。
- 供应商编辑表单增加会话容量和空闲释放时间配置。
- Sticky 关闭时清理对应平台的内存绑定并恢复原有路由行为。

## 后续修正（2026-08-18 15:03:59 CST）

- 增加 Codex `x-codex-turn-metadata`、`x-codex-parent-thread-id` 与 `client_metadata` 显式会话字段解析，修复 Codex TUI 活跃连接缺少会话编号、无法打开供应商切换弹窗的问题。
- 活跃连接行增加可见的“切换”按钮；未开启 Sticky 或未识别稳定会话时展示对应状态，不再依赖隐藏的整行点击入口。
- 修复候选供应商卡片被长名称或不可用原因撑宽的问题；文字在卡片内完整换行，窄窗口自动使用单列布局。
- 排除全局按钮强制布局对会话切换卡片和按钮的覆盖，补充悬停、键盘焦点和深浅色不可用状态，并阻止过期候选请求覆盖当前会话。

## 验证结果

- `gofmt`：通过。
- `go test -count=1 ./services/...`：通过。
- `cd frontend && pnpm test:unit`：通过（64 个测试文件，493 个测试）。
- `cd frontend && pnpm exec vue-tsc --noEmit`：通过。
- 中英文 locale JSON 解析与 `git diff --check`：通过。

### 后续修正验证

- `go test ./services/...`：通过。
- `cd frontend && pnpm test:unit`：通过（67 个测试文件，506 个测试）。
- 中英文 locale JSON 解析与 `git diff --check`：通过。
- `cd frontend && pnpm exec vue-tsc --noEmit`：通过。

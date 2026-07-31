# README 按最新功能重写并新增英文版

## 变更时间

2026-07-31 18:52:47 (CST+0800)

## 涉及范围

- `README.md`（整体重写，仅「界面预览」小节原样保留）
- `README_EN.md`（新增，与中文版结构完全对应的英文版）

## 变更内容

### README.md 重写

1. **定位语简化**：顶部标语不再列举具体 CLI，改为在正文新增「支持哪些 CLI？」小节集中说明（Claude Code / Codex / Gemini CLI / OpenCode / 自定义 CLI，含各自接入方式）。
2. **功能介绍全面扩写**：按最新功能重组为 15 个小节——供应商管理、智能代理与自动降级、Claude 模型路由与映射、可靠性与黑名单、配额保护与预算、用量统计与成本核算、可用性监控、认证中心（Codex OAuth）、MCP 服务器管理、技能市场、提示词管理、API 测速、环境变量检测、控制台、WebDAV 云同步。其中配额自动化（耗尽自动停用 + 后台恢复 + 通知）、五档预算、双数据源用量统计、模型路由体系（映射开关/思考强度/1M 声明/Subagent/兜底）、双计数双阈值黑名单等均为本次新增描述。
3. **移除深度链接描述**：`codeswitch://` 深链导入后端服务已注册但前端确认弹窗未挂载，端到端不可用，按确认从 README 移除。
4. **工作原理更新**：补充五组代理端点路由表（`/v1/messages`、`/responses`、`/gemini/*`、`/custom/:toolId/*`、`/v1/models`），注明仅监听回环地址；说明 OpenCode 为配置直写、不经代理。
5. **常见问题修正**：
   - 「关闭应用后 CLI 还能用吗」：修正为关窗驻留托盘代理不停、仅托盘「退出」才停止（与 main.go 托盘菜单行为一致）。
   - 「如何备份配置」：文件清单更新为实际目录内容（`app.db`、`app.json`、`claude-code.json`、`providers/`、`mcp.json`、`skill.json`、`webdav.json`），并推荐 WebDAV 云同步。
6. **下载与安装**：发布产物文件名更新为带版本号的实际命名（`CodeSwitch-vX.X.X-amd64-installer.exe`、`CodeSwitch-vX.X.X-macos-arm64.zip`、`CodeSwitch-vX.X.X.AppImage` 等，与 `.github/workflows/release.yml` 一致）。
7. 快速开始步骤与当前首页交互对齐（平台标签页 + 新建按钮 + 每平台独立代理开关）。

### README_EN.md 新增

- 与中文版逐节对应的完整英文翻译；截图区块复用相同图片路径，alt 文本译为英文；文件名、路径、配置键名保持原文。

## 验证结果

- 功能描述逐项与源码核实：代理路由（`services/providerrelay.go` 路由注册）、OpenCode 配置直写（`services/opencodeservice.go`）、黑名单默认配置（`services/settingsservice.go` `DefaultBlacklistLevelConfig`）、配置文件目录（`~/.code-switch/` 实际内容）、发布产物名（`.github/workflows/release.yml`）、托盘退出行为（`main.go` 托盘菜单）。
- 「界面预览」小节逐字节保留，图片路径未改动。
- 文档间链接（README.md ↔ README_EN.md）相互指向，Releases / Issues 链接沿用原地址。

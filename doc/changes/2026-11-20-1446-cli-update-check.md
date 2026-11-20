<!--
@name: 新增应用更新查询
@Descripttion: 记录 CLI 更新页面、发布源与验证结果。
@version: 1.0.0
@Author: sm
@Date: 2026-11-20 14:46:48
@LastEditTime: 2026-11-20 14:46:48
@FilePath: doc/changes/2026-11-20-1446-cli-update-check.md
-->

# 新增应用更新查询

变更时间为 2026-11-20 14:46:48 CST（Asia/Shanghai）。

涉及系统控制侧栏、应用更新页面、中英文文案和 Wails 版本查询服务。

## 变更内容

- 进入「应用更新」时自动重新检查，也可以点击「重新检查」。页面显示本机版本、最新版本和更新状态。
- 支持 Claude Code、Codex、Antigravity CLI、Gemini CLI、OpenCode、Grok Build、OpenClaw、Hermes 和 Pi。通过本机命令读取版本，通过 npm、Antigravity 发布清单和 GitHub Releases 查询最新版本。
- 并行检查各工具，单个工具探测失败时仍会显示其他工具的结果。整个操作仅查询版本信息。

## 评审后调整

调整时间为 2026-11-20 15:24:50 CST（Asia/Shanghai）。

- 补充 pnpm 全局安装路径和 Grok 原生安装路径。已在系统 `PATH` 中找到的启动器沿用原有搜索顺序，通过备用目录找到的启动器可优先使用同目录的运行时。
- 只接受独立版本号或带工具名称的版本行，防止 Node 等依赖的诊断输出被认作 CLI 版本。
- Claude Code 采用用户级 `autoUpdatesChannel` 的 `stable` 设置。Codex、Gemini CLI 和 OpenCode 根据本机预发布版本识别发布标签，并比较对应发布版本。
- 本机探测和上游查询同时进行。重新检查期间保留上次结果，整次调用失败时显示旧结果和重试提示。

## 发布

发布准备时间为 2026-11-20 15:27:04 CST（Asia/Shanghai）。

版本号更新至 `v2.12.2`，同步平台构建元数据和发布说明。GitHub Actions 由同名 Git 标签触发。

## 验证结果

- Go 服务测试通过，覆盖版本比较、预发布版本、本机命令缺失和发布源异常。真实发布源已确认返回版本字段。
- 前端 606 项单元测试和 TypeScript 类型检查通过，覆盖首次进入、再次进入、手动刷新和过期响应。
- Wails 入口通过 Go 测试编译，Windows 版本探测测试通过交叉编译。桌面窗口和 Windows 命令执行未实机验证。
- 评审修复后的后端回归覆盖安装路径、`PATH` 顺序、诊断输出、稳定渠道和预发布标签。前端回归覆盖刷新和整次查询失败时保留上次结果。

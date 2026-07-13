# AGENTS.md

This file provides guidance to Codex (Codex.ai/code) when working with code in this repository.

## 项目概述

**Code Switch R** 是一个基于 Wails 3 的桌面应用，用于集中管理 Codex、Codex 和 Gemini CLI 的 AI 供应商配置。它在本地启动代理服务器（`:18100`），实现供应商的平滑切换、智能降级、用量统计和黑名单管理等功能。

## 技术栈

- **后端**: Go 1.24 + Gin + SQLite (via `modernc.org/sqlite` + `xgo/xdb` ORM)
- **前端**: Vue 3 + TypeScript + Tailwind CSS 4.x
- **框架**: Wails 3 (锁定 `v3.0.0-alpha.38`)
- **构建工具**: Vite 7
- **包管理**: pnpm (前端), Go modules (后端)
- **任务管理**: Task (基于 Taskfile.yml)

## 开发环境要求

- Go 1.24+
- Node.js `^20.19.0 || >=22.12.0`
- pnpm
- Wails 3 CLI: `go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-alpha.38`

**Linux 额外依赖**：
```bash
# Ubuntu/Debian
sudo apt-get install build-essential pkg-config libgtk-3-dev libwebkit2gtk-4.1-dev

# Fedora
sudo dnf install gtk3-devel webkit2gtk4.1-devel
```

## 常用命令

```bash
# 开发运行（前后端热重载）
wails3 task dev

# Go 测试
go test ./services/...                      # 运行所有后端测试
go test ./services/ -run TestXxx            # 运行单个测试
go test ./services/ -run TestXxx -v         # 带详细输出
go test ./resources/model-pricing/...       # 模型定价资源测试

# 前端测试
cd frontend && pnpm test:unit               # 运行所有前端单元测试

# 前端独立开发
cd frontend && pnpm install && pnpm dev     # 开发服务器
cd frontend && pnpm build                   # 生产构建
cd frontend && pnpm build:dev               # 开发模式构建（无压缩）

# 构建打包
wails3 task common:update:build-assets      # 首次构建或版本更新后必须执行
wails3 task package                         # 打包当前平台
wails3 task build                           # 仅构建当前平台二进制

# macOS 交叉编译 Windows
env ARCH=amd64 wails3 task windows:build
env ARCH=amd64 wails3 task windows:package
```

## 项目架构

### 核心工作原理

1. **代理服务器**：应用启动时在 `:18100` 创建 HTTP 代理（`ProviderRelayService`）
2. **请求路由**：
   - `/v1/messages` → Codex 供应商
   - `/responses` → Codex 供应商
   - Gemini 请求通过 `GeminiService` 单独处理
3. **优先级调度**：基于 Level 1-10 分组，同级内按顺序尝试（支持轮询模式）
4. **智能降级**：失败后自动切换到下一优先级的供应商
5. **黑名单机制**：连续失败达到阈值（默认 3 次）自动拉黑 30 分钟，每分钟自动恢复检查

### 服务初始化顺序（关键）

`main.go` 中的初始化有严格时序依赖，不能随意调整：

1. `InitDatabase()` — 必须最先执行，初始化 SQLite 连接池和 PRAGMA 设置
2. `InitGlobalDBQueue()` — 依赖数据库连接，初始化双队列写入系统
3. 各业务 Service 构造 — 可安全使用数据库
4. `providerRelay.Start()` — 在 goroutine 中启动代理服务器
5. 后台定时器（黑名单恢复、健康检查、更新检查）

### 双队列数据库写入架构

为消除 SQLite `SQLITE_BUSY` 错误，使用了双队列架构（`services/dbqueue.go`）：

- **GlobalDBQueue**（单次写入队列）：用于异构写入（blacklist、app_settings 等不同表）
- **GlobalDBQueueLogs**（批量写入队列）：仅用于高频 `request_log` INSERT（50 条/批，100ms 超时提交）

所有数据库写入必须通过队列，不要直接执行 SQL INSERT/UPDATE。

### 前后端通信

- 前端通过 Wails 3 的 `@wailsio/runtime` 调用 Go 服务
- 所有 Go 服务在 `main.go` 中通过 `application.NewService()` 注册
- 前端封装层位于 `frontend/src/services/*.ts`，与后端服务一一对应
- 路由使用 Hash 模式（`createWebHashHistory`），页面组件位于 `frontend/src/components/*/Index.vue`

### 版本管理

- 应用版本号定义在 `version_service.go` 的 `AppVersion` 常量
- 修改版本号后必须执行 `wails3 task common:update:build-assets` 同步构建元数据
- 发布通过 Git Tag 触发 GitHub Actions 自动构建

### 嵌入资源

- `frontend/dist` → 通过 `//go:embed` 嵌入前端静态文件
- `assets/icon.png`, `assets/icon-dark.png` → 系统托盘图标
- `resources/model-pricing/` → 模型定价数据（Go 包，有独立测试）

## 开发规范

### Go 代码规范

- **服务层**：所有业务逻辑封装在 `services/` 目录，每个 Service 是一个 struct
- **导出方法**：需要被前端调用的方法必须首字母大写
- **数据库**：SQLite，文件位于 `~/.code-switch/app.db`，使用 `xgo/xdb` ORM
- **数据库迁移**：修改数据库结构时，在对应服务的 `initDB` 方法中添加迁移逻辑
- **配置文件**：供应商配置等 JSON 文件位于 `~/.code-switch/` 目录

### Vue 3 代码规范

- **组合式 API**：统一使用 `<script setup lang="ts">`
- **组件命名**：PascalCase（如 `Index.vue`, `ListRow.vue`），子组件放在 `components/` 子目录、模态框放在 `modals/` 子目录
- **国际化**：使用 `vue-i18n`，新增文案必须同时更新 `locales/zh.json` 和 `locales/en.json`
- **主题支持**：所有组件必须同时支持亮色和暗色主题，使用 `resolvedTheme` 变量判断
- **定时器清理**：添加定时器时必须在 `onUnmounted` 中清理
- **Wails API**：必须从 `@wailsio/runtime` 导入，不要直接访问 `window.go`

### 变更文档规范

- 每次修改代码、配置或文档时，必须在 `doc/changes/` 新增一份独立的变更 Markdown，不得覆盖已有变更文档。
- 文件名统一使用 `YYYY-MM-DD-HHmm-变更名称.md`，变更名称使用简短的小写英文和连字符。
- 文档必须包含：变更名称、变更时间（`YYYY-MM-DD HH:mm:ss`，注明时区）、涉及范围、变更内容、验证结果。
- 一次任务对应一份变更文档；同一任务的后续修正应更新该任务文档，不重复创建。
- 变更文档必须在任务完成前写入，并与实际代码行为保持一致。

## 重要注意事项

### 开发限制

1. **不要运行 lint**：项目未配置 lint，不要尝试运行相关命令
2. **不要自动构建/启动**：除非用户明确要求，否则不要运行 `wails3 task build`、`package` 或 `dev`
3. **首次构建**：必须先执行 `wails3 task common:update:build-assets`

## 发布流程

1. 修改 `version_service.go` 中的 `AppVersion`
2. 执行 `wails3 task common:update:build-assets`
3. 创建 Git Tag 并推送：`git tag v1.x.x && git push origin v1.x.x`
4. GitHub Actions 自动构建全平台产物（macOS arm64/amd64, Windows installer/portable, Linux AppImage/DEB/RPM）

## 参考文档

- **黑名单功能指南**: `/BLACKLIST_FRONTEND_GUIDE.md`

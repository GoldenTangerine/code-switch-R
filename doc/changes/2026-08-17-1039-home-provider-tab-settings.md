/**
 * @name: 首页供应商 Tab 设置修复
 * @Descripttion: 修复首页供应商 Tab 显隐持久化并支持拖拽排序
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-17 10:39:31
 * @LastEditTime: 2026-08-17 10:39:31
 * @FilePath: doc/changes/2026-08-17-1039-home-provider-tab-settings.md
 */

# 首页供应商 Tab 设置修复

- 变更时间：2026-08-17 10:39:31 CST（Asia/Shanghai）
- 涉及范围：应用设置服务、首页供应商 Tab、通用设置页、国际化与回归测试

## 变更内容

- 补齐 Grok Build、Claude Desktop、OpenClaw、Hermes、Pi 的后端设置白名单，避免保存时被过滤。
- 设置页区分已显示与未显示 Tab，至少保留一个已显示项。
- 已显示 Tab 支持拖拽与键盘排序；重新启用的隐藏项追加到末尾。
- 显隐和排序变更即时保存并同步首页；活动 Tab 被隐藏时切换到第一个可见 Tab。
- 返回首页前等待防抖与排队保存完成，并防止旧设置请求覆盖最新结果。
- 首页按照持久化顺序渲染供应商 Tab，不恢复此前已被过滤的旧缓存。
- 中等与窄窗口下 Tab 和操作区分行布局，补充焦点、拖拽目标和减少动画状态。
- 新增纯函数回归测试，覆盖显隐下限、顺序、拖拽、键盘移动与活动项回退。

## 根因与预防

- 根因：前端 Tab 列表与后端允许列表分别维护，新供应商未同步到后端；同时设置保存与首页读取缺少完成和最新请求约束。
- 预防：变更供应商 Tab 时必须同步前端选项、后端白名单及两端归一化测试；并发读取设置时仅允许最新请求更新页面状态。

## 验证结果

- `go test ./services/...`：通过。
- `cd frontend && pnpm test:unit -- src/data/homeProviderTabs.test.ts src/services/appSettings.test.ts`：通过，共 63 个测试文件、482 个测试。
- `cd frontend && pnpm exec vue-tsc --noEmit`：通过。
- `git diff --check`：通过。
- 未运行 lint、Wails 构建和开发服务，符合项目限制。

/**
 * @name: macOS 主窗口 WebView 延迟释放
 * @Descripttion: 主窗口关闭后按设置延迟销毁 WebView，降低后台驻留内存
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-07-23 18:27:37
 * @LastEditTime: 2026-07-23 18:27:37
 * @FilePath: doc/changes/2026-07-23-1827-main-window-webview-release.md
 */

# macOS 主窗口 WebView 延迟释放

- 变更时间：2026-07-23 18:27:37 CST（Asia/Shanghai）
- 涉及范围：macOS 主窗口生命周期、应用设置、前端设置界面与内存优化指南

## 变更内容

- macOS 主窗口关闭后先隐藏，按设置等待 `0-300` 秒，超时后销毁主窗口 WebView。
- 宽限期内从托盘或 Dock 重开会取消销毁；销毁后再次打开将从首页重建。
- 新增“主窗口释放延迟”设置，默认 30 秒，`0` 表示立即释放，仅在 macOS 显示。
- 释放延迟改用专用即时保存接口，设置快照先更新，并通过请求版本避免快速输入时旧请求覆盖新值。
- 主窗口生命周期封装为可测试状态机，窗口创建使用独立互斥锁，不再持锁进入 Wails 原生窗口创建。
- Windows/Linux 继续保留原有隐藏主窗口行为；托盘窗口和后台 relay、日志、监控服务不受影响。
- 应用退出时取消待执行计时器，并使用代次校验避免旧计时器误销毁刚重开的窗口。
- 发布版本更新为 `v2.9.15`，同步 Wails 全平台构建元数据与发布说明。

## 验证结果

- `go test . -count=1`：通过。
- `go test -race . -run 'TestMainWindowLifecycleState' -count=1`：通过。
- `go test ./services/... -count=1`：通过。
- `go test ./services -run TestMainWindowDestroyDelayDefaultsAndNormalizes -count=1`：通过。
- `cd frontend && pnpm test:unit`：通过，57 个测试文件、419 项测试。
- `cd frontend && pnpm exec vue-tsc --noEmit`：通过。
- `wails3 task common:update:build-assets`：通过，已同步 `v2.9.15` 全平台构建元数据。
- `git diff --check`：通过。
- `go test ./... -count=1`：仓库既有 `cmd/updater` macOS 构建约束和 `scripts` 多个 `main` 冲突导致无法全量执行；主包、`services` 与模型定价测试均单独通过。
- macOS 打包态 RSS 下降比例与窗口交互：待实机验证。

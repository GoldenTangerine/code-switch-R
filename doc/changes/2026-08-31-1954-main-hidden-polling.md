<!--
@name: 主窗口隐藏态轮询治理
@Descripttion: 记录主页面周期轮询在窗口隐藏时暂停并在恢复时安全重启的实现与验证
@version: 1.0.0
@Author: sm
@Date: 2026-08-31 19:54:22
@LastEditTime: 2026-08-31 19:54:22
@FilePath: doc/changes/2026-08-31-1954-main-hidden-polling.md
-->

# 主窗口隐藏态轮询治理

## 变更时间

2026-08-31 19:54:22 CST（Asia/Shanghai）

## 涉及范围

- `main.go`
- `frontend/src/components/Main/Index.vue`
- `frontend/src/components/Main/composables/useMainPageShell.ts`
- `frontend/src/components/Main/composables/mainPollingLifecycle.ts`
- `frontend/src/components/Main/composables/mainPollingLifecycle.test.ts`
- `doc/code-performance-optimization-plan.md`

## 变更内容

- 将并发状态 Timer 改为显式启动和停止，并与 Provider 统计、额度、更新和黑名单轮询交由 Main shell 统一管理。
- 使用 Wails `common:WindowHide`、`common:WindowShow` 作为主窗口可见性主信号，`visibilitychange` 作为最小化和浏览器预览补充。
- Wails v3.0.0-beta.6 的 Linux 默认映射缺少 `WindowHide`，因此只在主窗口现有 Hide/Show 控制点补发同名 common 事件；Windows 和 macOS 保持原生映射。
- 新增纯轮询生命周期：隐藏时停止全部 Main 周期 Timer，恢复时只执行一次数据刷新，刷新完成后恢复原周期；重复可见性信号和刷新期间再次隐藏均不会重复启动。
- 组件卸载时取消 Wails/DOM 监听并停止全部 Timer。
- 未修改公共 API、Wails 调用签名、轮询周期、后台 Provider 服务、配置、数据库、依赖或用户可见结果。

## 性能对比

固定假 Timer 按现有 Wails 轮询周期测量：

- 修改前：主窗口可见或隐藏 60 秒均产生 41 个高层 Wails 轮询轮次。
- 修改后：可见 60 秒保持 41 个；隐藏 60 秒新增 0 个；恢复后 60 秒保持 41 个。
- 以上数据衡量高层轮询调度，不等同于底层 Wails 方法调用总数。

## 验证结果

- 新增生命周期测试 5 项通过，覆盖隐藏、恢复、重复信号、刷新期间再次隐藏和监听清理。
- Main 相关测试 48 项通过。
- `pnpm test:unit`：70 个文件、543 项通过。
- `pnpm exec vue-tsc --noEmit` 通过。
- `go test . -count=1` 通过。
- `go test ./services/... -count=1` 通过。
- `go test ./resources/model-pricing/... -count=1` 通过。
- Go 测试仅有项目既有 macOS 链接目标版本警告。
- 未运行 lint、构建或开发服务，符合项目限制。

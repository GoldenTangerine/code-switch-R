<!--
@name: Main KeepAlive 离页轮询治理
@Descripttion: 记录 Main 页面停用期间暂停轮询及返回恢复的实现与验证
@version: 1.0.0
@Author: sm
@Date: 2026-09-01 01:41:36
@LastEditTime: 2026-09-01 01:41:36
@FilePath: doc/changes/2026-09-01-0141-main-keepalive-polling.md
-->

# Main KeepAlive 离页轮询治理

## 变更时间

2026-09-01 01:41:36 CST（Asia/Shanghai）

## 涉及范围

- `frontend/src/components/Main/composables/mainPollingLifecycle.ts`
- `frontend/src/components/Main/composables/mainPollingLifecycle.test.ts`
- `frontend/src/components/Main/composables/useMainPageShell.ts`
- `doc/code-performance-optimization-plan.md`

## 变更内容

- Main 轮询状态机新增独立页面激活状态，窗口可见且路由激活时才运行五组 Timer。
- `useMainPageShell` 接入 `onActivated/onDeactivated`；离页立即暂停，返回执行一次既有合并刷新后恢复原周期。
- 代次保护覆盖返回刷新期间再次离页，首次激活和重复激活不会重复刷新或创建 Timer。
- 保持根级 `KeepAlive`、轮询间隔、刷新内容、Wails 方法和 UI 行为不变。

## 验证结果

- 新增测试在修改前 4 项失败，实现后目标 9 项通过。
- `pnpm exec vue-tsc --noEmit` 通过。
- `pnpm test:unit`：71 个测试文件、556 项测试通过。
- 假 Timer 测量：激活态 41 轮/60 秒，离页态 0 轮/60 秒，返回刷新 1 次后恢复 41 轮/60 秒。
- `git diff --check` 通过。

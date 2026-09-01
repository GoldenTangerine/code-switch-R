<!--
@name: Logs KeepAlive 离页刷新治理
@Descripttion: 记录 Logs 页面停用期间暂停自动刷新及返回恢复的实现与验证
@version: 1.0.0
@Author: sm
@Date: 2026-09-01 01:47:08
@LastEditTime: 2026-09-01 01:47:08
@FilePath: doc/changes/2026-09-01-0147-logs-keepalive-refresh.md
-->

# Logs KeepAlive 离页刷新治理

## 变更时间

2026-09-01 01:47:08 CST（Asia/Shanghai）

## 涉及范围

- `frontend/src/components/Logs/Index.vue`
- `frontend/src/components/Logs/composables/useLogsAutoRefresh.ts`
- `frontend/src/components/Logs/composables/useLogsAutoRefresh.test.ts`
- `doc/code-performance-optimization-plan.md`

## 变更内容

- Logs 自动刷新 composable 新增页面激活和代次状态，停用时立即停止倒计时。
- Logs 页面接入 `onActivated/onDeactivated`；返回时刷新一次，再从当前完整间隔倒计时。
- 首次激活、重复激活、初始化中离页和刷新中再次离页均不会创建重复 Timer 或重叠请求。
- 0 秒关闭语义、默认 30 秒、手动刷新、筛选、分页、设置格式和 Wails 方法保持不变。

## 验证结果

- 新增测试在修改前 4 项失败，实现后目标 8 项通过。
- `pnpm exec vue-tsc --noEmit` 通过。
- `pnpm test:unit`：71 个测试文件、560 项测试通过。
- 假 Timer 测量：默认 30 秒离页刷新 2 次/60 秒降为 0；返回刷新 1 次后按 30 秒继续；0 秒配置为 0。
- `git diff --check` 通过。

# `Console/Index.vue` 详细迁移文档

- 源文件：`frontend/src/components/Console/Index.vue`
- 当前行数：`947`
- 当前路由：`/console`
- 优先级：`P1`

---

## 1. 当前问题画像

`Console` 页虽然没到几千行，但已经具备典型的可拆信号：

- 日志加载与自动刷新。
- 自动滚动与底部定位。
- 级别筛选与关键字筛选。
- level 归一化、message 推断级别、diagnostic tags 提取。
- 清空确认弹窗。
- 复制、错误链接命中等辅助交互。

现在继续堆功能，很快就会从“还能忍”变成“谁碰谁裂开”。

---

## 2. 迁移目标

- 让页面只负责壳子和刷新节奏。
- 把控制栏、日志列表、清空弹窗拆开。
- 把日志格式化、级别推断、tag 提取搬到工具或 composable。

---

## 3. 推荐目录结构

```text
frontend/src/components/Console/
  Index.vue
  components/
    ConsoleToolbar.vue
    ConsoleLogList.vue
    ConsoleLogRow.vue
  modals/
    ConsoleClearConfirmModal.vue
  composables/
    useConsoleLogs.ts
    useConsoleFilters.ts
  utils.ts
  types.ts
```

---

## 4. 拆分建议

- `ConsoleToolbar.vue`：返回、刷新、筛选、关键字、清空入口。
- `ConsoleLogList.vue`：日志容器和滚动控制。
- `ConsoleLogRow.vue`：单行日志渲染、tag 展示、复制交互。
- `ConsoleClearConfirmModal.vue`：清空确认。
- `useConsoleLogs.ts`：日志拉取、定时刷新、清空。
- `useConsoleFilters.ts`：关键字过滤、级别过滤、展示日志派生。
- `utils.ts`：时间格式化、level 归一化、诊断标签抽取。

---

## 5. 分阶段迁移步骤

### Phase 1
- 抽 toolbar 和 clear modal。

### Phase 2
- 抽 log list 与 row。

### Phase 3
- 抽 filters 与 logs composable。

---

## 6. 高风险点

- 自动滚动逻辑依赖 DOM 更新时机，拆分后要保留 `nextTick` 时序。
- level 推断和 diagnostic tag 提取属于显示质量关键逻辑，不能改歪。
- 自动刷新要确保 onUnmount 时清理 interval。

---

## 7. 回归清单

- 日志能正常加载。
- 自动刷新正常。
- 级别筛选、关键字筛选正常。
- 自动滚动开关正常。
- 清空日志与复制内容正常。

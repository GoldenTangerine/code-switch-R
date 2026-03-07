# `General/Index.vue` 详细迁移文档

- 源文件：`frontend/src/components/General/Index.vue`
- 当前行数：`3116`
- 当前路由：`/settings`
- 优先级：`P0`

---

## 1. 当前问题画像

设置页现在已经不是“设置页”，而是：

- 应用设置中心。
- 预算面板。
- 更新管理面板。
- 拉黑配置面板。
- 数据导入面板。
- WebDAV 同步中心。
- 模型价格弹窗入口。

展示上虽然借了 `Setting` 目录下的一些组件，但状态、轮询、缓存、WebDAV 流程、预算计算还全塞在根文件里，还是过胖。

---

## 2. 迁移目标

- 按业务域拆 section，而不是继续堆表单项。
- 把预算、更新、拉黑、导入、WebDAV 五套逻辑分开。
- 根文件只做 section 编排和共享状态协调。

---

## 3. 推荐目录结构

```text
frontend/src/components/General/
  Index.vue
  sections/
    GeneralAppSection.vue
    GeneralBudgetSection.vue
    GeneralUpdateSection.vue
    GeneralBlacklistSection.vue
    GeneralImportSection.vue
    GeneralWebdavSection.vue
  modals/
    GeneralWebdavManageModal.vue
    GeneralWebdavUploadModal.vue
    GeneralWebdavDownloadModal.vue
  composables/
    useGeneralSettings.ts
    useBudgetSettings.ts
    useUpdateManager.ts
    useBlacklistSettings.ts
    useImportSettings.ts
    useWebdavSync.ts
  types.ts
  constants.ts
```

---

## 4. 拆分边界建议

### 4.1 section 划分

- `GeneralAppSection.vue`：主题、语言、基础开关、应用级配置。
- `GeneralBudgetSection.vue`：预算周期、展示、刷新、预测方式。
- `GeneralUpdateSection.vue`：检查更新、下载、安装、进度展示。
- `GeneralBlacklistSection.vue`：拉黑总开关、等级拉黑、阈值与时长。
- `GeneralImportSection.vue`：外部配置导入状态与执行。
- `GeneralWebdavSection.vue`：WebDAV 配置入口、同步入口、状态概览。

### 4.2 modal 划分

- `GeneralWebdavManageModal.vue`：配置编辑。
- `GeneralWebdavUploadModal.vue`：上传预览和上传进度。
- `GeneralWebdavDownloadModal.vue`：下载进度、备份提示、错误日志。

### 4.3 composable 划分

- `useGeneralSettings.ts`：基础应用设置加载与持久化。
- `useBudgetSettings.ts`：预算计算、ticker、缓存同步。
- `useUpdateManager.ts`：更新状态、下载、安装、错误处理。
- `useBlacklistSettings.ts`：拉黑配置加载、保存、回滚。
- `useImportSettings.ts`：导入状态和执行。
- `useWebdavSync.ts`：WebDAV 配置、上传、下载、事件流。

---

## 5. 分阶段迁移步骤

### Phase 1：按 section 切页面

先不动业务逻辑，先把模板按 section 切出去，让 `Index.vue` 不再直接承载所有面板。

### Phase 2：抽预算逻辑

预算相关的：

- 周期配置。
- 预算已用值计算。
- 定时刷新。
- localStorage 缓存。

统一移到 `useBudgetSettings.ts`。

### Phase 3：抽更新和 WebDAV 逻辑

- 更新管理收口到 `useUpdateManager.ts`。
- WebDAV 上传 / 下载 / 管理统一收口到 `useWebdavSync.ts`。

### Phase 4：抽导入和拉黑逻辑

把剩余状态域收尾，根文件只留下组装层。

---

## 6. 高风险点

- 预算逻辑有 `claude / codex` 双平台分支，拆的时候很容易同步失效。
- 更新与 WebDAV 都是异步长流程，loading / progress / error 状态不能丢。
- localStorage 缓存同步如果拆坏，会出现页面闪烁或表单状态回跳。
- WebDAV 事件监听要注意注册和销毁，否则内存和状态全串。

---

## 7. 手工回归清单

- 基础设置开关保存后能生效。
- 预算显示、刷新、预测、周期切换正常。
- 检查更新、下载更新、安装更新流程正常。
- 拉黑开关、等级拉黑、阈值配置正常。
- 导入路径、导入状态、导入动作正常。
- WebDAV 配置保存、测试、上传、下载流程正常。
- 模型价格弹窗能正常打开和关闭。

---

## 8. 完成定义

- `Index.vue` 以 section 装配为主，控制在 `600 ~ 800` 行。
- WebDAV、预算、更新三大逻辑域已迁出。
- 页面后续新增设置项时，不需要继续往根文件堆逻辑。

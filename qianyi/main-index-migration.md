# `Main/Index.vue` 详细迁移文档

- 源文件：`frontend/src/components/Main/Index.vue`
- 当前行数：`5047`
- 当前路由：`/`
- 优先级：`P0`

---

## 1. 当前问题画像

首页文件现在像个“全功能控制塔”：

- 首页横幅、首次使用提示。
- 平台 Tab 切换。
- Provider 卡片列表与单卡操作。
- Provider 新增 / 编辑大表单。
- 模型列表弹窗。
- CLI 配置相关内容。
- 黑名单状态、可用性状态、更新状态轮询。
- 自定义 CLI 工具与代理状态。
- 热力图、更新提示、跳转逻辑、事件监听。

这文件继续长下去，后面改个 Provider 字段都得担心把更新轮询和黑名单一起整炸了，属实离谱。

---

## 2. 迁移目标

- 让首页只保留页面框架和平台切换 orchestration。
- 将 Provider 卡片域、编辑表单域、CLI 工具域、状态轮询域彻底拆开。
- 将平台间数据转换逻辑从页面层剥离。
- 让每个新增功能都能找到明确归属目录。

---

## 3. 推荐目录结构

```text
frontend/src/components/Main/
  Index.vue
  components/
    MainHeroBanner.vue
    MainToolbar.vue
    MainPlatformTabs.vue
    ProviderCardGrid.vue
    ProviderCard.vue
    MainUpdateBanner.vue
    MainStatusBadges.vue
  modals/
    ProviderEditModal.vue
    CliConfigModal.vue
    CliDeleteConfirmModal.vue
    ProviderModelListModal.vue
  composables/
    useProviderCards.ts
    useProviderForm.ts
    useBlacklistState.ts
    useAvailabilityState.ts
    useUpdatePolling.ts
    useCustomCliTools.ts
  adapters/
    providerCardMappers.ts
  types.ts
  constants.ts
```

---

## 4. 拆分边界建议

### 4.1 根文件保留内容

- 当前激活平台。
- 页面入口数据加载时机。
- 跨区块协调，例如编辑完 Provider 后刷新卡片列表。
- 路由跳转与外层事件注册。

### 4.2 UI 区块拆分

- `MainHeroBanner.vue`：首次使用提示、顶部说明区。
- `MainToolbar.vue`：全局操作区、刷新、跳转类动作。
- `MainPlatformTabs.vue`：平台切换与平台说明。
- `ProviderCardGrid.vue`：卡片列表容器。
- `ProviderCard.vue`：单卡 UI 与按钮事件。
- `MainStatusBadges.vue`：黑名单、可用性、当前使用、代理状态等徽章整合。

### 4.3 Modal 拆分

- `ProviderEditModal.vue`：供应商新增 / 编辑表单全量迁出。
- `CliConfigModal.vue`：CLI 工具配置弹窗。
- `CliDeleteConfirmModal.vue`：CLI 删除确认。
- `ProviderModelListModal.vue`：继续保留，但从页面 orchestrator 调用。

### 4.4 composable 拆分

- `useProviderCards.ts`：Provider 列表加载、排序、转换。
- `useProviderForm.ts`：表单状态、校验、提交、默认值。
- `useBlacklistState.ts`：黑名单状态查询、倒计时、解禁操作。
- `useAvailabilityState.ts`：可用性结果加载与徽章映射。
- `useUpdatePolling.ts`：更新状态轮询、GitHub 入口、提示状态。
- `useCustomCliTools.ts`：自定义 CLI 工具列表、选中态、代理状态。

### 4.5 adapter 拆分

Provider / Gemini / custom CLI 的互转逻辑不要继续留在页面文件里，建议集中到：

- `providerCardMappers.ts`
- `types.ts`

---

## 5. 分阶段迁移步骤

### Phase 1：先把模板分块

- 抽 `Hero / Tabs / CardGrid / StatusBadge`。
- 保留原事件处理函数不动，先减少模板长度。

### Phase 2：把大表单 modal 整体迁出

- `ProviderEditModal.vue` 直接接管新增 / 编辑流程。
- 表单默认值、校验、提交拆到 `useProviderForm.ts`。

### Phase 3：状态域迁出

- 黑名单。
- 可用性。
- 更新轮询。
- 自定义 CLI 工具。

### Phase 4：收口平台转换逻辑

- 清理页面文件里对 Gemini 和 others 平台的特殊分支。
- 把映射逻辑沉到 adapters。

---

## 6. 高风险点

- `claude / codex / gemini / others` 四个平台逻辑并不完全对称，拆分时不能硬套一个组件把差异抹平。
- Provider 编辑表单字段很多，直接拆容易漏掉 deprecated 字段兼容。
- 黑名单、可用性、更新轮询都带定时器和事件监听，拆时要严格清理副作用。
- `others` 平台依赖 CLI 工具列表，状态同步关系比较脆，得单独兜住。

---

## 7. 手工回归清单

- Tab 切换正常，平台数据不串。
- Provider 新增 / 编辑 / 删除 / 复制正常。
- 模型列表弹窗能正常打开。
- 黑名单状态刷新、手动解禁正常。
- 可用性徽章与提示文本正常。
- 更新状态轮询、跳转 GitHub、更新提示正常。
- `others` 平台选择自定义 CLI 工具、配置文件编辑正常。

---

## 8. 完成定义

- `Index.vue` 降到 `700` 行以内。
- Provider 表单完整迁出。
- 黑名单 / 可用性 / 更新 / custom CLI 至少 3 个状态域迁出到 composable。
- 平台转换逻辑不再散落在页面主体中。

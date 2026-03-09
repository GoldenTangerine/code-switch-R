# `Main/Index.vue` 详细迁移文档

- 源文件：`frontend/src/components/Main/Index.vue`
- 当前行数：`514`（2026-03-09，收尾修正后）
- 当前路由：`/`
- 优先级：`P0`
- 当前状态：`已完成（2026-03-09，首页根文件已收敛为页面 orchestrator）`

---

## 0. 当前进展（2026-03-09）

### 0.1 已完成

- 已将首页根文件从 `5047` 行收敛到 `514` 行，根文件已切换为页面 orchestrator 角色。
- 已落地 `components/`：
  - `MainToolbar.vue`
  - `MainHeroBanner.vue`
  - `MainPlatformTabs.vue`
  - `MainCustomCliToolsBar.vue`
  - `MainUsageHeatmap.vue`
  - `ProviderCardGrid.vue`
  - `ProviderCard.vue`
- 已落地 `modals/`：
  - `ProviderEditModal.vue`
  - `CliConfigModal.vue`
  - `CliDeleteConfirmModal.vue`
  - `ProviderModelListModal.vue`
- 已落地 `composables/`：
  - `useProviderCards.ts`
  - `useProviderForm.ts`
  - `useProviderStats.ts`
  - `useBlacklistState.ts`
  - `useAvailabilityState.ts`
  - `useUpdatePolling.ts`
  - `useCustomCliTools.ts`
  - `useMainPageShell.ts`
- 已落地 `adapters / 基础类型 / 常量`：
  - `providerCardMappers.ts`
  - `providerFormMappers.ts`
  - `types.ts`
  - `constants.ts`
- 已将拆分后仍依赖首页根样式的子组件样式收口回组件边界：
  - `components/MainToolbar.vue`
  - `components/MainHeroBanner.vue`
  - `components/MainCustomCliToolsBar.vue`
  - `components/ProviderCard.vue`
  - `modals/ProviderEditModal.vue`
  - `modals/CliConfigModal.vue`
- 已新增 `styles/`：
  - `provider-card.css`
  - `provider-edit-modal.css`
  - `cli-config-modal.css`
- 已移除全局 `Index.css` 兜底，避免 `Main` 页拆分后样式泄漏到其他页面。
- 已统一 Provider 表单默认值、旧字段兼容和落库字段映射，避免弹窗与页面保存链路口径分叉。
- 已将 Provider 新增 / 编辑 / 删除 / 复制 / 模型列表弹窗编排下沉到 `useProviderForm.ts`，根文件不再直接持有这组表单会话逻辑。
- 已将 Provider 统计展示、定价缓存预热、点击刷新与定时轮询下沉到 `useProviderStats.ts`，避免根文件继续堆积列表派生逻辑。
- 已将热力图展示与 tooltip 状态整体迁出到 `MainUsageHeatmap.vue`，根文件不再持有这组高密度 UI 逻辑。
- 已将首页主题、导入状态、首次运行提示、代理切换与页面级刷新编排下沉到 `useMainPageShell.ts`。
- 已修正热力图 `reload` 暴露链路，并补齐事件驱动切换平台时的激活刷新副作用，避免拆分后出现运行时行为偏差。
- 已修复 `others` 平台在 CLI 工具切换 / 新增 / 删除后的代理状态不同步问题，并将自定义 CLI 代理状态拉取改为并发，避免拆分后出现状态陈旧和串行加载拖慢首页初始化。

### 0.2 说明

- 文档建议中的 `MainUpdateBanner.vue`、`MainStatusBadges.vue` 没有单独新建文件。
- 这两块职责已经分别收口到 `MainToolbar.vue` 与 `ProviderCard.vue` / `ProviderCardGrid.vue`，功能和样式保持与原页面一致，因此本轮不再为了“凑文件名”额外拆空壳组件。

### 0.3 已验证

- `npx vue-tsc --noEmit`：已通过。
- `wc -l frontend/src/components/Main/Index.vue`：`514`，已满足 `700` 行以内目标。
- `npm run build:dev`：未完成。
  - 原因：当前本地 Node 版本为 `18.20.8`，而当前 Vite 依赖要求 `20.19+`，构建失败属于环境约束，不是本轮改造直接引入的 TS 编译错误。

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
    MainCustomCliToolsBar.vue
    MainUsageHeatmap.vue
    ProviderCardGrid.vue
    ProviderCard.vue
  modals/
    ProviderEditModal.vue
    CliConfigModal.vue
    CliDeleteConfirmModal.vue
    ProviderModelListModal.vue
  composables/
    useProviderCards.ts
    useProviderForm.ts
    useProviderStats.ts
    useBlacklistState.ts
    useAvailabilityState.ts
    useUpdatePolling.ts
    useCustomCliTools.ts
    useMainPageShell.ts
  adapters/
    providerCardMappers.ts
    providerFormMappers.ts
  styles/
    provider-card.css
    provider-edit-modal.css
    cli-config-modal.css
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
- 样式边界已补收口，拆分出的子组件不再依赖首页根文件的全局 class 泄漏。

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

- 当前状态：`已完成（2026-03-09）`
- 抽 `Hero / Tabs / CardGrid / StatusBadge`。
- 保留原事件处理函数不动，先减少模板长度。

### Phase 2：把大表单 modal 整体迁出

- 当前状态：`已完成（2026-03-09，弹窗、mapper 与 form composable 均已落地）`
- `ProviderEditModal.vue` 直接接管新增 / 编辑流程。
- 表单默认值、校验、提交拆到 `useProviderForm.ts`。

### Phase 3：状态域迁出

- 当前状态：`已完成扩展目标（2026-03-09，含 Provider 统计 / 定价刷新）`
- 黑名单。
- 可用性。
- 更新轮询。
- 自定义 CLI 工具。
- Provider 统计展示与定价缓存刷新。

### Phase 4：收口平台转换逻辑

- 当前状态：`已完成（2026-03-09，Provider 映射已收口到 adapters，根文件仅保留必要的页面壳层条件分支）`
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

- [x] `Index.vue` 降到 `700` 行以内。
- [x] Provider 表单完整迁出。
- [x] Provider 统计 / 定价刷新逻辑已从根文件下沉。
- [x] 黑名单 / 可用性 / 更新 / custom CLI 至少 3 个状态域迁出到 composable。
- [x] 平台转换逻辑不再散落在页面主体中。

# 前端超大功能文件组件化改造任务书

> 创建时间：2026-03-07
> 目标：将前端超大功能文件逐步拆分为 `Page Shell + Feature Components + Composables + Services/Utils`，降低单文件复杂度，提升后续迭代与排障效率。

---

## 1. 背景与问题

当前前端多个功能页面长期以单个 `Index.vue` 承载完整页面模板、状态管理、接口调用、弹窗逻辑、图表配置、工具函数和样式，已经明显超出可维护范围。

这类文件的共同问题：

- 单文件职责过多，新增需求时容易“改一处，崩三处”。
- 页面逻辑、展示逻辑、数据逻辑混杂，阅读和定位成本极高。
- 同类功能无法复用，后续只能继续往原文件里堆，越堆越大。
- Review、联调、回归验证粒度过粗，任何一个小改动都像拆炸弹。

本次先不直接大面积改代码，先产出统一改造任务书，作为后续分阶段实施的依据。

---

## 2. 当前现状盘点

### 2.1 超大功能文件清单

| 优先级 | 文件 | 路由/模块 | 当前行数 | 主要问题 |
|---|---|---|---:|---|
| P0 | `frontend/src/components/Logs/Index.vue` | `/logs` | 5828 | 过滤器、统计卡片、图表、表格、存储弹窗、详情弹窗、tooltip、热力图、样式全部堆在一起 |
| P0 | `frontend/src/components/Main/Index.vue` | `/` | 5047 | 首页卡片、Provider 管理、CLI 工具、代理状态、更新状态、黑名单、可用性、弹窗表单全混合 |
| P0 | `frontend/src/components/General/Index.vue` | `/settings` | 3116 | 应用设置、预算设置、更新管理、拉黑配置、导入、WebDAV 同步等多个域被塞在同页 |
| P1 | `frontend/src/components/Mcp/index.vue` | `/mcp` | 1336 | 页面入口文件已明显偏大，后续容易继续膨胀 |
| P1 | `frontend/src/components/Skill/Index.vue` | `/skill` | 1090 | 页面逻辑集中，后续扩展风险高 |
| P1 | `frontend/src/components/Console/Index.vue` | `/console` | 947 | 已接近需要拆分的阈值 |

### 2.2 需要纳入第二梯队的共享大组件

| 优先级 | 文件 | 当前行数 | 说明 |
|---|---|---:|---|
| P1 | `frontend/src/components/common/CLIConfigEditor.vue` | 1854 | 共享编辑器体量过大，应在主功能页拆分稳定后继续下钻 |
| P1 | `frontend/src/components/Setting/ModelPricingModal.vue` | 793 | 业务复杂度偏高，后续适合拆成子弹窗 + 编辑区 |

### 2.3 当前项目已有的可复用基础

项目并不是“零基础乱炖”，已经具备继续模块化的条件：

- 已有 `frontend/src/services/`，适合继续沉淀 API 访问与数据适配。
- 已有 `frontend/src/composables/`，例如 `useAdaptiveHeatmap.ts`，说明状态逻辑可外提。
- 已有 `frontend/src/components/common/` 与 `frontend/src/components/Setting/`，说明基础组件和局部子组件模式已经存在。
- 当前路由集中在 `frontend/src/router/index.ts`，后续可以保持路由入口文件不变，只拆内部实现。

---

## 3. 本次改造的总体目标

### 3.1 目标

- 将超大页面拆成“页面壳子 + 业务分区组件 + 状态编排逻辑 + 工具函数”。
- 保持外部行为不变：路由不变、核心交互不变、现有 i18n key 尽量不变。
- 将后续新增需求的改动粒度，从“改整个页面大文件”降到“改某个区块组件或 composable”。
- 为后续补测试、补文档、补代码审查规范提供边界清晰的落点。

### 3.2 非目标

- 本轮不做视觉重设计。
- 本轮不追求把所有重复逻辑一次性抽成全局通用组件，避免过度抽象。
- 本轮不改路由结构，不改后端接口契约，不做大规模命名迁移。

---

## 4. 拆分原则

### 4.1 职责分层原则

每个超大页面拆分后，建议按以下层次组织：

- `Index.vue`：只保留页面级组装、路由跳转、顶层状态拼装。
- `components/` 或 `sections/`：负责具体 UI 区块展示与局部交互。
- `modals/`：负责弹窗类独立子功能。
- `composables/`：负责页面级状态逻辑、轮询、tooltip、筛选、分页、热力图等复用逻辑。
- `services/`：继续复用已有全局服务；若某功能存在较重的前端组装逻辑，可在 feature 内增加轻量 adapter。
- `types.ts` / `constants.ts` / `utils.ts`：沉淀纯类型、常量和无副作用工具函数。

### 4.2 稳定边界原则

- 保持 `router` 入口文件路径不变。
- 保持页面对外事件和主要交互文案不变。
- 保持接口调用来源尽量不变，优先拆视图和状态，不先乱改服务层。

### 4.3 渐进式原则

- 单个 PR 只聚焦一个页面或一个共享大组件。
- 先拆结构最混乱、收益最高的页面，再拆次级页面。
- 每完成一个页面，都要先做手工回归，再推进下一个页面。

### 4.4 命名与目录原则

- 页面根文件继续使用 `Index.vue`。
- 子组件统一使用功能前缀命名，避免跨目录后难以识别归属。
- 避免出现 `Modal.vue`、`List.vue` 这种看一眼完全不知道是谁家的名字。

示例：

```text
frontend/src/components/Logs/
  Index.vue
  components/
    LogsHeaderBar.vue
    LogsFilterBar.vue
    LogsSummaryCards.vue
    LogsChartsPanel.vue
    LogsTable.vue
  modals/
    LogsStorageModal.vue
    LogsCostDetailModal.vue
    LogsTokenDetailModal.vue
    LogsPayloadDetailModal.vue
  composables/
    useLogsFilters.ts
    useLogsQuery.ts
    useLogsStorage.ts
    useLogTooltips.ts
  types.ts
  constants.ts
  utils.ts
```

---

## 5. 拆分判定标准

后续改造时，建议使用以下标准判断一个功能文件是否必须拆分：

### 5.1 强制拆分条件

满足任意一条即可进入改造清单：

- 单文件超过 `800` 行。
- 同时包含 `页面模板 + 接口调用 + 复杂弹窗 + 多组 computed/watch`。
- 同时维护 `3` 个以上业务域状态。
- 存在大量局部工具函数，且这些函数不依赖完整页面上下文。

### 5.2 目标落地标准

- 页面入口 `Index.vue` 目标控制在 `300 ~ 600` 行。
- 普通业务区块组件尽量控制在 `300` 行以内。
- 复杂弹窗或编辑器组件尽量控制在 `500` 行以内。
- 单个 composable 尽量聚焦单一职责，通常不超过 `300` 行。

这不是死规定，但至少得让文件别再膨胀成“谁碰谁倒霉”的级别。

---

## 6. 分模块改造方案

## 6.1 P0：`Logs` 页面

### 现状问题

`frontend/src/components/Logs/Index.vue` 同时承载：

- 页面头部与刷新逻辑。
- 日志筛选表单。
- 汇总卡片与统计交互。
- 图表展示。
- 日志表格与 tooltip。
- 存储分析弹窗与热力图。
- 金额 / Token / Payload 等多个详情弹窗。
- 大量格式化与数据拼装函数。

### 推荐拆分结构

```text
frontend/src/components/Logs/
  Index.vue
  components/
    LogsHeaderBar.vue
    LogsFilterBar.vue
    LogsSummaryCards.vue
    LogsChartsPanel.vue
    LogsTable.vue
    LogsPagination.vue
  modals/
    LogsStorageModal.vue
    LogsCostDetailModal.vue
    LogsTokenDetailModal.vue
    LogsPayloadDetailModal.vue
  composables/
    useLogsFilters.ts
    useLogsPageData.ts
    useLogsStorageHeatmap.ts
    useLogInfoTooltip.ts
    useLogCostDetail.ts
  types.ts
  constants.ts
  utils.ts
```

### 拆分任务

- 把头部刷新区和返回按钮拆成 `LogsHeaderBar.vue`。
- 把筛选区整体拆成 `LogsFilterBar.vue`，输入输出使用明确的 `props/emits`。
- 把统计卡片和卡片点击行为拆成 `LogsSummaryCards.vue`。
- 把图表区拆成 `LogsChartsPanel.vue`。
- 把日志表格拆成 `LogsTable.vue`，tooltip 展示逻辑尽量下沉到 composable。
- 把存储分析相关 UI 拆成 `LogsStorageModal.vue`。
- 把金额、Token、Payload 明细弹窗拆成独立 modal 组件。
- 将日期格式化、tooltip 定位、金额/Token 展示计算等纯函数抽到 `utils.ts`。
- 将筛选、分页、请求刷新、自动刷新计时器等逻辑抽到 `composables/`。

### 完成标准

- `Logs/Index.vue` 仅负责页面组装与顶层数据协调。
- 弹窗逻辑不再和日志表格主体糊成一坨。
- tooltip 和热力图相关逻辑不再散落在页面大文件里。

---

## 6.2 P0：`Main` 页面

### 现状问题

`frontend/src/components/Main/Index.vue` 当前包含：首页横幅、平台切换、Provider 卡片列表、配置表单、CLI 工具、黑名单状态、可用性状态、更新状态、热力图、多个模态框等，是典型的“首页兼控制台兼管理后台兼弹窗仓库”。

### 推荐拆分结构

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
    MainConnectivityBadge.vue
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
    useUpdateState.ts
    useCustomCliTools.ts
  adapters/
    providerCardMappers.ts
  types.ts
  constants.ts
```

### 拆分任务

- 将顶部横幅、更新提示、Tab 切换栏拆为独立展示组件。
- 将 Provider 卡片列表与单卡交互拆成 `ProviderCardGrid.vue` + `ProviderCard.vue`。
- 将供应商新增/编辑表单整体拆成 `ProviderEditModal.vue`。
- 将 CLI 工具相关弹窗、配置文件列表、代理注入配置拆成独立 modal 组件。
- 将黑名单、可用性、更新轮询、自定义 CLI 工具等状态逻辑抽出 composable。
- 将 Provider / Gemini / custom CLI 的数据转换逻辑下沉到 `adapters/` 或 `utils/`。

### 完成标准

- `Main/Index.vue` 不再直接持有海量表单字段和所有业务状态。
- 单个组件职责清晰：卡片展示、表单编辑、CLI 配置、状态徽章分别归位。
- Provider 转换逻辑从页面文件中剥离，避免继续堆业务兼容代码。

---

## 6.3 P0：`General` 设置页面

### 现状问题

`frontend/src/components/General/Index.vue` 已经复用了部分 `Setting` 子组件，但页面仍承担了应用设置、预算设置、更新管理、拉黑配置、数据导入、WebDAV 同步、模型价格弹窗等多个业务域。

### 推荐拆分结构

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
    useWebdavSync.ts
  types.ts
  constants.ts
```

### 拆分任务

- 按业务域拆出 6 个 section 组件。
- WebDAV 的上传、下载、管理相关弹窗单独拆出。
- 预算相关的计算、刷新 ticker、缓存同步逻辑抽到 `useBudgetSettings.ts`。
- 更新相关逻辑抽到 `useUpdateManager.ts`。
- 拉黑、导入、WebDAV 各自下沉为 composable。

### 完成标准

- `General/Index.vue` 只负责组织 section 和共享顶层状态。
- WebDAV 与更新逻辑不再直接塞满页面文件。
- 设置页面可以按业务域独立维护，不再动一处牵全身。

---

## 6.4 P1：其余页面与共享组件

### 页面类

- `frontend/src/components/Mcp/index.vue`
- `frontend/src/components/Skill/Index.vue`
- `frontend/src/components/Console/Index.vue`

### 共享组件类

- `frontend/src/components/common/CLIConfigEditor.vue`
- `frontend/src/components/Setting/ModelPricingModal.vue`

### 改造策略

- 优先按“页面区块 + 表单区块 + 弹窗区块”拆分。
- 先稳定功能页边界，再对跨页面共享的大组件做二次拆分。
- 对共享组件的抽象要克制，避免为了“看起来通用”把业务细节硬抹平。

---

## 7. 推荐实施顺序

建议按以下顺序推进：

### Phase 0：基线确认

- 建立超大文件清单与目标目录方案。
- 确认每个页面的手工回归 checklist。
- 确认命名规范和目录规范。

### Phase 1：优先拆 `Logs`

- 原因：行数最大、业务块天然分区明显、拆分收益最高。
- 产出：`components/`、`modals/`、`composables/` 基础骨架。

### Phase 2：拆 `Main`

- 原因：首页是高频变更区域，继续堆代码只会更离谱。
- 产出：Provider 卡片域、表单 modal 域、状态轮询域彻底分离。

### Phase 3：拆 `General`

- 原因：设置项天然按业务域划分，适合稳定成 section 架构。
- 产出：多个 section + 多个 composable。

### Phase 4：处理中等页面与共享大组件

- 包括 `Mcp`、`Skill`、`Console`、`CLIConfigEditor`、`ModelPricingModal`。

---

## 8. 每个页面的标准改造步骤

后续执行时，建议所有页面都按同一套步骤推进：

1. 先识别页面中的业务分区、弹窗分区、纯工具函数分区。
2. 保留原 `Index.vue` 作为页面入口，不先改路由。
3. 先抽“纯展示区块组件”，确保模板边界稳定。
4. 再抽“状态逻辑 composable”，把 watch、轮询、tooltip、表单状态迁出去。
5. 再抽 `utils.ts`、`types.ts`、`constants.ts`。
6. 最后整理样式归属，避免样式继续全部留在根文件。
7. 每完成一步都做最小回归，确认功能没被拆坏。

这个顺序比较稳，不容易一上来把依赖关系拆成一锅浆糊。

---

## 9. 验收标准

每个模块完成后，至少满足以下要求：

- 路由路径不变，页面入口不变。
- 现有核心交互不变：查询、切换、弹窗、保存、删除、刷新等功能正常。
- 主要接口调用行为不变。
- 页面入口文件明显瘦身，职责变成“页面编排”而不是“全功能垃圾场”。
- 拆出的子组件、composable、utils 具备清晰命名和清晰边界。
- 手工回归通过，未出现明显 UI 倒退或状态同步错误。

---

## 10. 风险与规避

### 风险 1：拆成一堆组件，但状态还是耦合

规避方式：

- 组件只负责展示与局部交互。
- 页面级状态统一通过 composable 编排。
- 不把所有 `ref/computed/watch` 原样复制到各个组件里继续污染。

### 风险 2：为了通用化过度抽象

规避方式：

- 先按 feature 内聚组织。
- 两个页面以上都稳定复用后，再考虑升级成 `common/`。

### 风险 3：拆分过程中功能回归

规避方式：

- 一次只拆一个页面。
- 拆分前先列出关键交互清单。
- 保持服务层和路由层尽量不动。

### 风险 4：样式继续堆在根文件

规避方式：

- 样式跟随组件迁移。
- 页面根文件只保留布局级样式。

---

## 11. 后续执行建议

建议下一步直接从 `Logs` 页面开始出第一版拆分任务，按以下节奏推进：

- 第一步：建立 `Logs/components`、`Logs/modals`、`Logs/composables` 目录骨架。
- 第二步：先拆 `Header / Filter / Summary / Table / StorageModal` 五块大区。
- 第三步：再拆 tooltip、详情弹窗和工具函数。
- 第四步：完成一轮手工回归后，再进入 `Main` 页面。

如果后续要继续落地，可以直接基于本文档再拆出每个页面的“子任务清单 + 预计 PR 切分方案”。

---

## 12. 可直接执行的任务清单

### P0

- [ ] 建立统一的前端功能页拆分规范。
- [ ] 完成 `Logs` 页面目录骨架搭建。
- [ ] 完成 `Logs` 页面第一轮组件拆分。
- [ ] 完成 `Main` 页面目录骨架搭建。
- [ ] 完成 `Main` 页面第一轮组件拆分。
- [ ] 完成 `General` 页面目录骨架搭建。
- [ ] 完成 `General` 页面第一轮组件拆分。

### P1

- [ ] 完成 `Mcp` 页面拆分。
- [ ] 完成 `Skill` 页面拆分。
- [ ] 完成 `Console` 页面拆分。
- [ ] 完成 `CLIConfigEditor` 二次拆分。
- [ ] 完成 `ModelPricingModal` 二次拆分。

---

## 13. 结论

这次改造的核心，不是“把一个 5000 行文件砍成 10 个小文件看着舒服”，而是把页面里的职责边界重新立起来。只要边界立住，后面加功能、查问题、做回归都能轻松不少；边界立不住，就算文件名拆开了，本质上还是一锅乱炖。

先从 `Logs` 开刀，最合适，也最能立规矩。

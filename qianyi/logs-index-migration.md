# `Logs/Index.vue` 详细迁移文档

- 源文件：`frontend/src/components/Logs/Index.vue`
- 原始行数：`1843`
- 当前行数：`508`
- 当前路由：`/logs`
- 优先级：`P0`
- 当前状态：`已完成（Index.vue 已收敛为 508 行页面 orchestrator，2026-03-08）`

---

## 0. 当前进度

- [x] `Phase 1`：抽页面区块（已完成，`2026-03-08`）
- [x] `Phase 2`：抽弹窗（已完成，`2026-03-08`）
- [x] `Phase 3`：抽状态逻辑（已完成，`2026-03-08`）
- [x] `Phase 4`：抽纯工具（已完成，`2026-03-08`）

---

## 1. 当前问题画像

这个文件已经不是“页面组件”了，基本是一个前端小系统，职责严重超载，当前至少同时承担了：

- 页面头部、返回首页、手动刷新、自动刷新倒计时。
- 筛选表单：平台、Provider、日期类型、年月日范围切换。
- 汇总卡片及卡片点击交互。
- 图表区：折线图、环图、统计趋势展示。
- 日志表格：数据渲染、tooltip、明细按钮、分页。
- 存储分析弹窗：热力图、日期聚合、按日日志分页。
- 金额 / Token / Payload 详情弹窗。
- 定价快照、tooltip 定位、格式化函数、大量 computed/watch。

现在继续往里堆功能，后面维护的人纯属给自己找罪受。

---

## 2. 迁移目标

- 让 `Index.vue` 退化成页面壳子和数据编排入口。
- 把筛选、统计、图表、表格、存储弹窗、详情弹窗彻底拆开。
- 把 tooltip、热力图、自动刷新、分页查询等状态逻辑移到 composable。
- 把格式化与纯计算函数沉淀到 `utils.ts` / `constants.ts`。

目标状态：后续再加一个“新统计卡片”或者“新明细弹窗”，不需要再去翻 5000 多行的祖传大锅。

---

## 3. 推荐目录结构

```text
frontend/src/components/Logs/
  Index.vue
  Index.css
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
    useLogsAutoRefresh.ts
    useLogsChartsPresentation.ts
    useLogsCostTooltip.ts
    useLogsDetailModals.ts
    useLogsFilters.ts
    useLogsInfoTooltip.ts
    useLogsPageData.ts
    useLogsPayloadDetail.ts
    useLogsPricingDetails.ts
    useLogsStorageHeatmap.ts
    useLogsStorageModalController.ts
  constants.ts
  types.ts
  utils.ts
```

---

## 4. 建议拆分边界

### 4.1 页面壳子保留在 `Index.vue`

保留内容：

- 页面级装配。
- 路由跳转。
- 顶层请求触发。
- 子组件之间的状态协调。

不要再保留：

- 复杂 tooltip 定位细节。
- 热力图明细加载细节。
- 详情弹窗内部展示逻辑。
- 大量格式化辅助函数。

### 4.2 展示组件拆分建议

- `LogsHeaderBar.vue`：返回按钮、刷新按钮、倒计时、存储按钮。
- `LogsFilterBar.vue`：筛选表单与日期选择器。
- `LogsSummaryCards.vue`：统计卡片列表与点击事件抛出。
- `LogsChartsPanel.vue`：所有图表容器和图表 props 接收。
- `LogsTable.vue`：日志表格、列展示、行内操作入口。
- `LogsPagination.vue`：页码、总数、切换事件。

### 4.3 弹窗拆分建议

- `LogsStorageModal.vue`：存储统计、热力图、按日详情。
- `LogsCostDetailModal.vue`：金额来源和价格明细。
- `LogsTokenDetailModal.vue`：Token 分类明细。
- `LogsPayloadDetailModal.vue`：请求体 / 返回体预览。

### 4.4 composable 拆分建议

- `useLogsPageData.ts`：页面数据请求、Provider 选项加载、分页与 dashboard 刷新。
- `useLogsFilters.ts`：筛选状态、日期联动、查询范围归一化与汇总提示。
- `useLogsAutoRefresh.ts`：倒计时、轮询、手动刷新收口。
- `useLogsStorageHeatmap.ts`：热力图数据、日期选择、按日明细分页与请求竞态控制。
- `useLogsStorageModalController.ts`：存储弹窗开关、清理确认、热力图 controller 与 modal formatters / handlers 装配。
- `useLogsInfoTooltip.ts` / `useLogsCostTooltip.ts`：模型 / 校验 / 金额 tooltip 的显隐、延时、定位与异步补全。
- `useLogsDetailModals.ts`：汇总卡片点击、金额 / Token 明细弹窗控制。
- `useLogsPayloadDetail.ts`：Payload 详情加载、复制与请求竞态收口。
- `useLogsChartsPresentation.ts`：`statsCards`、model share、趋势图配置与 `logsTableFormatters` 装配。
- `useLogsPricingDetails.ts`：模型定价快照加载、tooltip detail builder 与 price source 细节装配。

---

## 5. 分阶段迁移步骤

### Phase 1：抽页面区块

状态：`已完成（2026-03-08）`

- 先抽 `HeaderBar / FilterBar / SummaryCards / ChartsPanel / Table`。
- 暂时允许顶层 `Index.vue` 继续持有状态，只做模板瘦身。
- 这一阶段不先碰接口逻辑，先把模板边界拉开。

### Phase 2：抽弹窗

状态：`已完成（2026-03-08）`

- 把存储、金额、Token、Payload 四类弹窗拆出去。
- 由根文件通过 `props + emits` 控制显隐和数据传递。
- 顺手把弹窗内部局部样式迁走。

### Phase 3：抽状态逻辑

状态：`已完成（2026-03-08）`

- [x] 新增 `useLogsAutoRefresh.ts`，收口倒计时、轮询、手动刷新。
- [x] 新增 `useLogsStorageHeatmap.ts`，收口热力图日期选择、按日明细分页、tooltip 定位与请求竞态。
- [x] 新增 `useLogsInfoTooltip.ts`、`useLogsCostTooltip.ts`，收口表格 tooltip 的显隐、延时、定位与异步补全。
- [x] 新增 `useLogsFilters.ts`，收口筛选状态、日期联动、查询范围归一化与汇总提示。
- [x] 新增 `useLogsPageData.ts`，收口页面数据请求、Provider 选项加载、分页与顶层 dashboard 刷新。

### Phase 4：抽纯工具

状态：`已完成（2026-03-08）`

- [x] 新增 `constants.ts`，收口 `PER_MILLION_TOKENS`、`COST_TOOLTIP_DIFF_EPSILON`、`TOKENS_PER_SECOND_MIN_WINDOW_SEC`。
- [x] 新增并补齐 `utils.ts`，收口时间、流式、token、currency、pricing、cache create 等纯工具函数。
- [x] 第二批继续收口 `normalizeModelShareKey`、`buildAlphaColor`、`resolveModelVerifyStatus`、`resolvePriceSource`、`resolveGroupMultiplier`、`mergeCostTooltipNotes` 等低风险纯 helper。
- [x] 第三批继续收口 `MODEL_SHARE_COLORS`、`buildModelShareRows`、`buildLineAreaGradient`、`formatSeriesLabel`、`resolveChartLegendColor`、`resolveChartTickColor`、`formatNumber`、`intensityClass` 等图表 / model share 纯工具。
- [x] 第四批继续收口 `buildModelPricingLookup`、`resolvePricingRow` 等定价匹配纯逻辑，缩小 `Index.vue` 的价格索引与匹配职责。
- [x] 第五批继续收口 `CacheCreateTier`、`buildCacheCreateCostDetails`、`buildTokenRatePriceLines`、`buildObservedCostPriceLines`、`buildProviderApiPerCallPriceLines` 等 cache / cost 纯 helper，并在页面内保留翻译文案薄包装。
- [x] 第六批继续收口 `buildProviderApiTokenPricingContext`、`buildBuiltinTokenPricingContext` 等 pricing context 纯计算，把 Provider API / builtin tooltip 里的数字归一化与费率推导从页面层剥离。
- [x] 第七批继续收口 `buildLogsCostTooltipLabels`、`buildProviderApiTokenFormula`、`buildBuiltinTokenFormula` 等 tooltip labels / 公式 builder，把页面内 `t()` 文案装配和公式拼装进一步下沉到 `utils.ts`，`Index.vue` 已压到 `1957` 行。
- [x] 第八批继续收口 `buildLogsInfoTooltipLabels`、`resolveTooltipModelDisplayValue`、`buildModelInfoTooltipDetailData`、`buildVerifyInfoTooltipDetailData` 等 info tooltip builder，把 model / verify tooltip 的缺省值解析、price source 显示与 rows 装配继续下沉到 `utils.ts`，`Index.vue` 已压到 `1866` 行。
- [x] 第九批评估后确认 `1000` 行附近这组轻量文案包装虽然单个函数很薄，但它们共同挂在 `logsTableFormatters / storageModalFormatters` 契约面上，适合成组收口为 `buildLogsTableTextFormatters`，因此继续下沉到 `utils.ts`，`Index.vue` 已压到 `1843` 行。
- [x] `Index.vue` 已切换为从 `constants.ts / utils.ts` 导入，页面内仅保留页面级 orchestrator 与少量高耦合 tooltip 细节装配，避免翻译文本和公式逻辑继续堆在根文件。
- [x] `useLogsFilters.ts`、`useLogsStorageHeatmap.ts` 已复用公共日期工具，避免重复实现。
- [x] 最终收口新增 `useLogsDetailModals.ts`、`useLogsPayloadDetail.ts`、`useLogsChartsPresentation.ts`、`useLogsPricingDetails.ts`、`useLogsStorageModalController.ts`，并把根组件样式整体迁到 `Index.css`，模板结构与 class 命名保持不变。
- [x] `Index.vue` 已由 `1843` 行收敛到 `508` 行，符合“页面壳子 + 顶层 orchestrator”目标。
- [x] 评估 `statsCards`、`storageModalFormatters`、`logsTableFormatters` 后，确认不再继续做额外工厂化：前两者已分别沉到 presentation / storage controller，`logsTableHandlers` 保留在页面层作为顶层接线，能避免过度抽象造成理解成本反弹。
- [x] 最终执行 `cd frontend && npx vue-tsc --noEmit` 通过。

---

## 6. 数据边界建议

`LogsTable.vue` 建议只吃渲染所需数据和回调：

- `items`
- `loading`
- `onOpenPayload`
- `onOpenCostDetail`
- `onHoverModelInfo`
- `onPageChange`

`LogsFilterBar.vue` 建议只输出标准化查询参数，不直接在内部调用接口。

这点很关键，别拆成组件以后又偷偷让每个组件自己发请求，最后状态比现在还乱。

---

## 7. 关键风险

- 日期筛选器联动复杂，拆分后容易出现同步不一致。
- tooltip 依赖 DOM 和鼠标事件，拆分时容易丢定位。
- 存储热力图和按日分页有请求竞态，要保留 requestId 防抖思路。
- 卡片点击和详情弹窗之间存在隐式耦合，拆分时要把事件名字讲清楚。

---

## 8. 手工回归清单

- 能正常返回首页。
- 手动刷新与自动刷新倒计时正常。
- 平台 / Provider / 日期筛选组合查询正常。
- 汇总卡片点击能打开对应详情。
- 图表显示正常，无空白或 tooltip 错位。
- 表格分页、hover、payload 详情正常。
- 存储弹窗打开、切换粒度、选日期、翻页正常。
- 清理日志、清理统计类操作仍按预期执行。

---

## 9. 完成定义

- [x] `Index.vue` 控制在 `600` 行左右（当前 `508` 行）。
- [x] 四类弹窗均已独立。
- [x] 热力图、tooltip、自动刷新逻辑已从根文件迁出。
- [x] 后续新增日志展示需求时，不需要再改 `3` 个以上无关区域。

---

## 10. 最终落地结果

- `Index.vue`：只保留页面壳子、路由返回、生命周期、顶层 tooltip 接线与页面级 orchestrator。
- `Index.css`：承接原根组件样式，拆分后界面结构、class 命名与视觉表现保持一致。
- `components/`：页面头部、筛选区、汇总卡片、图表区、表格、分页均已独立。
- `modals/`：存储、金额、Token、Payload 四类弹窗均已独立。
- `composables/`：自动刷新、筛选、页面数据、tooltip、图表展示、定价明细、Payload 明细、存储弹窗 controller 等状态逻辑均已拆出。
- 验证结果：`cd frontend && npx vue-tsc --noEmit` 已通过。

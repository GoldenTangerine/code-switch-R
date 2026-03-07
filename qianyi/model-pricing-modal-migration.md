# `ModelPricingModal.vue` 详细迁移文档

- 源文件：`frontend/src/components/Setting/ModelPricingModal.vue`
- 当前行数：`793`
- 组件类型：设置页业务弹窗
- 优先级：`P1`

---

## 1. 当前问题画像

这文件虽然不到千行，但业务密度高，已经有明显拆分价值：

- 价格表列表展示。
- 搜索、过滤、source badge。
- 创建 / 编辑模型价格。
- Claude 官方价格预览。
- Claude 官方价格同步。
- 同步菜单开关与全局点击关闭。
- 打开弹窗时刷新、关闭时重置状态。

继续在这儿堆新供应商价格逻辑，迟早也得长残。

---

## 2. 迁移目标

- 将列表、工具栏、同步菜单、Claude 预览拆分。
- 将同步流程、列表加载、搜索过滤提到 composable。
- 保留现有业务能力和打开即刷新的体验。

---

## 3. 推荐目录结构

```text
frontend/src/components/Setting/model-pricing/
  ModelPricingModal.vue
  ModelPricingToolbar.vue
  ModelPricingTable.vue
  ModelPricingSyncMenu.vue
  composables/
    useModelPricingRows.ts
    useModelPricingSync.ts
  utils.ts
  types.ts
```

如果暂时不挪目录，也建议先按上面的角色拆子组件。

---

## 4. 拆分建议

- `ModelPricingToolbar.vue`：新增、刷新、搜索、同步入口。
- `ModelPricingTable.vue`：行展示、badge、tooltip、编辑事件。
- `ModelPricingSyncMenu.vue`：Claude 预览、同步菜单、点击外部关闭。
- `useModelPricingRows.ts`：列表加载、搜索、过滤、刷新。
- `useModelPricingSync.ts`：同步任务、预览请求、toast 汇总、竞争控制。
- `utils.ts`：source badge、tooltip 时间格式化。

---

## 5. 分阶段迁移步骤

### Phase 1
- 抽 toolbar 和 table。

### Phase 2
- 抽 sync menu 与 Claude preview 交互。

### Phase 3
- 抽 rows / sync composable。

---

## 6. 高风险点

- 打开 modal 自动刷新是关键体验，拆分后不能丢。
- sync 菜单依赖全局 pointerdown，事件解绑要干净。
- 预览与正式同步共享 loading 状态，拆的时候要避免并发污染。

---

## 7. 回归清单

- 打开弹窗自动刷新正常。
- 搜索、过滤、列表显示正常。
- 新增 / 编辑模型价格正常。
- Claude 预览、Claude 同步正常。
- 点击外部关闭菜单正常。

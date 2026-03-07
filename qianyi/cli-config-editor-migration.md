# `CLIConfigEditor.vue` 详细迁移文档

- 源文件：`frontend/src/components/common/CLIConfigEditor.vue`
- 当前行数：`1854`
- 组件类型：共享大组件
- 优先级：`P1`

---

## 1. 当前问题画像

这玩意儿已经不是普通表单组件了，里头同时揉了：

- 折叠面板头部。
- 锁定字段展示。
- 可编辑字段展示。
- 自定义字段管理。
- 智能粘贴解析（JSON / TOML / ENV）。
- Preview / Current 双视图编辑。
- 配置快照加载、磁盘内容同步、保存竞态防御。
- 预览与当前配置的不同编辑模式切换。

这种共享大组件最怕继续“顺手再加一个功能”，加着加着就成公共泥潭。

---

## 2. 迁移目标

- 把 UI 分区和编辑流程拆开。
- 把智能粘贴、字段管理、快照编辑流程模块化。
- 保持父组件使用方式尽量不变，避免牵一大片。

---

## 3. 推荐目录结构

```text
frontend/src/components/common/cli-config-editor/
  CLIConfigEditor.vue
  CLIConfigHeader.vue
  CLIConfigLockedFields.vue
  CLIConfigEditableFields.vue
  CLIConfigCustomFields.vue
  CLIConfigPreviewTabs.vue
  composables/
    useCliConfigFields.ts
    useCliConfigCustomFields.ts
    useCliConfigSmartPaste.ts
    useCliConfigSnapshots.ts
  utils.ts
  types.ts
```

如果不想挪目录，也至少要先在 `common/` 下加子组件与 composable，别继续把所有逻辑塞在一个文件里。

---

## 4. 拆分建议

- `CLIConfigHeader.vue`：折叠头、平台 badge、恢复默认入口。
- `CLIConfigLockedFields.vue`：锁定字段展示。
- `CLIConfigEditableFields.vue`：可编辑字段区。
- `CLIConfigCustomFields.vue`：自定义字段增删改。
- `CLIConfigPreviewTabs.vue`：Preview / Current 两套视图与保存入口。
- `useCliConfigFields.ts`：字段值读取、更新、对象 JSON 更新。
- `useCliConfigCustomFields.ts`：自定义字段 id、顺序、草稿行保持。
- `useCliConfigSmartPaste.ts`：JSON / TOML / ENV 智能解析。
- `useCliConfigSnapshots.ts`：快照拉取、预览编辑、当前编辑、保存防重入。

---

## 5. 分阶段迁移步骤

### Phase 1
- 抽 header、locked、editable、custom 四个可见区块。

### Phase 2
- 抽 Preview / Current 视图与保存流程。

### Phase 3
- 抽 smart paste 和 custom fields 状态逻辑。

### Phase 4
- 收口共享类型与工具函数。

---

## 6. 高风险点

- 自定义字段草稿行不能在 watch 同步里被吞掉，这块原本就挺绕。
- Preview / Current 的保存语义不一样，拆分后不能混。
- 平台切换时的竞态校验必须保留，否则旧结果会覆盖新界面。
- 智能粘贴不能误伤普通输入框粘贴。

---

## 7. 回归清单

- 折叠 / 展开正常。
- 锁定字段、可编辑字段显示正常。
- 自定义字段新增、改 key、删 key 正常。
- 智能粘贴 JSON / TOML / ENV 正常。
- Preview 保存、Current 保存、恢复默认正常。
- 平台切换后状态不串。

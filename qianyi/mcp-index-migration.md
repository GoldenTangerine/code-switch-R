# `Mcp/index.vue` 详细迁移文档

- 源文件：`frontend/src/components/Mcp/index.vue`
- 当前行数：`1336`
- 当前路由：`/mcp`
- 优先级：`P1`

---

## 1. 当前问题画像

MCP 页面已经出现典型膨胀信号：

- Server 列表展示。
- 新增 / 编辑 modal。
- JSON 表单编辑模式。
- 批量导入弹窗。
- 全屏面板。
- 图标选择、平台选择、占位符校验、dirty 判定。
- Session ID 防竞态。

现在还没膨胀到 `Logs` 那种离谱程度，但再放着不管，很快也会肥成球。

---

## 2. 迁移目标

- 把 MCP 列表页、表单页、JSON 编辑器、批量导入分区清晰化。
- 保留现有交互体验，但把 modal 内的复杂状态迁走。
- 让后续再加新的 server 类型时，不必直接改 1000 多行页面。

---

## 3. 推荐目录结构

```text
frontend/src/components/Mcp/
  index.vue
  components/
    McpHeaderBar.vue
    McpServerList.vue
    McpServerCard.vue
    McpFormBasicFields.vue
    McpJsonEditorPanel.vue
  modals/
    McpServerModal.vue
    BatchImportModal.vue
  composables/
    useMcpServers.ts
    useMcpServerForm.ts
    useMcpJsonEditor.ts
    useMcpModalSession.ts
  types.ts
  constants.ts
```

---

## 4. 拆分建议

- `McpServerList.vue`：负责服务列表渲染和列表操作入口。
- `McpServerModal.vue`：统一包裹新增 / 编辑表单。
- `McpFormBasicFields.vue`：基础表单字段。
- `McpJsonEditorPanel.vue`：JSON 配置编辑器、锁定 / 解锁、dirty 判断。
- `useMcpModalSession.ts`：管理 sessionId，防止 modal 切换时旧异步回调回灌。
- `useMcpServerForm.ts`：平台校验、占位符检测、表单提交前归一化。

---

## 5. 分阶段迁移步骤

### Phase 1
- 抽列表区与 modal 容器。

### Phase 2
- 抽 JSON 编辑器与基础字段区。

### Phase 3
- 抽 session 与校验 composable。

### Phase 4
- 收口样式与文案依赖。

---

## 6. 高风险点

- JSON 编辑器 dirty 判定容易因为格式化差异误判。
- 异步回调竞态不能丢，`sessionId` 机制必须保留。
- 平台勾选和占位符校验属于保存前关键规则，不能漏。

---

## 7. 回归清单

- MCP 列表加载正常。
- 新增 / 编辑 / 删除 server 正常。
- JSON 模式锁定 / 解锁 / 应用 / 回滚正常。
- 批量导入弹窗正常。
- 图标、平台、website 等字段保存正常。

# 超大文件迁移文档索引

> 目录用途：集中存放前端超大文件的单文件迁移方案，后续拆分时按文件逐个推进，避免多人同时乱改把项目掰折了。
> 创建时间：2026-03-07

---

## 1. 适用范围

本目录覆盖当前已识别的超大文件，包括：

- 页面入口文件：`Logs`、`Main`、`General`、`Mcp`、`Skill`、`Console`
- 共享大组件：`CLIConfigEditor`、`ModelPricingModal`

---

## 2. 迁移文档清单

| 优先级 | 源文件 | 行数 | 迁移文档 |
|---|---|---:|---|
| P0 | `frontend/src/components/Logs/Index.vue` | 5828 | `qianyi/logs-index-migration.md` |
| P0 | `frontend/src/components/Main/Index.vue` | 5047 | `qianyi/main-index-migration.md` |
| P0 | `frontend/src/components/General/Index.vue` | 3116 | `qianyi/general-index-migration.md` |
| P1 | `frontend/src/components/Mcp/index.vue` | 1336 | `qianyi/mcp-index-migration.md` |
| P1 | `frontend/src/components/Skill/Index.vue` | 1090 | `qianyi/skill-index-migration.md` |
| P1 | `frontend/src/components/Console/Index.vue` | 947 | `qianyi/console-index-migration.md` |
| P1 | `frontend/src/components/common/CLIConfigEditor.vue` | 1854 | `qianyi/cli-config-editor-migration.md` |
| P1 | `frontend/src/components/Setting/ModelPricingModal.vue` | 793 | `qianyi/model-pricing-modal-migration.md` |

---

## 3. 统一迁移规则

后续所有超大文件都按同一套规矩处理，别今天一个思路、明天一个套路，最后谁看都脑瓜子嗡嗡的：

1. 保持原路由和原功能入口不变。
2. 优先拆分 `展示区块`，再拆 `状态编排`，最后拆 `utils / types / constants`。
3. 页面根文件 `Index.vue` 只保留页面壳子、路由跳转、顶层 orchestrator。
4. 弹窗、表格、图表、表单编辑器优先独立成组件，不继续挂在根文件底下当拖油瓶。
5. `watch / timer / polling / tooltip / modal session` 这类状态逻辑优先外提到 `composables/`。
6. 两个页面以上稳定复用之后，再考虑上提到 `common/`，别一上来就过度抽象。

---

## 4. 推荐推进顺序

### 第一批

- `Logs`
- `Main`
- `General`

### 第二批

- `Mcp`
- `Skill`
- `Console`
- `CLIConfigEditor`
- `ModelPricingModal`

---

## 5. 建议 PR 切分方式

每个超大文件建议拆成 2 ~ 4 个小 PR：

- PR 1：搭目录骨架 + 提取纯展示组件。
- PR 2：提取 composable 和工具函数。
- PR 3：样式搬迁与回归修正。
- PR 4：若需要，再处理共享抽象与复用收口。

别一口气上来 2000 行 diff，那玩意儿审起来跟考古差不多。

---

## 6. 阅读方式

如果你准备实际开拆，建议顺序：

1. 先看当前文件对应的迁移文档。
2. 按文档里的“拆分阶段”和“回归清单”执行。
3. 每完成一个阶段就做一次最小手工回归。
4. 不要跨文件混拆，先把一个文件拆干净再碰下一个。

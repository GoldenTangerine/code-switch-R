# `Skill/Index.vue` 详细迁移文档

- 源文件：`frontend/src/components/Skill/Index.vue`
- 当前行数：`1090`
- 当前路由：`/skill`
- 优先级：`P1`

---

## 1. 当前问题画像

这个页面的问题不在于纯模板复杂，而在于它把：

- Platform Tabs。
- Installed / Available skills 分组。
- 安装位置选择弹窗。
- 仓库输入弹窗。
- 启用 / 禁用 / 卸载 / 打开目录 / 打开 GitHub。
- Skills 聚合与去重逻辑。

全部塞在了一个文件里，后续再扩功能，复杂度会继续抬头。

---

## 2. 迁移目标

- 把 tabs、分组列表、弹窗和安装逻辑拆开。
- 保持 `SkillCard.vue` 作为现有单卡组件继续复用。
- 把“数据获取 / 状态变更 / 仓库解析”从页面主体里迁出。

---

## 3. 推荐目录结构

```text
frontend/src/components/Skill/
  Index.vue
  SkillCard.vue
  components/
    SkillPlatformTabs.vue
    SkillInstalledGroups.vue
    SkillAvailableList.vue
  modals/
    SkillInstallLocationModal.vue
    SkillRepositoryModal.vue
  composables/
    useSkillCatalog.ts
    useSkillInstall.ts
    useSkillRepository.ts
  types.ts
```

---

## 4. 拆分建议

- `SkillPlatformTabs.vue`：平台切换。
- `SkillInstalledGroups.vue`：Project / User 分组展示。
- `SkillAvailableList.vue`：可安装技能区。
- `SkillInstallLocationModal.vue`：安装位置确认。
- `SkillRepositoryModal.vue`：仓库录入和解析。
- `useSkillCatalog.ts`：当前平台技能拉取、分组、去重。
- `useSkillInstall.ts`：安装、卸载、切换启用状态。
- `useSkillRepository.ts`：GitHub 地址解析、校验、外链打开。

---

## 5. 分阶段迁移步骤

### Phase 1
- 抽 tabs 和两大列表区。

### Phase 2
- 抽两个 modal。

### Phase 3
- 抽 catalog / install / repository 三个 composable。

---

## 6. 高风险点

- Installed 与 available skills 合并时，优先级规则不能变。
- 不同平台的 install_location 判定要保持原逻辑。
- GitHub repo 解析要兼容 URL 和 owner/name 两种输入。

---

## 7. 回归清单

- 平台切换正常。
- Installed / Available 分组数量正常。
- 安装、卸载、启用 / 禁用正常。
- 打开目录、打开 GitHub 正常。
- 仓库输入 modal 正常。

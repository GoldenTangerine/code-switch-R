<!--
@name: 供应商编辑保存丢失修复
@Descripttion: 记录编辑期间列表重新加载导致供应商修改静默丢失的原因与修复。
@version: 1.0.0
@Author: sm
@Date: 2026-10-20 17:30:29
@LastEditTime: 2026-10-20 17:30:29
@FilePath: doc/changes/2026-10-20-1730-provider-edit-stale-card.md
-->
# 供应商编辑保存丢失修复

变更时间：2026-10-20 17:30:29（Asia/Shanghai）

涉及范围：`frontend/src/components/Main/composables/useProviderForm.ts`、对应单元测试与中英文文案。

## 问题原因

编辑弹窗打开时持有卡片对象引用。编辑期间窗口重新可见会触发 `refreshAllData()` → `loadProviders()`，卡片列表被整体替换为新对象，弹窗仍引用旧对象。保存时修改写入已脱离列表的旧对象，而 `persistProviders` 序列化的是列表中的新对象，导致修改未落盘且无任何提示。用户侧表现为配置自定义额度查询脚本并保存后，首页与再次编辑均显示未配置。

## 变更内容

- 保存供应商时按 `providerRef`（无则按 `id`）在当前列表中重新定位编辑中的卡片；卡片已不存在时提示并保持弹窗打开。
- 模型映射开关、隐藏日志徽标开关采用同样的定位方式，失败时回滚开关状态。
- `persistProviders` 返回 `false`（同列表正在保存、自定义 CLI 未选择工具）时回滚内存修改并保留弹窗，不再关闭后静默丢弃。
- 新增 `editingProviderMissing` 中英文文案。

## 验证结果

- 新增 5 个回归用例覆盖列表重新加载后保存额度配置、卡片已删除、保存被跳过、模型映射开关、日志徽标开关；其中前 4 个在修复前全部失败。
- 前端单元测试 75 个文件、587 个用例全部通过，`vue-tsc --noEmit` 通过。

## 发布

- 随 Code Switch R v2.11.23 发布；同步应用版本、平台构建元数据与更新日志。

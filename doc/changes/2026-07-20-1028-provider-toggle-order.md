/**
 * @name: 首页供应商切换排序调整
 * @Descripttion: 记录首页供应商启用状态切换后的组内排序调整
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-07-20 10:28:58
 * @LastEditTime: 2026-07-20 10:28:58
 * @FilePath: doc/changes/2026-07-20-1028-provider-toggle-order.md
 */

# 首页供应商切换排序调整

## 变更时间

2026-07-20 10:28:58 CST（Asia/Shanghai）

## 涉及范围

- 首页全部供应商页签的启用状态切换、排序持久化与编辑失败回滚。

## 变更内容

- 已有供应商关闭后移动到未启用组首位，连续关闭时最近关闭的供应商优先显示。
- 未启用供应商重新启用后移动到启用组首位。
- 新增供应商、手动拖拽及组内排序持久化规则保持不变。
- 编辑弹窗切换启用状态后若保存失败，恢复完整供应商列表及弹窗卡片引用，避免业务置顶规则破坏原排序。
- 排序字段注释统一为“持久化组内顺序”，与实际行为保持一致。

## 验证结果

- `pnpm exec vitest run src/components/Main/utils/providerOrder.test.ts src/components/Main/composables/useProviderForm.test.ts`：2 个测试文件、21 项测试全部通过。
- `pnpm test:unit`：51 个测试文件、366 项测试全部通过。
- `pnpm exec vue-tsc --noEmit`：通过。
- `git diff --check`：通过。
- 按项目规范未执行 lint、build、package 或 dev。

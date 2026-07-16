/**
 * @name: 模型映射位置调整
 * @Descripttion: 记录首页供应商编辑中模型映射与预算额度的顺序调整
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-07-16 15:46:40
 * @LastEditTime: 2026-07-16 15:46:40
 * @FilePath: doc/changes/2026-07-16-1546-model-mapping-order.md
 */

# 模型映射位置调整

## 变更时间

2026-07-16 15:46:40 CST（Asia/Shanghai）

## 涉及范围

- 首页供应商编辑弹窗

## 变更内容

- 将模型映射移动到预算额度正上方。
- 模型白名单保持原有位置，模型映射的显隐、交互及保存逻辑保持不变。

## 验证结果

- `pnpm exec vue-tsc --noEmit`：通过。
- `pnpm test:unit`：49 个测试文件、343 项测试全部通过。
- `git diff --check`：通过。

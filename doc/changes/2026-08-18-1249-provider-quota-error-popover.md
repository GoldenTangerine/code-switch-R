# 供应商余额查询失败悬浮详情

- 变更名称：provider-quota-error-popover
- 变更时间：2026-08-18 12:49:15（Asia/Shanghai）
- 涉及范围：首页供应商卡片、供应商余额查询缓存、前端国际化

## 变更内容

- 余额查询失败时清除历史余额缓存，卡片第一层仅显示错误图标和刷新按钮。
- 错误详情改为错误图标悬浮窗，支持延迟打开、点击锁定、触控点击、Esc/外部关闭。
- 双击错误详情可复制本次查询的完整错误信息。
- 错误图标改为主题兼容的圆形感叹号 SVG，补充中英文复制反馈文案。
- 卡片统计状态或额度行布局变化时自动重算悬浮窗位置。
- 增加悬浮交互、复制 payload、失败后恢复成功的回归测试。

## 验证结果

- `pnpm test:unit --run src/components/Main/components/ProviderCard.test.ts src/components/Main/composables/useProviderQuotas.test.ts`：通过。
- `pnpm test:unit --run src/components/Main/utils/providerQuotaErrorInteraction.test.ts`：通过。
- `pnpm test:unit`：65 个测试文件、497 个测试通过。
- `pnpm exec vue-tsc --noEmit`：通过。

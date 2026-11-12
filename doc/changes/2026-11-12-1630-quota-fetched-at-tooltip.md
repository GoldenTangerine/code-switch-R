<!--
@name: 额度悬停提示显示获取时间
@Descripttion: 记录首页额度进度条悬停提示新增数据获取时间的实现与验证。
@version: 1.0.0
@Author: sm
@Date: 2026-11-12 16:30:11
@LastEditTime: 2026-11-12 16:30:11
@FilePath: doc/changes/2026-11-12-1630-quota-fetched-at-tooltip.md
-->

# 额度悬停提示显示获取时间

- 变更时间：2026-11-12 16:30:11（Asia/Shanghai，UTC+08:00）。
- 涉及范围：首页供应商卡片额度进度条的悬停提示、额度展示数据映射。

## 问题

- 额度进度条只显示用量与重置倒计时，看不出读数是多久前拿到的；被动采样额度在长时间无请求时可能已过时。

## 变更内容

- `ProviderQuotaDisplayItem` 新增 `sampledAt`，`resolveProviderQuotaQueryDisplay` 透传后端返回的被动采样时间。
- `ProviderCard.vue` 的 `quotaTooltip` 在进度类额度提示中追加相对时间，复用 `formatProviderQuotaRelativeUpdatedAt`，不新增文案。
- 时间来源：被动采样项优先使用 `sampledAt`，其余使用 `queriedAt`；被动采样的 `queriedAt` 每次轮询都会刷新，不能代表数据实际获取时间。
- 原有 30 秒计时器改为在存在进度类额度时也启动，提示内容随之刷新；卸载时沿用已有清理逻辑。
- 余额与查询失败面板已直接显示更新时间，悬停提示不再重复追加。

## 验证结果

- `pnpm test:unit` 76 个文件、602 项测试通过；`vue-tsc --noEmit` 无错误。
- 未运行应用构建、打包或开发启动。

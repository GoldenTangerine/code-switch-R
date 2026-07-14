/**
 * @name: Claude 模型路由变更记录
 * @Descripttion: 记录 Claude 多供应商模型路由与模型聚合功能改造
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-07-14 15:03:35
 * @LastEditTime: 2026-07-14 15:03:35
 * @FilePath: doc/changes/2026-07-14-1503-claude-model-routing.md
 */

# Claude 多供应商模型路由与聚合

## 变更时间

2026-07-14 15:03:35 CST（Asia/Shanghai）

## 涉及范围

- Claude Provider 配置与模型映射
- Claude 请求路由与 `/v1/models`
- 供应商模型列表缓存与刷新
- 通用设置与供应商编辑界面
- 模型定价元数据

## 变更内容

- 新增默认关闭的 Claude 按模型路由和模型聚合开关。
- 新增激进/保守元数据合并策略及手动刷新状态。
- 新增供应商模型放过规则，支持精确、前缀和关键字通配。
- 新增持久化供应商模型缓存、24 小时刷新和失败保留机制。
- Claude 请求可按请求模型、实际模型、Level 和供应商排序严格路由。
- 聚合 `/v1/models` 返回去重、排序和分页后的请求模型列表。
- 新增旧模型配置自动迁移和完整设计文档。
- 修复供应商连续保存、连接刷新失败和旧刷新迟到导致的缓存与路由状态回退。
- 统一通配符映射优先级，空放过规则默认覆盖全部已验证模型。
- 模型库变更后立即重建路由，批量刷新统一持久化缓存和重建索引。
- 路由开启后无需开启模型聚合即可手动刷新供应商模型。
- 发布版本推进到 `v2.8.93`，同步 Wails 与各平台构建元数据和发布说明。

## 验证结果

- `go test ./services/...`：通过。
- `go test -race ./services/...`：通过。
- `go test ./resources/model-pricing/...`：通过。
- `pnpm test:unit`：44 个测试文件、314 项测试全部通过。
- `pnpm exec vue-tsc --noEmit`：通过。
- `git diff --check`：通过。
- `wails3 task common:update:build-assets`：构建元数据同步完成。
- 根包 `go test .` 已完成编译，但既有 `TestSeedMockRequestLogs` 依赖预先初始化的 `request_log` 数据库表，在隔离 HOME 下失败；与本次改动无关。

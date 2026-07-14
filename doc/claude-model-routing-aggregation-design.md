/**
 * @name: Claude 多供应商模型路由与聚合设计
 * @Descripttion: 记录 Claude Code 按模型路由、模型聚合和缓存刷新方案
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-07-14 15:03:35
 * @LastEditTime: 2026-07-14 15:03:35
 * @FilePath: doc/claude-model-routing-aggregation-design.md
 */

# Claude 多供应商模型路由与聚合设计

## 1. 目标

Claude Code 可以为计划与执行请求不同模型。本功能根据请求模型、供应商映射和供应商实际模型列表，将每次请求路由到兼容供应商，并允许 `/v1/models` 返回全部可路由请求模型。

核心约束：

- 仅作用于 Claude Code；Codex、Gemini 和自定义 CLI 保持原逻辑。
- 不自动跨模型降级，只在支持同一请求模型的供应商之间切换。
- 供应商选择顺序保持 `Level → 同 Level 页面排序`。
- 模型映射是显式授权，允许用户主动配置跨版本映射，但界面应提示风险。

## 2. 设置与兼容迁移

应用设置新增：

| 字段 | 默认值 | 说明 |
|---|---|---|
| `claude_model_routing_enabled` | `false` | 开启严格按模型选择供应商 |
| `claude_model_aggregation_enabled` | `false` | 聚合 Claude `/v1/models`，依赖模型路由 |
| `claude_model_metadata_merge_strategy` | `aggressive` | `aggressive` 或 `conservative` |

新安装默认关闭。旧配置没有路由字段，但任一 Claude 供应商已配置 `supportedModels` 或 `modelMapping` 时，自动开启模型路由，避免升级后原行为失效；模型聚合不自动开启。

关闭模型路由时：

- 不使用白名单、映射未命中策略或放过规则筛选供应商。
- 仍在选中供应商上应用已命中的 `modelMapping` 和 `requestBodyOverrides`。
- 映射未命中时原模型透传。

开启模型路由时：

- 请求模型必须存在于路由索引或命中显式放过规则。
- 未知模型返回当前兼容的 `404` 不支持模型响应。
- 请求未携带 `model` 时沿用旧的 Level/排序路由。

## 3. 供应商模型规则

`Provider` 新增 `modelPassthroughPatterns`：

- 仅在模型路由开启且 `modelMappingMissPolicy=passthrough` 时生效。
- 区分大小写。
- 支持精确、前缀和关键字通配，例如 `glm-4`、`glm-*`、`*glm*`。
- 没有显式规则时等价于 `*`，放过全部已由远端模型列表、白名单或映射证明支持的模型；未知模型仍不放过。
- `modelMapping` 继续只支持一个 `*`，保证正向替换和反向展开唯一。

最终模型处理顺序：

```text
请求 model
  → modelMapping
  → requestBodyOverrides.model
  → 供应商实际模型校验
  → Level / 排序选择
```

## 4. 模型来源与路由索引

供应商模型优先复用首页模型列表获取链路：

```text
/api/pricing
  → One-Hub /api/available_model
  → /v1/models
  → supportedModels 与有效本地模型库
```

合并规则：

- 远端模型与 `supportedModels` 同时存在时取交集。
- 白名单为空时使用远端模型。
- 远端不可用时回退白名单、映射目标和有效模型库。
- 精确映射暴露映射左侧请求模型。
- 单通配符映射优先从远端实际模型反向展开，再做正向一致性校验。
- 有映射时聚合列表只暴露映射左侧；没有映射时暴露可验证的直接模型。
- 同一请求模型按 ID 去重，但保留所有兼容供应商作为同模型降级链。

示例：

```text
A / Level 1：claude-4-5 → vendor-a/claude-4-5
B / Level 1：claude-5   → vendor-b/claude-5
C / Level 2：claude-5   → vendor-c/claude-5

Plan 请求 claude-5：B → C
执行请求 claude-4-5：A
```

## 5. 缓存与刷新

缓存文件：`~/.code-switch/claude-model-routing-cache.json`。

- 使用版本化 JSON 和原子写入。
- 保存规范化模型、价格、能力、来源、分组和时间等字段。
- 配置指纹包含 API Key 的 SHA-256 摘要，不保存 API Key、认证头、原始 HTTP 响应或调试记录。
- 缓存有效期 24 小时，过期期间继续服务旧数据并异步刷新。
- 刷新失败 30 分钟后重试；同一配置有历史结果时不清空模型，连接配置变化后不复用旧结果。
- 批量刷新并发上限 4，单请求沿用 20 秒超时。
- 单供应商最多缓存 5,000 个模型，响应体上限 8 MB。
- 每批刷新只持久化一次缓存并重建一次路由；旧配置的迟到结果在提交前按指纹丢弃。

刷新触发：

- 开启模型路由。
- 应用启动且路由已开启。
- 启用供应商或修改连接、认证、API 格式。
- 24 小时过期检查。
- 设置页手动刷新。

删除供应商时删除缓存；停用时移出路由但暂留缓存。仅修改映射、白名单、放过规则、Level 或排序时复用远端缓存并同步重建索引。手动新增、删除、Claude 同步或云端同步模型后同步重建索引。

## 6. `/v1/models` 聚合契约

聚合关闭时保持转发第一个可用供应商。聚合开启时从路由索引生成 Anthropic 格式响应，不在请求期间访问上游。

- 请求模型 ID 字典序排序。
- 支持 `limit`、`before_id`、`after_id`。
- `limit` 默认 20，范围 1–1000。
- 返回 `data`、`has_more`、`first_id`、`last_id`。
- 空结果返回 HTTP 200 和空列表。
- 临时黑名单不影响列表，只影响真实请求。

模型对象包含 `id`、`type`、`display_name`、`created_at`、`max_input_tokens`、`max_tokens`、`capabilities`。供应商元数据优先，本地模型库次之，缺失时使用模型 ID、Unix epoch、数值 0 和空能力对象。

元数据策略：

- 激进：Token 上限取最大值，能力取并集，发布时间取最新。
- 保守：Token 上限取最小已知正数，能力取交集。
- 未知值不参与最大或最小计算。

## 7. 前端交互

通用设置在“同 Level 轮询”附近提供：

- 按模型路由开关。
- 聚合 Claude 模型开关。
- 激进/保守分段控制。
- 手动刷新按钮、最近成功时间和成功/失败数量。

关闭模型路由时自动关闭并禁用模型聚合。供应商编辑页在未命中策略选择“原样放过”时显示放过规则编辑器。

## 8. Codex 后续扩展参考

后续支持 Codex 时可复用缓存生命周期、路由索引、通配符规则、元数据合并和刷新状态，但必须独立处理：

- Codex `/models` 的响应结构。
- Codex OAuth `/models` 认证。
- Responses API 的模型能力字段。
- 独立设置字段与缓存命名，不能复用 Claude 开关隐式改变 Codex。

## 9. 验证标准

- Claude 5 请求只进入 Claude 5 供应商，失败后仅切换到其他 Claude 5 供应商。
- Claude 4.5 请求不会进入仅支持 Claude 5 的供应商。
- 聚合列表只返回至少有一个路由的请求模型。
- 设置、缓存、分页、迁移、通配符与两种元数据策略均有单元测试。
- 验证命令：`go test ./services/...`、`pnpm test:unit`、`pnpm exec vue-tsc --noEmit`。

/**
 * @name: Claude 映射优先路由变更记录
 * @Descripttion: 记录 Claude 显式模型映射改为可信路由规则的行为调整
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-07-14 19:07:53
 * @LastEditTime: 2026-07-14 19:07:53
 * @FilePath: doc/changes/2026-07-14-1907-claude-mapping-trust.md
 */

# Claude 映射优先路由

## 变更时间

2026-07-14 19:07:53 CST（Asia/Shanghai）

## 涉及范围

- Claude 模型映射与请求路由
- Claude `/v1/models` 聚合列表
- Claude 供应商配置验证
- 模型映射界面提示

## 变更内容

- Claude 按模型路由开启后，显式 `modelMapping` 命中即允许供应商参与路由。
- 映射目标不再依赖 `supportedModels` 或供应商模型接口列表验证。
- 模型库外的请求模型可在请求阶段动态命中通配符映射。
- 映射目标缺少远端元数据时回退到请求模型的本地元数据和稳定默认值。
- 未命中映射的模型继续按白名单和原样放过规则严格校验。
- Codex 和自定义 CLI 保持原有严格模型验证行为。
- 聚合列表继续只返回模型库展开出的具体请求模型，不暴露通配符字符串。
- 发布版本推进到 `v2.8.94`，同步应用版本、Wails 构建元数据和发布说明。

## 验证结果

- `go test ./services/...`：通过。
- `go test -race ./services/...`：通过。
- `go test ./resources/model-pricing/...`：通过。
- `pnpm test:unit`：44 个测试文件、314 项测试全部通过。
- `pnpm exec vue-tsc --noEmit`：通过。
- `wails3 task common:update:build-assets`：构建元数据同步完成。
- `git diff --check`：通过。

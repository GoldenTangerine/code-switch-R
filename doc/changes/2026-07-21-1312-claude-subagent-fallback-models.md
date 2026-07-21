/**
 * @name: Claude Subagent 与默认兜底模型
 * @Descripttion: 记录 Claude 托管路由新增 Subagent 和默认兜底模型设置
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-07-21 13:12:00
 * @LastEditTime: 2026-07-21 13:12:00
 * @FilePath: doc/changes/2026-07-21-1312-claude-subagent-fallback-models.md
 */

# Claude Subagent 与默认兜底模型

- 变更时间：2026-07-21 13:12:00 CST（Asia/Shanghai）
- 涉及范围：Claude 供应商模型映射、代理请求路由、Claude CLI 配置恢复、前端多语言与测试

## 变更内容

- Claude 编辑供应商的模型映射列表末尾新增固定 Subagent 设置，下方新增独立默认兜底模型设置。
- 两项均支持目标模型、思考强度和 Anthropic 格式下的 1M 声明；清空目标模型同步停用并清理元数据。
- 默认兜底使用 `*` 保留映射，保持精确映射和更具体通配符优先；Subagent 优先使用专用映射，缺失时使用默认兜底。
- 托管代理按配置临时管理 `CLAUDE_CODE_SUBAGENT_MODEL`，配置清空或代理关闭时恢复原值，用户手动修改后不覆盖。
- 内部 Subagent 别名禁止通过普通通配符或未命中透传发送给上游。
- v2 到 v3 状态迁移分别处理 API Key 与 Subagent 基线，避免代理占位值覆盖原始 API Key。
- Subagent 注入与恢复使用可续接的过渡状态，设置写入中断后可在重试、启动协调或关闭代理时收敛。
- 模型映射开关失败时仅回滚当前规则，固定映射在保存期间禁止编辑，避免覆盖并发表单修改。
- Claude 配置协调移到供应商保存锁之外，并通过独立协调锁重新读取最新供应商配置，减少阻塞且避免并发保存反序覆盖。

## 验证结果

- `go test ./services/...`：通过；存在已有 macOS target-version linker warning，不影响测试结果。
- `pnpm test:unit`：51 个测试文件、386 个测试全部通过。
- `pnpm exec vue-tsc --noEmit`：通过。
- `git diff --check`：通过。

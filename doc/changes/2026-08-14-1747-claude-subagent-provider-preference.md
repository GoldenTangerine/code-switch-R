/**
 * @name: Claude Subagent 会话供应商偏好
 * @Descripttion: 记录 Claude Subagent 优先跟随当前会话供应商的路由能力
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-14 17:47:57
 * @LastEditTime: 2026-08-14 17:47:57
 * @FilePath: doc/changes/2026-08-14-1747-claude-subagent-provider-preference.md
 */

# Claude Subagent 会话供应商偏好

- 变更时间：2026-08-14 17:47:57 CST（Asia/Shanghai）
- 涉及范围：Claude 代理请求路由、会话状态、活动连接、请求日志、前端路由详情与测试

## 变更内容

- 新增内存会话供应商偏好状态，按平台与会话标识记录主请求最近成功使用的供应商，不跨应用重启保留。
- 主请求转发期间使用 generation 管理临时偏好，支持并发请求按最新活动优先、失败回滚及旧请求成功不覆盖新状态。
- Claude 模型路由开启且请求具有稳定会话标识时，`code-switch-r-subagent` 优先跟随当前会话主请求供应商。
- 会话首选供应商跨 Level 绝对优先；供应商不可用或请求失败后，继续按原有 Level 与轮询顺序降级。
- Subagent 请求仅读取会话偏好，不更新状态；未识别的模型、缺少稳定会话标识或未开启模型路由时保持原路由行为。
- 会话缓存最多保留 4096 个非活动会话，并按最近使用顺序淘汰，避免长期运行时无界增长。
- 活动连接与请求日志新增会话首选供应商及选择结果字段，前端模型提示展示“已跟随”或“已降级”；旧日志无对应字段时不展示。
- 保持旧供应商会话亲和开关关闭，避免改变普通主请求的原有负载与降级策略。
- 首选供应商失败后的降级保留原 Level 与轮询起点；并发槽未获取时不发布临时偏好，并增加真实代理链路回归测试。

## 验证结果

- `go test ./services/...`：通过；存在已有 macOS target-version linker warning，不影响测试结果。
- `pnpm test:unit`：60 个测试文件、458 个测试全部通过。
- `pnpm exec vue-tsc --noEmit`：通过。
- `git diff --check`：通过。

/**
 * @name: 供应商成功率三态判定
 * @Descripttion: 使用成功、失败和排除三态结果优化供应商成功率口径
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-07-23 11:01:26
 * @LastEditTime: 2026-07-23 11:01:26
 * @FilePath: doc/changes/2026-07-23-1101-request-outcome-success-rate.md
 */

# 供应商成功率三态判定

- 变更时间：2026-07-23 11:01:26 CST（Asia/Shanghai）
- 涉及范围：代理流转发、请求日志、供应商统计、错误列表、可用性统计和日志页展示

## 变更内容

- 请求日志新增 `request_outcome` 和 `outcome_reason`，区分 `success`、`failure` 和 `excluded`。
- 收到 `response.completed` 或成功的 `response.done` 后即认定供应商完成；随后发生的客户端关闭仅保留诊断信息，不再覆盖为 `499/502` 失败。
- 完成前的客户端取消、本地并发限制和代理内部错误从供应商成功率分母排除。
- 上游非 2xx、网络错误、协议失败、缺少终止事件和 `unexpected EOF` 仍统计为供应商失败。
- 成功率改为 `success / (success + failure)`；`total_requests`、Token 和费用仍包含被排除请求。
- 小时和日聚合表新增 `excluded_requests`；旧日志不重分类，字段为空时继续使用原 HTTP 口径。
- 非法、未知或带多余空格的结果值统一规范化；未知值按 HTTP 状态回退，避免汇总、失败列表、可用性和聚合统计出现分歧。
- 失败列表、未读失败数、供应商性能样本和可用性统计统一使用三态结果。
- 日志页 Provider 统计新增“已排除”列；首页成功率提示展示成功、失败和排除数，无有效样本时显示 `—`。

## 验证结果

- `go test ./services/...`：通过。
- `cd frontend && pnpm test:unit`：通过。
- `cd frontend && pnpm exec vue-tsc --noEmit`：通过。
- `git diff --check`：通过。

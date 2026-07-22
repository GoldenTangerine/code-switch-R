/**
 * @name: 流式断流诊断与供应商失败优化
 * @Descripttion: 记录断流分类、单次供应商尝试、失败日志和默认拉黑阈值调整
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-07-22 19:02:07
 * @LastEditTime: 2026-07-22 19:02:07
 * @FilePath: doc/changes/2026-07-22-1902-stream-failure-handling.md
 */

# 流式断流诊断与供应商失败优化

- 变更时间：2026-07-22 19:02:07 CST（Asia/Shanghai）
- 涉及范围：Claude/Codex、Gemini、自定义 CLI 代理转发，请求日志，供应商日志，黑名单默认配置

## 变更内容

- 每次代理请求对单个供应商最多执行一次路由尝试，失败后继续切换下一个供应商；移除 HTTP 客户端网络重试，仅保留一次 Claude 协议参数修正回退，不累计供应商失败。
- 客户端取消统一记录为 `499/client_abort`，立即停止转发，不累计供应商失败；真实上游断流记录为 `502` 并累计一次失败。
- Gemini 同步识别下游 `broken pipe` 等写入中断，每次供应商尝试独立落库，降级成功后仍保留前一供应商的失败记录。
- 请求日志新增脱敏、最大 2048 字节的 `error_message`，覆盖 JSON、Bearer/Basic 认证、API Key 和纯文本密码格式；项目日志和供应商日志可直接查看代理诊断。
- 新安装默认拉黑阈值从 3 调整为 5；已有数据库中的用户配置保持不变，不强制覆盖。
- 新增普通代理与 Gemini 实际落库、黑名单开启时单次调用和供应商切换等回归测试。
- 发布版本推进到 `v2.9.10`，同步发布说明与各平台构建元数据。

## 验证结果

- 断流分类、日志迁移、实际落库、单次路由尝试、Claude 协议兼容回退、Gemini 降级日志和纯文本凭证脱敏定向 Go 测试：通过。
- `cd frontend && pnpm test:unit`：通过，53 个测试文件、399 个测试。
- `cd frontend && pnpm exec vue-tsc --noEmit`：通过。
- `go test ./services/...`：通过。
- `git diff --check`：通过。

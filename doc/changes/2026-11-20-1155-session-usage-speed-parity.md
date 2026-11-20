<!--
@name: 会话用量来源扩展与速度首字统计
@Descripttion: 记录参照 cc-switch 新增本地会话用量来源、会话行速度与首字估算的实现与验证。
@version: 1.0.0
@Author: sm
@Date: 2026-11-20 11:55:24
@LastEditTime: 2026-11-20 11:55:24
@FilePath: doc/changes/2026-11-20-1155-session-usage-speed-parity.md
-->

# 会话用量来源扩展与速度首字统计

- 变更时间：2026-11-20 11:55:24（Asia/Shanghai，UTC+08:00）。
- 涉及范围：会话用量同步、请求日志来源与持久化、日志页展示。

## 背景

- cc-switch 的用量统计可展示 Gemini 系等本地 CLI 会话的用量、速度与首字；本项目此前只导入 Claude、Codex、Gemini、OpenCode、Grok 的会话用量，且所有会话行没有计时。

## 变更内容

### 新增会话来源（`services/session_usage_<应用>.go`）

| 来源 | platform | data_source | 计时 |
| --- | --- | --- | --- |
| Antigravity | `antigravity` | `agy_session` | 原生库 `conversations/<id>.db` 字段 11（首字）与 12（流式时长） |
| Kimi Code | `kimi` | `kimi_session` | `llmFirstTokenLatencyMs + llmStreamDurationMs`，缺失时按请求时间估算 |
| Copilot CLI | `copilot` | `copilot_session` | `duration` 与 `timeToFirstTokenMs` |
| WorkBuddy | `workbuddy` | `workbuddy_session` | 无 |
| DeepSeek Harness | `deepseek-harness` | `dsh_session` | `step/start` 至 `assistant/message`，首字取最早流式分片 |
| MiniMax Code | `minimax-code` | `mcode_session` | `request_duration_ms` 与 `decode_duration_ms` |
| Oh My Pi | `omp` | `omp_session` | `duration`，首字取 `ttft` |
| Pi | `pi` | `pi_session` | `duration`，缺失时按时间戳估算 |

- platform 键与后续首页供应商 Tab id 保持一致；所有会话来源集中登记在 `requestlog_source.go` 的 `sessionUsageSourceDefs`，来源谓词由注册表生成。
- Antigravity 的模型解析沿用 cc-switch 规则：继承 `Model Selection` 切换、编排模式与工作角色从原生库取实际模型，原生库暂不可读时保留已解析的模型。

### 计时估算（`services/session_usage_timing.go`）

- Claude：沿父链跳过 attachment 找请求起点，结束取回复各内容块最晚时间；首块为思考块时用 `thinkingDurationMs` 推算首字；增量读取时回看游标前 256KB 补齐对话链；仅对有 `stop_reason` 且可见首块的回复估算。
- Codex：移植 cc-switch 的 RequestTimer，按请求边界、工具结果、`token_usage_record` 与 `item_completed.started_at_ms` 估算，状态随解析游标持久化。
- 估算耗时限定在 100ms 至 1 小时，首字须落在 (0, 耗时]。

### 持久化与去重

- 会话行写入 `duration_sec` / `first_token_sec`；仅计时变化时走轻量更新，不重新计价、不重置代理去重匹配；`created_at` 按 UTC 秒精度字符串比较。
- 新增 `session_usage_receipt` 永久回执：会话行入库即登记，用户删除后不会因会话文件重读而复活；“清空并重新导入会话”时清空回执，Grok 全量覆盖时同步清理回执。
- 一次性迁移：重置 Claude / Codex 游标补写历史计时，原行号记入 `session_log_sync.backfill_line_offset`，此前导入的部分只更新已有行。
- Gemini CLI 支持 `.jsonl` 回放（`$set` 检查点、`$rewindTo` 回退），`.jsonl` 会话按消息 id 去重，旧版 `.json` 保留原键。
- 单个会话文件出错只记录错误，不中断同一来源的其他文件；读取前记录文件状态，避免读取期间追加的内容被漏读。
- `latestProviderPerformance` 限定代理来源，会话估算值不进入首页与托盘的供应商性能统计。

### 日志页

- 会话行的耗时、首字、速度加“≈”前缀与悬停说明；速度仅在输出 ≥200 Token 且耗时 ≥1 秒时显示。
- 平台筛选新增 OpenCode、Grok Build 与 8 个新来源，下拉限制高度可滚动；表格、存储弹窗统一显示应用名；来源标签统一为“会话”。
- 供应商统计中会话来源的平均耗时加“≈”；存储弹窗的会话耗时同样加“≈”；移除不再使用的会话来源文案。

## 已知限制

- DeepSeek Harness 默认写入的 `.jsonl.zstd` 压缩日志暂不解码，需引入 zstd 依赖后支持。
- 内置价格表缺少 Gemini 4、Kimi 等新模型，相关会话行成本为 $0。
- Pi / MiniMax Code 来源自带的成本字段未采用，统一按价格表计价；Token 为 0 的失败请求不入库。

## 验证结果

- `go test ./services/... ./resources/...` 全部通过，新增来源解析、计时估算、回执防复活、历史回填只更新、Gemini 检查点去重、来源可见性等测试。
- 使用本机 Antigravity 真实会话核对：27 条请求的 Token、速度与首字与 cc-switch 展示一致，重复同步零导入。
- `pnpm test:unit` 76 个文件、605 项测试通过；`vue-tsc --noEmit` 无错误。
- 未运行应用构建、打包或开发启动。

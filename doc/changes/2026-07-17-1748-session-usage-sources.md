# 日志会话用量来源

- 变更时间：2026-07-17 17:48:19 CST（Asia/Shanghai）
- 涉及范围：会话日志同步、请求日志存储、用量去重、日志页面、国际化与测试

## 变更内容

- 参考 `cc-switch`，增量同步 Claude、Codex、Gemini CLI 本地会话用量；Windows 支持手动扫描全部 WSL 发行版。
- 日志页新增“代理 / 会话 / 全部”来源切换并记忆选择，统一作用于明细、汇总、趋势、Provider、模型统计及存储热力图。
- 启动时同步一次，之后每 60 秒检查文件变化；手动刷新先同步会话。文件指纹、字节游标和 Codex 累计解析状态持久化，未变化文件不重复解析。
- “全部”模式以平台、输入、输出、缓存读取和十分钟时间窗建立一对一持久化匹配，隐藏已匹配会话记录，避免代理与会话重复计算。
- Claude 按 `message.id` 合并并更新跨周期最终快照；Codex 计算累计 Token 差值、跳过子代理历史重放，并在归档后继承原游标。
- Gemini 文件未变化时直接跳过；文件变化时全量比较消息，仅对新增或 Token、模型发生变化的记录重新计价和写库，零 Token 占位消息补全后可正常导入。
- Gemini 的输出与思考 Token 分别写入 `output_tokens` 和 `reasoning_tokens`，确保代理与会话记录使用相同去重口径。
- 来源切换使用最后一次有效筛选作为回退，费用详情、热力图及当天日志始终与页面已应用来源一致。
- 清理全部日志时可选择下次同步重新导入历史会话，默认关闭；会话记录不提供请求体和响应体详情。
- 预算、托盘、配额、健康统计及既有聚合表继续只使用代理日志口径。

## 数据结构

- `request_log` 新增 `data_source`、`source_record_id`、`session_id`、`dedup_core`。
- 新增 `session_log_sync`、`session_usage_dedup`、`session_usage_dedup_state`，分别保存增量游标、去重关系和去重水位。

## 验证结果

- `go test ./services/... ./resources/model-pricing/... -count=1`：通过。
- `pnpm test:unit`：51 个测试文件、364 个测试全部通过。
- `pnpm exec vue-tsc --noEmit`：通过。
- `git diff --check`：通过。
- 按项目规范未执行 lint、build、package 或 dev。

<!--
@name: Codex 被动采样套餐额度
@Descripttion: 记录 Token Plan 新增 Codex 被动采样及额度展示实现。
@version: 1.0.0
@Author: sm
@Date: 2026-11-06 17:58:54
@LastEditTime: 2026-11-06 17:58:54
@FilePath: doc/changes/2026-11-06-1758-codex-passive-token-plan-quota.md
-->

# Codex 被动采样套餐额度

- 变更时间：2026-11-06 17:58:54（Asia/Shanghai，UTC+08:00）。
- 涉及范围：额度配置类型、Token Plan 配置弹窗、中英文文案、首页额度刷新与展示、代理转发、额度查询与自动停用服务，以及相关测试。
- 参考实现：sub2api `backend/internal/service/openai_gateway_usage.go` 的 `ParseCodexRateLimitHeaders` 及 `openai_gateway_passthrough.go` 的 Codex 用量响应头透传逻辑。

## 变更内容

- 在“额度查询配置 → Token Plan → Token Plan 供应商”新增“Codex 被动采样”，保存为 `tokenPlanProvider: codex_passive`，查询类型为 `token_plan_codex_passive`，复用供应商 API 地址和密钥。
- 经本地代理转发 Codex 请求时，从 `x-codex-primary-*` 和 `x-codex-secondary-*` 采集 `used-percent`、`reset-after-seconds` 和 `window-minutes`；支持普通响应、流式响应和 429 响应，查询样本不额外发送 HTTP 请求。
- 使用率按百分制展示，按实际 `window-minutes` 显示 `5h`、`7d` 等周期，不固定主、次窗口与具体周期的对应关系；缺少周期时显示中英文主、次窗口名称，缺少重置信息时不臆造倒计时。
- 将相对重置秒数换算为采样时刻对应的重置时间；过滤负数、NaN、Infinity 和异常整数，保留 0% 与超过 100% 的真实使用率。部分响应仅更新对应数据，窗口重置或周期改变时不沿用旧窗口使用率。
- 按 API 地址和密钥的 SHA-256 摘要隔离样本，通过全局数据库写入队列保存到 `provider_codex_quota_samples` 表；服务重建后恢复，采样表不保存明文地址或密钥。Claude 采样表与行为保持兼容。
- 首次采样前显示等待提示；窗口重置后或样本超过窗口时长时提示过期，无周期信息时最长保留 5 小时有效期。首页沿用 30 秒刷新定时器，直接读取本地样本，不受远端缓存时长或活跃供应商筛选限制。
- 与 Claude 被动模式一致，隐藏远端查询时序配置，仅展示用量，不因额度耗尽自动停用供应商。

## 验证结果

- Codex 与 Claude 被动采样后端专项测试通过，覆盖窗口周期互换、异常和部分响应、过期窗口、数据库恢复与身份隔离、并发读写、普通/流式/429 转发及仅展示模式。
- `pnpm test:unit` 通过，76 个测试文件、599 个测试。
- `pnpm exec vue-tsc --noEmit` 通过。
- `go test ./services/...` 全量复跑通过；首次全量运行失败，使用 JSON 记录输出后的复跑未复现，首次失败原因未确认。Go 检查使用临时目录 `GOCACHE=/private/tmp/code-switch-r-codex-passive-go-cache`，避开沙箱不可写的默认缓存目录。
- `go test -race ./services/ -run 'Test(Codex|Claude)PassiveQuotaConcurrentReadsAndSamples' -count=1` 通过。
- `git diff --check` 和中英文新增翻译键检查通过。
- 未运行应用构建、打包或开发启动，未连接真实 Codex 账号联调。

## 使用条件

- 启用供应商的本地代理托管，并让 Codex 请求经过本地代理；选择被动采样后发送请求，再刷新额度。
- 上游需返回 Codex 额度响应头；缺少这些头时显示等待采样。sub2api 的 OpenAI 透传路径已显式放行这些头，其他路径取决于实际过滤配置。
- 数据是本次命中的上游账号最后一次采样的套餐用量，并非下游 Key 的独立配额，也不能换算成精确剩余 Token 或金额；账号池切换后读数可能变化。

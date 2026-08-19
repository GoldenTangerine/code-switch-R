<!--
@name: 会话粘滞首位供应商优先
@Descripttion: 记录新会话优先使用首位零会话供应商的路由调整
@version: 1.0.0
@Author: sm
@Date: 2026-08-20 01:05:06
@LastEditTime: 2026-08-20 01:05:06
@FilePath: doc/changes/2026-08-20-0105-session-affinity-first-provider.md
-->

# 会话粘滞首位供应商优先

- 变更时间：2026-08-20 01:05:06 CST（Asia/Shanghai）
- 涉及范围：Claude、Codex、Gemini、自定义 CLI 会话粘滞路由与托盘默认供应商预览

## 变更内容

- 新会话尚未绑定供应商时，首页排序第一的当前可用供应商若为零会话，则跨 Level 优先尝试。
- 首位供应商已有临时或已确认会话时，继续使用现有 Level、会话容量和负载排序。
- 并发新会话竞争首位供应商时，在会话锁内重新校验零会话条件并创建临时绑定；只有首个请求占用首位，其余请求继续原有调度顺序。
- 既有会话保持原供应商绑定；首位不可用或尝试失败时，保留原有过滤、重试和降级逻辑。
- 托盘与首页的下一默认供应商预览同步使用真实路由规则。

## 验证结果

- 会话排序与默认供应商预览目标测试通过。
- Provider 与 Gemini 真实调度入口、并发冷启动及 `-race` 回归测试通过。
- `go vet ./services/...` 与 `git diff --check` 通过。
- `env TZ=UTC GOCACHE=/tmp/code-switch-r-go-cache go test ./services/... -count=1` 完整通过。
- 上海时区凌晨直接运行 `go test ./services/... -count=1` 时，既有日期边界用例 `TestProviderDailyStats_UsesLatestFiveSuccessfulStreamingSamplesAcrossDays` 失败，单独运行同样失败；该用例的 UTC 自然日数据与本地日界线在当前时段不一致，与本次会话路由改动无关。

<!--
@name: ProviderDailyStats 午夜时区测试夹具
@Descripttion: 将日志统计测试时间锚定到本地当日中段以消除午夜时区误报
@version: 1.0.0
@Author: sm
@Date: 2026-09-01 03:40:36
@LastEditTime: 2026-09-01 03:40:36
@FilePath: doc/changes/2026-09-01-0340-provider-stats-timezone-fixture.md
-->

# ProviderDailyStats 午夜时区测试夹具

## 变更时间

2026-09-01 03:40:36 CST（Asia/Shanghai）

## 涉及范围

- `services/logservice_provider_stats_test.go`
- `doc/code-performance-optimization-plan.md`

## 变更内容

- 将跨天性能样本夹具从 UTC 当前时刻改为本地当日零点锚点。
- 当日请求固定放在本地 09:00-11:00，历史成功流式样本固定放在前 1-6 日中段。
- SQLite 写入仍使用 UTC `timeLayout`，生产统计逻辑、接口和数据格式均未修改。

## 验证结果

- 旧夹具在 `TZ=Etc/GMT-5` 的本地午夜窗口稳定复现预期 2、实际 0。
- 新夹具在 `TZ=UTC`、`TZ=Asia/Shanghai`、`TZ=Etc/GMT-5` 下各连续 5 次通过。
- 完整 `go test ./services/... -count=1` 无需排除该用例即可通过；仅有既有 macOS 链接目标版本警告。

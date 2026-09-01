<!--
@name: Gemini Provider 所有权边界
@Descripttion: 记录 Gemini Provider 深拷贝、并发所有权和失败回退保护
@version: 1.0.0
@Author: sm
@Date: 2026-09-01 01:12:26
@LastEditTime: 2026-09-01 01:12:26
@FilePath: doc/changes/2026-09-01-0112-gemini-provider-ownership.md
-->

# Gemini Provider 所有权边界

## 变更时间

2026-09-01 01:12:26 CST（Asia/Shanghai）

## 涉及范围

- `services/geminiservice.go`
- `services/geminiservice_test.go`
- `doc/code-performance-optimization-plan.md`

## 变更内容

- 新增 Gemini Provider 完整字段克隆入口，递归复制 Wails/JSON 产生的嵌套对象和数组，并复制环境变量 map、并发上限、预算及额度查询指针。
- `GetProviders` 返回服务持有数据的副本；`AddProvider` 和 `UpdateProvider` 保存调用方对象前取得独立所有权。
- `SetForcedPriority` 保存失败时恢复深层快照；`DuplicateProvider` 的嵌套配置和返回值不再与内部副本共享。
- 保留 nil/空容器、Provider 顺序、JSON 字段、SQLite payload、公共 Wails 签名、错误文本和复制供应商现有字段选择。
- 新增完整字段、传入/返回对象、保存失败回退、复制返回值和并发副本测试。
- 实测发现的 Gemini 改名事务等待登记为 OPT-021，午夜时区测试失稳登记为 OPT-022；本批未修改相关生产或测试逻辑。

## 验证结果

- B15 目标所有权测试连续 5 次通过，包内耗时 0.871 秒。
- B15 目标 `go test -race` 通过，包内耗时 2.432 秒。
- Gemini、ProviderStore 和 Gemini 代理专项回归通过。
- `go test ./services/... -count=1 -skip '^TestProviderDailyStats_UsesLatestFiveSuccessfulStreamingSamplesAcrossDays$'` 通过。
- OPT-022 用例在 `TZ=UTC` 下单独通过；默认 Asia/Shanghai 01:07 CST 运行时稳定复现预期 2、实际 1。
- `go test ./resources/model-pricing/... -count=1` 通过。
- `go test . -count=1` 通过。
- 仅出现项目既有 macOS 链接目标版本警告；本批不宣称性能提升。

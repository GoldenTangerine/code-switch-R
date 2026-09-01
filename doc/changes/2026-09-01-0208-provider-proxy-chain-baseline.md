<!--
@name: Provider 代理链路深拷贝基线
@Descripttion: 记录 Provider 深拷贝在固定成功代理完整链路中的耗时与分配占比
@version: 1.0.0
@Author: sm
@Date: 2026-09-01 02:08:24
@LastEditTime: 2026-09-01 02:08:24
@FilePath: doc/changes/2026-09-01-0208-provider-proxy-chain-baseline.md
-->

# Provider 代理链路深拷贝基线

## 变更时间

2026-09-01 02:08:24 CST（Asia/Shanghai）

## 涉及范围

- `services/provider_proxy_chain_benchmark_test.go`
- `services/provider_snapshot_benchmark_test.go`
- `doc/code-performance-optimization-plan.md`

## 变更内容

- 新增 0/1/10/100 Provider 的固定成功代理完整链路 Benchmark。
- 复用富字段 Provider 夹具，通过真实 Gin 路由、本地 `httptest` 上游和日志队列验证状态码、响应体及上游调用次数。
- 基准期间隔离 stdout 与 `xlog` 输出，避免终端 I/O 污染计时，不修改生产代码、配置、依赖或数据格式。
- 根据数据延期共享只读快照实现，保留现有深拷贝所有权边界。

## 验证结果

- 夹具、Provider 快照和通用调度目标测试通过。
- 完整链路与快照对照各完成 5 轮。
- 1/10/100 Provider 的克隆耗时约占完整链路 0.04%/0.38%/3.84%，分配字节约占 0.53%/4.97%/29.75%。
- 从 1 增至 100 Provider，克隆约解释完整链路 66.9% 的分配增量；缺少真实规模和生产 heap profile，不宣称生产性能提升。
- 排除独立登记的 OPT-022 后 `go test ./services/... -count=1` 通过；仅有既有 macOS 链接目标版本警告。

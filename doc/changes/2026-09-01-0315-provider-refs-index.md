<!--
@name: ProviderRefs 部分覆盖索引
@Descripttion: 为 ProviderRefs 查询落地部分覆盖索引并稳定消除临时分组
@version: 1.0.0
@Author: sm
@Date: 2026-09-01 03:15:37
@LastEditTime: 2026-09-01 03:15:37
@FilePath: doc/changes/2026-09-01-0315-provider-refs-index.md
-->

# ProviderRefs 部分覆盖索引

## 变更时间

2026-09-01 03:15:37 CST（Asia/Shanghai）

## 涉及范围

- `services/logservice.go`
- `services/providerrelay.go`
- `services/log_provider_refs_benchmark_test.go`
- `doc/code-performance-optimization-plan.md`

## 变更内容

- 新增 `idx_request_log_provider_refs` 幂等部分覆盖索引，不新增列或迁移数据。
- 有 platform 的 ProviderRefs 查询明确选择该索引；无 platform 由 SQLite 自动选择。
- B20 候选测试切换为生产索引，保留旧计划、结果等价、写入和存储对照。
- 公共方法、Wails 绑定、来源、旧名称合并、同名多 ID 和排序语义不变。

## 验证结果

- 六类生产查询均使用覆盖索引且无临时分组 B-tree，结果逐字段、逐顺序一致。
- 100k 高基数 5 轮中位数约由 53.22-102.30 ms 降至 9.79-23.49 ms，提升约 3.9-5.4 倍；分配基本不变。
- B20 已测得代价：10k 行事务写入中位数约增加 14.2%，索引占用 71.39 B/行。
- ProviderRefs 目标 `-race`、request_log schema、仪表盘查询计划及排除 OPT-022 的完整 services 回归通过。
- 仅有既有 macOS 链接目标版本警告。

<!--
@name: ProviderRefs 查询性能基线
@Descripttion: 记录 ProviderRefs 查询增长、查询计划及候选索引读写与存储成本
@version: 1.0.0
@Author: sm
@Date: 2026-09-01 03:00:06
@LastEditTime: 2026-09-01 03:00:06
@FilePath: doc/changes/2026-09-01-0300-provider-refs-query-baseline.md
-->

# ProviderRefs 查询性能基线

## 变更时间

2026-09-01 03:00:06 CST（Asia/Shanghai）

## 涉及范围

- `services/logservice_provider_refs_test.go`
- `services/log_provider_refs_benchmark_test.go`
- `services/logservice.go`、`services/providerrelay.go`（仅测量）
- `doc/code-performance-optimization-plan.md`

## 变更内容

- 补充 ProviderRefs 的来源、平台、改名、旧名称合并、同名多 ID 和空值 SQL 语义测试。
- 新增 1k/10k/100k、低/高 Provider 基数、三种来源、有/无 platform、冷/暖查询矩阵与查询计划。
- 仅在测试库比较部分覆盖索引，逐字段验证结果，并测量 10k 行事务写入与 100k 行存储代价。
- 生产 SQL、schema、Wails 签名、刷新周期和用户数据均未修改。

## 验证结果

- 基线无 platform 为全表扫描，有 platform 使用既有平台索引，全部组合使用临时分组 B-tree。
- 100k 高基数六类场景 5 轮中位数约由 51.4-102 ms 降至 9.7-23.9 ms，约提升 4.2-5.3 倍，结果逐字段一致。
- 10k 行事务写入中位数 1.039 → 1.187 s，约增加 14.2%；100k 行索引增加 7,139,328 bytes，即 71.39 B/行。
- 查询候选和写入对照均完成 5 轮；100k 存储、查询计划和 ProviderRefs 语义通过，相关查询路径在 `-race` 下通过。
- 仅有既有 macOS 链接目标版本警告。

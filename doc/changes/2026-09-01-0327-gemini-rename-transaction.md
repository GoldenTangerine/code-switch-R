<!--
@name: Gemini 改名事务单 writer 化
@Descripttion: 将 Gemini ProviderStore 与关联身份改名合并到同一数据库队列事务
@version: 1.0.0
@Author: sm
@Date: 2026-09-01 03:27:19
@LastEditTime: 2026-09-01 03:27:19
@FilePath: doc/changes/2026-09-01-0327-gemini-rename-transaction.md
-->

# Gemini 改名事务单 writer 化

## 变更时间

2026-09-01 03:27:19 CST（Asia/Shanghai）

## 涉及范围

- `services/dbqueue.go`、`services/dbqueue_test.go`
- `services/providerstore.go`
- `services/geminiservice.go`、`services/geminiservice_test.go`
- `doc/code-performance-optimization-plan.md`

## 变更内容

- 事务组任务增加受控 `sql.Tx` 函数入口，继续由单 worker 负责 Begin、Commit 和 Rollback。
- 抽取 ProviderStore 替换任务与 Gemini 序列化行构造，普通保存入口保持不变。
- Gemini 有改名时，把既有身份同步与 ProviderStore DELETE/INSERT 放入同一队列事务。
- 任一必需身份更新失败时，数据库事务和 Gemini 内存状态整体回退。

## 验证结果

- DBWriteQueue 事务函数成功提交、失败回滚、现有 SQL 回滚和任务复用通过。
- Gemini 有关联日志改名成功，以及注入关联更新失败后的存储、日志和内存回退通过。
- 目标测试连续 20 次及 `-race` 通过；Gemini、ProviderStore、DBWriteQueue 专项通过。
- 修改前调用链曾超过 20 秒测试总超时；修改后目标单轮显示 0.00 s，连续 20 轮总测试 1.53 s。
- 排除 OPT-022 的完整 services 回归通过；仅有既有 macOS 链接目标版本警告。

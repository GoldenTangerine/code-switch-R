<!--
@name: 代码与性能优化计划
@Descripttion: 记录 Code Switch R 首版代码质量与性能优化规划文档的创建
@version: 1.0.0
@Author: sm
@Date: 2026-08-31 15:01:40
@LastEditTime: 2026-08-31 15:01:40
@FilePath: doc/changes/2026-08-31-1501-code-performance-optimization-plan.md
-->

# 代码与性能优化计划

- 变更时间：2026-08-31 15:01:40 CST（Asia/Shanghai）
- 涉及范围：项目代码与性能静态分析、现有测试基线、后续优化批次规划

## 变更内容

- 新增 `doc/code-performance-optimization-plan.md`，作为后续跨会话逐批优化的唯一总计划。
- 基于当前代码记录 12 个具有具体文件、函数或调用链证据的 OPT 问题。
- 将首批工作拆分为 10 个可独立执行、验证和回退的批次。
- 将未取得性能数据的候选统一标记为“待测量”，并先安排测量批次。
- 明确公共 API、现有 Wails 方法、UI 行为、配置与数据库数据格式、调度、代理协议和统计口径均不得改变。
- 确认可新增兼容型 Wails 聚合方法，但必须完整保留现有方法与返回格式。
- 确认 SQLite 只可在测量证明有效且再次确认后新增可回退索引，不允许改变列或数据格式。
- 记录尚未深入分析的模块，避免将首版规划误认为完整审计。
- 未修改任何业务代码、测试代码、配置、依赖或既有专题文档。

## 验证结果

- `go test ./services/...`：通过；仅有本机 macOS 链接目标版本警告。
- `go test ./resources/model-pricing/...`：通过。
- `pnpm test:unit`：通过，69 个测试文件、536 项测试全部通过。
- 文档结构包含要求的 11 个章节、OPT-001 至 OPT-012 和 B01 至 B10。
- `git diff --check`：通过。
- 按项目限制未运行 lint、构建或开发服务。

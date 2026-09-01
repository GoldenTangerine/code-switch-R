<!--
@name: 代码与性能优化计划第二轮
@Descripttion: 记录 Code Switch R 第二轮代码质量与性能优化问题及分批实施规划
@version: 1.0.0
@Author: sm
@Date: 2026-09-01 00:21:06
@LastEditTime: 2026-09-01 00:21:06
@FilePath: doc/changes/2026-09-01-0021-code-performance-optimization-next-wave.md
-->

# 代码与性能优化计划第二轮

- 变更时间：2026-09-01 00:21:06 CST（Asia/Shanghai）
- 涉及范围：现有优化计划、Gemini Provider 所有权、KeepAlive 页面轮询、代理快照、黑名单并发和日志 ProviderRefs 查询规划

## 变更内容

- 更新 `doc/code-performance-optimization-plan.md`，完整保留 B01-B14 的历史证据、执行记录和前后数据。
- 新增 OPT-016 至 OPT-020，并为仍缺数据的前端离页调用、黑名单竞争和 ProviderRefs 查询明确标记“待测量”。
- 将 OPT-005 追加到完整代理链路测量批次，不依据既有微基准直接实施快照优化。
- 新增 B15-B20 共 6 个可独立执行、验证和回退的批次，下一批为 Gemini Provider 深拷贝与并发所有权。
- 补充 Gemini 配置、Main/Logs KeepAlive 数据流和其余待分析模块，避免把第二轮规划视为完整仓库审计。
- 未修改任何业务代码、测试代码、配置、依赖、数据库 schema 或既有变更记录。

## 验证结果

- `go test ./services/... -count=1`：通过；仅有既有 macOS 链接目标版本警告。
- `go test ./resources/model-pricing/... -count=1`：通过。
- `pnpm test:unit`：通过，71 个测试文件、552 项测试全部通过。
- 规划文档包含 OPT-001 至 OPT-020、B01 至 B20；第二轮新增 5 个问题和 6 个批次。
- `git diff --check`：通过。
- 按项目限制未运行 lint、构建或开发服务。

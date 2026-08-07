/**
 * @name: 供应商今日性能趋势
 * @Descripttion: 为供应商数据概览增加首字延迟与生成速度趋势图
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-07 16:14:20
 * @LastEditTime: 2026-08-07 16:14:20
 * @FilePath: doc/changes/2026-08-07-1614-provider-performance-trend.md
 */

# 供应商今日性能趋势

- 变更时间：2026-08-07 16:14:20 CST（Asia/Shanghai）
- 涉及范围：首页供应商数据概览、日志统计服务、中英文文案

## 变更内容

- 新增供应商今日性能趋势查询，按本地自然日每 15 分钟分别计算首字延迟和生成速度均值。
- 仅使用成功流式请求的有效样本，首字延迟与生成速度保持独立样本口径。
- 在四个概览统计卡片下新增全宽性能面板，原有 7 日金额、请求、Tokens 与额度面板下移。
- 使用紫色首字延迟与青绿色生成速度双轴平滑曲线，增加同色渐变填充并隐藏全部圆点。
- 无样本区间跨空连接，性能查询失败时仅在新面板展示错误，不影响原有概览内容。
- 补充服务调用、分桶聚合、时间范围与空样本映射测试。
- 修复弹窗数据刷新时复用已脱离 DOM 的图表实例导致图表空白的问题。
- 性能趋势改为 SQLite 直接按 15 分钟聚合，仅查询所需字段，不再加载完整日志或执行定价解析。
- 性能趋势范围限制为最多 100 个分桶，兼容 25 小时夏令时自然日并避免异常范围分配大量内存。
- 为性能图增加屏幕阅读器可访问的数据表，包含各有效时间点的指标值与样本数。
- 补充旧日志供应商名称回退与超长范围拒绝测试。
- 发布版本推进到 `v2.9.22`，同步更新发布说明与全平台构建元数据。

## 验证结果

- `go test ./services/ -run TestProviderPerformanceTrend15m -count=1`：通过。
- `go test ./services/...`：通过。
- `cd frontend && pnpm exec vitest run src/services/logs.test.ts src/components/Main/modals/providerDataOverview.test.ts`：通过，2 个测试文件、10 项测试。
- `cd frontend && pnpm test:unit`：通过，60 个测试文件、455 项测试。
- `cd frontend && pnpm exec vue-tsc --noEmit`：通过。
- `git diff --check`：通过。
- 按项目限制未启动开发服务、未执行构建和 lint。

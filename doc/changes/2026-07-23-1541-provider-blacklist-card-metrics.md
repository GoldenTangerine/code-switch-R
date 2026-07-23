/**
 * @name: 供应商卡片失败计数与拉黑状态调整
 * @Descripttion: 将失败计数迁入成功率提示并压缩拉黑状态展示
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-07-23 15:41:27
 * @LastEditTime: 2026-07-23 15:41:27
 * @FilePath: doc/changes/2026-07-23-1541-provider-blacklist-card-metrics.md
 */

# 供应商卡片失败计数与拉黑状态调整

- 变更时间：2026-07-23 15:41:27 CST（Asia/Shanghai）
- 涉及范围：首页供应商卡片、黑名单状态加载、成功率提示与中英文文案

## 变更内容

- 移除供应商卡片上的失败计数横幅，将请求失败计数与巡检失败计数合并到成功率悬浮提示。
- 无黑名单记录时使用当前全局阈值展示 `0/阈值`；暂无请求统计时显示“成功率：— · 暂无数据”。
- 活跃拉黑状态改为在花费后显示“已拉黑 + 倒计时”，点击后查看触发来源、原因并执行解除拉黑或清零等级。
- 保留未拉黑供应商现有的降级等级提示和清零入口。
- 阈值接口失败或返回非法值时不再伪造默认阈值；成功读取的阈值缓存 60 秒，并复用并发请求。
- 修复窄屏浮层定位，统一拉黑状态与花费指标的字号和行高；优化英文触发来源文案。

## 验证结果

- `cd frontend && pnpm exec vitest run src/components/Main/composables/useBlacklistState.test.ts`：通过。
- `cd frontend && pnpm exec vitest run src/components/Main/components/ProviderCard.test.ts`：通过。
- `cd frontend && pnpm test:unit`：通过，57 个测试文件、417 项测试。
- `cd frontend && pnpm exec vue-tsc --noEmit`：通过。
- `git diff --check`：通过。

<!--
@name: 设置页初始化并行编排
@Descripttion: 记录 General 页面初始化加载并行化、生命周期清理和验证结果
@version: 1.0.0
@Author: sm
@Date: 2026-08-31 20:23:18
@LastEditTime: 2026-08-31 20:23:18
@FilePath: doc/changes/2026-08-31-2023-general-page-loading.md
-->

# 设置页初始化并行编排

## 变更时间

2026-08-31 20:23:18 CST（Asia/Shanghai）

## 涉及范围

- `frontend/src/components/General/Index.vue`
- `frontend/src/components/General/composables/useGeneralPageData.ts`
- `frontend/src/components/General/composables/useGeneralPageData.test.ts`
- `doc/code-performance-optimization-plan.md`

## 变更内容

- 将 General 页初始化加载、WebDAV 事件订阅和卸载清理抽离为具体 composable，Loader 通过明确参数注入。
- AppSettings 保持第一段加载，模型路由、版本、更新、黑名单、导入和 WebDAV 六项确认独立的读取在其后并行执行。
- 使用 `Promise.allSettled` 隔离独立读取失败；现有各 Loader 的错误回退和状态赋值保持不变。
- WebDAV 事件仍在全部初始化读取完成后订阅；卸载仍调用 `flushPendingPersist` 并取消订阅。
- 增加重复挂载保护；初始化尚未完成时卸载，不会在完成后产生迟到订阅。
- 未拆分页面表单，未修改保存防抖、save queue、localStorage key、Toast、UI 文案、Wails 签名、配置或依赖。

## 性能对比

使用相同 100 ms 假延迟比较初始化编排：

- 修改前：AppSettings 加六项读取串行等待，共 700 ms。
- 修改后：AppSettings 100 ms，六项独立读取并行 100 ms，共 200 ms。
- 数据只证明等待层数由 7 层降为 2 层，不代表真实设备或 Wails 调用延迟。

## 验证结果

- 新增生命周期测试 6 项通过，覆盖依赖顺序、并行启动、单项失败、重复挂载、卸载和初始化中卸载。
- General composable 与相关 service 测试共 25 项通过。
- `pnpm exec vue-tsc --noEmit` 通过。
- `pnpm test:unit`：71 个文件、549 项通过。
- 未运行 lint、构建或开发服务，符合项目限制。

<!--
@name: 界面细节与供应商日志分页优化
@Descripttion: 记录首页、可用性页、供应商日志弹窗的样式调整及日志滚动加载与性能优化。
@version: 1.0.0
@Author: sm
@Date: 2026-10-27 10:45:00
@LastEditTime: 2026-10-27 10:45:00
@FilePath: doc/changes/2026-10-27-1045-ui-polish-provider-logs-paging.md
-->

# 界面细节与供应商日志分页优化

- 变更时间：2026-10-27 10:45:00（Asia/Shanghai，UTC+08:00）。
- 涉及范围：`frontend/src/style.css`、`components/Main`（首页组件、`ProviderLogsModal.vue`）、`components/Availability/Index.vue`、中英文文案。

## 问题原因

- 供应商日志弹窗只提供手动“加载更多”按钮，滚动到底部不会触发分页；浏览器预览模式下仅有 4 条写死的模拟数据，表现为“固定几条”。
- 每次追加分页都会对全部已加载日志重新解析错误体并重新高亮，`ensureConsoleCoverage` 与渲染计算各执行一次，加载越多越卡。
- 加载更多失败时写入全局 `error`，整个列表被错误页替换。
- `--mac-surface-hover` 未定义，引用它的悬停背景实际为透明。
- 首页标题关闭时 `MainHeroBanner` 根节点仍渲染，在 flex 布局中多出一段间距。

## 变更内容

- 日志弹窗：
  - 新增底部哨兵，通过 `IntersectionObserver`（root 为 `.modal-body`，预留 320px）自动加载下一页；内容不足一屏时重新观察以继续加载；空页时按实际条数收敛，避免重复请求。
  - 加载更多的失败单独记录为 `loadMoreError`，只在底部提示并提供重试；全部加载后显示总条数。
  - 错误体解析结果按日志对象缓存（`WeakMap`），详情高亮结果按文本缓存，重新加载时清空。
  - 日志卡片启用 `content-visibility: auto`，减少视口外卡片的布局与绘制开销。
  - 布局调整：移除重复筛选标签与光斑，条数并入标题；卡片左侧色条改为严重程度淡渐变，去掉与标题重复的状态码标签及占位类型；去掉代码块仿窗口圆点，复制按钮并入详情工具栏且无内容时隐藏。
  - 预览模式补充 32 条分页模拟日志，覆盖已读／未读混合场景。
- 可用性页：统计卡与供应商卡的左侧色条（及亮色主题统计卡底部色条）改为按状态着色的 135° 淡渐变。
- 首页：Hero 无内容时不渲染；平台标签补充悬停底色与焦点环；热力图圆角、描边、阴影与卡片统一（内边距与 `useAdaptiveHeatmap` 计算绑定，未改动）；首页底部留白收紧。
- 全局：在亮暗主题中定义 `--mac-surface-hover`。
- 新增文案：`loadMoreFailed`、`retryLoadMore`、`allLoaded`（中英文）。

## 验证结果

- 前端单元测试 76 个文件、590 个用例全部通过，`vue-tsc --noEmit` 通过。
- 样式效果已在开发预览中对照截图调整；真实后端数据的滚动分页需在桌面版中验证。

## 发布

- 随 Code Switch R v2.11.25 发布。

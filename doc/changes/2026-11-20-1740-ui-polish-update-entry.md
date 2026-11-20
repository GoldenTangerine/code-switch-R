<!--
@name: 页面样式与更新入口变更
@Descripttion: 记录 Skills、OpenClaw、Hermes 页面样式优化、侧边栏菜单调整与版本号检查更新入口的实现和验证。
@version: 1.0.0
@Author: sm
@Date: 2026-11-20 17:40:00
@LastEditTime: 2026-11-20 17:40:00
@FilePath: doc/changes/2026-11-20-1740-ui-polish-update-entry.md
-->

# 页面样式优化与版本号检查更新

变更时间为 2026-11-20 17:40:00（Asia/Shanghai，UTC+08:00）。

涉及范围包括 Skills、OpenClaw、Hermes 记忆、设置页、侧边栏及中英文文案。

发布版本为 v2.12.4，同步各平台构建元数据和版本更新说明。

## 变更内容

- 新增公共组件 `PageHero`（左对齐标题与说明）和 `SegmentedTabs`（分段切换）。Skills、OpenClaw、Hermes 三个页面统一使用，去掉原先包裹 Tab 的整行外壳。
- 分段切换改用 grid 按内容计算列宽，并显式声明按钮宽高。`public/style.css` 中模板遗留的 `button { width: 60px; height: 30px }` 曾导致标签文字溢出，浅色系统下的 `#f9f9f9` 背景曾让「纳入管理」按钮文字不可见。
- `style.css` 全局按钮规则的排除列表新增 `segmented-tab`、`skill-action-btn`、`app-switch`、`skill-name`、`skill-link` 和 `status-card__version`，避免强制内边距破坏这些控件。
- OpenClaw：修复 `EnvPanel` 中行 id 计数器在首次使用后才声明导致 setup 抛错、环境变量页空白的问题；状态改为标签展示，配置文件路径单独一行；三个子面板的圆角、间距与小号按钮统一。
- Hermes：状态标签下方增加字符预算进度条，超出预算时以琥珀色提示；两组记忆设置改为并排卡片，当前编辑文件对应的卡片高亮。
- Skills：视图切换与搜索同行；目录名与技能名相同时不再重复展示；开关统一为主题色；分组标题显示数量；空状态改为虚线框。
- 侧边栏「日志」改名为「统计」并换用柱状图图标，「控制台」改名为「日志」并换用文档图标，「技能」改为「Skills」。
- 更新弹窗抽取为 `UpdateCheckModal`，打开即检查更新，负责下载与安装重启。设置页「立即检查」和侧边栏版本号共用该组件，检查失败时不再同时显示「检查中」。

## 验证结果

- `pnpm test:unit` 通过（78 个文件，607 个用例）。
- `pnpm exec vue-tsc --noEmit` 通过。
- 在浏览器预览中实测三个页面全部可见按钮无文字溢出，点击侧边栏版本号可打开更新弹窗。
- 浏览器预览无后端数据，技能卡片、真实的更新检查与下载安装未在桌面应用中手动验证。

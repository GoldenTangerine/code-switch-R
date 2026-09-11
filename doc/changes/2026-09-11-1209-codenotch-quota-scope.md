<!--
@name: 联动供应商范围与额度状态
@Descripttion: 记录全量联动推送与 Codenotch 本地筛选的配套变更。
@version: 1.0.0
@Author: sm
@Date: 2026-09-11 12:09:38
@LastEditTime: 2026-09-11 12:09:38
@FilePath: doc/changes/2026-09-11-1209-codenotch-quota-scope.md
-->
# 联动供应商范围与额度状态

变更时间：2026-09-11 12:09:38（Asia/Shanghai）

涉及范围：`services/codenotch_integration.go`、`services/traysnapshotservice.go`、联动回归测试和协议说明；配套 Codenotch 的显示范围、管理列表和订阅读取。

## 变更内容

- 全量订阅包含已启用及额度自动停用供应商，移除平台代理门槛，手动关闭仍排除。
- Gemini 转换保留额度自动停用字段；自定义 CLI 和隐藏平台不依赖代理配置。
- 版本 1 协议新增可选范围能力、额度状态和自动停用标记，不输出供应商配置或凭证。
- 保留原托盘行为、查询并发控制、缓存和租约回收；Codenotch 独立选择刘海范围，完整管理列表不被筛选。

## 验证结果

Go 联动、托盘和额度自动化回归已通过，包含新增的 Gemini 自动停用与恢复测试（`go test ./services -run 'TestCodenotch|TestTraySnapshot|Test.*QuotaAutomation' -count=1`）。Codenotch 使用隔离快照验证全量订阅、五种过滤、未知额度、旧版提示、修订复用和撤销；完整 macOS 应用测试受本机缺少 Xcode 限制。

## 发布

- 随 Code Switch R v2.11.22 发布，配套 Codenotch v1.6.18；同步应用版本、平台构建元数据与更新日志。

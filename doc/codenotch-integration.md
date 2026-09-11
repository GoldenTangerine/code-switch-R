<!--
@name: CodeNotch 联动说明
@Descripttion: 说明托盘共享快照的展示行为、协议与验证方法。
@version: 1.0.0
@Author: sm
@Date: 2026-09-08 17:18:00
@LastEditTime: 2026-09-08 17:18:00
@FilePath: doc/codenotch-integration.md
-->
# CodeNotch 联动

两端更新到包含联动代码的版本后，同时运行即可。CodeNotch 默认在原有供应商之后显示 Code Switch R 托盘供应商，可在 CodeNotch 设置中关闭联动。

有调用时展示正在调用的供应商，没有调用时展示默认供应商。平台可见范围、顺序以及自定义 CLI 名称由 Code Switch R 决定。悬停可查看调用状态、额度、余额、重置时间与供应商今日统计；不包含平台级预算汇总。

## 数据与生命周期

`TraySnapshotService.GetSnapshot` 是托盘供应商数据的统一来源。后台每 250 毫秒检查运行态，额度按供应商缓存 60 秒，今日统计在同平台供应商之间共享 60 秒缓存；查询配置变化或没有缓存的供应商立即异步加载。普通供应商配置复用现有版本指纹，Gemini 只复制查询所需字段。托盘关闭不影响刷新。慢查询不会阻塞运行态发布，同一配置不并发重复查询。

运行中的禁用供应商仍按原 ID 展示，不借用同名供应商的身份或额度。仅无 ID 且名称唯一时允许兼容历史名称；旧统计也仅在名称唯一时回退。托盘心跳内容未变化时仅更新倒计时，不重复重建供应商视图；预算定时器仅在周期改变时重新创建。

macOS 发布 `~/Library/Caches/code-switch/tray-snapshot-v1.json`，目录权限 0700、文件权限 0600，使用临时文件原子替换。文件只包含展示字段，不包含供应商配置、凭证或请求内容；远端原始错误和脚本附加诊断不导出。

缓存目录被清理或首次写入失败后会继续尝试发布，并自动重建目录。退出时取消额度 HTTP 请求和脚本执行，等待工作线程结束后才继续关闭数据库。脚本执行另有 30 秒上限。

CodeNotch 对未开始的额度显示“周期尚未开始”，不会将其当作 0% 的有效读数。品牌图标使用随应用打包的 Lobe 1.73.0 离线 PNG 资源，保留与托盘相同的彩色别名；未知图标使用通用图形。

顶层字段为 `version=1`、`session`、`sequence`、`heartbeatAt`（Unix 毫秒）和有序 `platforms`。平台包含 `platform/name/icon/error/providers`；供应商包含 `providerId/providerName/icon/activeRequests/status/loading/updatedAt/quotas/stats`。额度区分 `progress/balance/error`，统计沿用 `ProviderDailyStat` 数值字段。

CodeNotch 监听目录并每 500 毫秒兜底检查，正常运行时供应商变化目标为 1 秒内可见。退出删除当前进程的文件；异常退出超过 3 秒没有心跳时隐藏。新进程使用新的会话标识，旧序号和已退役会话不能覆盖新结果。联动数据不写入 CodeNotch 的供应商配置或历史缓存。

## 验证

- Go：`go test ./services -run 'Test(TraySnapshot|ProviderConcurrencyService|ProviderDailyStats|ResolveFiveHourQuotaStatusByProviderAt|CostByProvider|Budget)' -count=1`
- 前端：在 `frontend` 执行 `pnpm exec vue-tsc --noEmit` 和 `pnpm test:unit src/components/Tray`。
- CodeNotch：完整 Xcode 环境执行 `make test`，包含 `CodeSwitchBridgeTests`。
- 手动验收：交换两端启动顺序，切换及并发调用供应商，关闭托盘，退出并重启 Code Switch R，确认原有条目始终在前、联动自动隐藏和恢复。

## 全量供应商与本地筛选

新版 Codenotch 在联动开启期间持续订阅 `enabled` 全量快照，接收供应商开关开启或 `QuotaAutoDisabled=true` 的供应商。平台无需开启代理托管，隐藏平台和自定义 CLI 同样包含；手动关闭的供应商不包含。原托盘快照的活跃／默认选择规则不变。

元信息增加可选 `providerScope: "enabled-or-quota-disabled"`，协议版本仍为 1。供应商增加可选 `quotaState`（`available`、`exhausted`、`unknown`）和 `quotaAutoDisabled`。自动额度停用直接标记耗尽；其他有效额度复用额度自动化判断，任一有限额度剩余不大于零即耗尽，零余额不因 `active=false` 被丢弃，无有效结果为未知。查询错误不作为零额度。

Codenotch 管理列表保留全量，刘海可选跟随托盘、全部、仅未耗尽、仅已耗尽、仅活跃（`activeRequests > 0`）；未知额度在仅未耗尽中保留。活动或等待会话可临时显示，手动隐藏优先。切换范围只做本地筛选；关闭联动或退出时撤销租约，发送端沿用缓存、并发限额和额外任务回收。

旧客户端忽略新增字段；新客户端遇到缺少范围标记的旧发送端时提示升级，仍使用可得的全量数据或原托盘回退。旧 `enabled` 偏好继续映射为全部供应商，不重置排序或隐藏记录。

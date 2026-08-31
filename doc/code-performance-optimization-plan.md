<!--
@name: Code Switch R 代码与性能优化计划
@Descripttion: 基于当前代码证据规划可独立执行、验证和回退的代码质量与性能优化批次
@version: 1.0.0
@Author: sm
@Date: 2026-08-31 15:01:40
@LastEditTime: 2026-08-31 15:01:40
@FilePath: doc/code-performance-optimization-plan.md
-->

# Code Switch R 代码与性能优化计划

## 1. 文档信息

- 创建时间：2026-08-31 15:01:40 CST（Asia/Shanghai）
- 最后更新时间：2026-08-31 22:07:28 CST（Asia/Shanghai）
- 当前状态：首轮已规划批次全部完成，保留待分析项
- 当前执行批次：无（B13 已完成）
- 已完成批次数：14
- 待处理批次数：0
- 分析基线：基于 2026-08-31 当前代码；`go test ./services/...`、`go test ./resources/model-pricing/...`、`pnpm test:unit` 均通过
- 文档范围：首版可靠框架和第一批高价值任务，不代表完整仓库审计

## 2. 优化目标

### 代码质量目标

- 降低代理调度、数据库写入和设置页数据编排的单函数复杂度与重复逻辑。
- 为高风险重构先建立可执行的行为基线，避免依赖人工记忆判断兼容性。
- 只在能形成清晰职责边界时拆分文件或函数，不进行无收益的大规模重写。

### 性能目标

- 先测量代理热路径、供应商快照、日志聚合、SQLite 写入和前端轮询的实际成本。
- 每项性能优化记录相同环境、数据规模和命令下的前后耗时、分配量、查询计划或调用次数。
- 不设置最低收益百分比；只要对比可重复即可记录，但没有前后数据不得标记为性能优化完成。

### 可维护性目标

- 每个批次只处理一个主题，能在一次独立任务中完成、验证和回退。
- 稳定使用 OPT 和批次编号，使后续会话可以从文档状态继续执行。
- 新发现的问题追加编号，不重排既有问题和批次。

### 功能保持不变原则

- 优化不改变现有功能、接口、配置、数据格式、路由协议、调度语义和用户可见结果。
- 测量无法证明收益时，保留现状并记录结果，不以“理论上更快”为由修改代码。
- 结构重构与性能优化分批执行，不在同一批次顺带改变无关行为。

## 3. 不可变行为

- 公共 Go API、现有 Wails 方法名称、参数顺序、参数类型和返回格式保持不变；允许新增兼容型 Wails 聚合方法，但不得替换或删除现有方法。
- UI 路由、页面功能、显示结果、操作反馈、主题、国际化和交互时序的用户可见语义保持不变。
- `~/.code-switch/` 下配置文件的路径、JSON 字段、默认值和兼容读取逻辑保持不变。
- SQLite 现有表、列、数据内容和迁移兼容性保持不变；测量证明有效时可规划新增可回退索引，但实施前必须单独确认。
- Provider 的 Level、同级顺序、强制优先、轮询起点、会话亲和、并发限制、失败降级和切换通知语义保持不变。
- 黑名单开关、失败来源、计数阈值、等级、持续时间、宽恕、自动恢复、手动恢复和成功清零语义保持不变。
- HTTP 路由、状态码、响应头、错误体透传、SSE 刷新、客户端中断和响应开始后禁止降级的语义保持不变。
- 请求日志字段、成功/失败/排除判定、Token、成本、首字耗时、吞吐和用量统计口径保持不变。
- 请求负载捕获、截断、脱敏、默认关闭状态和最大长度保持不变，除非另立需求明确变更行为。
- 默认配置、旧数据兼容、Claude/Codex/Gemini/自定义 CLI 兼容性和受支持平台保持不变。

## 4. 当前架构概览

| 模块 | 职责与关键入口 | 主要数据流与依赖 | 已有测试保护 |
|---|---|---|---|
| 应用初始化 | `main.go:main` 按顺序初始化数据库、双队列、业务服务、代理和后台任务 | `InitDatabase` → `InitGlobalDBQueue` → Service 构造/绑定 → `ProviderRelayService.Start`；关闭时停止服务并排空队列 | `main_test.go` 覆盖窗口生命周期；服务测试复用生产数据库初始化 |
| 供应商存储 | `ProviderService.LoadProviders/SaveProviders`、`providerstore.go` | SQLite `providers_store` → 内存快照 → 代理、健康检查和 Wails；每秒检查外部变化 | 保存、迁移、完整可变字段隔离和外部刷新均有测试；B04 建立基线，B13 覆盖有序 raw 指纹、保存一致性、空哨兵、损坏 payload 和 custom 语义 |
| HTTP 代理 | `ProviderRelayService.registerRoutes`、`proxyHandler`、`forwardRequestWithPlan`、`geminiProxyHandler`、`customCliProxyHandler` | Gin 请求 → 读取请求体 → Provider 筛选/排序 → 上游转发/流处理 → 黑名单与请求日志 | 代理行为、错误透传、流转换和负载脱敏测试较完整；B03 新增热路径行为与性能基线 |
| 调度与黑名单 | `buildProviderAttemptGroups`、轮询函数、会话亲和函数、`BlacklistService` | Provider 快照、AppSettings 和黑名单快照共同决定尝试顺序；失败/成功经写队列落库并刷新运行时快照 | 会话亲和、轮询预览、失败透传、黑名单计数与健康检查集成已有测试，但缺统一组合矩阵 |
| SQLite 写入 | `DBWriteQueue.worker`、`batchWorker`、`ExecTxGroup` | `GlobalDBQueue` 串行处理异构写入；`GlobalDBQueueLogs` 以最多 50 条、10ms 窗口批量写入日志；部分运行时维护与改名同步仍直连写入；B14 通过 DSN `_pragma` 让每个池连接应用 30 秒 `busy_timeout` | `dbqueue_test.go` 覆盖队列语义与 3 个子基准；B07/B14 覆盖双队列、直写事务、5 连接属性和逐连接 timeout 竞争基线 |
| 日志统计 | `LogService` 的分页、汇总、趋势、Provider/模型统计方法 | Wails 调用 → SQLite 读取 → Go 聚合/定价适配 → 前端图表和列表；B06 仪表盘聚合复用一次主范围日志 | 日志范围、来源、Provider、模型、定价、热力图和分页测试较多；B05/B06 覆盖筛选等价、查询计划、调用次数和 81 个子基准 |
| 前端主页面 | `Main/Index.vue` 组合 cards、composables 和 modals | 多组 Wails 读取、Provider 事件和定时轮询驱动卡片状态 | Main composables、ProviderCard、配额、统计和事件有较多 Vitest |
| 日志页面 | `Logs/Index.vue` + `useLogsPageData` 等 composables | 单次刷新并发拉取分页、摘要、趋势、模型、Provider 和筛选选项 | Logs composables、services 和工具函数已有较完整 Vitest |
| 设置页面 | `General/Index.vue` | AppSettings、更新、黑名单、导入、WebDAV、预算和模型路由集中编排 | 共享工具和 service 有测试；页面初始化、保存编排及事件生命周期没有独立测试 |

当前测试基线：

- `go test ./services/...`：通过；仅出现本机 macOS 链接目标版本警告。
- `go test ./resources/model-pricing/...`：通过。
- `pnpm test:unit`：71 个测试文件、549 项测试全部通过。
- 仓库已有 4 个代理相关 Go Benchmark；B02 新增 3 个 DBWriteQueue 子基准，B03 新增 32 个代理热路径子基准，B04 新增 15 个 Provider 快照子基准，B05/B06 共维护 81 个日志仪表盘前后对照子基准；B07 新增 3 个 SQLite 写竞争场景。

## 5. 优化问题总表

| ID | 模块 | 文件位置 | 类型 | 问题 | 代码证据 | 影响 | 优先级 | 风险 | 所属批次 | 状态 |
|---|---|---|---|---|---|---|---|---|---|---|
| OPT-001 | HTTP 代理 | `services/providerrelay.go` 的 `proxyHandler`、`forwardRequestWithPlan`、Gemini 与自定义 CLI Handler | 代码质量 | B10 已部分收敛通用 Handler，但代理核心文件职责仍过多，继续拆分缺少独立收益边界 | 文件由 11,447 行降至 11,402 行，`proxyHandler` 由 737 行降至 620 行；`forwardRequestWithPlan`、Gemini 和自定义 CLI 路径未进入本批 | 通用调度理解成本下降，但全文件协议与平台职责仍需后续按证据单独分析 | P1 | 高 | B10 | 待分析 |
| OPT-002 | 供应商调度 | `services/providerrelay.go` 的 `providerAttemptState`、`stopProviderAttemptIfNeeded`、`proxyHandler` | 代码质量 | 已优化：四条通用调度分支共享尝试状态、成功/失败记录、响应开始/客户端中断停止和最终错误透传 | 分支内不再重复维护 last error/provider/duration/attempts；429 优先、上游原始错误透传、拉黑计数、会话释放和停止条件由窄 helper 统一；分支特有顺序、重试、日志和通知保留 | 降低失败语义漂移风险，同时保持所有调度与协议结果不变 | P0 | 高 | B10 | 已完成 |
| OPT-003 | 调度测试 | `services/providerrelay_scheduling_matrix_test.go`、`services/providerrelay_test.go`、`services/providerrelay_error_passthrough_test.go` | 测试 | 已建立统一的调度开关组合行为矩阵 | B01 建立 5 个场景；B10 补充“固定拉黑 + 已绑定会话”组合，共 6 个场景，固定调用顺序、失败计数、成功 Provider 和绑定迁移 | 为通用调度收敛和后续修改提供行为基线 | P0 | 中 | B01、B10 | 已完成 |
| OPT-004 | 代理热路径 | `services/providerrelay.go:4166-4175`、`:5405-5486`、`:6398-6491`、`services/providerrelay_hotpath_benchmark_test.go` | 内存 | 已测量：请求读取、非流式 Hook 和可选负载捕获均随体积增加分配；是否影响真实请求吞吐仍待端到端负载确认 | 1 MiB 请求读取 0.272-0.286 ms/约 5.24 MB；非流式 Hook 且关闭捕获 2.696-2.802 ms/约 8.39 MB；8 MiB 附近分别为 1.750-2.516 ms/约 41.70 MB 和 19.663-21.276 ms/约 66.90 MB | 大负载下会增加单请求堆分配；当前数据未包含网络、调度和数据库写入，不能直接外推整机吞吐 | P1 | 高 | B03 | 待分析 |
| OPT-005 | 供应商快照 | `services/providerservice.go:194-209`、`:248-317`、`:706-724`、`services/provider_snapshot_benchmark_test.go` | 内存 | 已测量：快照命中成本几乎全部来自完整深拷贝；是否影响真实代理吞吐仍待结合 Provider 数量和请求率判断 | 1/10/100 个 Provider 的 `cloneProviders` 分别为 3.806-3.974 µs/5.7 KB、38.181-38.996 µs/57.2 KB、385.745-435.149 µs/568.0 KB；`LoadProviders` 快照命中范围基本一致 | 高频请求会按 Provider 数量线性产生分配；当前样本约 1.6 KB/Provider，不代表用户实际配置复杂度 | P1 | 中 | B04 | 待分析 |
| OPT-006 | 供应商快照 | `services/providerservice.go` 的 `providerStoreFingerprint`、`refreshProviderSnapshots`，`services/providerstore.go` 的 `saveProvidersToStoreWithFingerprint`、`providerStoreRowsFingerprint` | I/O | 已优化：SQLite 平台直接哈希有序存储行；保存端返回本次事务对应指纹；raw 非语义变化只同步指纹，custom 保留 Provider 语义指纹 | 1/10/100 个 Provider 稳定刷新中位数从 30.325/226.384/2,077.316 µs 降至 12.345/61.383/446.699 µs；allocs/op 从 238/2,169/约 21,525 降至 38/178/约 1,624 | 固定夹具中耗时约降 59.3%/72.9%/78.5%；消除保存后查询竞态窗口，并保持 nil/空、排序、ID 回填、损坏 payload 和 custom 语义 | P1 | 中 | B04、B13 | 已完成 |
| OPT-007 | SQLite 双队列 | `services/dbqueue_test.go`、`services/dbqueue.go:168-749` | 测试 | 已建立队列专属正确性、并发和性能基线 | B02 覆盖顺序、事务回滚、批量、取消、关闭排空及拒绝写入；3 个子基准记录端到端耗时、分配和现有 QueueStats | 后续队列修改已有可重复的行为和性能对照 | P0 | 中 | B02 | 已完成 |
| OPT-008 | SQLite 写入 | `services/providerservice.go:626-676`、`services/geminiservice.go:729-763`、`services/healthcheckservice.go:1358-1381`、`services/logstorage_service.go` | 数据库 | 已审计并测量：运行时简单写入与复合事务会和双队列争用 SQLite writer；初始化、迁移、维护和独立数据库直写不能按同一规则处理 | B07 旧连接设置下加入直写出现 66-595/596 次 `SQLITE_BUSY`；B14 逐连接 timeout 后连续 5 轮均为 0/596，事务与最终数据一致 | 竞争根因已由 B14 修复；当前合成数据不支持继续把复合事务强行迁入队列，保留真实调用观察 | P1 | 高 | B07、B14 | 已完成 |
| OPT-009 | 日志仪表盘 | `services/logservice.go`、`frontend/src/services/logs.ts`、`useLogsPageData.ts`、`services/log_dashboard_benchmark_test.go` | 数据库 | 已优化：新增兼容聚合方法复用一次主范围日志，前端刷新不再分别调用四个统计方法 | 100k Proxy 冷缓存 5 轮中位数从 3.435 s、2,617.259 MB、36,810,567 allocs 降至 1.248 s、783.718 MB、11,193,116 allocs；完整日志对象由 170,031 降至 50,019 | 固定夹具中耗时约降 63.7%、分配字节约降 70.1%；旧 Wails 方法、数据格式和统计结果全部保留 | P0 | 高 | B05、B06 | 已完成 |
| OPT-010 | 设置页面 | `frontend/src/components/General/Index.vue`、`General/composables/useGeneralPageData.ts` | 前端 | 已优化：初始化加载、事件订阅和卸载清理已从 4,398 行页面中抽离为具体 composable；其余表单业务按批次边界保留 | 新增 75 行编排模块和 6 项契约测试，Loader 通过明确参数注入，未拆分预算、更新、黑名单、导入或 WebDAV 表单 | 初始化性能调整已有独立测试边界，避免继续扩大页面内生命周期逻辑 | P1 | 中 | B09 | 已完成 |
| OPT-011 | 设置页加载 | `General/Index.vue` 的 `useGeneralPageData`、`General/composables/useGeneralPageData.test.ts` | 前端 | 已优化：AppSettings 保持先行，模型路由、版本、更新、黑名单、导入和 WebDAV 六项独立读取并行执行 | 相同 100 ms 假延迟下，旧七段串行编排为 700 ms，新编排为 200 ms；单项拒绝不阻断其他读取，重复挂载和初始化中卸载均有测试 | 等延迟模型的等待层数由 7 层降为 2 层；数据仅证明编排收益，不外推真实 Wails 延迟 | P2 | 中 | B09 | 已完成 |
| OPT-012 | 主页面轮询 | `main.go` 的 `emitLinuxMainWindowVisibility`、`Main/Index.vue` 的并发状态 Timer、`useMainPageShell.ts` 的 `pollingLifecycle`、`mainPollingLifecycle.test.ts` | 前端 | 已优化：主窗口隐藏时暂停主页面周期轮询，恢复时先刷新一次再恢复原周期 | 固定假 Timer 测量中，原配置每 60 秒产生 41 个高层 Wails 轮询轮次；新生命周期可见时保持 41 个、隐藏时新增 0 个、恢复后保持 41 个，重复信号未重复刷新或启动 Timer | 消除延迟销毁或隐藏窗口期间的周期 Wails 调用，同时保持可见态节奏 | P1 | 低 | B08 | 已完成 |
| OPT-013 | SQLite 事务组 | `services/dbqueue.go` 的 `ExecTxGroup`、`services/dbqueue_test.go` | 并发 | 已修复：事务正常完成时等待全部任务结果通道关闭后再返回，worker 不再访问调用方任务对象 | 修改前 1000 次双任务切片顺序复用在 `-race` 下稳定报告 `ExecTxGroup` 写与 `commitTxGroup` 读竞态；修改后普通、20 次重复和竞态测试均通过，2000 条写入完整 | 消除正常返回后顺序复用切片的竞态和潜在 `send on closed channel`；30 秒超时后任务继续执行的既有语义不变 | P1 | 中 | B11 | 已完成 |
| OPT-014 | 负载脱敏 | `services/providerrelay.go` 的 payload quick checks、`maybeSanitizeRequestLogPayload`、`prepareRequestLogPayloadForPersistence` | CPU | 已优化：ASCII payload 使用无分配折叠扫描，Unicode 回退原 quick regex；会话标识和已捕获请求体不再重复脱敏 | 1 MiB quick check 中位数：会话 131.527 ms → 0.949 ms，敏感词 71.852 ms → 0.678 ms；1 MiB raw 410.463 ms → 2.051 ms、sanitized 1.334 s → 334.701 ms；近 8 MiB raw 3.219 s → 16.699 ms、sanitized 10.498 s → 2.626 s | 固定夹具中 raw 约降 99.5%，sanitized 约降 75%；脱敏分配字节约降 30%，捕获关闭保持百纳秒级 | P1 | 中 | B12 | 已完成 |
| OPT-015 | SQLite 连接池 | `services/database.go:15-47`、`services/sqlite_runtime_write_contention_test.go` | 数据库 | 已优化：默认 DSN 使用 modernc `_pragma`，使连接池每个新连接继承现有 30 秒 `busy_timeout` | 同时持有 5 个连接均返回 30000，20 次重复通过；默认配置压力组 5 轮均为 596/596 成功、0 次 `SQLITE_BUSY/LOCKED` | 消除夹具中的快速写失败；写竞争现在表现为有界等待，默认配置总耗时中位数 106.120 ms，日志成功 P50/P95 为 42.234/50.176 ms | P0 | 中 | B14 | 已完成 |

说明：没有测量数据的性能类问题显式标记“待测量”；已测量项只按基准适用范围描述，不直接外推为整机瓶颈。

## 6. 批次总览

| 批次 | 目标 | 包含问题 | 前置批次 | 风险 | 预计范围 | 状态 |
|---|---|---|---|---|---|---|
| B01 | 固定通用代理调度行为 | OPT-003 | 无 | 低 | 代理调度矩阵测试 | 已完成 |
| B02 | 建立双队列正确性和性能基线 | OPT-007 | 无 | 低 | DBWriteQueue 专属测试与 Benchmark | 已完成 |
| B03 | 测量代理热路径分配与日志成本 | OPT-004 | 无 | 低 | 代理 Benchmark 数据，不改生产行为 | 已完成 |
| B04 | 测量供应商克隆与快照刷新成本 | OPT-005、OPT-006 | 无 | 低 | ProviderService/Store Benchmark，不改生产行为 | 已完成 |
| B05 | 测量日志仪表盘重复聚合 | OPT-009 | 无 | 低 | LogService 基准、查询计划和调用次数 | 已完成 |
| B06 | 以兼容 Wails 方法实现单次日志聚合 | OPT-009 | B05 | 中 | LogService、前端 logs service、Logs composable 与测试 | 已完成 |
| B07 | 审计并测量运行时 SQLite 直写竞争 | OPT-008 | B02 | 中 | Provider 改名、健康历史及其他运行时写入 | 已完成 |
| B08 | 暂停主页面隐藏态周期轮询 | OPT-012 | 无 | 低 | Main 页面轮询生命周期与 Vitest | 已完成 |
| B09 | 抽离设置页数据编排并行化边界 | OPT-010、OPT-011 | 无 | 中 | General 初始化编排、composable 与 Vitest | 已完成 |
| B10 | 收敛通用代理调度内核 | OPT-001、OPT-002 | B01 | 高 | 仅 Claude/Codex/GrokBuild 通用 Handler 和测试 | 已完成 |
| B11 | 收紧事务组任务所有权边界 | OPT-013 | B02 | 中 | ExecTxGroup 内部任务与回归测试 | 已完成 |
| B12 | 优化负载脱敏快速检查与重复处理 | OPT-014 | B03 | 中 | payload 脱敏函数、测试和 B03 基准 | 已完成 |
| B13 | 以稳定存储内容计算 Provider 指纹 | OPT-006 | B04 | 中 | ProviderStore 指纹、快照刷新与基准 | 已完成 |
| B14 | 让 SQLite `busy_timeout` 对连接池逐连接生效 | OPT-015 | B07 | 中 | 默认数据库 DSN、连接池验证与 B07 复测 | 已完成 |

## 7. 分批实施计划

### B01：通用代理调度行为基线

#### 目标

只为现有 Provider 尝试顺序、降级终止条件和副作用建立统一行为测试，不修改生产逻辑。

#### 对应问题

- OPT-003

#### 前置条件

- 重新读取 `proxyHandler`、轮询、会话亲和和黑名单相关测试。
- 确认当前测试基线通过。

#### 涉及范围

- `services/providerrelay_test.go`
- `services/providerrelay_error_passthrough_test.go`
- 必要时新增一个仅测试使用的辅助文件，不修改生产代码。

#### 当前证据

- `proxyHandler` 根据黑名单模式和会话亲和状态进入多套 Provider 循环。
- 已有测试覆盖单项行为，但没有在同一数据集下固定主要开关组合的尝试顺序、失败次数、通知和最终响应。

#### 修改方案

- 使用表驱动测试构造相同 Provider 列表和可控 `httptest` 上游。
- 覆盖黑名单开关、轮询开关、会话亲和、强制优先、已拉黑跳过和跨 Level 降级的代表性组合。
- 固定成功清零、失败计数、Provider 切换通知、上游错误透传、客户端中断以及响应开始后停止降级的行为。
- 只断言外部行为和可观察副作用，不复制生产调度算法到测试中。

#### 明确不修改

- 不修改 Provider 排序、轮询起点、重试次数或黑名单阈值。
- 不修改任何 Handler、日志字段、状态码或错误文案。
- 不为 Gemini 和自定义 CLI 扩大本批范围。

#### 验证方式

- 单元测试：运行新增表驱动测试。
- 最小回归测试：运行全部 `providerrelay` 相关测试。
- 性能基准：本批不做性能结论。
- 行为一致性检查：记录每个组合的尝试顺序、终止原因、HTTP 结果和黑名单副作用。

#### 验收标准

- 主要调度组合均有稳定、可重复的自动化断言。
- 新测试在当前生产代码上通过，且不依赖执行顺序或真实网络。
- `go test ./services/...` 全部通过。

#### 风险与回退

- 风险：测试绑定非契约性的内部日志或实现细节。
- 控制：只断言请求顺序、响应、持久化结果和通知等外部行为。
- 回退：删除本批新增测试文件或测试用例即可，不涉及生产逻辑和数据。

#### 执行清单

- [x] 修改前重新读取相关代码
- [x] 确认现有行为
- [x] 补充必要测试
- [x] 完成最小修改
- [x] 运行验证
- [x] 记录前后对比
- [x] 更新问题和批次状态
- [x] 更新变更文档

#### 执行记录

- 开始时间：2026-08-31 15:13:53 CST（Asia/Shanghai）
- 完成时间：2026-08-31 15:17:39 CST（Asia/Shanghai）
- 实际修改：新增 `services/providerrelay_scheduling_matrix_test.go`，使用同一组 4 个 Provider 和可控 `httptest` 上游覆盖 5 个调度组合；未修改生产代码。
- 验证结果：新增 5 个子测试通过；`go test -race ./services/ -run '^TestProxyHandlerSchedulingBehaviorMatrix$' -count=1` 通过；`go test ./services/...` 通过。
- 性能对比：本批为行为基线批次，不作性能结论。优化前无统一组合矩阵，优化后新增 1 个表驱动测试、5 个代表场景。
- 行为证据：固定了实际 Provider 调用顺序、成功响应来源、最后使用 Provider、固定黑名单失败计数和会话绑定迁移；响应开始、客户端中断、最终错误透传和通知目标选择继续由既有专项测试保护。
- 遗留问题：B10 的生产调度代码收敛尚未执行，本批只提供低风险重构所需的行为保护。

### B02：SQLite 双队列正确性与性能基线

#### 目标

为 `DBWriteQueue` 建立专属正确性、并发和性能基线，不改变生产队列实现或统计字段。

#### 对应问题

- OPT-007

#### 前置条件

- 使用临时 SQLite 数据库，不读取用户实际数据库。
- 确认单次队列与日志批量队列的当前配置和关闭顺序。

#### 涉及范围

- 新增 `services/dbqueue_test.go`
- `services/dbqueue.go` 仅在测试暴露必要性被证实时另立后续修改，不在本批顺带调整。

#### 当前证据

- 双队列各自拥有 worker，日志队列按 50 条或 10ms 提交。
- 现有测试通过 ProviderStore 和日志写入间接使用队列，没有直接覆盖取消、排空、事务组回滚和并发 Shutdown。
- `QueueStats` 的提交耗时不包含入队等待，不能直接作为端到端延迟基准。

#### 修改方案

- 测试单次写入顺序、事务组原子回滚、批量提交、上下文取消、队列关闭后拒绝写入及关闭排空。
- 使用 `go test -race` 覆盖并发入队与关闭的代表性场景。
- 增加 Benchmark，分别记录单次、批量和事务组的吞吐、端到端延迟、`B/op` 与 `allocs/op`。
- 基准使用调用方计时计算入队到结果返回的延迟，并单独记录现有 `QueueStats`，不混用两种口径。

#### 明确不修改

- 不修改队列容量、批量上限、10ms 窗口或 30 秒超时。
- 不修改 `QueueStats` 字段、关闭顺序或双队列架构。
- 不引入第三方压测或统计依赖。

#### 验证方式

- 单元测试：`go test ./services/ -run 'TestDBWriteQueue' -count=1`。
- 最小回归测试：`go test ./services/...`。
- 性能基准：`go test ./services/ -run '^$' -bench 'BenchmarkDBWriteQueue' -benchmem -count=5`。
- 行为一致性检查：验证提交顺序、原子性、错误传播和关闭结果。

#### 验收标准

- 单次、批量、事务组、取消和关闭均有专属测试。
- 基准命令可重复执行并记录测试环境、数据量和 5 次结果。
- 本批只建立事实基线，不将现有统计直接认定为性能瓶颈。

#### 风险与回退

- 风险：并发测试存在时间敏感或偶发失败。
- 控制：使用通道和条件同步，不依赖任意 `Sleep` 判断完成。
- 回退：删除本批新增测试文件，不影响生产队列。

#### 执行清单

- [x] 修改前重新读取相关代码
- [x] 确认现有行为
- [x] 补充必要测试
- [x] 完成最小修改
- [x] 运行验证
- [x] 记录前后对比
- [x] 更新问题和批次状态
- [x] 更新变更文档

#### 执行记录

- 开始时间：2026-08-31 15:24:55 CST（Asia/Shanghai）
- 完成时间：2026-08-31 15:34:12 CST（Asia/Shanghai）
- 实际修改：新增 `services/dbqueue_test.go`，使用临时 SQLite 覆盖单次顺序、事务组成功/回滚、50 条并发批量写入、单次/批量上下文取消、单次/批量关闭排空和关闭后拒绝写入；未修改生产代码。
- 验证结果：目标测试通过并连续运行 20 次稳定；`go test -race ./services/ -run 'TestDBWriteQueue' -count=1`、`go test ./services/...` 均通过，仅有既有 macOS 链接目标版本警告。
- 测量环境：macOS 27.0、Apple M4、darwin/arm64、Go 1.25.5；临时 SQLite WAL、单连接、队列容量 5000；默认 1 秒基准窗口，批量场景使用 `RunParallel`。
- 性能结论：以下仅为 B02 基线，`QueueStats` 只测提交阶段，不能与调用方端到端耗时直接比较，也不据此认定现有实现存在性能瓶颈。

| 场景 | 5 次迭代数 | 5 次端到端 ns/op | 5 次 ops/s | B/op | allocs/op |
|---|---|---|---|---|---|
| 单次写入 | 32470、31893、30561、28062、33531 | 37038、35034、36718、37756、35490 | 26999、28543、27234、26486、28177 | 657-658 | 13 |
| 批量写入 | 1194、1190、1184、1183、1177 | 1006037、1000656、1005640、1006196、1002918 | 994.0、999.3、994.4、993.8、997.1 | 706-717 | 15 |
| 双语句事务组 | 26226、27240、27532、25701、23871 | 44606、42787、47471、48801、43626 | 22419、23371、21066、20491、22922 | 1491-1493 | 32 |

- QueueStats 对照：单次 `queue-avg-ms` 为 0-0.0000654、`queue-p99-ms` 为 0；批量分别为 0-0.06797 和 0-0.1，平均每次提交 9.941-10.00 条；事务组分别为 0-0.0006615 和 0。
- 前后对比：优化前没有 DBWriteQueue 专属测试或基准；优化后新增 5 个测试入口（含 4 个单次/批量子场景）和 3 个子基准。
- 新发现：初始事务组基准复用任务切片时触发 worker `send on closed channel`；当前生产调用方每次新建切片，基准已按实际调用模式执行，风险登记为 OPT-013/B11。
- 遗留问题：B02 仅建立基线；不修改队列容量、批量窗口、超时、统计口径或实现。

### B03：代理热路径分配与日志成本测量

#### 目标

测量请求读取、响应 Hook、负载捕获和脱敏在不同负载下的实际 CPU、内存和代理准备成本，不实施优化。

#### 对应问题

- OPT-004

#### 前置条件

- B01 不是硬依赖，但执行前必须确认代理测试通过。
- 使用 `httptest` 和生成数据，不访问真实 Provider 或用户日志。

#### 涉及范围

- `services/providerrelay_test.go` 或新增代理 Benchmark 测试文件
- `services/providerrelay_payload_sanitize_test.go`
- 不修改 `services/providerrelay.go` 生产逻辑。

#### 当前证据

- 请求入口使用 `io.ReadAll` 保存完整请求体，以支持模型路由、重试和会话识别。
- 非流式响应存在 Hook 时使用 `io.ReadAll`，流式响应逐行执行多个 Hook。
- 负载捕获最多保存 8 MiB 请求体和响应体，并可能执行多组正则脱敏。

#### 修改方案

- 基准负载固定为 1 KiB、64 KiB、1 MiB，并额外记录接近 8 MiB 上限的内存结果。
- 分别测量捕获关闭、捕获开启且脱敏关闭、捕获和脱敏均开启三种配置。
- 分别覆盖非流式 JSON、典型 SSE 行和包含敏感字段的负载。
- 记录 `ns/op`、`B/op`、`allocs/op`；必要时保存 CPU/heap profile 命令和摘要，不提交 profile 二进制。
- 测量结果只更新 OPT-004 证据；确认具体热点后再新增实施批次和 OPT 编号。

#### 明确不修改

- 不改变 8 MiB 上限、捕获默认值、脱敏规则或请求体读取方式。
- 不减少日志、不修改 SSE Hook、不调整 GC 或 HTTP 超时。
- 不把静态代码形态直接写成已确认瓶颈。

#### 验证方式

- 单元测试：运行负载截断与脱敏现有测试。
- 最小回归测试：`go test ./services/...`。
- 性能基准：`go test ./services/ -run '^$' -bench 'BenchmarkRelay' -benchmem -count=5`。
- 行为一致性检查：基准前后输出字节、截断标记和脱敏结果完全一致。

#### 验收标准

- 每种负载和开关组合均有可重复数据。
- 测量环境、Go 版本、命令、样本数和结果写入本计划执行记录。
- 未实施生产优化时，OPT-004 保持“待分析”或根据数据更新为“待处理/放弃”。

#### 风险与回退

- 风险：微基准无法代表完整代理请求。
- 控制：同时提供纯函数基准和 `httptest` 端到端基准，并明确适用范围。
- 回退：删除 Benchmark 代码；测量记录保留在文档中。

#### 执行清单

- [x] 修改前重新读取相关代码
- [x] 确认现有行为
- [x] 补充必要测试
- [x] 完成最小修改
- [x] 运行验证
- [x] 记录前后对比
- [x] 更新问题和批次状态
- [x] 更新变更文档

#### 执行记录

- 开始时间：2026-08-31 15:43:27 CST（Asia/Shanghai）
- 完成时间：2026-08-31 16:04:13 CST（Asia/Shanghai）
- 实际修改：新增 `services/providerrelay_hotpath_benchmark_test.go`，包含 3 个行为测试入口和 32 个子基准；使用生成负载、生产内部函数和 `httptest`，未修改生产代码。
- 行为结果：捕获关闭、原始捕获、脱敏捕获、8 MiB 截断、非流式输出和 SSE 输出均保持现有结果；目标测试、`-race` 和完整 services 回归通过。
- 测量环境：macOS 27.0、Apple M4、darwin/arm64、Go 1.25.5；1 MiB 及以下使用默认基准窗口并独立运行 5 次，8 MiB 附近因冒烟单次达到 10 秒级，使用 `-benchtime=1x` 独立运行 5 次。
- 适用边界：以下是单进程微基准，不包含 Provider 调度、真实网络、SQLite 写入或整机并发，不能直接外推用户请求吞吐。

| 热路径场景 | 负载 | 5 轮 ns/op 范围 | B/op 范围 | allocs/op |
|---|---:|---:|---:|---:|
| 请求读取并恢复 Body | 1 KiB | 318.1-338.6 | 2,944 | 7 |
| 请求读取并恢复 Body | 64 KiB | 19,989-20,268 | 285,057 | 20 |
| 请求读取并恢复 Body | 1 MiB | 271,933-285,656 | 5,241,229-5,241,230 | 31 |
| 请求读取并恢复 Body | 8 MiB 附近 | 1,750,125-2,516,084 | 41,703,808-41,703,920 | 40-42 |
| 非流式直接复制 | 1 MiB | 55,289-60,262 | 1,050,002 | 14 |
| 非流式日志 Hook、捕获关闭 | 1 MiB | 2,695,972-2,801,704 | 8,389,418-8,389,434 | 44 |
| 非流式直接复制 | 8 MiB 附近 | 248,792-352,292 | 8,398,224 | 14 |
| 非流式日志 Hook、捕获关闭 | 8 MiB 附近 | 19,662,625-21,275,917 | 66,896,656-66,896,752 | 53-54 |

| 捕获场景 | 负载 | 5 轮 ns/op 范围 | B/op 范围 | allocs/op 范围 |
|---|---:|---:|---:|---:|
| 捕获关闭 | 1 KiB-8 MiB 附近 | 119-3,000 | 1,152 | 1 |
| 原始捕获 | 1 KiB | 343,997-345,713 | 4,226-4,236 | 4 |
| 脱敏捕获 | 1 KiB | 1,075,858-1,091,037 | 35,614-35,652 | 58 |
| 原始捕获 | 64 KiB | 26,065,420-27,795,419 | 197,858-198,413 | 4-5 |
| 脱敏捕获 | 64 KiB | 85,537,327-88,381,224 | 2,169,976-2,176,164 | 68-84 |
| 原始捕获 | 1 MiB | 419,085,958-451,244,445 | 3,147,824-3,155,997 | 5-19 |
| 脱敏捕获 | 1 MiB | 1,345,618,833-1,418,649,000 | 34,677,440-34,708,656 | 217-288 |
| 原始捕获 | 8 MiB 附近 | 3,142,767,542-3,431,393,000 | 25,176,584-25,202,520 | 6-49 |
| 脱敏捕获 | 8 MiB 附近 | 10,319,476,708-10,432,383,375 | 276,940,768-276,967,104 | 293-395 |

| 1 MiB 脱敏分解 | 5 轮 ns/op | B/op 范围 | allocs/op 范围 |
|---|---:|---:|---:|
| 干净负载完整脱敏 | 209,090,658-224,784,325 | 5,472-14,297 | 9-32 |
| 敏感负载完整脱敏 | 304,643,938-305,932,219 | 10,513,004-10,519,206 | 77-90 |
| 无匹配会话标识检查 | 135,046,797-157,595,905 | 177-3,705 | 0-6 |
| 无匹配敏感关键词检查 | 74,378,278-76,380,186 | 94-563 | 0-1 |

| 64 KiB SSE 场景 | 5 轮 ns/op | B/op 范围 | allocs/op 范围 |
|---|---:|---:|---:|
| 无 Hook | 56,251-59,658 | 276,045 | 1,551 |
| Hook、捕获关闭 | 923,740-1,004,310 | 429,447-430,108 | 3,078-3,079 |
| Hook、原始捕获 | 10,367,931-10,535,977 | 784,707-788,793 | 3,098-3,099 |
| Hook、脱敏捕获 | 32,483,355-32,741,431 | 1,689,936-1,700,530 | 3,902-3,908 |

- B12 对照原始值：1 MiB 原始捕获为 451.244、419.656、419.325、419.086、432.882 ms，脱敏捕获为 1365.896、1347.666、1345.619、1364.879、1418.649 ms；8 MiB 附近原始捕获为 3204.703、3187.370、3431.393、3238.649、3142.768 ms，脱敏捕获为 10387.716、10417.133、10319.477、10432.383、10370.900 ms。
- 热点证据：无匹配会话标识与敏感关键词检查合计已接近干净负载完整脱敏耗时；`prepareRequestLogPayloadForPersistence` 在从 `requestBodyBytes` 捕获后又对 `RequestBody` 调用一次 `maybeSanitizeRequestLogPayload`。
- 性能结论：默认关闭捕获时捕获收尾成本很低；开启捕获后，正则快速检查和重复请求脱敏是已确认热点，登记为 OPT-014/B12。请求读取和响应 Hook 的分配已建立基线，但是否需要修改仍保持待分析。
- 验证说明：一次将完整回归与高内存 `-race` 并行执行时完整回归失败且日志被截断，未作为结果；随后两项串行独立执行，`go test ./services/... -count=1` 与目标 `-race` 均通过。
- 遗留问题：B03 不实施优化；真实端到端收益必须在 B12 及后续完整代理负载中重新验证。

### B04：供应商快照克隆与刷新测量

#### 目标

分别测量请求侧深拷贝和后台指纹刷新成本，不预先选择缓存或克隆优化方案。

#### 对应问题

- OPT-005
- OPT-006

#### 前置条件

- 使用临时数据库与生成 Provider，不读取用户供应商配置。
- 测试必须包含带嵌套 Map、模型映射、配额配置和可选指针字段的 Provider。

#### 涉及范围

- `services/providerservice_test.go`
- `services/providerstore_test.go`
- 可新增一个 Provider Benchmark 测试文件；不修改生产代码。

#### 当前证据

- 快照命中后仍深拷贝全部 Provider，JSON-like Map 采用 Marshal/Unmarshal 克隆。
- 后台每秒对已加载平台计算指纹，当前实现读取、解析并重新序列化完整 Provider 列表。

#### 修改方案

- 以 1、10、100 个 Provider 测量 `cloneProviders` 的耗时与分配。
- 测量空闲快照刷新一次所需查询数、耗时、读取字节和分配。
- 增加行为测试证明快照返回值不能共享可变 Map、Slice 或指针字段。
- 比较“当前语义指纹”和“直接基于稳定存储行计算指纹”的可行性，但本批不替换实现。
- 根据结果新增独立实施批次；没有稳定差异时将候选延期或放弃。

#### 明确不修改

- 不返回共享可变对象，不改变外部修改检测周期。
- 不改变 Provider JSON 语义、字段类型或快照一致性。
- 不在测量批次引入新的缓存层、事件总线或依赖。

#### 验证方式

- 单元测试：快照深拷贝隔离、外部变化刷新和存储兼容测试。
- 最小回归测试：`go test ./services/...`。
- 性能基准：`go test ./services/ -run '^$' -bench 'BenchmarkProvider(Snapshot|Clone)' -benchmem -count=5`。
- 行为一致性检查：对克隆结果的修改不得污染快照或其他调用结果。

#### 验收标准

- 请求侧克隆与后台刷新有分离的可重复数据。
- 数据规模、嵌套字段和数据库环境写入执行记录。
- 未取得测量结果前不提交生产优化代码。

#### 风险与回退

- 风险：生成 Provider 与用户实际复杂度不一致。
- 控制：样本字段来源于当前 `Provider` 结构和测试数据，不读取真实配置。
- 回退：删除新增 Benchmark；保留测量结论。

#### 执行清单

- [x] 修改前重新读取相关代码
- [x] 确认现有行为
- [x] 补充必要测试
- [x] 完成最小修改
- [x] 运行验证
- [x] 记录前后对比
- [x] 更新问题和批次状态
- [x] 更新变更文档

#### 执行记录

- 开始时间：2026-08-31 16:15:03 CST（Asia/Shanghai）
- 完成时间：2026-08-31 16:25:11 CST（Asia/Shanghai）
- 实际修改：新增 `services/provider_snapshot_benchmark_test.go`，补齐 CLIConfig、Claude Desktop 路由、并发限制、额度配置和内部错误切片的深拷贝隔离测试，并增加 15 个子基准；未修改生产代码。
- 数据集：1/10/100 个生成 Provider，每个约 1.6 KB 语义 JSON，包含嵌套 Map、Slice、模型映射、请求覆盖和全部可选指针；SQLite 使用 TestMain 临时数据库，不读取用户配置。
- 测量环境：macOS 27.0、Apple M4、darwin/arm64、Go 1.25.5；命令 `go test ./services/ -run '^$' -bench 'BenchmarkProvider(Snapshot|Clone)' -benchmem -count=5`。

| Provider 数 | 语义字节 | 直接克隆 5 轮 ns/op | 快照命中 5 轮 ns/op | B/op 范围 | allocs/op |
|---:|---:|---:|---:|---:|---:|
| 1 | 1,601 | 3,806-3,974 | 3,834-3,894 | 5,682 | 96 |
| 10 | 16,026 | 38,181-38,996 | 38,624-41,152 | 57,212-57,213 | 951 |
| 100 | 161,544 | 385,745-435,149 | 390,187-414,853 | 567,993-568,000 | 9,502 |

| Provider 数 | 指纹方案 | 5 轮 ns/op | B/op 范围 | allocs/op 范围 | 查询数 |
|---:|---|---:|---:|---:|---:|
| 1 | 当前语义指纹 | 30,960-35,670 | 16,931-16,936 | 238 | 1 |
| 1 | 存储行候选 | 12,540-13,877 | 6,720-6,723 | 38 | 1 |
| 10 | 当前语义指纹 | 236,726-289,141 | 164,655-164,781 | 2,169 | 1 |
| 10 | 存储行候选 | 67,746-106,784 | 63,912-64,158 | 178 | 1 |
| 100 | 当前语义指纹 | 2,127,158-4,039,838 | 1,665,207-1,726,065 | 21,524-21,526 | 1 |
| 100 | 存储行候选 | 478,789-610,579 | 647,085-663,189 | 1,623-1,625 | 1 |

| Provider 数 | 稳定刷新 5 轮 ns/op | B/op 范围 | allocs/op 范围 | 查询数 |
|---:|---:|---:|---:|---:|
| 1 | 32,396-35,306 | 16,934-16,936 | 238 | 1 |
| 10 | 232,154-262,290 | 164,594-164,853 | 2,169 | 1 |
| 100 | 2,128,046-2,274,193 | 1,672,585-1,722,478 | 21,524-21,526 | 1 |

- B13 对照原始值：100 个 Provider 当前语义指纹为 3136.025、4039.838、2203.234、2127.158、2350.900 µs；存储行候选为 478.789、588.255、497.886、537.664、610.579 µs，中位数约改善 4.37 倍。
- 请求侧结论：快照锁和查找开销相对深拷贝不可见；克隆成本随 Provider 数量近似线性，但缺少真实 Provider 数量和请求率，OPT-005 保持待分析。
- 后台结论：稳定刷新每个已加载平台每秒执行 1 次查询；存储行候选在三个规模均稳定减少耗时、分配和对象数，进入 B13，但当前候选哈希值与语义哈希不同，只证明能检测 payload 变化，尚不能实施。
- 验证结果：新增与既有快照测试、目标 `-race`、5 轮基准和 `go test ./services/... -count=1` 均通过；仅有既有 macOS 链接目标版本警告。
- 遗留问题：B04 不修改克隆、轮询周期或指纹实现；custom 平台、nil/空哨兵、旧数据和损坏 payload 的变更检测等价性由 B13 验证。

### B05：日志仪表盘聚合性能基线

#### 目标

量化一次日志仪表盘刷新涉及的 Wails 调用、SQLite 查询、完整日志构建、耗时和分配，不修改现有查询或接口。

#### 对应问题

- OPT-009

#### 前置条件

- 使用临时 SQLite 数据库生成固定数据集。
- 保留 Proxy、Session、All 三种来源和 Provider/模型筛选语义。

#### 涉及范围

- `services/log_dashboard_benchmark_test.go`
- `frontend/src/components/Logs/composables/useLogsPageData.test.ts`
- 不修改生产 LogService 或前端调用链。

#### 当前证据

- `loadDashboardByQuery` 一次并发调用最多 7 个后端方法。
- Stats、Summary、ProviderStats、ModelStats 分别调用相同的 `loadRequestLogsForAggregationV3`。
- Summary 在当前范围之外还可能加载一个对比范围。

#### 修改方案

- 生成 1k、10k、100k 条日志，包含成功、失败、排除、旧数据回退、成本快照和多数据来源。
- 分别基准四个统计方法，并记录组合调用的总耗时、查询次数、读取行数、`B/op` 和 `allocs/op`。
- 对主要筛选组合执行 `EXPLAIN QUERY PLAN`，记录是否使用现有索引。
- 前端测试固定一次仪表盘刷新实际调用的方法和次数。
- 测量结果决定 B06 是否实施；无法形成稳定数据时 B06 标记阻塞或放弃。

#### 明确不修改

- 不新增索引、不修改 SQL、不添加缓存或 Wails 方法。
- 不改变统计字段、来源过滤、定价补全或比较区间。
- 不读取用户实际 `app.db`。

#### 验证方式

- 单元测试：运行现有日志统计、来源、定价和页面数据测试。
- 最小回归测试：`go test ./services/...` 与 `pnpm test:unit`。
- 性能基准：使用固定数据集运行 LogDashboard 相关 Benchmark 5 次。
- 行为一致性检查：基准夹具同时验证四类统计输出。

#### 验收标准

- 三种数据规模和主要筛选模式均有完整测量记录。
- 明确记录一次刷新实际执行的查询/调用数量。
- B06 的执行或放弃依据写入本计划，不以猜测推进。

#### 风险与回退

- 风险：100k 数据集测试耗时过长。
- 控制：基准与普通单测分离，普通测试使用小夹具。
- 回退：删除 Benchmark/夹具生成代码，不影响生产数据。

#### 执行清单

- [x] 修改前重新读取相关代码
- [x] 确认现有行为
- [x] 补充必要测试
- [x] 完成最小修改
- [x] 运行验证
- [x] 记录前后对比
- [x] 更新问题和批次状态
- [x] 更新变更文档

#### 执行记录

- 开始时间：2026-08-31 16:35 CST（Asia/Shanghai）
- 完成时间：2026-08-31 17:09:29 CST（Asia/Shanghai）
- 实际修改：新增 `services/log_dashboard_benchmark_test.go`，增加四类聚合筛选等价测试、4 组 `EXPLAIN QUERY PLAN` 断言和 63 个子基准；在 `useLogsPageData.test.ts` 固定首次与缓存命中刷新调用次数；未修改生产代码、SQL、接口、配置或依赖。
- 数据集：1k/10k/100k 条生成日志，80% 位于当前 24 小时范围、20% 位于 Summary 比较范围；覆盖 Proxy、Session、All、Provider ID、有效计价模型、成功/失败/排除、旧 outcome/data_source 回退和成本快照。SQLite 使用 TestMain 临时 HOME，不读取用户 `app.db`。
- 测量环境：macOS 27.0、Apple M4、darwin/arm64、Go 1.25.5；Debug SQL 保持格式化但写入 `io.Discard`，排除终端吞吐差异。
- 基准命令：`go test ./services -run '^$' -bench '^BenchmarkLogDashboardAggregation$' -benchtime=1x -count=5`，共 63 个子基准，耗时 99.315 秒。

`fullrows/op` 只统计每次调用在 Go 中构建的完整 `ReqeustLog` 数量及分页最多 15 条，不代表 SQLite 实际扫描行数。以下 B/op 使用十进制 MB。

| 数据量 | 场景 | 日志调用 | SQL 冷/暖 | fullrows/op | 冷缓存 5 轮耗时（中位数） | 暖缓存 5 轮耗时（中位数） | 冷缓存 B/op 范围 | 冷缓存 allocs/op 范围 |
|---:|---|---:|---:|---:|---:|---:|---:|---:|
| 1k | Proxy，无过滤 | 6 | 9 / 8 | 1,731 | 24.846-25.918 ms（25.500） | 24.917-25.367 ms（25.217） | 26.833-26.857 MB | 376,977-377,074 |
| 1k | Session + Provider | 6 | 9 / 8 | 206 | 3.613-3.706 ms（3.674） | 3.733-3.872 ms（3.761） | 3.434 MB | 47,649-47,650 |
| 1k | All + Provider + 模型 | 7 | 10 / 9 | 292 | 5.082-5.575 ms（5.179） | 5.017-5.220 ms（5.161） | 4.786-4.823 MB | 66,552-66,573 |
| 10k | Proxy，无过滤 | 6 | 9 / 8 | 17,031 | 290.088-305.790 ms（297.098） | 283.312-320.836 ms（297.270） | 261.576-261.609 MB | 3,689,975-3,690,009 |
| 10k | Session + Provider | 6 | 9 / 8 | 1,906 | 35.958-36.448 ms（36.087） | 34.589-35.328 ms（34.875） | 29.570-29.637 MB | 418,383-418,437 |
| 10k | All + Provider + 模型 | 7 | 10 / 9 | 2,792 | 56.363-104.843 ms（59.030） | 51.218-54.983 ms（53.315） | 43.153-43.171 MB | 608,592-608,609 |
| 100k | Proxy，无过滤 | 6 | 9 / 8 | 170,031 | 3.124-3.201 s（3.162） | 2.908-3.076 s（2.921） | 2,617.261-2,617.276 MB | 36,810,562-36,810,577 |
| 100k | Session + Provider | 6 | 9 / 8 | 18,906 | 406.193-487.589 ms（419.098） | 379.884-389.667 ms（383.340） | 290.886-290.918 MB | 4,124,047-4,124,069 |
| 100k | All + Provider + 模型 | 7 | 10 / 9 | 27,792 | 553.552-590.596 ms（556.145） | 536.300-565.557 ms（544.530） | 426.446-426.466 MB | 6,026,144-6,026,168 |

100k Proxy 单入口数据证明四个统计方法各自承担完整对象构建，Provider 暖缓存只去掉性能聚合 SQL，不去掉主日志加载：

| 入口 | SQL | fullrows/op | 5 轮耗时 | B/op 范围 | allocs/op 范围 |
|---|---:|---:|---:|---:|---:|
| Stats | 1 | 40,004 | 665.725-706.615 ms | 620.789 MB | 8,779,230-8,779,233 |
| Summary | 2 | 50,004 | 824.259-845.631 ms | 773.163-773.178 MB | 10,909,084-10,909,099 |
| Provider（冷） | 2 | 40,004 | 792.945-954.133 ms | 611.836-611.838 MB | 8,579,259-8,579,267 |
| Provider（暖） | 1 | 40,004 | 644.823-710.116 ms | 611.823 MB | 8,579,165-8,579,171 |
| Model | 1 | 40,004 | 666.342-734.891 ms | 611.184 MB | 8,539,159-8,539,164 |

- 查询计划：All/Proxy 时间范围使用 `idx_request_log_created_at`；Session + platform 使用 `idx_request_log_platform_created_at`；All + Provider + 模型使用 `idx_request_log_platform_provider_id_created_at`。All 模式额外出现按 `session_usage_dedup` 主键执行的 correlated scalar subquery；本批没有证据支持新增索引。
- 调用次数：选中模型的首次全平台刷新为 7 个日志服务调用 + 3 个配置调用；60 秒 Provider 配置缓存命中后刷新仅新增 7 个日志调用。日志链路冷/暖缓存分别执行 10/9 条 SQL；未选模型时为 6 个日志调用、9/8 条 SQL。配置服务内部 SQL 不计入本基准。
- B06 决策：执行。三个规模和三种筛选的时间、分配与对象数均可重复；100k Proxy 当前范围 40,004 行会在一次刷新中构建 170,031 个完整日志对象，证明共享主范围加载有明确收益空间。B06 仍需用同一基准记录前后数据，不能按理论收益标记完成。
- 验证结果：新增筛选等价与查询计划测试、单文件 15 项前端测试、5 轮基准、`go test ./services/... -count=1`、69 个文件/537 项 `pnpm test:unit` 和 `git diff --check` 均通过；Go 仅有既有 macOS 链接目标版本警告。
- 遗留问题：本批不修改查询、索引、缓存、Wails 方法或前端调用链；All 来源 correlated subquery 与 Provider refs 无日期范围的成本不并入 B06，后续需要证据时另立批次。

### B06：日志仪表盘单次聚合

#### 目标

在 B05 证明重复聚合成本可复现后，以一个新增兼容 Wails 方法复用同一范围日志，减少重复读取和对象构建。

#### 对应问题

- OPT-009

#### 前置条件

- B05 已完成并记录稳定基线。
- 现有四个统计方法的输出已具备相同夹具下的回归断言。
- 若 B05 无法证明可重复差异，本批直接记录放弃，不实施代码。

#### 涉及范围

- `services/logservice.go` 及相关测试
- `frontend/src/services/logs.ts`
- `frontend/src/components/Logs/composables/useLogsPageData.ts` 及测试

#### 当前证据

- 四个统计入口对相同筛选范围重复执行完整日志加载和 Go 对象构建。
- 前端已使用 `Promise.all` 并发调用，降低了等待串行时间，但没有消除后端重复工作。
- B05 的 100k Proxy 夹具中，当前范围 40,004 行在一次刷新中被构建为 170,031 个完整日志对象；冷缓存 5 轮为 3.124-3.201 s、约 2.617 GB/op。
- B05 已用相同夹具证明 Stats、Summary、ProviderStats 和 ModelStats 的筛选后请求数一致，并覆盖 Proxy、Session、All、Provider 和模型筛选。

#### 修改方案

- 新增兼容返回类型 `LogDashboardAggregateV1`，包含现有 `summary`、`stats`、`model_stats`、`provider_stats` 字段。
- 新增 `LogService.DashboardAggregateRangeV1(platform, provider, pricingModel, sourceMode, startAt, endAt string, includeProviderStats bool)`。
- 主范围日志只加载一次，并由共享的内部纯聚合函数生成四类结果；Summary 的历史比较范围仍按原语义独立读取。
- 现有 `StatsRangeV3`、`SummaryRangeV3`、`ProviderStatsRangeV3`、`ModelStatsRangeV3` 方法和签名全部保留，并复用相同内部聚合逻辑或继续兼容调用。
- 前端仪表盘改用新增聚合方法；分页日志、Provider 选项和必要的模型选项调用保持独立。
- 对 Proxy、Session、All、Provider/模型筛选和旧日志数据执行新旧结果等价测试。

#### 明确不修改

- 不删除、重命名或改变任何现有 Wails 方法。
- 不改变 JSON 字段的现有子结构、统计公式、排序或比较区间。
- 不新增数据库列；索引优化若有证据，另立需确认的独立批次。
- 不引入跨刷新 TTL 缓存，避免新增可见陈旧窗口。

#### 验证方式

- 单元测试：共享聚合纯函数、新旧 Wails 结果等价和前端调用映射测试。
- 最小回归测试：`go test ./services/...`、`pnpm test:unit`。
- 性能基准：复用 B05 完全相同的命令、数据集和环境记录前后数据。
- 行为一致性检查：新聚合结果拆分后与四个旧方法逐字段一致；浮点字段使用现有测试容差。

#### 验收标准

- 现有 Wails 接口完整保留且所有回归测试通过。
- 一次前端仪表盘刷新不再分别调用四个统计方法。
- 记录 1k、10k、100k 数据集的前后查询数、耗时和分配量；不要求最低改善百分比。
- 没有前后数据时不得将 OPT-009 或 B06 标记完成。

#### 风险与回退

- 风险：共享聚合遗漏某个旧方法的特殊计算或排序。
- 控制：先提取并验证纯聚合函数，再切换前端调用；用新旧等价测试覆盖所有来源模式。
- 回退：前端恢复调用旧方法并删除新增兼容方法；数据库和现有 API 不受影响。

#### 执行清单

- [x] 修改前重新读取相关代码
- [x] 确认现有行为
- [x] 补充必要测试
- [x] 完成最小修改
- [x] 运行验证
- [x] 记录前后对比
- [x] 更新问题和批次状态
- [x] 更新变更文档

#### 执行记录

- 开始时间：2026-08-31 17:10 CST（Asia/Shanghai）
- 完成时间：2026-08-31 17:54:09 CST（Asia/Shanghai）
- 实际修改：在 `LogService` 中提取 Stats、Summary、Provider、Model 的共享聚合函数，新增 `LogDashboardAggregateV1` 与 `DashboardAggregateRangeV1`；主范围日志只读取和完成定价适配一次，Summary 比较范围与 Provider 性能 SQL 保留原语义。
- 兼容性：`StatsRangeV3`、`SummaryRangeV3`、`ProviderStatsRangeV3`、`ModelStatsRangeV3` 及全部旧 Wails 签名继续存在并复用相同聚合函数；没有数据库、配置或 JSON 子结构变更。
- 前端修改：`frontend/src/services/logs.ts` 新增类型化动态 Wails 封装；`useLogsPageData` 以一个聚合请求更新四类统计，继续独立加载分页、Provider refs 和选中模型时的无模型过滤选项；独立 Provider 延迟加载入口保持不变。
- 行为保护：生成夹具下对 Proxy、Session、All、Provider、模型和 `includeProviderStats=false` 执行新旧逐字段 `DeepEqual`；前端固定参数、筛选快照、过期响应、Loading、Provider 延迟加载和配置缓存调用次数。
- 测量环境与数据集同 B05；基准命令 `go test ./services -run '^$' -bench '^BenchmarkLogDashboardAggregation$' -benchtime=1x -count=5`，81 个子场景均有 5 个样本，耗时 129.264 秒。

以下为冷缓存完整刷新数据；耗时列为 5 轮范围（中位数），B/op 使用十进制 MB，降幅按中位数计算。

| 数据量 | 场景 | 旧耗时 → 新耗时 | 耗时降幅 | 旧 MB/op → 新 MB/op | 字节降幅 | 旧 allocs/op → 新 allocs/op | fullrows/op | 调用 | SQL |
|---:|---|---:|---:|---:|---:|---:|---:|---:|---:|
| 1k | Proxy，无过滤 | 24.764-25.576 ms（25.038）→ 8.518-8.835 ms（8.715） | 65.2% | 26.843 → 8.255 | 69.2% | 376,978 → 117,396 | 1,731 → 519 | 6 → 3 | 9 → 6 |
| 1k | Session + Provider | 3.495-4.098 ms（3.629）→ 1.626-1.727 ms（1.704） | 53.1% | 3.434 → 1.271 | 63.0% | 47,650 → 17,413 | 206 → 71 | 6 → 3 | 9 → 6 |
| 1k | All + Provider + 模型 | 5.077-5.387 ms（5.172）→ 2.953-3.047 ms（3.017） | 41.7% | 4.826 → 2.683 | 44.4% | 66,582 → 36,933 | 292 → 160 | 7 → 4 | 10 → 7 |
| 10k | Proxy，无过滤 | 293.866-313.377 ms（309.446）→ 104.092-146.350 ms（106.459） | 65.6% | 261.608 → 78.537 | 70.0% | 3,690,007 → 1,124,541 | 17,031 → 5,019 | 6 → 3 | 9 → 6 |
| 10k | Session + Provider | 35.947-36.904 ms（36.470）→ 15.794-16.409 ms（15.997） | 56.1% | 29.620 → 9.116 | 69.2% | 418,424 → 130,122 | 1,906 → 571 | 6 → 3 | 9 → 6 |
| 10k | All + Provider + 模型 | 49.864-51.646 ms（50.957）→ 29.614-30.774 ms（30.238） | 40.7% | 43.152 → 22.707 | 47.4% | 608,591 → 321,052 | 2,792 → 1,460 | 7 → 4 | 10 → 7 |
| 100k | Proxy，无过滤 | 3.380-3.983 s（3.435）→ 1.154-1.268 s（1.248） | 63.7% | 2,617.259 → 783.718 | 70.1% | 36,810,567 → 11,193,116 | 170,031 → 50,019 | 6 → 3 | 9 → 6 |
| 100k | Session + Provider | 415.313-434.220 ms（422.956）→ 188.870-331.378 ms（196.721） | 53.5% | 290.904 → 87.330 | 70.0% | 4,124,065 → 1,256,374 | 18,906 → 5,571 | 6 → 3 | 9 → 6 |
| 100k | All + Provider + 模型 | 548.758-622.310 ms（563.822）→ 329.658-359.590 ms（334.434） | 40.7% | 426.464 → 223.086 | 47.7% | 6,026,165 → 3,160,745 | 27,792 → 14,460 | 7 → 4 | 10 → 7 |

- 暖缓存查询数同步由 8 降至 5；选中模型场景由 9 降至 6。100k Proxy 暖缓存中位数从 3.028 s 降至 1.041 s。
- 收益边界：选中模型时无模型过滤的 Model 选项仍需独立读取，因此 All + Provider + 模型的对象与分配降幅低于无模型场景；本批不改变该筛选语义。
- 验证结果：后端新旧等价测试、现有四类统计测试、前端服务和 composable 20 项目标测试、`go test -race`、`vue-tsc --noEmit`、5 轮基准、`go test ./services/... -count=1`、69 个文件/538 项 `pnpm test:unit` 和 `git diff --check` 全部通过；Go 仅有既有 macOS 链接目标版本警告。
- 回退：前端恢复四个旧统计调用并删除新增聚合封装；后端旧方法无需恢复。若完整回退，再删除新增 Wails 方法和共享 helper，数据库与配置不受影响。
- 遗留问题：Provider refs 仍不带日期范围，All 来源仍执行去重 correlated subquery；两者未纳入 B06 修改范围。

### B07：运行时 SQLite 直写审计与竞争测量

#### 目标

区分允许的初始化/维护直写和可能与双队列并发的运行时直写，并用临时库压力测试确认实际竞争；本批不修改生产数据库配置或事务接口。

#### 对应问题

- OPT-008

#### 前置条件

- B02 已固定双队列行为和基线：满足。
- 审计和测量只使用源码及 `t.TempDir()` 临时数据库，未访问用户数据库：满足。

#### 涉及范围

- `services/providerservice.go`
- `services/geminiservice.go`、`services/provider_identity_sync.go`
- `services/healthcheckservice.go`
- `services/logstorage_service.go`、`services/webdavsyncservice.go` 等直接写入调用点
- `services/database.go`
- `services/sqlite_runtime_write_contention_test.go`

#### 当前证据

生产 Go 文件中的 `db.Exec`、`db.Begin`、`tx.Exec` 已按调用入口和数据库归属完成分类：

| 分类 | 代表调用链与代码证据 | 审计结论 |
|---|---|---|
| 初始化与迁移 | `InitDatabase`、`ensureRequestLogTableWithDB`、request log stats/source/quota storage、`MigrateProviderJSONFilesToStore`、`HealthCheckService.Start → ensureTable` | 均在服务启动或 schema/backfill 阶段执行，部分发生在队列初始化之前；允许直连，不迁入运行时队列 |
| 运行时队列降级 | `BlacklistService.bindProviderIdentity` 和 `LogService.markProviderFailedRequestLogsRead` 只在 `GlobalDBQueue == nil` 时回退 `db.Exec` | 正常初始化后走队列；保留兼容降级，本批不据此认定常态竞争 |
| 运行时简单/维护写入 | Wails `HealthCheckService.CleanupOldRecords → db.Exec(DELETE)`；`LogService.ClearLogStats` 直删两张统计表 | 可与代理日志及其他队列写并发；有返回行数或多语句顺序要求，不能只按“单条 SQL”机械替换 |
| 运行时复合事务 | `ProviderService.SaveProviders → GlobalDBQueue.ExecTxGroup → syncProviderIdentityRenamesBestEffort → db.Begin`；Gemini `saveProvidersWithIdentitySync → db.Begin`；日志按 Provider/日期清理直接事务 | 必须保持跨表原子性、保存/改名顺序和 best-effort/强错误语义；迁入队列前需独立设计，B07 不改 |
| 显式维护与恢复 | 日志清理的 trigger/checkpoint、WebDAV `VACUUM INTO`、`BEGIN IMMEDIATE`、`ATTACH/DETACH`、全库恢复 | 属于用户显式维护或全库替换，普通队列无法表达其独占、连接绑定和恢复语义；继续独立处理 |
| 独立 SQLite 数据库 | `SuiStore` 使用系统配置目录下 `suidemo.db`；Codex 历史迁移写外部状态库 | 不写 `~/.code-switch/app.db`，不会与本项目双队列争用同一 writer，不纳入 B14 |

连接配置证据：

- `services/database.go` 使用 `app.db?cache=shared&mode=rwc`，连接池上限为 5；`PRAGMA busy_timeout = 30000` 只通过一次 `db.Exec` 执行。
- modernc SQLite `v1.44.3` 的 driver 在每次建立连接时应用 DSN `_pragma`；一次池级 `Exec` 不能保证后续新连接继承该连接级设置。
- 压力夹具使用与生产一致的 shared cache、WAL、5 个连接和一次显式 `PRAGMA`；固定写入 500 条日志批任务、80 条单次队列任务，并在竞争组加入 12 个 Provider 改名事务与 4 次健康历史清理。每个改名事务更新 250 条请求日志及黑名单、健康历史、小时/日统计和配额周期状态。

5 轮结果（括号内为中位数；P50/P95 仅统计成功调用，快速失败耗时不作为性能收益）：

| 场景 | 操作数/轮 | 成功数/轮 | `SQLITE_BUSY`/轮 | 总耗时 | 关键成功延迟 |
|---|---:|---:|---:|---:|---|
| 当前配置，仅双队列 | 580 | 480-580 | 0-100（50） | 10.348-14.017 ms（12.812） | 日志 P50 2.782 ms、P95 4.720 ms；单次队列 P50 10.682 ms、P95 12.268 ms |
| 当前配置，双队列 + 直写 | 596 | 1-530（3） | 66-595（593） | 4.257-13.278 ms（4.563） | 多轮无足够成功样本；较短总耗时来自快速失败，不可视为更快 |
| 候选：每连接 `_pragma=busy_timeout%3d30000` | 596 | 596 | 0 | 86.683-137.365 ms（131.145） | 日志 P50 42.921 ms、P95 50.195 ms；改名 P50 24.403 ms、P95 69.683 ms |

- 三个场景均验证成功日志/队列写行数；Provider 改名成功时所有关联表统一为新名称，失败时事务整体保留旧名称；健康清理行数与成功操作一致。
- 候选组把快速失败转换为 SQLite writer 等待，证明 OPT-015 的最小修复方向；该结果是合成压力，不代表真实 Wails 调用频率或整机吞吐。

#### 修改方案

- 已完成生产直写点分类并记录调用入口、数据库归属和事务约束。
- 已新增单个测试文件，在独立临时库中执行双队列对照、双队列加直写、每连接 timeout 候选三个场景。
- 已记录成功数、`SQLITE_BUSY/LOCKED`、尝试与成功 P50/P95、吞吐、连接池等待及最终数据一致性。
- 测量把首要修复收敛为 OPT-015/B14；直写事务是否迁入队列待 B14 完成后复测，不提前新增通用事务抽象。

#### 明确不修改

- 不把初始化、迁移、VACUUM、ATTACH/DETACH 或 WebDAV 恢复强行塞入普通队列。
- 不新增通用事务回调、反射或泛型数据库抽象。
- 不修改数据库 schema、队列容量或业务事务边界。
- 不修改生产 DSN、PRAGMA、Wails API、配置、数据或用户行为。

#### 验证方式

- 单元测试：`go test ./services -run '^TestSQLiteRuntimeWriteContentionBaseline$' -count=1 -v`。
- 重复测量：同一目标测试 `-count=5 -v`，记录三个场景的区间与中位数。
- 竞态检测：`go test -race ./services -run '^TestSQLiteRuntimeWriteContentionBaseline$' -count=1`。
- 最小回归测试：`go test ./services/... -count=1`。
- 行为一致性检查：临时数据库的最终行数、跨表改名原子性及健康清理结果全部由断言验证。

#### 验收标准

- 所有已搜索到的生产直写点已按六类归档，运行时候选有具体调用链证据：满足。
- 三个竞争场景可重复执行，不使用用户数据，并记录 5 轮结果：满足。
- 测试同时验证成功与失败后的数据一致性，普通模式和 `-race` 均通过：满足。
- 后续修复已分配 OPT-015/B14，本批未修改生产实现：满足。

#### 风险与回退

- 风险：合成压力无法覆盖真实 Wails 并发。
- 风险：高并发启动栅栏高于常见用户操作频率，错误率不能直接外推生产。
- 控制：只确认“存在可重复竞争”和“逐连接 timeout 可消除该夹具错误”，不宣称真实错误率或吞吐收益。
- 回退：删除新增测试文件；生产行为和用户数据无需恢复，审计记录可保留。

#### 执行清单

- [x] 修改前重新读取相关代码
- [x] 确认现有行为
- [x] 补充必要测试
- [x] 完成最小修改
- [x] 运行验证
- [x] 记录前后对比
- [x] 更新问题和批次状态
- [x] 更新变更文档

#### 执行记录

- 开始时间：2026-08-31 18:04:42 CST
- 完成时间：2026-08-31 18:21:47 CST
- 实际修改：新增临时库 SQLite 写竞争测试；完成生产直写分类；未修改任何生产 Go、前端、配置、依赖或数据库 schema。
- 验证结果：目标测试单轮与 5 轮、竞态检测和完整 services 测试全部通过；仅有既有 macOS 链接目标版本警告。
- 性能对比：当前直写竞争组 5 轮中位数为 593/596 次 `SQLITE_BUSY`；逐连接 timeout 候选为 0/596，成功数由中位数 3/596 提升为 596/596；代价是总耗时中位数由快速失败的 4.563 ms 变为等待完成的 131.145 ms。
- 遗留问题：真实应用并发频率、等待分布和 30 秒上限需在 B14 复测；直写事务是否迁入队列、显式维护是否需要暂停队列均未在本批决定。

### B08：主页面隐藏态轮询治理

#### 目标

主页面不可见时暂停周期 Wails 轮询，恢复可见时立即执行一次合并刷新并按原周期继续。

#### 对应问题

- OPT-012

#### 前置条件

- 先测量当前页面可见和隐藏各 60 秒的 Wails 调用次数。
- 确认主窗口存在跨平台 hide/show 信号；`document.visibilityState` 只作最小化和浏览器预览补充，不能把普通失焦等同隐藏。

#### 涉及范围

- `main.go`（仅 Linux 标准窗口事件补发）
- `frontend/src/components/Main/Index.vue`
- `frontend/src/components/Main/composables/useMainPageShell.ts`
- `frontend/src/components/Main/composables/mainPollingLifecycle.ts`
- `useBlacklistState.ts`、`useProviderStats.ts`、`useProviderQuotas.ts`、`useUpdatePolling.ts`
- 新增最小轮询生命周期测试

#### 当前证据

- 并发状态 Timer 在组件 setup 阶段直接启动，每 2 秒调用 Wails。
- 其他 Timer 在 `useMainPageShell.onMounted` 启动，只在 `onUnmounted` 停止。
- Wails `v3.0.0-beta.6` 定义 `common:WindowHide/WindowShow`；Windows/macOS 原生映射完整，Linux 默认只映射 `WindowShow`，需在应用已有 Hide/Show 控制点补发同名标准事件。
- 托盘页面已有 focus/blur/visibility 生命周期控制；主窗口只使用 hide/show 与 `visibilitychange`，避免切换到其他应用时错误暂停仍可见页面。

#### 修改方案

- 将并发状态 Timer 改为显式 start/stop，并保持原 2 秒周期；与黑名单、Provider 统计、配额和更新 Timer 一起由 shell 统一编排。
- 以 Wails `WindowHide/WindowShow` 为主信号，Linux 在主窗口现有 Hide/Show 控制点补发缺失映射，`visibilitychange` 为补充；隐藏时停止全部周期轮询，恢复时执行一次 `refreshAllData + concurrency + update check` 后重启原周期。
- 用纯生命周期 helper 管理 ready/visible/active 与异步恢复代次；重复 show/visibility 事件不得重复刷新或创建 Timer，恢复刷新未完成时再次隐藏不得重启。
- 卸载时取消 Wails/DOM 监听并停止全部 Timer；Provider 事件监听随 `startStatusSync/stopStatusSync` 暂停，恢复全量刷新补齐遗漏状态。
- 用可注入 Timer/visibility 的纯生命周期测试验证隐藏、恢复、重复事件和卸载。

#### 明确不修改

- 不改变页面可见时的轮询间隔、Wails 方法或展示结果。
- 不改变后台 Provider 健康检查、额度恢复和通知服务。
- 不新增自定义 Wails 事件协议，只补齐 Wails 已定义的 common 窗口事件。
- 不把所有前端 Timer 重构为全局调度器。

#### 验证方式

- 单元测试：假 Timer 验证调用次数、幂等启动、隐藏停止、恢复刷新和卸载清理。
- 最小回归测试：Main composables 测试与 `pnpm test:unit`。
- 性能基准：记录页面隐藏 60 秒的 Wails 周期调用数前后对比。
- 行为一致性检查：可见状态下 60 秒调用节奏与原实现一致，恢复后状态立即更新。

#### 验收标准

- 隐藏期间周期 Wails 调用数为 0。
- 恢复可见时每类数据最多立即刷新一次，不产生重复 Timer。
- 页面可见时的功能、周期和事件结果不变。
- 所有新增监听与 Timer 在卸载时清理。

#### 风险与回退

- 风险：部分平台隐藏窗口不触发预期 visibility 事件。
- 控制：使用 Wails 标准 hide/show 事件作为主信号，不依赖 focus/blur；纯状态机保证 Wails 与 DOM 重复事件收敛到单一活动状态。
- 回退：恢复原有各 Timer 启动位置并删除 visibility 生命周期代码。

#### 执行清单

- [x] 修改前重新读取相关代码
- [x] 确认现有行为
- [x] 补充必要测试
- [x] 完成最小修改
- [x] 运行验证
- [x] 记录前后对比
- [x] 更新问题和批次状态
- [x] 更新变更文档

#### 执行记录

- 开始时间：2026-08-31 19:00:40 CST
- 完成时间：2026-08-31 19:54:22 CST
- 实际修改：并发状态 Timer 改为显式启停；shell 统一管理 Provider 统计、额度、更新、黑名单和并发状态轮询；Wails `WindowHide/WindowShow` 为主信号，`visibilitychange` 为补充；Linux 在主窗口唯一 Hide/Show 控制点补发 beta.6 缺少的同名 common 事件；新增纯轮询生命周期及 5 项测试。
- 验证结果：生命周期测试 5 项、Main 相关测试 48 项、前端完整测试 70 个文件/543 项、`vue-tsc --noEmit`、root Go、services 和 model-pricing 资源测试全部通过；Go 测试仅有既有 macOS 链接目标版本警告。
- 性能对比：固定假 Timer 测量中，原配置可见和隐藏状态每 60 秒均为 41 个高层 Wails 轮询轮次；修改后可见 41 个、隐藏新增 0 个、恢复后 41 个。该数据用于比较轮询调度，不等同于底层 Wails 方法调用总数。
- 遗留问题：未启动桌面 GUI；跨平台原生 hide/show 投递留待发布前人工冒烟，本批不改变 Wails 调用签名、轮询周期、后台服务或用户可见结果。

### B09：设置页数据编排边界

#### 目标

只抽离设置页初始化加载和生命周期编排，使独立读取可并行、错误可隔离并具备测试；不拆分全部 UI 业务域。

#### 对应问题

- OPT-010
- OPT-011

#### 前置条件

- 记录当前挂载时各 Wails 调用顺序、失败行为和页面就绪耗时。
- 确认哪些 Loader 相互独立；任何依赖 AppSettings 结果的调用保持依赖顺序。

#### 涉及范围

- `frontend/src/components/General/Index.vue`
- 新增 `frontend/src/components/General/composables/useGeneralPageData.ts`
- 新增对应 Vitest

#### 当前证据

- 页面挂载依次 await 七类数据，单个失败由各 Loader 内部处理。
- WebDAV 事件在全部加载结束后订阅，卸载时刷新待保存设置并取消订阅。
- General 目录当前只有 `Index.vue`，初始化和保存编排没有独立测试。

#### 修改方案

- 抽离一个具体的 General 页面数据编排 composable，Loader 通过明确参数注入，避免在 composable 内硬编码 Wails 方法名。
- 保持 AppSettings 加载及其预算刷新语义；将模型路由、版本/更新、黑名单、导入和 WebDAV 等确认独立的读取以 `Promise.allSettled` 并行。
- 保持每个 Loader 的原错误回退，不因单项失败阻断其他数据。
- 保持 WebDAV 事件订阅和卸载取消时机；保持 `flushPendingPersist` 在返回和卸载时执行。
- 测试正常加载、单项失败、重复挂载保护、卸载清理和待保存刷新。

#### 明确不修改

- 不拆分预算、WebDAV、更新、黑名单等页面模板和表单。
- 不改变保存防抖、save queue、localStorage key、Toast 或 UI 文案。
- 不新增状态管理库或组件测试依赖。

#### 验证方式

- 单元测试：使用 mock Loader 验证并行、依赖、错误隔离和清理。
- 最小回归测试：General 相关 service 测试与 `pnpm test:unit`。
- 性能基准：记录相同 mock 延迟和本地 Wails 环境下的页面初始化前后耗时。
- 行为一致性检查：所有设置值、错误回退、事件订阅和保存结果一致。

#### 验收标准

- 初始化编排不再直接写在 `Index.vue` 的 `onMounted` 中。
- 独立 Loader 并行执行，依赖 Loader 保持顺序。
- 任一独立读取失败不阻断其他读取，结果与现有行为一致。
- 有自动化测试覆盖初始化和卸载资源清理，并记录初始化耗时前后对比。

#### 风险与回退

- 风险：错误并行化具有隐式依赖的 Loader，造成短暂状态覆盖。
- 控制：先记录读写字段和调用依赖，只并行确认独立的读取。
- 回退：恢复原 `onMounted` 串行编排并删除新增 composable；设置数据格式不受影响。

#### 执行清单

- [x] 修改前重新读取相关代码
- [x] 确认现有行为
- [x] 补充必要测试
- [x] 完成最小修改
- [x] 运行验证
- [x] 记录前后对比
- [x] 更新问题和批次状态
- [x] 更新变更文档

#### 执行记录

- 开始时间：2026-08-31 20:17:57 CST
- 完成时间：2026-08-31 20:23:18 CST
- 实际修改：新增具体的 General 页面数据生命周期 composable；保留 AppSettings 首段依赖，将模型路由、版本、更新、黑名单、导入和 WebDAV 六项独立 Loader 以 `Promise.allSettled` 并行；WebDAV 事件仍在初始化完成后订阅，卸载仍刷新待保存设置并取消订阅；初始化中卸载不会产生迟到订阅。
- 验证结果：新增 6 项生命周期测试、General 相关 25 项测试、`vue-tsc --noEmit` 和完整前端 71 个文件/549 项测试全部通过；未发现调试输出、重复实现或类型绕过。
- 性能对比：相同 100 ms 假延迟下，原七段串行编排为 700 ms，新编排为 AppSettings 100 ms 加六项并行 100 ms，共 200 ms；该数据只比较编排等待层数，未启动桌面应用测量真实 Wails 延迟。
- 遗留问题：`General/Index.vue` 仍承载各业务表单，本批按范围明确不拆分；未修改保存队列、缓存键、Toast、配置格式、Wails 签名或 UI 行为。

### B10：通用代理调度内核收敛

#### 目标

在 B01 行为基线保护下，收敛 Claude、Codex 和 GrokBuild 共用 `proxyHandler` 内的 Provider 尝试与失败处理重复代码，不做性能声明。

#### 对应问题

- OPT-001
- OPT-002

#### 前置条件

- B01 完成且全部调度组合测试通过。
- 修改前重新读取 `proxyHandler`、`forwardRequestWithPlan`、黑名单和会话亲和实现。

#### 涉及范围

- `services/providerrelay.go`
- 必要时新增 `services/providerrelay_scheduler.go`
- B01 及现有 ProviderRelay 测试

#### 当前证据

- 通用 Handler 在黑名单启用/关闭、会话可绑定/不可绑定等路径中重复维护 last error、尝试计数、绑定恢复、失败记录和最终响应。
- Gemini 与自定义 CLI 有各自 Handler 和 Provider 类型，首批同时泛化会显著扩大风险。

#### 修改方案

- 仅使用具体 `Provider` 类型提取通用的候选尝试状态和失败结果处理，不引入单实现接口、反射或跨平台泛型框架。
- 保留 Provider 构建、模型计划、会话排序和实际 `forwardRequestWithPlan` 调用的现有边界。
- 统一响应开始、客户端中断、本地并发限制、上游错误透传、失败计数和绑定恢复的处理出口。
- 逐个分支迁移并运行 B01 测试；所有现有注释原样保留，不顺带修改日志、命名或协议转换。
- `geminiProxyHandler`、`customCliProxyHandler` 和流转换函数明确不进入本批。

#### 明确不修改

- 不改变路由、Provider 顺序、重试次数、黑名单、轮询、会话亲和或通知语义。
- 不修改请求日志 SQL、成本计算、模型映射或 HTTP Client 配置。
- 不追求一次拆小整个 `providerrelay.go`，不按行数目标继续重构。

#### 验证方式

- 单元测试：B01 调度矩阵和所有 ProviderRelay 测试。
- 最小回归测试：`go test ./services/...`。
- 性能基准：本批是结构重构，不标记性能优化完成；可运行 B03 基准确认无明显回退。
- 行为一致性检查：逐组合比较尝试顺序、响应、日志记录、黑名单、通知和会话绑定。

#### 验收标准

- B01 和现有代理测试全部通过。
- 通用 Handler 的重复失败/收尾路径由单一内部实现负责。
- 现有公共方法、路由、HTTP 结果和数据格式不变。
- 没有把 Gemini、自定义 CLI 或无关协议代码纳入改动。

#### 风险与回退

- 风险：多个调度分支存在未被注意的细微差异。
- 控制：先建立行为矩阵，再小步迁移；任何无法解释的差异均保留原分支并更新文档。
- 回退：恢复通用 Handler 内原分支结构；本批不包含数据迁移或外部接口变更。

#### 执行清单

- [x] 修改前重新读取相关代码
- [x] 确认现有行为
- [x] 补充必要测试
- [x] 完成最小修改
- [x] 运行验证
- [x] 记录前后对比
- [x] 更新问题和批次状态
- [x] 更新变更文档

#### 执行记录

- 开始时间：2026-08-31 20:24:00 CST
- 完成时间：2026-08-31 20:48:16 CST
- 实际修改：新增具体 `providerAttemptState`，统一通用 Handler 的最后错误、Provider、耗时和尝试次数；提取成功/失败记录、响应已开始/客户端中断停止、会话恢复回调以及 429/上游原始错误终止出口；四条分支特有的排序、重试、通知和日志保持原位；B01 矩阵新增固定拉黑与已绑定会话组合。
- 验证结果：6 场景矩阵单轮与 5 轮、矩阵竞态、6 项关键错误透传、4 项会话迁移、完整 services 和 root Go 测试全部通过；仅有既有 macOS 链接目标版本警告。
- 前后对比：`proxyHandler` 由 737 行降至 620 行，减少 117 行（约 15.9%）；`providerrelay.go` 由 11,447 行降至 11,402 行，净减少 45 行；矩阵由 5 个增至 6 个场景。本批为结构重构，不声明运行时性能提升。
- 遗留问题：`providerrelay.go` 仍承担多平台与协议职责，OPT-001 保持待分析；Gemini、自定义 CLI、协议转换、日志 SQL、模型映射和 HTTP Client 未修改。

### B11：事务组任务所有权边界

#### 目标

只保证 `ExecTxGroup` 返回时 worker 已不再访问调用方传入的任务切片，使同一切片可安全顺序复用，不改变事务和错误语义。

#### 对应问题

- OPT-013

#### 前置条件

- B02 完成，保留事务成功、原子回滚和事务组基准。
- 修改前用独立回归测试稳定复现任务切片复用问题。

#### 涉及范围

- `services/dbqueue.go`
- `services/dbqueue_test.go`

#### 当前证据

- `ExecTxGroup` 会把新结果通道写回调用方切片，并只等待 `group[0].Result` 后返回。
- `commitTxGroup` 会依次写入并关闭组内所有结果通道；调用方在返回后复用切片，可能与 worker 访问后续任务并发。
- B02 初始 Benchmark 复用双任务切片时触发 `send on closed channel`；当前唯一生产调用方 `SaveProvidersToStore` 每次重新构造任务切片，尚未发现现有用户路径回归。

#### 修改方案

- 先新增顺序复用同一 `[]WriteTask` 的回归测试，并用 `go test -race` 固定返回边界。
- 最小修改 `ExecTxGroup` 的结果等待：首个结果确定事务结果后，返回前等待全部结果通道关闭，确保 worker 不再访问该任务组。
- 保留调用方任务中 `Result` 的现有回填行为，不新增类型、锁、队列或依赖。

#### 明确不修改

- 不修改事务开启、SQL 执行顺序、回滚、提交和错误文本。
- 不修改单次队列、批量队列、超时、容量或 QueueStats 字段与计算口径。
- 不调整 ProviderStore 调用方式，不扩大到并发复用同一切片的未定义场景。

#### 验证方式

- 单元测试：事务成功、回滚和同一任务切片顺序复用均通过。
- 最小回归测试：`go test ./services/...`。
- 性能基准：修改前后各运行 5 次 `BenchmarkDBWriteQueue/tx_group`，记录 `ns/op`、`B/op` 和 `allocs/op`。
- 行为一致性检查：事务结果、落库内容、错误传播和 QueueStats 任务数保持一致。

#### 验收标准

- 同一双任务切片至少顺序复用 1000 次，普通测试与 `-race` 均不 panic、无竞态。
- 原子提交和任一语句失败整体回滚测试保持通过。
- 公共方法签名、错误文本、事务语义和生产调用结果不变。
- 事务组 5 轮基准有修改前后数据；本批不宣称吞吐提升。

#### 风险与回退

- 风险：等待其余已缓冲结果可能暴露 worker 结果分发异常，并轻微延长方法返回边界。
- 控制：结果通道容量为 1，worker 已为每个任务发送结果；使用超时和竞态测试确认不会新增阻塞。
- 回退：恢复只等待首个结果的实现并删除复用回归测试；不涉及数据库或配置迁移。

#### 执行清单

- [x] 修改前重新读取相关代码
- [x] 确认现有行为
- [x] 补充必要测试
- [x] 完成最小修改
- [x] 运行验证
- [x] 记录前后对比
- [x] 更新问题和批次状态
- [x] 更新变更文档

#### 执行记录

- 开始时间：2026-08-31 20:49:00 CST
- 完成时间：2026-08-31 20:59:57 CST
- 实际修改：新增同一双任务切片顺序复用 1000 次的回归测试；`ExecTxGroup` 收到首个事务结果后等待组内全部结果通道关闭再返回，确保 worker 已完成对所有任务 `Result` 的发送和关闭；公共签名、事务 SQL、错误文本、队列参数和统计口径不变。
- 验证结果：修改前竞态测试稳定报告 `ExecTxGroup` 写与 `commitTxGroup` 读冲突；修改后目标普通测试、20 次重复、目标竞态、全部 DBWriteQueue 竞态、ProviderStore、完整 services 和 root Go 测试通过；仅有既有 macOS 链接目标版本警告。
- 性能对比：`tx_group` 5 轮中位数 39,830 → 39,360 ns/op，区间重叠且修改后有 54,857 ns/op 调度离群值；分配保持约 1,491-1,494 B/op、32 allocs/op。结论为无可确认回退，不宣称吞吐提升。
- 遗留问题：30 秒超时路径按既有契约返回时任务仍可能继续执行，超时后并发复用同一切片不在本批定义范围；当前生产调用方仍每次构造新切片。

### B12：负载脱敏快速检查与重复处理优化

#### 目标

只降低负载捕获开启时的快速检查和重复脱敏成本，保持所有脱敏输出、截断结果、默认开关和日志字段完全不变。

#### 对应问题

- OPT-014

#### 前置条件

- B03 完成并保留 1 MiB、8 MiB 附近及 SSE 的 5 轮基线。
- 先为 JSON、纯文本、Header、Query、大小写变体和会话标识建立逐字节输出基线。

#### 涉及范围

- `services/providerrelay.go`
- `services/providerrelay_payload_sanitize_test.go`
- `services/providerrelay_hotpath_benchmark_test.go`

#### 当前证据

- 1 MiB 无匹配负载的会话标识正则检查耗时 135.047-157.596 ms，敏感关键词正则检查耗时 74.378-76.380 ms。
- 1 MiB 原始捕获耗时 419.086-451.244 ms，脱敏捕获耗时 1.346-1.419 s；8 MiB 附近分别达到 3.143-3.431 s 和 10.319-10.432 s。
- `prepareRequestLogPayloadForPersistence` 从 `requestBodyBytes` 捕获并脱敏后，会再次对 `RequestBody` 执行 `maybeSanitizeRequestLogPayload`。

#### 修改方案

- 先用表驱动测试固定所有现有正则匹配与不匹配结果，包括大小写、连字符/下划线、引号、数字、布尔值和无效 JSON 文本。
- 使用单次 ASCII 大小写无关扫描或等价的无分配快速门控替代两组宽泛正则快速检查；门控允许误报但不得漏报，实际替换仍复用现有正则。
- 当本次持久化已从 `requestBodyBytes` 完成捕获时，不再对同一 `RequestBody` 重复脱敏；预先填充 `RequestBody` 的既有路径仍执行脱敏。
- 不新增依赖、缓存、配置或并发状态。

#### 明确不修改

- 不改变敏感字段和会话标识清单、`[REDACTED]` 文本、大小写兼容或纯文本规则。
- 不改变 8 MiB 上限、UTF-8 截断、PayloadBytes、PayloadCaptured 和截断标记。
- 不改变捕获默认关闭、脱敏默认开启、SSE Hook、数据库字段或保存时机。
- 不处理请求 `io.ReadAll`、非流式响应缓冲或其他代理逻辑。

#### 验证方式

- 单元测试：现有脱敏测试、新增逐字节 golden 测试、B03 捕获/截断/转发测试。
- 最小回归测试：`go test ./services/... -count=1`。
- 性能基准：修改前后各运行 5 次 `BenchmarkRelayPayloadQuickChecks`、1 MiB 捕获基准和 8 MiB 固定迭代捕获基准。
- 行为一致性检查：所有输入的输出字符串、截断标记、PayloadBytes 和 PayloadCaptured 完全一致。

#### 验收标准

- 所有现有与新增脱敏用例逐字节一致，普通测试和 `-race` 均通过。
- 1 MiB 与 8 MiB 原始/脱敏捕获均记录 5 轮修改前后数据，修改后中位数低于 B03 基线且分配不增加。
- 捕获关闭基准无可确认回退；公共 API、配置、日志数据和用户行为不变。
- 没有性能对比数据或行为存在差异时，不得标记 B12 完成。

#### 风险与回退

- 风险：快速门控漏报会泄露敏感值，重复处理收敛可能遗漏预填充请求体路径。
- 控制：门控只决定是否运行原替换正则，允许误报不允许漏报；对捕获来源分别建立测试。
- 回退：恢复原正则快速检查和无条件二次脱敏；不涉及数据、配置或接口迁移。

#### 执行清单

- [x] 修改前重新读取相关代码
- [x] 确认现有行为
- [x] 补充必要测试
- [x] 完成最小修改
- [x] 运行验证
- [x] 记录前后对比
- [x] 更新问题和批次状态
- [x] 更新变更文档

#### 执行记录

- 开始时间：2026-08-31 21:00:00 CST
- 完成时间：2026-08-31 21:27:16 CST
- 实际修改：以无分配 ASCII 大小写折叠扫描作为会话标识和敏感关键词快速门控；遇到非 ASCII 时回退原 quick regex，保留 Unicode SimpleFold 且避免干净多语言 payload 误入全部替换正则；开启通用脱敏时只执行一次会话脱敏；从 `requestBodyBytes` 已捕获的请求体不再持久化前重复处理，预填充路径保持原行为。
- 验证结果：新增 8 组逐字节 golden、敏感/会话门控矩阵、Unicode 干净与折叠用例、捕获/预填充一致性测试；payload/热路径普通与竞态、B01 矩阵、完整 services 和 root Go 测试全部通过；仅有既有 macOS 链接目标版本警告。
- 性能对比：1 MiB quick check 中位数会话 131.527 → 0.949 ms、敏感词 71.852 → 0.678 ms；1 MiB raw 410.463 → 2.051 ms、sanitized 1.334 s → 334.701 ms；近 8 MiB raw 3.219 s → 16.699 ms、sanitized 10.498 s → 2.626 s。1 MiB/近 8 MiB 脱敏 B/op 中位数约 34.71 → 24.13 MB、276.95 → 192.99 MB；capture_off 保持百纳秒级。
- 遗留问题：非 ASCII payload 为保留 Go regexp Unicode 折叠语义继续使用原 quick regex；实际替换正则、敏感字段清单、8 MiB 截断、SSE Hook、日志字段和捕获默认值未修改。

### B13：稳定存储内容 Provider 指纹

#### 目标

只优化统一 SQLite 存储的外部变更指纹计算，避免每秒把全部 payload 反序列化为 Provider 后再次序列化；保持快照内容和刷新时机不变。

#### 对应问题

- OPT-006

#### 前置条件

- B04 完成并保留当前语义指纹、存储行候选和稳定刷新的 5 轮基线。
- 先证明新旧方案对所有有语义变化/无语义变化场景作出相同刷新决定。

#### 涉及范围

- `services/providerstore.go`
- `services/providerservice.go`
- `services/providerstore_test.go`
- `services/providerservice_test.go`
- `services/provider_snapshot_benchmark_test.go`

#### 当前证据

- SQLite 平台的轮询指纹直接基于 `ORDER BY sort_index, id` 的完整存储行计算，不再把 payload 解码为 Provider 后重新编码。
- 保存端使用本次实际构造并写入的行计算同一指纹，避免事务完成后再次查询把其他写入者的数据指纹绑定到当前 Provider 快照。
- 测试已覆盖公共列、payload 格式、损坏 payload、顺序、ID 回填、nil/空哨兵、保存后一致性和 custom 文件格式；raw 非语义变化通过旧版 JSON 语义比较只更新指纹。

#### 修改方案

- 为统一 SQLite 平台建立稳定的有序存储内容指纹，优先直接组合按 `sort_index, id` 排序的 raw payload，避免 Provider JSON 解码/编码往返。
- 保存完成后的快照与后台轮询必须使用同一指纹语义，避免每次保存后的首次轮询产生伪变更。
- 表驱动覆盖未初始化、空哨兵、多行顺序、payload 内容/格式变化、ID 回填、公共列变化和损坏 payload；只比较是否触发刷新，不要求新旧哈希字节相同。
- custom 平台继续使用现有文件加载和语义指纹路径，不强行套用 SQLite 行算法。
- 不新增依赖、缓存、数据库列或后台任务。

#### 明确不修改

- 不改变 1 秒检查周期、快照深拷贝、外部变更二次确认或 Claude 路由通知。
- 不改变 Provider JSON、SQLite schema、排序、nil/空语义、损坏行跳过和迁移行为。
- 不修改 SaveProviders、LoadProviders 的公共签名或返回数据。
- 不处理 OPT-005 请求侧深拷贝。

#### 验证方式

- 单元测试：指纹决策等价矩阵、保存后稳定、外部变更刷新、空哨兵、损坏 payload 和 custom 回退。
- 最小回归测试：`go test ./services/... -count=1`。
- 性能基准：修改前后各运行 5 次 `BenchmarkProviderSnapshotFingerprint` 和 `BenchmarkProviderSnapshotRefresh`。
- 行为一致性检查：相同存储内容不刷新；任何影响加载结果的变更均刷新；返回 Provider 逐字段一致。

#### 验收标准

- 新旧方案的刷新决策矩阵全部一致，保存后下一轮稳定检查不替换快照。
- 现有外部刷新测试在普通模式与 `-race` 下均通过。
- 1/10/100 个 Provider 均记录 5 轮修改前后数据；修改后中位数低于 B04 当前语义指纹基线且分配不增加。
- 公共 API、配置、数据库、custom 平台和用户行为不变；缺少任一证据不得标记完成。

#### 风险与回退

- 风险：新指纹漏掉影响 Provider 加载结果的字段，或保存端与轮询端指纹不一致导致漏刷新/伪刷新。
- 控制：使用决策等价矩阵覆盖每个存储字段与兼容分支，并保留外部变更二次指纹确认。
- 回退：恢复 `LoadProvidersFromStore` 后 `json.Marshal` 的语义指纹；不涉及数据或配置迁移。

#### 执行清单

- [x] 修改前重新读取相关代码
- [x] 确认现有行为
- [x] 补充必要测试
- [x] 完成最小修改
- [x] 运行验证
- [x] 记录前后对比
- [x] 更新问题和批次状态
- [x] 更新变更文档

#### 执行记录

- 开始时间：2026-08-31 21:28:00 CST（Asia/Shanghai）
- 完成时间：2026-08-31 22:07:28 CST（Asia/Shanghai）
- 实际修改：新增统一的存储行指纹和空哨兵行构造；SQLite 保存返回本次事务的确定性指纹，`saveProvidersLocked` 不再保存后查询；轮询复用同一行指纹。custom 继续使用文件加载后的 Provider JSON 语义指纹。
- 行为保护：raw 格式或冗余公共列变化时，使用旧版 JSON 语义比较确认 Provider 未变，只更新指纹并保留原快照运行时字段；语义变化仍执行原快照替换、Claude 路由通知和协调流程。

| Provider 数 | 修改前语义指纹中位数 | 修改后存储行指纹中位数 | 修改前稳定刷新中位数 | 修改后稳定刷新中位数 |
|---:|---:|---:|---:|---:|
| 1 | 29.330 µs | 11.914 µs | 30.325 µs | 12.345 µs |
| 10 | 223.853 µs | 59.615 µs | 226.384 µs | 61.383 µs |
| 100 | 2,079.893 µs | 442.021 µs | 2,077.316 µs | 446.699 µs |

- 修改后稳定刷新 5 轮范围：1 个为 12.322-12.364 µs、6,721-6,723 B/op、38 allocs/op；10 个为 61.104-61.625 µs、63,886-63,942 B/op、178 allocs/op；100 个为 446.071-452.709 µs、646,024-653,173 B/op、1,623-1,624 allocs/op。
- 验证结果：保存/回读指纹、raw 变化矩阵、语义稳定、nil/空哨兵、custom 格式、外部刷新普通与竞态测试通过；修改后基准完成 5 轮；`go test ./services/... -count=1`、`go test . -count=1` 通过，仅有既有 macOS 链接目标版本警告。
- 遗留问题：不修改 1 秒周期、快照深拷贝、二次确认、数据库 schema 或公共 API；OPT-005 请求侧克隆仍保持待分析。

### B14：SQLite 连接池逐连接 busy timeout

#### 目标

只让默认 `app.db` 连接池中的每个 modernc SQLite 连接继承现有 30 秒 `busy_timeout`，避免双队列或运行时直写在 writer 短暂占用时立即返回 `SQLITE_BUSY`。

#### 对应问题

- OPT-015

#### 前置条件

- B07 已完成直写分类、竞争复现和逐连接 DSN 候选测量：满足。
- 修改前已重跑 B07 目标测试并保留 5 轮错误区间：满足。

#### 涉及范围

- `services/database.go`
- `services/sqlite_runtime_write_contention_test.go`
- `services/testdb_main_test.go`
- `services/session_usage_test.go`
- 不涉及业务 Service、前端或 schema

#### 当前证据

- 修改前默认 DSN 为 `app.db?cache=shared&mode=rwc`，连接池最多建立 5 个连接；显式 `PRAGMA busy_timeout = 30000` 只在池内当次取得的连接上执行。
- modernc `v1.44.3` 在每个新连接建立时应用 DSN `_pragma`；新增连接属性测试同时持有 5 个连接，修改后每个连接均返回 `busy_timeout=30000`，连续 20 次通过。
- 修改前 5 轮生产等价直写竞争组：成功 1-201/596（中位数 4），`SQLITE_BUSY` 395-595/596（中位数 592），总耗时中位数 4.682 ms；短耗时来自快速失败。
- 修改后压力测试保留无逐连接 timeout 的 legacy 组，并让默认配置组直接调用生产 `defaultSQLiteDSN`，避免测试与生产参数漂移。

修改后 5 轮默认配置结果（P50/P95 均只统计成功调用）：

| 指标 | 范围 | 中位数 |
|---|---:|---:|
| 成功操作 | 596/596 | 596/596 |
| `SQLITE_BUSY/LOCKED` | 0/596 | 0/596 |
| 总耗时 | 80.138-109.045 ms | 106.120 ms |
| 成功吞吐 | 5,465.6-7,437.2 ops/s | 5,616.3 ops/s |
| 日志批写 P50 / P95 | P50 39.562-44.952 ms；P95 49.961-50.518 ms | 42.234 / 50.176 ms |
| Provider 改名 P50 / P95 | P50 21.347-24.708 ms；P95 38.526-66.490 ms | 23.268 / 42.970 ms |
| 单次队列 P50 / P95 | P50 43.096-106.974 ms；P95 44.747-108.542 ms | 47.108 / 48.499 ms |

#### 修改方案

- 新增最小 `defaultSQLiteDSN` helper，在默认 DSN 中加入 `_pragma=busy_timeout%3d30000`，由 `InitDatabase` 直接使用。
- 保留既有显式 `PRAGMA busy_timeout`、WAL、cache 和 temp store 初始化，不改变已有启动顺序或日志。
- 新增连接池级测试，同时持有 5 个 `sql.Conn` 并逐个断言 `PRAGMA busy_timeout=30000`。
- 压力测试的默认配置组复用生产 helper；旧 DSN 只保留为 legacy 对照。
- 全局测试库恢复、Provider 隔离库和 Session Usage 隔离库统一复用生产 helper，避免测试中途重绑后丢失连接属性。

#### 明确不修改

- 不改变 30 秒数值、连接池大小、WAL、cache、temp store 或队列容量/批大小。
- 不迁移 Provider 改名、健康清理、日志维护或 WebDAV 事务。
- 不新增重试循环、全局锁、事务抽象、依赖、配置项、环境变量或 schema。
- 不改变 Wails API、错误文案、数据格式或用户操作语义。

#### 验证方式

- 测试先行：新增连接属性测试后先确认因 `defaultSQLiteDSN` 不存在而编译失败，再实现最小修复。
- 单元测试：`TestDefaultSQLiteDSNAppliesBusyTimeoutToEveryConnection` 单次及 `-count=20`。
- 性能测量：`TestSQLiteRuntimeWriteContentionBaseline` 运行 5 轮，记录成功数、busy、成功 P50/P95、吞吐和等待。
- 竞态检测：两个 B14 目标测试以 `-race` 运行。
- 完整回归：`go test ./services/... -count=1`、`go test ./resources/model-pricing/... -count=1`。
- 行为一致性：临时库跨表改名、日志行数和健康清理断言全部保留；未读取用户数据库。

#### 验收标准

- 连接池 5 个同时持有的连接均报告 `busy_timeout=30000`，连续 20 次通过：满足。
- 生产默认配置场景连续 5 轮均为 596/596 成功、0 次 `SQLITE_BUSY/LOCKED`：满足。
- 所有数据一致性、竞态、完整 services 和资源测试通过：满足。
- 修改前后成功数、busy 与成功调用 P50/P95 已记录：满足。

#### 风险与回退

- 风险：原本快速失败的写入将等待 writer，可能放大用户操作延迟并占用连接池。
- 控制：保持既有 30 秒上限；最终 5 轮默认组总耗时最高 109.045 ms，所有 596 次操作完成；未同时增加重试或改队列结构。
- 回退：从默认 DSN 移除 `_pragma` 参数并恢复测试预期；不涉及数据、schema 或配置迁移。

#### 执行清单

- [x] 修改前重新读取相关代码
- [x] 确认现有行为
- [x] 补充必要测试
- [x] 完成最小修改
- [x] 运行验证
- [x] 记录前后对比
- [x] 更新问题和批次状态
- [x] 更新变更文档

#### 执行记录

- 开始时间：2026-08-31 18:31:54 CST
- 完成时间：2026-08-31 18:42:19 CST
- 实际修改：新增默认 SQLite DSN helper 并用于生产初始化；新增 5 连接属性测试；压力测试默认组与三处测试数据库重绑入口复用同一 helper。
- 验证结果：目标单测、20 次重复、5 轮压力、目标竞态、完整 services 和模型定价资源测试全部通过；仅有既有 macOS 链接目标版本警告。
- 性能对比：修改前直写竞争组 busy 中位数 592/596、成功中位数 4/596；修改后默认配置 5 轮均为 busy 0/596、成功 596/596，总耗时中位数 106.120 ms，日志成功 P50/P95 中位数 42.234/50.176 ms。
- 遗留问题：数据来自固定临时库合成压力，不能外推真实调用频率；其他连接级 PRAGMA 与 WebDAV 独立快照连接不在本批范围。

## 8. 待分析模块

以下模块尚未完成足以形成 OPT 问题的深入分析，首版不得视为完整审计：

| 模块 | 后续分析重点 | 当前处理 |
|---|---|---|
| Gemini 与自定义 CLI 代理 | 与通用 Handler 的可复用边界、流式转发和错误语义 | B10 明确排除，后续单独分析 |
| BlacklistService | 全局 `operationMu`、失败路径读写次数、全表快照刷新和按 Provider 并发可能性 | 先保留现状，需独立并发测量 |
| AppSettings 快照 | 每秒外部变化检查、克隆和保存回调成本 | 尚未测量 |
| 健康检查与可用性 | goroutine 上限、探测并发、历史清理和后台轮询 | 仅分析了与黑名单和直写相关入口 |
| 日志清理与 WebDAV 恢复 | 大事务、VACUUM、ATTACH/DETACH 和队列协作 | 视为显式维护路径，需独立恢复测试 |
| 模型转换与 SSE | Claude/OpenAI/Gemini 转换状态机的 CPU、分配和缓冲策略 | 已有较多测试，尚无系统性能基准 |
| 更新与下载服务 | 后台 Timer、下载内存、临时文件和退出清理 | 未深入分析 |
| 其余大型前端文件 | `ProviderEditModal.vue`、`Availability/Index.vue`、`Console/Index.vue`、`CLIConfigEditor.vue` 等 | 只记录体量，不据此认定性能问题 |
| Wails 调用封装 | generated bindings 与 `Call.ByName` 混用、调用去重和类型保护 | 未确认有实际缺陷，后续按调用链分析 |
| 主进程后台 goroutine | 更新、会话同步、路由刷新、健康检查的启动/关闭协作 | 当前均有主要停止路径，尚未发现可确认泄漏 |

## 9. 已完成优化

以下批次已经实施并验证完成：

| 批次 | 对应问题 | 完成时间 | 实际修改 | 验证结果 | 前后数据 |
|---|---|---|---|---|---|
| B01 | OPT-003 | 2026-08-31 15:17:39 CST | 新增通用代理调度表驱动行为矩阵，不改生产逻辑 | 新增测试、竞态检测和完整 services 测试通过 | 无统一组合矩阵 → 1 个矩阵、5 个代表场景 |
| B02 | OPT-007 | 2026-08-31 15:34:12 CST | 新增 DBWriteQueue 专属测试和 3 个子基准，不改生产逻辑 | 目标测试、20 次重复、竞态检测、5 轮基准和完整 services 测试通过 | 无专属基线 → 5 个测试入口、3 个子基准；详见 B02 执行记录 |
| B03 | OPT-004 | 2026-08-31 16:04:13 CST | 新增代理热路径行为测试和 32 个子基准，不改生产逻辑 | 目标测试、竞态检测、分组 5 轮基准和完整 services 测试通过 | 无热路径基线 → 请求读取、响应 Hook、捕获、脱敏和 SSE 均有数据；确认 OPT-014 |
| B04 | OPT-005、OPT-006 | 2026-08-31 16:25:11 CST | 新增 Provider 深拷贝隔离测试和 15 个快照子基准，不改生产逻辑 | 目标测试、竞态检测、5 轮基准和完整 services 测试通过 | 无 Provider 快照基线 → 克隆、命中、指纹和稳定刷新均有数据；确认 B13 |
| B05 | OPT-009 | 2026-08-31 17:09:29 CST | 新增日志聚合筛选/查询计划测试、63 个子基准和前端精确调用次数断言，不改生产逻辑 | 目标测试、5 轮基准、完整 Go/前端测试和 diff 检查通过 | 无重复聚合基线 → 100k Proxy 冷刷新 3.124-3.201 s、约 2.617 GB/op、170,031 个完整对象；确认 B06 |
| B06 | OPT-009 | 2026-08-31 17:54:09 CST | 新增兼容聚合 Wails 方法，复用一次主范围日志并切换前端刷新调用 | 新旧逐字段等价、竞态、类型、完整回归和 81 个子场景 5 轮基准通过 | 100k Proxy 冷缓存中位数 3.435 s → 1.248 s，2,617.259 MB → 783.718 MB；OPT-009 完成 |
| B07 | OPT-008 | 2026-08-31 18:21:47 CST | 完成六类 SQLite 直写审计，新增双队列/直写/逐连接 timeout 临时库压力测试，不改生产逻辑 | 目标测试 5 轮、竞态检测、原子一致性和完整 services 测试通过 | 当前直写组 `SQLITE_BUSY` 中位数 593/596；逐连接 timeout 候选为 0/596，确认 OPT-015/B14 |
| B14 | OPT-015 | 2026-08-31 18:42:19 CST | 默认 SQLite DSN 加入逐连接 `busy_timeout`，生产与隔离测试库复用同一 DSN helper | 5 连接断言 20 次、目标 5 轮、竞态、完整 services 与资源测试通过 | 旧直写组修改前 busy 中位数 592/596；默认配置修改后连续 5 轮 0/596、596/596 成功 |
| B08 | OPT-012 | 2026-08-31 19:54:22 CST | 统一主页面轮询生命周期，隐藏时暂停、恢复刷新后按原周期继续，并补齐 Linux common 可见性事件 | 生命周期 5 项、Main 48 项、前端 543 项、类型检查及分包 Go 测试通过 | 每 60 秒高层 Wails 轮询轮次：原隐藏态 41 → 新隐藏态 0；可见与恢复态均保持 41 |
| B09 | OPT-010、OPT-011 | 2026-08-31 20:23:18 CST | 抽离 General 初始化生命周期，AppSettings 后六项独立读取并行，订阅和卸载清理统一管理 | 新增 6 项、General 25 项、完整前端 549 项及类型检查通过 | 相同 100 ms 假延迟：七段串行 700 ms → 一段依赖加六项并行 200 ms |
| B10 | OPT-001、OPT-002 | 2026-08-31 20:48:16 CST | 收敛通用代理尝试状态、成功/失败记录、停止条件、会话恢复和最终错误出口；补充拉黑会话矩阵 | 6 场景矩阵重复与竞态、错误透传、会话迁移、完整 services 与 root 测试通过 | `proxyHandler` 737 → 620 行；文件 11,447 → 11,402 行；OPT-002 完成，OPT-001 保持待分析 |
| B11 | OPT-013 | 2026-08-31 20:59:57 CST | 正常完成时等待事务组全部结果通道关闭后返回，新增双任务切片 1000 次复用回归 | 修改前竞态稳定复现；修改后目标重复/竞态、全部队列竞态、ProviderStore、services 与 root 测试通过 | `tx_group` 中位数 39,830 → 39,360 ns/op，32 allocs/op 不变；无可确认回退 |
| B12 | OPT-014 | 2026-08-31 21:27:16 CST | 无分配 ASCII quick check、Unicode 正则回退，并移除会话标识和已捕获请求体的重复脱敏 | 逐字节/门控/Unicode/路径等价、竞态、矩阵、完整 services 与 root 测试通过 | 1 MiB raw 410.463 → 2.051 ms、sanitized 1.334 s → 334.701 ms；近 8 MiB raw 3.219 s → 16.699 ms、sanitized 10.498 s → 2.626 s |
| B13 | OPT-006 | 2026-08-31 22:07:28 CST | SQLite 平台改用有序存储行指纹，保存端返回事务对应指纹；raw 非语义变化只同步指纹 | 决策矩阵、保存一致性、custom、外部刷新竞态、5 轮基准、完整 services 与 root 测试通过 | 100 个 Provider 稳定刷新中位数 2,077.316 → 446.699 µs，约降 78.5%；allocs/op 约 21,525 → 1,624 |

## 10. 放弃或延期项

| 项目 | 状态 | 原因 | 重新评估条件 |
|---|---|---|---|
| 一次性重写整个 `providerrelay.go` | 延期 | 风险过高且没有独立收益证据；B10 只收敛通用调度路径 | 多个后续批次证明其他边界可独立拆分 |
| 直接减少代理日志或负载捕获 | 延期 | 可能改变诊断能力和用户配置语义 | B03 证明具体日志/捕获步骤是主要成本，并另立需求确认行为 |
| 新增数据库列或改变现有数据格式 | 放弃 | 违反本计划不可变行为 | 只能由独立产品需求重新授权，不属于优化任务 |
| 无测量数据的缓存层 | 放弃 | 会引入失效和陈旧语义，收益不可验证 | 出现可重复性能数据并能定义严格失效规则 |
| 将所有运行时直写统一迁入 DBWriteQueue | 延期 | B14 已消除合成压力中的 busy；复合事务、维护操作和外部数据库语义不同，继续统一没有明确收益且会扩大风险 | 真实调用仍出现 busy，或测量证明队列外事务等待不可接受 |

## 11. 后续执行规则

1. 每次只执行一个已确认批次。
2. 执行前重新读取当前代码，文档内容不能替代代码事实。
3. 不自动执行下一个批次。
4. 完成后立即更新本 MD 的状态、证据和执行记录。
5. 新发现的问题分配新的 OPT 编号，不打乱已有编号。
6. 方案与当前代码不符时，先更新文档再实施。
7. 没有性能对比数据时，不得标记为性能优化完成。
8. 批次可以持续新增，不要求一次规划完整个项目。
9. 性能问题的“待测量”只能在可重复数据产生后移除。
10. 测量批次可以完成而对应问题仍保持“待分析”；是否实施必须单独记录。
11. 新增兼容 Wails 方法不得替代、删除或改变现有方法。
12. 新增 SQLite 索引必须有基准和查询计划证据，并在实施前单独确认；不允许顺带修改表列或数据格式。
13. 每个实施批次更新本计划，并按项目规则在 `doc/changes/` 创建或更新该任务唯一变更记录。
14. 回退说明只描述恢复范围和方式，不自动执行 Git 提交、重置或回滚操作。

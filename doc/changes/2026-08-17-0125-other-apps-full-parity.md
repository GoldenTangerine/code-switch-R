/**
 * @name: 其他应用配置完整对齐
 * @Descripttion: 跟踪多应用配置、统一存储、资源管理与验证的完整实施状态
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-17 01:25:16
 * @LastEditTime: 2026-08-17 01:25:16
 * @FilePath: doc/changes/2026-08-17-0125-other-apps-full-parity.md
 */

# 其他应用配置完整对齐

- 变更时间：2026-08-17 01:25:16 CST（Asia/Shanghai）
- 参考项目：`/Users/shumin/Desktop/工作/杂项/功能项目/cc-switch/cc-switch`
- 涉及范围：统一 SQLite 存储、应用配置、代理、托管认证、MCP、Skill、Prompt、会话、用量、首页入口、图标与国际化

## 状态说明

- `待修改`：已纳入范围，尚未开始。
- `修改中`：正在实现或验证，不能视为完成。
- `已完成`：实现完成且对应验证通过。
- `未修改`：明确保持现状，并记录原因。
- `不适用`：目标应用不支持该能力。

## 实施状态矩阵

| 阶段 / 应用 | 存储迁移 | 供应商 | 原生配置 | 代理 | 认证 | MCP | Skill | Prompt | 会话 / 用量 | UI / i18n | 图标 | 测试 |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 共享基础 | 已完成 | 已完成 | 不适用 | 不适用 | 不适用 | 不适用 | 不适用 | 不适用 | 不适用 | 已完成 | 已完成 | 已完成 |
| OpenCode | 已完成 | 已完成 | 已完成 | 不适用 | 不适用 | 已完成 | 已完成 | 已完成 | 已完成 | 已完成 | 已完成 | 已完成 |
| Grok Build | 已完成 | 已完成 | 已完成 | 已完成 | 已完成 | 已完成 | 已完成 | 已完成 | 已完成 | 已完成 | 已完成 | 已完成 |
| Claude Desktop | 已完成 | 已完成 | 已完成 | 已完成 | 不适用 | 不适用 | 不适用 | 不适用 | 不适用 | 已完成 | 已完成 | 已完成 |
| OpenClaw | 已完成 | 已完成 | 已完成 | 不适用 | 不适用 | 不适用 | 不适用 | 已完成 | 不适用 | 已完成 | 已完成 | 已完成 |
| Hermes | 已完成 | 已完成 | 已完成 | 不适用 | 不适用 | 已完成 | 已完成 | 已完成 | 不适用 | 已完成 | 已完成 | 已完成 |
| Pi | 已完成 | 已完成 | 已完成 | 不适用 | 不适用 | 不适用 | 已完成 | 已完成 | 已完成 | 已完成 | 已完成 | 已完成 |

## 涉及文件

### 已修改

- `doc/changes/2026-08-17-0125-other-apps-full-parity.md`：创建任务主文档和状态矩阵。
- `services/platform.go`（新建）：应用平台常量（复用 CLIPlatform）与能力判定（IsAdditivePlatform / PlatformSupportsProxy / SupportsMCP / SupportsSkill / SupportsPrompt）。
- `services/providerstore.go`（新建）：providers_store 统一存储表、三平台格式 CRUD（Provider / OpenCodeProvider / GeminiProvider）、失败自动恢复、nil/空哨兵语义、JSON→SQLite 事务式迁移器（含 codex 顶层旧位置回退）。
- `services/testdb_main_test.go`（新建）、`services/providerstore_test.go`（新建）、`services/platform_test.go`（新建）：测试基建与统一存储/平台判定单测。
- `services/database.go`：InitDatabase 挂载 providers_store 建表与启动迁移。
- `services/providerservice.go`：Load/Save/Duplicate 切统一存储（签名不变），custom CLI 保持 JSON 回退，快照指纹改读存储。
- `services/geminiservice.go` / `services/opencodeservice.go`：底层存取切统一存储；OpenCode 保留 live 首次导入。
- `services/directapply_helpers.go` / `services/appsettings.go`：直读 JSON 的两处生产代码切统一存储。
- 存量测试适配：session_usage_test / logstorage_service_test teardown 恢复全局库；useIsolatedHomeDir 升级为 HOME+DB+队列全套隔离；9 个 fixture 类测试改「写 JSON+触发迁移器」模式。

### 计划修改

- 数据库初始化、供应商、MCP、Skill、Prompt、代理、认证及各应用原生配置服务。
- 首页应用切换、供应商表单、资源页面、会话页面、设置页、图标及中英文文案。
- 与迁移、原生配置、代理、认证、资源和界面状态相关的测试。

## 明确未修改范围

- 不改变现有 Claude、Codex、Gemini 的代理协议语义，只替换存储适配并扩展共享资源能力。
- 不删除、重置或默认展示“其他”自定义 CLI 数据。
- 不自动纳管、覆盖首次发现的原生配置。
- 不新增 JSONC、TOML、YAML 解析依赖。
- 不新增代理端口，继续复用 `18100`。
- 不迁移合作标识、推广文案、邀请参数或推广 URL。
- 不修改版本号、构建元数据，不执行 Git 提交、Tag、推送、发布、Wails 构建或打包。

## 变更记录

- 2026-08-17 01:25:16：创建主文档；所有计划项尚未开始。
- 2026-08-17 04:20:00：阶段 0（共享基础）完成——`services/platform.go`（平台常量+能力判定）、`services/providerstore.go`（统一存储+迁移器）、providerservice/geminiservice/opencodeservice 存取切 SQLite、directapply/appsettings 两处直读 JSON 修复、测试基建（TestMain+隔离 helper）、9 个存量测试适配、5 个新单测。WebDAV/import/deeplink/customcli 排查确认无需改动。
- 2026-08-17 05:20:00：阶段 1（OpenCode 升级）完成——预设裁剪 42→17（剥离 6 处推广字段、移除 omo 类型）；MCP 投影（mcpservice.go syncOpenCodeServers/loadOpenCodeEnabledServers/buildOpenCodeEntry，local/remote 格式）；Skill（getInstallPath → ~/.config/opencode/skills）；Prompt（OpenCode 桶 → ~/.config/opencode/AGENTS.md）；用量采集（session_usage_opencode.go 只读解析 opencode.db，data_source=opencode_session）；前端 mcp.ts/skill.ts/Prompts 页平台扩展 + i18n。
- 2026-08-17 06:00:00：阶段 2（Grok Build）完成——groksettings.go（config.toml 读写走通用 map 保留 mcp_servers 等未知顶层键、代理接管三字段替换、官方态判定、首次导入）；Provider.ConfigTOML 字段；代理路由 /grokbuild/v1/{responses,chat/completions} + /grokbuild/v1/models（proxyHandler 复用降级/轮询/黑名单）；xaioauthservice.go（设备码流 + discovery + 7 单测）；session_usage_grokbuild.go（updates.jsonl 差量/全量双模式，data_source=grok_session）；MCP/Skill/Prompt 的 grokbuild 投影（[mcp_servers]、~/.grok/skills、~/.grok/AGENTS.md）；前端全套（tab、grokSettings.ts、预设 4 个、TOML 编辑弹窗、adapters/composables、i18n）。
- 2026-08-17 06:40:00：阶段 3（Claude Desktop）完成——claudedesktopservice.go（macOS/Windows 双目录 4 文件事务式写入与回滚、Direct/Proxy 双模式、gateway token 存 app_settings、默认四模型路由、官方 1p 态恢复、首次导入，5 单测）；Provider 新增 claudeDesktopMode/claudeDesktopModelRoutes；main.go 注册；前端全套（tab、claudeDesktopSettings.ts、模式单选+模型路由编辑弹窗、预设 2 个、composables 全链路、i18n）。
- 2026-08-17 07:10:00：阶段 4（OpenClaw）完成——openclaw_json5.go（自研 JSON5 归一化：裸键/单引号/尾逗号/注释/十六进制，6 语料测试）；openclawservice.go（additive 模式 CRUD/SetCurrentProvider/live 子树替换保留用户顶层键/首次导入/Get/SetEnvConfig/ToolsConfig/AgentsConfig，10 单测）；providerstore OpenClaw 适配层；Skill（~/.openclaw/skills）+Prompt（~/.openclaw/AGENTS.md）case；前端全套（tab、openClaw.ts 13 方法封装、env/tools/agents 三个专属子页+路由 /openclaw-config+侧栏入口、预设 3 个、i18n 51 键）；联调修复 5 缺陷（Set 参数拆分/id 改 string/导入数量判断/状态键名兼容/useProviderForm 遗漏转换）+ GetPrompts 补 opencode/grokbuild 既有缺口。
- 2026-08-17 07:45:00：阶段 5（Hermes）完成——hermesservice.go（yaml.v3 Node 模式只替换 custom_providers/model 子树保留未知键与注释、无 id 手写条目按 name+base_url 匹配回写、切换更新顶层 model 节、首次导入，10 单测）；hermes_memory.go（MEMORY.md/USER.md 整文件读写 + § 条目切分 + config.yaml 四键设置）；providerstore Hermes 适配层；MCP（config.yaml mcp_servers）/Skill（~/.hermes/skills）/Prompt（~/.hermes/SOUL.md）case；前端全套（tab、hermes.ts 13 方法、MemoryPanel 子页+/hermes-memory 路由+侧栏、预设 3 个、i18n）；openclaw 载荷缺陷修复（apiUrl→baseUrl 键对齐 + id 去数字化）。
- 2026-08-17 08:30:00：阶段 6（Pi）完成——piservice.go（additive：~/.pi/agent/models.json providers 子树读写、Model 为应用侧元数据不入 live、8 单测）；Skill（~/.pi/agent/skills）/Prompt（~/.pi/agent/AGENTS.md）；前端全套（tab、pi.ts、预设 3 个、i18n）。
- 2026-08-17 08:45:00：收尾阶段完成——MainPlatformTabs.vue tab 自适应（≤4 图标+名称 / >4 纯图标+tooltip，判定基于用户显隐后的实际渲染数）；openclaw/hermes/pi 三应用品牌图标（claw.svg 内联渐变重命名、hermes/pipellm 缩图内嵌 SVG，platformBrandIcons.ts + providerIconAssets 注册）；托盘核实零改动（品牌图标数据驱动自动生效，预算面板仅 claude/codex 属设计使然）；i18n zh/en 2253 键零差异。
- 2026-08-17 09:30:00：全量 code review（四路并行审查）+ 修复——发现 P0×3（providers_store 排序丢失 sort_index；OpenClaw 前端缺专用 mapper 致 baseUrl/model 读取丢失+保存抹除 live；OpenClaw 字符串 id 被 Number() 成 NaN 致删除/拖拽失效）与 P1×18，全部修复：dbqueue 新增 ExecTxGroup 事务组（替换写入原子化）、原行读取 fail-fast、DuplicateProvider 改 cloneProvider、四 additive 平台空哨兵不再复活导入、迁移单平台损坏隔离+跳过补 .bak、重复 id 显式报错、Grok EnableProxy 幂等+DisableProxy 死锁解除+api_backend 保留、Grok 会话记录 scope 隔离、Grok/CD 导入跳过代理态、Hermes 手写条目仅补 id 不覆盖、CD 官方态只清 thirdPartyDir、OpenClaw/Hermes/Pi Add/Update 返回完整 provider 供前端精确回填、GetStatus 补 settingsPath/providerId、前端保存锁+dirty 未保存保护+ID 负数区间防碰撞+Grok apiUrl 同步 TOML、三平台 additive 持久化回归测试固化 P0。新增/扩展测试 28 项。

## 验证结果

- 2026-08-17 04:20:00（阶段 0）：`go vet ./services/` 通过；`go test ./services/ -count=1` 全量 `ok`（543+ 测试，0 失败）；新增 5 个统一存储单测全部 PASS（迁移幂等、跳过已初始化平台、nil/空语义、字段往返、Gemini/OpenCode 往返）。
- 2026-08-17 05:20:00（阶段 1）：`go vet ./services/` 通过；`go test ./services/ -count=1` 全量 `ok`；前端 `pnpm test:unit` 60 文件/458 测试全过；`vue-tsc --noEmit` 无错误。
- 2026-08-17 06:00:00（阶段 2）：`go vet ./services/` 通过；`go test ./services/ -count=1` 全量 `ok`（含 xAI OAuth 7 新测试）；`go build ./...` 仅余 cmd/updater 的 Windows 平台约束预期报错；`vue-tsc` 无错误；`pnpm test:unit` 61 文件/463 测试全过（含 Grok TOML 工具 5 新测试）；locale JSON 解析校验通过。
- 2026-08-17 06:40:00（阶段 3）：`go vet ./services/` 通过；`go test ./services/ -count=1` 全量 `ok`（含 Claude Desktop 5 新测试）；`go build ./...` 仅余 cmd/updater 预期报错；`vue-tsc` 无错误；`pnpm test:unit` 61 文件/463 测试全过。
- 2026-08-17 07:10:00（阶段 4）：`go vet`/`go test`（含 OpenClaw 16 新测试）通过；`vue-tsc` 无错误（联调修复后复验）；`pnpm test:unit` 61 文件/463 测试全过；前后端 13 个方法名核对一致。
- 2026-08-17 07:45:00（阶段 5）：`go vet`/`go test`（含 Hermes 10 新测试）通过；`go build ./...` 仅余 cmd/updater 预期报错；`vue-tsc` 无错误；`pnpm test:unit` 61 文件/463 测试全过。
- 2026-08-17 08:50:00（工程终验）：六项交付门槛全部通过——`go vet` 零告警；`go test ./services/` 全量 ok（17.8s）；`go build ./...` 仅余 cmd/updater Windows 平台约束预期报错；`vue-tsc` 零错误；`pnpm test:unit` 61 文件/463 测试全过；zh/en locale 解析校验通过（2253 键零差异）。
- 2026-08-17 09:35:00（review 修复终验）：六项复验全过——`go vet` 零告警；`go test ./services/` 全量 ok（17.8s，含新增 15 项后端测试）；`vue-tsc` 零错误；`pnpm test:unit` 62 文件/476 测试全过；前后端契约核对一致（三平台 Add/Update 返回值解析、GetStatus 键、JSON tag 全对齐）。

遗留（记录不修，供后续迭代）：additive 平台 composables 工厂化收敛（约 250-300 行重复）；指纹 O(1) 化；JSON5 代理对/Infinity 边界；xAI OAuth 前端登录 UI 与 relay token 注入（服务端已就绪）；OpenClaw/Pi 导入多条目 enabled 单选语义。

<!--
@name: Hooks 会话供应商联动变更
@Descripttion: 记录 CLI 会话标识与实际供应商活动的跨应用关联。
@version: 1.0.0
@Author: sm
@Date: 2026-09-09 12:40:00
@LastEditTime: 2026-09-09 12:40:00
@FilePath: doc/changes/2026-09-09-1240-codenotch-hook-session-routing.md
-->

# CLI Hooks 会话供应商联动

变更时间：2026-09-09 12:40:00，Asia/Shanghai。

## 涉及范围

CodeSwitchR 的请求转发、托盘快照服务及会话关联测试；相邻 Codenotch 项目的 macOS CLI hooks、通知设置、会话状态和供应商展示。不涉及 CodeSwitchR 前端、Windows 接入或远程审批。

## 变更内容

- 在实际供应商转发路径记录会话关联，独立于会话粘滞开关；并行会话分别记录，后续降级尝试更新对应会话的供应商。
- 复用 Claude 请求会话解析，支持 Codex 明确的会话／线程请求头和结构化标识；不将缓存键或仅有的父会话标识猜测为当前 CLI 会话。
- Claude 请求存在 `x-claude-code-agent-id` 时不覆盖父 CLI 的关联。
- 托盘快照保持版本 1，在平台对象增加可选 `sessionBindings` 数组。旧客户端忽略新字段，新客户端仍兼容旧快照。
- 每项包含 `sessionKey`、`providerId`、`providerName`、`icon`、`sequence`、`updatedAt`。会话键为工具名称、一个 LF 和去除首尾空白的原始会话 ID 拼接后的 SHA-256 小写十六进制值；`sequence` 在本次进程内单调递增，`updatedAt` 为 Unix 毫秒时间戳。两端使用同一组固定样例验证键的一致性。
- 关联仅保存在内存，最多 4,096 项，24 小时过期；请求结束不会立即清除关联，以便显示等待用户的供应商。快照不包含凭证、提示词、工具参数或原始请求正文。
- Codenotch 使用 hooks 识别运行、完成和等待，按关联展示供应商；请求结束后仍能保留等待入口和最近额度。断线或无法可靠关联时回退到 CLI 工具入口，不误标其他供应商，不重复响铃。

## 验证结果

- CodeSwitchR 托盘、会话标识、转发和会话相关 Go 回归测试通过。
- 新增关联测试通过，包括两端会话键、明确标识提取、并行会话、降级更新、过期、无敏感字段和并发发布；`go test -race ./services -run 'TestHookSession|TestTrayHook' -count=1` 通过。
- Codenotch 新增 19 项 Swift Testing 检查通过，覆盖状态转换、重复／乱序事件、并行等待、退出、配置安装、真实 socket 和三边明暗主题渲染。
- Swift 应用源码类型检查及模块编译链接、helper 编译和独立通信验证通过；XcodeGen 工程生成与项目文件校验通过；中文本地化完整性检查通过。
- AppKit 离屏渲染确认供应商问号角标、等待详情与会话行可见。
- 本机 CLI 版本为 Codex 0.153.4、Claude Code 2.1.263；隔离 Codex 配置确认 hooks 默认启用。未修改个人 CLI 配置、未启动真实模型会话，因此真实提问／审批到终端跳转的完整联调尚未执行。
- 环境缺少 Xcode 和 XCTest，未执行 Xcode 应用打包及原有完整 XCTest 测试套件；未启动或打包 CodeSwitchR。

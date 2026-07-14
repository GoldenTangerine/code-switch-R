# 代理转发低延迟优化

- 变更时间：2026-07-15 00:22:00 CST（Asia/Shanghai）
- 涉及范围：Claude、Codex、自定义 CLI 转发、供应商与黑名单配置、请求日志、日志详情

## 变更内容

- 供应商、应用设置和黑名单路由状态改为并发安全内存快照；供应商与应用设置快照对可变字段执行深拷贝，避免调用方修改污染缓存。
- 快照 watcher 改为由应用生命周期显式启动和停止，临时服务及单元测试不再因构造函数自动启动后台任务而泄漏 goroutine。
- 黑名单本地写入和设置保存后立即刷新；后台完整校准由每秒一次调整为每分钟一次，并在扫描异常时保留上一份完整状态。
- 请求计划改为按实际尝试供应商懒生成，避免大请求体按候选供应商数量重复复制。
- 上游请求共享 HTTP Transport 连接池，开启 HTTP/2 并增加每主机空闲连接容量。
- 客户端断开时取消上游请求，取消错误不进入失败计数或黑名单。
- 流式转发移除 `Peek(1024)`，首个完整 SSE 行到达后立即转发和 Flush；保留单行 JSON 回退、Hook 和错误透传。
- 控制台捕获改为 4096 条有界保序队列，日志批量落库窗口由 100ms 缩短为 10ms。
- 请求日志新增代理准备、DNS、TCP、TLS、上游首字节、转发等待和连接复用指标，在请求详情中展示；兼容性重试按每次尝试累计耗时，首写时间不再包含客户端写阻塞。

## 验证结果

- `TZ=UTC go test ./services/... -count=1`：通过。
- 针对快照、连接复用、流式首行和取消传播的 `go test -race`：通过。
- 针对快照深拷贝、黑名单异常回退、兼容重试计时和 watcher 生命周期的聚焦测试：通过。
- `GOCACHE=/tmp/code-switch-r-go-cache TZ=UTC go test ./services/... ./resources/model-pricing/... -count=1`：通过，仅使用沙箱临时缓存与隔离测试 HOME。
- `GOCACHE=/tmp/code-switch-r-go-cache TZ=UTC go test -race ./services -run 'Test(AppSettingsSnapshot|ProviderServiceSnapshot|RelayPerformanceTrace|RelayTimedResponseWriter|BlacklistSnapshot)' -count=1`：通过。
- `pnpm --dir frontend exec vue-tsc --noEmit`：通过。
- `pnpm --dir frontend test:unit`：44 个测试文件、314 个测试通过。
- 4KB / 256KB / 2MB 请求准备基准约为 0.003ms / 0.126ms / 0.705ms，未触发 3ms 第二阶段门槛。
- 默认 Asia/Shanghai 时区在每日 00:00–01:00 会触发既有 `TestProviderDailyStats_UsesLatestFiveSuccessfulStreamingSamplesAcrossDays` 日界线用例失败；固定 UTC 后全量通过，与本次变更无关。

## 发布信息

- 版本：`v2.8.95`
- 发布说明：已写入 `RELEASE_NOTES.md`。
- 触发方式：推送 `v2.8.95` Tag 后由 GitHub Actions 自动构建全平台安装包。

<!--
@name: 负载脱敏快速路径
@Descripttion: 记录请求日志负载快速门控和重复脱敏优化的行为保护与性能对比
@version: 1.0.0
@Author: sm
@Date: 2026-08-31 21:27:16
@LastEditTime: 2026-08-31 21:27:16
@FilePath: doc/changes/2026-08-31-2127-payload-sanitize-fastpath.md
-->

# 负载脱敏快速路径

## 变更时间

2026-08-31 21:27:16 CST（Asia/Shanghai）

## 涉及范围

- `services/providerrelay.go`
- `services/providerrelay_payload_sanitize_test.go`
- `services/providerrelay_hotpath_benchmark_test.go`
- `doc/code-performance-optimization-plan.md`

## 变更内容

- 使用无分配 ASCII 大小写折叠扫描替代 ASCII payload 的两组正则快速检查；实际脱敏仍使用原替换正则。
- 非 ASCII payload 回退原 quick regex，保留 Go regexp 的 Unicode SimpleFold 语义，并避免干净多语言内容进入全部替换正则。
- `maybeSanitizeRequestLogPayload` 在开启通用脱敏时不再预先重复执行会话标识脱敏。
- `prepareRequestLogPayloadForPersistence` 从 `requestBodyBytes` 完成捕获后，不再对同一 RequestBody 重复脱敏；预填充 RequestBody 路径保持原处理。
- 新增逐字节 JSON、纯文本、Header、Query、数值、布尔、无效 JSON、Unicode 和干净 payload 行为基线。
- 未修改敏感字段和会话标识清单、`[REDACTED]`、8 MiB 上限、UTF-8 截断、SSE Hook、数据库字段、捕获默认值、公共 API、配置或依赖。

## 性能对比

同机同命令各 5 轮中位数：

| 场景 | 修改前 | 修改后 | 结果 |
|---|---:|---:|---:|
| 会话标识 quick check 1 MiB | 131.527 ms | 0.949 ms | 约 -99.3% |
| 敏感词 quick check 1 MiB | 71.852 ms | 0.678 ms | 约 -99.1% |
| raw 捕获 1 MiB | 410.463 ms | 2.051 ms | 约 -99.5% |
| sanitized 捕获 1 MiB | 1.334 s | 334.701 ms | 约 -74.9% |
| raw 捕获近 8 MiB | 3.219 s | 16.699 ms | 约 -99.5% |
| sanitized 捕获近 8 MiB | 10.498 s | 2.626 s | 约 -75.0% |

- 1 MiB sanitized B/op 中位数约 34.71 MB 降至 24.13 MB。
- 近 8 MiB sanitized B/op 中位数约 276.95 MB 降至 192.99 MB。
- capture_off 保持百纳秒级，代码路径未改变。

## 验证结果

- 逐字节 golden、快速门控、Unicode、捕获来源等价和既有 payload/截断/转发测试通过。
- 目标 payload 测试的 `go test -race` 通过。
- B01 调度矩阵通过。
- `go test ./services/... -count=1` 通过。
- `go test . -count=1` 通过。
- 三组修改前后基准均完成 5 轮。
- `git diff --check` 通过。
- Go 测试仅有项目既有 macOS 链接目标版本警告。

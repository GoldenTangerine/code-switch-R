# Code Switch R - macOS 内存优化指南

## 问题描述

应用在 macOS 上打包后，即使仅托盘模式运行（不打开主界面），内存占用约 **400MB**。目标是将空闲状态内存降低到 **150-200MB** 以内。

## 内存占用分析

### 内存分布估算（当前 ~400MB）

| 组件 | 估算内存 | 说明 |
|------|---------|------|
| WebKit 主窗口进程 | ~150-200MB | 即使 Hidden 也会加载 WebKit 引擎 |
| WebKit 托盘窗口进程 | ~80-120MB | macOS 独有的 tray 附属窗口 |
| Go 运行时 + 服务 | ~40-60MB | 30+ 个服务实例 + Gin HTTP 服务器 |
| Embed 前端资源 | ~3.5MB | frontend/dist 嵌入二进制 |
| SQLite + 连接池 | ~5-10MB | WAL 模式 + 预热连接 |
| 后台 goroutine | ~5-10MB | 健康检查、黑名单定时器、更新检查等 |

### 核心问题

**最大的内存消耗者是 WebKit/WebView 进程**。Wails 3 在 macOS 上使用 WKWebView，每个窗口实例都会创建独立的 WebKit 渲染进程。当前应用创建了 **2 个 WebView 窗口**：
1. `mainWindow` — 主窗口（启动即创建，通过 Hide 隐藏而非销毁）
2. `trayWindow` — 托盘附属窗口（macOS 专用，启动即创建）

---

## 优化方案（按优先级排列）

---

### P0: WebView 窗口延迟创建（预计节省 150-250MB）

**这是最关键的优化点。** 当前两个 WebView 在应用启动时立即创建，即使用户只需要托盘功能。

#### 方案 A: 主窗口延迟创建（推荐）

**文件**: `main.go`

**当前代码** (第 307-320 行):
```go
// 启动时就创建主窗口
mainWindow := app.Window.NewWithOptions(application.WebviewWindowOptions{
    Title:  "Code Switch R",
    Width:  1700,
    Height: 1040,
    // ...
    URL:    "/",
})
```

**优化为**:
```go
var mainWindow application.Window
var mainWindowMu sync.Mutex

createMainWindow := func() application.Window {
    mainWindowMu.Lock()
    defer mainWindowMu.Unlock()

    if mainWindow != nil {
        return mainWindow
    }

    mainWindow = app.Window.NewWithOptions(application.WebviewWindowOptions{
        Title:  "Code Switch R",
        Width:  1700,
        Height: 1040,
        // ... 保持原有配置
        URL:    "/",
    })

    mainWindow.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
        mainWindow.Hide()
        handleDockVisibility(dockService, false)
        e.Cancel()
    })

    return mainWindow
}

// showMainWindow 中延迟创建
showMainWindow := func(withFocus bool) {
    win := createMainWindow()
    // ... 后续逻辑不变
}
```

**关键**: 应用启动时不调用 `showMainWindow(false)`，仅启动托盘。

#### 方案 B: 托盘窗口使用原生菜单替代 WebView

**文件**: `main.go` (第 375-404 行)

当前 macOS 托盘使用了一个完整的 WebView 窗口（`trayWindow`），这会创建一个完整的 WebKit 进程。如果托盘功能相对简单，可以改用系统原生菜单：

```go
// 移除 trayWindow 的创建
// 直接使用 systray.SetMenu() 展示信息（类似 Windows/Linux 的实现方式）
if runtime.GOOS == "darwin" {
    // 不再创建 trayWindow，使用与 Windows 相同的菜单方式
    refreshTrayMenu := func() {
        used, total := getTrayUsage(logService, appSettings)
        trayMenu := buildUsageTrayMenu(used, total, func() {
            showMainWindow(true)
        }, func() {
            app.Quit()
        })
        systray.SetMenu(trayMenu)
    }
    refreshTrayMenu()
    systray.OnClick(func() {
        refreshTrayMenu()
        systray.OpenMenu()
    })
}
```

**注意**: 如果托盘窗口有复杂 UI（如图表、自定义样式），则此方案需要权衡功能和内存的取舍。可以改为「点击托盘时才创建 trayWindow，关闭后销毁」的策略。

#### 方案 C: 托盘窗口按需创建/销毁

如果确实需要 WebView 托盘窗口的富 UI：

```go
var trayWindow application.Window

showTrayWindow := func() {
    if trayWindow == nil {
        trayWindow = app.Window.NewWithOptions(application.WebviewWindowOptions{
            // ... 原有配置
            URL: "/#/tray",
        })
        // 注册关闭 hook
        trayWindow.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
            trayWindow.Hide()
            // 延迟销毁，释放 WebKit 内存
            go func() {
                time.Sleep(500 * time.Millisecond)
                if trayWindow != nil {
                    trayWindow.Destroy() // 需要确认 Wails 3 是否支持
                    trayWindow = nil
                }
            }()
            e.Cancel()
        })
    }
    trayWindow.Show()
}
```

---

### P1: Go 运行时内存优化（预计节省 10-30MB）

#### 1.1 设置 Go GC 目标

**文件**: `main.go`，在 `main()` 函数最开头添加:

```go
import "runtime/debug"

func main() {
    // 降低 GC 目标百分比，让 GC 更积极地回收内存
    // 默认值 100，降低到 50 意味着更频繁 GC，但内存占用更低
    debug.SetGCPercent(50)

    // 设置内存软限制（可选，Go 1.19+）
    // 限制 Go 堆内存在 100MB 以内
    debug.SetMemoryLimit(100 * 1024 * 1024)

    // ... 原有代码
}
```

#### 1.2 设置环境变量 GOMEMLIMIT

在 macOS 的应用包 Info.plist 或启动脚本中设置：
```bash
export GOMEMLIMIT=100MiB
```

---

### P2: HTTP 服务器优化（预计节省 5-15MB）

#### 2.1 Gin 使用 Release 模式

**文件**: `services/providerrelay.go`，在创建 Gin router 之前:

```go
gin.SetMode(gin.ReleaseMode)
router := gin.New() // 使用 gin.New() 替代 gin.Default()，避免加载默认中间件
```

确认当前是否已设置为 Release 模式。Debug 模式会保留更多日志和调试信息。

#### 2.2 HTTP Server 超时配置

**文件**: `services/providerrelay.go` (第 394-405 行附近)

当前的 HTTP Server 缺少超时配置，可能导致连接泄漏：

```go
prs.server = &http.Server{
    Addr:           prs.addr,
    Handler:        router,
    ReadTimeout:    30 * time.Second,     // 读取请求超时
    WriteTimeout:   120 * time.Second,    // 写入响应超时（流式需要长一些）
    IdleTimeout:    60 * time.Second,     // 空闲连接超时
    MaxHeaderBytes: 1 << 20,              // 最大请求头 1MB
}
```

#### 2.3 HTTP Transport 连接池调优

**文件**: `services/healthcheckservice.go` (第 125-134 行)

```go
Transport: &http.Transport{
    MaxIdleConns:        10,      // 从 20 降为 10
    IdleConnTimeout:     15 * time.Second,  // 从 30s 降为 15s
    MaxIdleConnsPerHost: 2,       // 从 5 降为 2
    DisableKeepAlives:   false,   // 保持连接复用
}
```

对其他使用 HTTP Client 的地方做同样调整。搜索项目中所有 `http.Transport` 和 `http.Client` 的使用。

---

### P3: 数据库连接池优化（预计节省 2-5MB）

#### 3.1 限制 SQLite 连接池大小

**文件**: `services/database.go` (第 47 行之后)

当前没有设置连接池参数，Go 默认 `database/sql` 会创建无限连接：

```go
db, err := xdb.DB("default")
if err != nil {
    return fmt.Errorf("获取数据库连接失败: %w", err)
}

// 添加连接池限制
sqlDB := db // 如果 xdb.DB 返回的是 *sql.DB
sqlDB.SetMaxOpenConns(5)      // SQLite 单文件，不需要太多连接
sqlDB.SetMaxIdleConns(2)      // 空闲连接保持 2 个
sqlDB.SetConnMaxLifetime(30 * time.Minute)  // 连接最大存活时间
sqlDB.SetConnMaxIdleTime(5 * time.Minute)   // 空闲连接超时
```

#### 3.2 SQLite 内存优化 PRAGMA

在 `InitDatabase()` 中添加：

```go
// 限制 SQLite 页缓存大小（默认 -2000，即约 2MB）
// 降低到 500 页（约 2MB → 500KB）
db.Exec("PRAGMA cache_size = -500")

// 限制 mmap 大小（0 = 禁用 mmap）
db.Exec("PRAGMA mmap_size = 0")

// 降低临时存储的内存使用
db.Exec("PRAGMA temp_store = FILE")  // 临时数据写磁盘而非内存
```

---

### P4: 数据库队列缓冲优化（预计节省 2-3MB）

**文件**: `services/dbqueue.go` (第 34、39 行)

当前两个队列各有 5000 缓冲容量，对于桌面应用来说过大：

```go
// 当前
GlobalDBQueue = NewDBWriteQueue(db, 5000, false)
GlobalDBQueueLogs = NewDBWriteQueue(db, 5000, true)

// 优化后
GlobalDBQueue = NewDBWriteQueue(db, 500, false)      // 5000 → 500
GlobalDBQueueLogs = NewDBWriteQueue(db, 1000, true)   // 5000 → 1000
```

桌面应用的写入量远低于服务端应用，500-1000 的缓冲已经足够。

---

### P5: 请求日志负载缓冲优化（预计节省 0-8MB/请求）

**文件**: `services/providerrelay.go` (第 64 行)

```go
// 当前：单个请求最大缓冲 8MB
const requestLogPayloadMaxBytes = 8 * 1024 * 1024

// 优化为：降低到 1MB（对于日志记录来说足够）
const requestLogPayloadMaxBytes = 1 * 1024 * 1024
```

同时确保响应体缓冲在请求处理完成后被及时释放：

```go
// 在请求处理完成后显式释放缓冲区
reqLog.responseBodyBuffer = nil
reqLog.RequestBody = ""
reqLog.ResponseBody = ""
```

---

### P6: 服务延迟初始化（预计节省 5-10MB）

#### 6.1 非核心服务延迟初始化

当前在 `main()` 中一次性创建了 **30+ 个服务实例**。某些服务在托盘模式下完全不需要：

**文件**: `main.go`

可以延迟初始化的服务：
- `speedTestService` — 只在用户触发速度测试时需要
- `connectivityTestService` — 只在用户触发连通性测试时需要
- `importService` — 只在导入时需要
- `deeplinkService` — 只在处理 deeplink 时需要
- `envCheckService` — 只在检查环境时需要
- `consoleService` — 只在打开控制台时需要
- `skillService` — 只在打开技能市场时需要
- `promptService` — 只在管理 prompt 时需要

**实现方式**: 使用 `sync.Once` 包装延迟初始化：

```go
// 示例：延迟初始化 SpeedTestService
type LazySpeedTestService struct {
    once    sync.Once
    service *services.SpeedTestService
}

func (l *LazySpeedTestService) Get() *services.SpeedTestService {
    l.once.Do(func() {
        l.service = services.NewSpeedTestService()
    })
    return l.service
}
```

**注意**: 需要确认 Wails 3 的 `application.NewService()` 是否支持延迟初始化的服务。如果不支持，可以在服务内部实现延迟初始化逻辑（构造函数只保存配置，首次调用方法时才真正初始化）。

#### 6.2 健康检查延迟启动

**文件**: `main.go` (第 140-142 行)

当前启动时立即初始化健康检查：
```go
if err := healthCheckService.Start(); err != nil {
    log.Fatalf("初始化健康检查服务失败: %v", err)
}
```

可以延迟到用户打开主窗口或首次使用时再启动，或延迟更长时间（如 30 秒后）。

---

### P7: 前端资源优化（预计减少二进制大小 1-2MB）

#### 7.1 代码分割

**文件**: `frontend/vite.config.ts`

当前所有前端代码打包成单个 3.1MB 的 JS 文件。使用代码分割可以减少初始加载内存：

```typescript
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  build: {
    // 启用代码分割
    rollupOptions: {
      output: {
        manualChunks: {
          // 将大型依赖分离
          'chart': ['chart.js', 'vue-chartjs'],
          'codemirror': ['codemirror', '@codemirror/lang-json', '@codemirror/lang-markdown', '@codemirror/state', '@codemirror/theme-one-dark', '@codemirror/view'],
          'vendor': ['vue', 'vue-router', 'vue-i18n'],
        }
      }
    },
    // 压缩优化
    minify: 'terser',
    terserOptions: {
      compress: {
        drop_console: true,      // 移除 console.log
        drop_debugger: true,
      }
    },
    // 设置 chunk 大小警告阈值
    chunkSizeWarningLimit: 500,
  }
})
```

#### 7.2 依赖瘦身

检查 `frontend/package.json` 中以下大型依赖是否都有使用：

| 依赖 | 大小 | 建议 |
|------|------|------|
| `chart.js` + `vue-chartjs` | ~200KB | 如果只用于简单图表，考虑用轻量替代（如 unovis） |
| `codemirror` 全套 | ~300KB+ | 仅加载需要的语言包 |
| `@lobehub/icons-static-svg` | 未知 | 确认是否所有图标都在使用 |
| `@vuepic/vue-datepicker` | ~100KB | 确认是否真的需要 |
| `@headlessui/vue` | ~50KB | 如果只用少量组件，考虑自行实现 |

#### 7.3 字体优化

当前嵌入了 `Inter-Medium.ttf` (308KB)。考虑：
- 使用 `woff2` 格式替代 `ttf`（通常小 30-50%）
- 使用字体子集化（subset），只保留用到的字符

---

### P8: 编译优化（预计减少二进制大小 10-30%）

#### 8.1 Go 编译标志

```bash
# 使用 ldflags 去除调试信息
go build -ldflags="-s -w" .

# -s: 去除符号表
# -w: 去除 DWARF 调试信息
```

确认 `Taskfile.yml` 中的构建命令是否已包含这些标志。

#### 8.2 使用 UPX 压缩二进制（可选）

```bash
# 压缩后通常可减少 50-70% 体积
upx --best ./build/bin/CodeSwitch
```

**注意**: UPX 可能影响 macOS 代码签名，需测试兼容性。

---

### P9: 后台 Goroutine 优化

#### 9.1 合并定时器

当前有多个独立的定时器 goroutine：

| Goroutine | 间隔 | 文件位置 |
|-----------|------|---------|
| 黑名单自动恢复 | 1 分钟 | main.go:176-190 |
| 健康检查轮询 | 可配置 | healthcheckservice.go |
| 更新检查 | 每日 | main.go:159-165 |
| 可用性监控 | 可配置 | main.go:192-213 |

**建议**: 将多个低频定时器合并为一个统一的调度器，减少 goroutine 数量和上下文切换：

```go
// 统一调度器
go func() {
    minuteTicker := time.NewTicker(1 * time.Minute)
    defer minuteTicker.Stop()

    for {
        select {
        case <-minuteTicker.C:
            // 所有按分钟级别的任务在这里处理
            blacklistService.AutoRecoverExpired()
            // ... 其他周期性任务
        case <-stopChan:
            return
        }
    }
}()
```

#### 9.2 停止不必要的后台服务

在托盘模式（主窗口隐藏）时，考虑暂停或降低以下服务的频率：
- 健康检查轮询频率降低（如从 30s 到 5min）
- 停止可用性监控（直到主窗口打开）

---

### P10: 内存监控与诊断

#### 10.1 添加内存监控端点

在代理服务器上添加一个内存诊断接口，方便后续排查：

```go
// 在 providerrelay.go 的路由注册中添加
router.GET("/debug/memory", func(c *gin.Context) {
    var m runtime.MemStats
    runtime.ReadMemStats(&m)
    c.JSON(200, gin.H{
        "alloc_mb":       m.Alloc / 1024 / 1024,
        "total_alloc_mb": m.TotalAlloc / 1024 / 1024,
        "sys_mb":         m.Sys / 1024 / 1024,
        "heap_alloc_mb":  m.HeapAlloc / 1024 / 1024,
        "heap_sys_mb":    m.HeapSys / 1024 / 1024,
        "heap_idle_mb":   m.HeapIdle / 1024 / 1024,
        "heap_inuse_mb":  m.HeapInuse / 1024 / 1024,
        "goroutines":     runtime.NumGoroutine(),
    })
})
```

使用 `curl http://localhost:18100/debug/memory` 查看 Go 端内存使用情况。这能帮助区分是 Go 还是 WebKit 消耗了内存。

#### 10.2 使用 pprof 进行深度分析

```go
import _ "net/http/pprof"

// 在开发模式下启用 pprof
go func() {
    log.Println(http.ListenAndServe(":6060", nil))
}()
```

然后使用：
```bash
go tool pprof http://localhost:6060/debug/pprof/heap
```

---

## 优化优先级总结

| 优先级 | 优化项 | 预计节省 | 难度 | 风险 |
|--------|--------|---------|------|------|
| **P0** | WebView 窗口延迟创建 | **150-250MB** | 中 | 中（需测试窗口生命周期） |
| **P1** | Go GC 调优 | 10-30MB | 低 | 低 |
| **P2** | HTTP 服务器优化 | 5-15MB | 低 | 低 |
| **P3** | 数据库连接池优化 | 2-5MB | 低 | 低 |
| **P4** | 队列缓冲优化 | 2-3MB | 低 | 低 |
| **P5** | 请求日志缓冲优化 | 0-8MB | 低 | 低 |
| **P6** | 服务延迟初始化 | 5-10MB | 中 | 中 |
| **P7** | 前端资源优化 | 1-2MB | 中 | 低 |
| **P8** | 编译优化 | 二进制减小 | 低 | 低 |
| **P9** | Goroutine 合并 | 1-2MB | 低 | 低 |
| **P10** | 内存监控 | 诊断工具 | 低 | 低 |

## 实施建议

1. **先加 P10 监控**，确定 Go 堆内存和 WebKit 各自的占比
2. **实施 P0**，这是收益最大的优化
3. **实施 P1-P5**，这些都是低风险、低难度的快速优化
4. **按需实施 P6-P9**

## 验证方式

优化后使用以下方式验证内存变化：

```bash
# macOS 活动监视器查看进程内存
# 或使用命令行
ps aux | grep -i codeswitch | awk '{print $6/1024 "MB", $11}'

# Go 内存（如果加了 P10 监控接口）
curl -s http://localhost:18100/debug/memory | python3 -m json.tool
```

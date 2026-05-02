# 模型白名单与映射功能测试指南

## 📦 测试文件清单

```
services/
├── providerservice_test.go    # 核心算法单元测试（~350行）
├── providerrelay_test.go      # 请求处理与端到端测试（~250行）
└── testdata/
    └── example-claude-config.json  # 测试配置示例
```

## 🧪 运行测试

### 运行所有测试
```bash
cd G:\claude-lit\code-switch-R
go test ./services/... -v
```

### 运行特定测试文件
```bash
# 测试核心算法
go test ./services/providerservice_test.go ./services/providerservice.go -v

# 测试请求处理
go test ./services/providerrelay_test.go ./services/providerrelay.go -v
```

### 运行特定测试用例
```bash
# 测试通配符匹配
go test ./services/... -run TestMatchWildcard -v

# 测试模型支持检查
go test ./services/... -run TestProvider_IsModelSupported -v

# 测试端到端场景
go test ./services/... -run TestModelMappingEndToEnd -v
```

### 运行性能测试
```bash
go test ./services/... -bench=. -benchmem
```

## 📊 测试覆盖率

### 查看测试覆盖率
```bash
go test ./services/... -cover
```

### 生成详细覆盖率报告
```bash
go test ./services/... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## 🎯 测试覆盖范围

### providerservice_test.go

#### 1. **通配符匹配测试** (`TestMatchWildcard`)
- ✅ 精确匹配
- ✅ 前缀通配符 (`claude-*`)
- ✅ 后缀通配符 (`*-4`)
- ✅ 中间通配符 (`claude-*-4`)
- ✅ 边界情况（空前缀、空后缀）

#### 2. **通配符映射应用测试** (`TestApplyWildcardMapping`)
- ✅ 前缀通配符映射 (`claude-*` → `anthropic/claude-*`)
- ✅ 中间通配符映射 (`claude-*-4` → `anthropic/claude-*-v4`)
- ✅ 无通配符场景
- ✅ 边界情况

#### 3. **模型支持检查测试** (`TestProvider_IsModelSupported`)
- ✅ 向后兼容（未配置白名单）
- ✅ 原生支持 - 精确匹配
- ✅ 原生支持 - 通配符匹配
- ✅ 映射支持 - 精确匹配
- ✅ 映射支持 - 通配符匹配
- ✅ 混合模式（原生 + 映射）

#### 4. **获取有效模型测试** (`TestProvider_GetEffectiveModel`)
- ✅ 无映射场景
- ✅ 精确映射
- ✅ 通配符映射
- ✅ 精确优先于通配符

#### 5. **配置验证测试** (`TestProvider_ValidateConfiguration`)
- ✅ 有效配置
- ✅ 无效映射（目标不在白名单）
- ✅ 警告：只配置映射未配置白名单
- ✅ 警告：自映射
- ✅ 通配符映射（跳过验证）

### providerrelay_test.go

#### 1. **请求体模型替换测试** (`TestReplaceModelInRequestBody`)
- ✅ 简单替换
- ✅ 复杂嵌套 JSON
- ✅ 特殊字符处理
- ✅ 错误场景（缺少 model 字段、无效 JSON）

#### 2. **端到端场景测试** (`TestModelMappingEndToEnd`)
- ✅ 完整降级流程模拟
- ✅ 通配符映射实际应用
- ✅ 多供应商场景
- ✅ 不支持的模型处理

#### 3. **配置验证集成测试** (`TestProviderConfigValidation`)
- ✅ 完美配置
- ✅ 错误配置
- ✅ 通配符配置

#### 4. **性能基准测试**
- ✅ `BenchmarkIsModelSupported` - 模型支持检查性能
- ✅ `BenchmarkGetEffectiveModel` - 模型映射性能
- ✅ `BenchmarkReplaceModelInRequestBody` - JSON 替换性能

## 🔍 测试场景详解

### 场景 1：基础精确匹配
```go
Provider {
    SupportedModels: {"claude-sonnet-4": true},
}
请求: claude-sonnet-4 → ✅ 支持
请求: gpt-4 → ❌ 不支持
```

### 场景 2：通配符白名单
```go
Provider {
    SupportedModels: {"claude-*": true},
}
请求: claude-sonnet-4 → ✅ 支持（通配符匹配）
请求: claude-opus-4 → ✅ 支持（通配符匹配）
请求: gpt-4 → ❌ 不支持
```

### 场景 3：精确映射
```go
Provider {
    SupportedModels: {"anthropic/claude-sonnet-4": true},
    ModelMapping: {"claude-sonnet-4": "anthropic/claude-sonnet-4"},
}
请求: claude-sonnet-4
  → IsModelSupported: ✅ true
  → GetEffectiveModel: "anthropic/claude-sonnet-4"
  → 请求体: {"model": "anthropic/claude-sonnet-4", ...}
```

### 场景 4：通配符映射
```go
Provider {
    SupportedModels: {"anthropic/claude-*": true},
    ModelMapping: {"claude-*": "anthropic/claude-*"},
}
请求: claude-sonnet-4
  → IsModelSupported: ✅ true（通配符匹配）
  → GetEffectiveModel: "anthropic/claude-sonnet-4"（通配符展开）
  → 请求体: {"model": "anthropic/claude-sonnet-4", ...}
```

### 场景 5：实际降级流程
```
1. 用户请求: {"model": "claude-sonnet-4", ...}
2. Provider A (Anthropic Official):
   - IsModelSupported("claude-sonnet-4") = true
   - GetEffectiveModel("claude-sonnet-4") = "claude-sonnet-4"
   - 转发请求 → 成功 ✅

3. 如果 Provider A 失败，降级到 Provider B (OpenRouter):
   - IsModelSupported("claude-sonnet-4") = true（映射支持）
   - GetEffectiveModel("claude-sonnet-4") = "anthropic/claude-sonnet-4"
   - ReplaceModelInRequestBody → {"model": "anthropic/claude-sonnet-4", ...}
   - 转发修改后的请求 → 成功 ✅
```

## ✅ 验证清单

运行测试后，验证以下输出：

```bash
$ go test ./services/... -v

=== RUN   TestMatchWildcard
=== RUN   TestMatchWildcard/精确匹配-成功
=== RUN   TestMatchWildcard/前缀通配符-成功
=== RUN   TestMatchWildcard/中间通配符-成功
...
--- PASS: TestMatchWildcard (0.00s)

=== RUN   TestApplyWildcardMapping
...
--- PASS: TestApplyWildcardMapping (0.00s)

=== RUN   TestProvider_IsModelSupported
...
--- PASS: TestProvider_IsModelSupported (0.00s)

=== RUN   TestProvider_GetEffectiveModel
...
--- PASS: TestProvider_GetEffectiveModel (0.00s)

=== RUN   TestProvider_ValidateConfiguration
...
--- PASS: TestProvider_ValidateConfiguration (0.00s)

=== RUN   TestReplaceModelInRequestBody
...
--- PASS: TestReplaceModelInRequestBody (0.00s)

=== RUN   TestModelMappingEndToEnd
...
--- PASS: TestModelMappingEndToEnd (0.00s)

=== RUN   TestProviderConfigValidation
...
--- PASS: TestProviderConfigValidation (0.00s)

PASS
ok      codeswitch/services     0.XXXs
```

## 🐛 常见问题

### 问题 1：`go: command not found`
**解决**：确保已安装 Go 1.24+ 并配置环境变量。

### 问题 2：依赖缺失
**解决**：运行 `go mod tidy` 安装依赖。

### 问题 3：测试超时
**解决**：增加超时时间 `go test ./services/... -timeout 30s`

## 📈 性能基准

预期性能指标（参考值）：

```
BenchmarkIsModelSupported-8            10000000    100 ns/op    0 B/op    0 allocs/op
BenchmarkGetEffectiveModel-8            5000000    200 ns/op   32 B/op    1 allocs/op
BenchmarkReplaceModelInRequestBody-8     500000   3000 ns/op  512 B/op    5 allocs/op
```

**性能特点**：
- ✅ 模型支持检查：O(1) 时间复杂度（map 查找）
- ✅ 模型映射：O(1) 精确匹配 + O(n) 通配符回退（n 通常很小）
- ✅ JSON 替换：O(k) k 为 JSON 深度（通常 <10）

## 🎓 下一步

测试通过后，建议：
1. 📝 更新 CLAUDE.md 文档
2. 🎨 开发前端 UI 组件
3. 🔧 创建用户配置示例
4. 🚀 在实际环境中测试降级功能

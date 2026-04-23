# 额度查询配置弹窗调试记录

> 记录时间：2026-04-23  
> 相关入口：首页 → 供应商卡片 → 编辑供应商 → 配置查询  
> 相关组件：`frontend/src/components/Main/Index.vue`、`frontend/src/components/Main/modals/ProviderEditModal.vue`、`frontend/src/components/Main/modals/ProviderQuotaQueryConfigModal.vue`、`frontend/src/components/common/InlineModal.vue`、`frontend/src/components/common/JsonCodeEditor.vue`

## 背景

首页「编辑供应商」中的「配置查询」按钮点击后，外层界面会出现弹窗遮罩，但「额度查询配置」弹窗内容不显示。前面多个版本已经围绕 `InlineModal`、`JsonCodeEditor / CodeMirror` 首帧挂载、WebKit / Wails 合成层做过收口，但打包版仍需要继续定位根因。

## 第一次调试：空白弹窗隔离验证（`v2.8.33`）

### 做法

将 `Main/Index.vue` 中的额度查询配置弹窗入口从原始 `ProviderQuotaQueryConfigModal.vue` 临时切到 `ProviderQuotaQueryConfigBlankModal.vue`。

空白诊断组件只保留：

- `InlineModal`
- `open` prop
- `close` 事件
- 与原组件一致的 `modelValue / providerApiUrl / providerApiKey / save` 接口占位
- 不渲染原额度配置内容，不加载 `JsonCodeEditor / CodeMirror`

### 观察结果

空白弹窗可以正常打开。

### 结论

这一步基本排除了：

- `ProviderEditModal` 中「配置查询」按钮点击事件没有触发
- `open-provider-quota-query-config` 事件没有传到 `Main/Index.vue`
- `providerQuotaQueryConfigModalState.open` 状态没有变更
- `InlineModal` 基础壳体、遮罩、层级栈完全失效

剩余重点转向原 `ProviderQuotaQueryConfigModal.vue` 的内容初始化、首帧渲染和子组件加载。

## 第二次调试：原弹窗内容初始化排查（`v2.8.34`）

### 分层验证

1. 使用 Node `22.20.0` 跑前端类型检查和构建，避免本机默认 Node `18.20.8` 触发 Vite `crypto.hash is not a function` 这种环境噪音。
2. 对原 `ProviderQuotaQueryConfigModal.vue` 做 SSR 级别诊断：
   - stub 掉 `InlineModal`，避免 SSR 环境缺少 `document / window` 干扰；
   - stub 掉 `JsonCodeEditor`，避免 CodeMirror 顶层读取 DOM 干扰；
   - 只验证额度配置内容自身的 template / computed / i18n 渲染。
3. 观察 `vue-i18n` 在渲染额度配置文案时的 stderr 输出。

### 关键报错

排查过程中观察到 `vue-i18n` message 编译错误，例如：

```text
Message compilation error: Not allowed nest placeholder
通用余额模版，默认请求 {{baseUrl}}/user/balance，并要求同源 + HTTPS。

Message compilation error: Invalid token in placeholder: 'request,'
脚本需要返回 { request, extractor }，extractor 可以返回单对象或对象数组。
```

### 根因

`frontend/src/locales/zh.json` 和 `frontend/src/locales/en.json` 中部分额度查询配置文案包含裸花括号：

- `{{baseUrl}}/user/balance`
- `{{baseUrl}} / {{apiKey}} / {{accessToken}} / {{userId}}`
- `{ request, extractor }`

`vue-i18n` 会把 `{...}` 当作 message interpolation 语法解析，而这些内容只是想作为普通代码/变量提示展示给用户。结果导致 message compiler 报错。

在普通浏览器调试环境中，这类错误可能只表现为 console error；但在 Wails / WebKit 打包环境里，它会和弹窗首帧渲染、二层 modal、CodeMirror 加载混在一起，表现成「遮罩出来了，但额度查询配置内容不显示」。

### 修复

已将相关文案改成不含裸花括号的描述：

- `返回 { request, extractor }` → `返回 request / extractor`
- `{{baseUrl}}/user/balance` → `baseUrl + /user/balance`
- `脚本需要返回 { request, extractor }` → `脚本需要返回包含 request 和 extractor 的对象`
- 英文文案同步改成无裸花括号写法

同时：

- `Main/Index.vue` 已切回原 `ProviderQuotaQueryConfigModal.vue`
- 删除临时诊断组件 `ProviderQuotaQueryConfigBlankModal.vue`

## 已执行验证

使用 Node `22.20.0` 执行：

```bash
npm exec vue-tsc -- --noEmit
npm run build:dev
```

结果：

- TypeScript / Vue 类型检查通过
- Vite development build 通过
- 针对额度查询相关文案的 i18n 编译检查通过，不再出现 `Message compilation error`

## 后续排查建议

如果后续仍出现「遮罩出现但内容不显示」：

1. 先检查控制台是否有 `Message compilation error`、Vue render error、`JsonCodeEditor` 创建失败日志。
2. 不要优先改 `z-index` 或 modal stack；先用空白弹窗或 stub 子组件确认是壳体问题还是内容问题。
3. 对新增 i18n 文案要避免裸 `{}`：
   - 需要插值时才写 `{name}`；
   - 想展示代码、JSON、模板变量时，尽量改写成普通文字，或使用不触发 i18n 编译的转义方案。
4. 如果怀疑 CodeMirror，先让弹窗壳体不挂 `JsonCodeEditor` 单独打开；只有壳体稳定后，再排查编辑器首帧挂载。

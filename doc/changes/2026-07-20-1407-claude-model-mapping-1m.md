# Claude 模型映射支持 1M

## 变更时间

2026-07-20 14:07:36 CST（Asia/Shanghai）

## 涉及范围

- 首页 Claude 供应商新增/编辑弹窗
- 模型映射配置持久化
- Claude 原生 Anthropic 请求转发

## 变更内容

- 为每条模型映射增加“声明支持 1M”勾选和 `1M` 状态徽标。
- 新增 `modelMappingSupports1M` 可选配置，映射重命名和删除时同步维护状态。
- 命中已声明 1M 的映射时，向原生 Anthropic 上游合并 `context-1m-2025-08-07` beta。
- 保留并去重已有 `anthropic-beta`；OpenAI Chat/Responses 格式不注入。
- 修复切换到 OpenAI 格式后，隐藏的未保存 1M 草稿状态可能写入新映射的问题。
- 隐藏 1M 选项时，编辑已有映射会保留原声明，新建映射不会继承隐藏草稿。

## 验证结果

- `pnpm exec vue-tsc --noEmit`：通过。
- `pnpm test:unit`：51 个测试文件、370 个测试全部通过。
- `go test ./services/...`：通过。
- 前端回归测试覆盖隐藏草稿隔离、已有声明保留和供应商保存链路。

import type { BudgetQuotaAdjustments, BudgetQuotaSettings } from '../utils/budgetUsage'
import type { ProviderQuotaQueryConfig, ProviderQuotaQueryType } from '../utils/providerQuotaQuery'

export type ModelMappingMissPolicy = 'block' | 'passthrough'

export type AutomationCard = {
  id: number
  providerRef?: string
  name: string
  apiUrl: string
  apiKey: string
  officialSite: string
  apiKeyUrl?: string
  icon: string
  tint: string
  accent: string
  enabled: boolean
  // Claude API 格式（仅 Claude 供应商使用）
  apiFormat?: 'anthropic' | 'openai_chat' | 'openai_responses'
  // Anthropic cache_control TTL 覆盖（仅 Claude 原生 Anthropic Messages 使用）
  anthropicCacheTTL?: '' | '5m' | '1h'
  // 隐藏排序字段：仅控制启用组 / 未启用组内部顺序
  sortOrder?: number
  // 持久化启用组内顺序
  enabledSortOrder?: number
  // 持久化未启用组内顺序
  disabledSortOrder?: number
  // 模型白名单：声明 provider 支持的模型（精确或通配符）
  supportedModels?: Record<string, boolean>
  // 模型映射：external model -> internal model
  modelMapping?: Record<string, string>
  // 已关闭的模型映射：模型映射 key -> true
  modelMappingDisabled?: Record<string, boolean>
  // 模型映射思考强度：模型映射 key -> 强制思考强度
  modelMappingReasoningEfforts?: Record<string, string>
  // 模型映射 1M 上下文声明：模型映射 key -> true
  modelMappingSupports1M?: Record<string, boolean>
  // 模型映射未命中策略：默认拦截；也可按原模型名透传
  modelMappingMissPolicy?: ModelMappingMissPolicy
  // 模型路由开启时允许原样透传的请求模型规则
  modelPassthroughPatterns?: string[]
  // 请求体强制字段：仅在命中当前供应商转发时应用
  requestBodyOverrides?: Record<string, any>
  // 优先级分组：数字越小优先级越高（1-10，默认 1）
  level?: number
  // 实时并发：供应商最多同时处理的请求数
  providerConcurrencyLimit?: number
  // 会话隔离：供应商最多承载的会话数
  sessionMaxSessions?: number
  // 会话隔离：会话空闲释放时间（分钟）
  sessionTTLMinutes?: number
  // API 端点路径（可选）：覆盖平台默认端点
  apiEndpoint?: string
  // OpenCode AI SDK 包名，例如 @ai-sdk/openai-compatible
  opencodeNpm?: string
  // OpenCode provider fragment，最终写入 opencode.json 的 provider.{id}
  opencodeSettingsConfig?: Record<string, any>
  // OpenCode 供应商分类 / 合作伙伴元数据。
  category?: string
  // 托管认证来源，例如 codex_oauth。
  authProvider?: string
  // 托管认证绑定账号 ID。
  authAccountId?: string
  partnerPromotionKey?: string
  // OpenCode additive mode：是否由本应用管理 live config 中的 provider key
  liveConfigManaged?: boolean
  // OpenCode additive mode：provider 当前是否存在于 live config
  isInConfig?: boolean
  // CLI 配置：存储供应商关联的 CLI 可编辑配置
  cliConfig?: Record<string, any>

  // === 可用性监控配置（新） ===
  // 可用性监控开关：是否启用后台健康检查
  availabilityMonitorEnabled?: boolean
  // 连通性自动拉黑：检测失败时是否自动拉黑该供应商
  connectivityAutoBlacklist?: boolean
  // 可用性高级配置：测试模型、端点和超时
  availabilityConfig?: {
    testModel?: string      // 测试用模型
    testEndpoint?: string   // 测试端点路径
    timeout?: number        // 超时时间（毫秒）
  }

  // === 供应商级别预算额度 ===
  // 供应商独立预算配置（5 小时 / 日 / 周 / 月 / 总额度），nil 表示未配置
  budgetQuotaSettings?: BudgetQuotaSettings
  // 供应商独立预算额度当前已使用校准值（5 小时 / 日 / 周 / 月 / 总额度）
  budgetQuotaUsedAdjustments?: BudgetQuotaAdjustments
  // 供应商额度查询类型（Token Plan）
  providerQuotaQueryType?: ProviderQuotaQueryType
  // 供应商额度查询完整配置
  providerQuotaQueryConfig?: ProviderQuotaQueryConfig

  // === 旧连通性字段（已废弃，仅用于兼容旧数据） ===
  /** @deprecated 已迁移到 availabilityMonitorEnabled */
  connectivityCheck?: boolean
  /** @deprecated 已迁移到 availabilityConfig.testModel */
  connectivityTestModel?: string
  /** @deprecated 已迁移到 availabilityConfig.testEndpoint */
  connectivityTestEndpoint?: string
  /** @deprecated 已迁移到可用性配置中的认证方式 */
  connectivityAuthType?: string
}

export const automationCardGroups: Record<'claude' | 'codex', AutomationCard[]> = {
  claude: [
    {
      id: 100,
      name: '0011',
      apiUrl: 'https://0011.ai',
      apiKey: '',
      officialSite: 'https://0011.ai',
      icon: 'aicoding',
      tint: 'rgba(10, 132, 255, 0.14)',
      accent: '#0aff5cff',
      enabled: false,
    },
    {
      id: 101,
      name: 'AICoding.sh',
      apiUrl: 'https://api.aicoding.sh',
      apiKey: '',
      officialSite: 'https://aicoding.sh',
      icon: 'aicoding',
      tint: 'rgba(10, 132, 255, 0.14)',
      accent: '#0a84ff',
      enabled: false,
    },
    {
      id: 102,
      name: 'Kimi',
      apiUrl: 'https://api.moonshot.cn/anthropic',
      apiKey: '',
      officialSite: 'https://kimi.moonshot.cn',
      icon: 'kimi',
      tint: 'rgba(16, 185, 129, 0.16)',
      accent: '#10b981',
      enabled: false,
    },
    {
      id: 103,
      name: 'Deepseek',
      apiUrl: 'https://api.deepseek.com/anthropic',
      apiKey: '',
      officialSite: 'https://www.deepseek.com',
      icon: 'deepseek',
      tint: 'rgba(251, 146, 60, 0.18)',
      accent: '#f97316',
      enabled: false,
    },
  ],
  codex: [
    {
      id: 201,
      name: 'AICoding.sh',
      apiUrl: 'https://api.aicoding.sh',
      apiKey: '',
      officialSite: 'https://www.aicoding.sh',
      icon: 'aicoding',
      tint: 'rgba(236, 72, 153, 0.16)',
      accent: '#ec4899',
      enabled: false,
    },
  ],
}

export function createAutomationCards(data: AutomationCard[] = []): AutomationCard[] {
  return data.map((item) => ({
    ...item,
    officialSite: item.officialSite ?? '',
  }))
}

export type ProviderQuotaTokenPlanProvider = 'glm' | 'kimi' | 'minimax'

export type ProviderQuotaTemplateType =
  | 'balance'
  | 'custom'
  | 'general'
  | 'newapi'
  | 'sub2api'
  | 'token_plan'

export type ProviderQuotaQueryType =
  | 'none'
  | 'balance'
  | 'custom'
  | 'general'
  | 'newapi'
  | 'sub2api'
  | 'token_plan_glm'
  | 'token_plan_kimi'
  | 'token_plan_minimax'

export type ProviderQuotaBalanceProvider =
  | 'deepseek'
  | 'stepfun'
  | 'siliconflow'
  | 'openrouter'
  | 'novita'

export interface ProviderQuotaQueryConfig {
  enabled: boolean
  templateType?: ProviderQuotaTemplateType
  code?: string
  timeout?: number
  apiKey?: string
  baseUrl?: string
  accessToken?: string
  userId?: string
  tokenPlanProvider?: ProviderQuotaTokenPlanProvider
  autoQueryInterval?: number
  autoIntervalMinutes?: number
}

export type ProviderQuotaQuerySaveValidationIssue =
  | 'missing_script'
  | 'missing_newapi_credentials'
  | 'missing_provider_credentials'
  | 'unsupported_balance_provider'

export const providerQuotaQueryTypes: ProviderQuotaQueryType[] = [
  'none',
  'balance',
  'custom',
  'general',
  'newapi',
  'sub2api',
  'token_plan_glm',
  'token_plan_kimi',
  'token_plan_minimax',
]

export const providerQuotaTemplateTypes: ProviderQuotaTemplateType[] = [
  'balance',
  'custom',
  'general',
  'newapi',
  'sub2api',
  'token_plan',
]

export const providerQuotaQueryTypeLabelKeyMap: Record<ProviderQuotaQueryType, string> = {
  none: 'components.main.form.options.providerQuotaQueryNone',
  balance: 'components.main.form.options.providerQuotaQueryOfficial',
  custom: 'components.main.form.options.providerQuotaQueryCustom',
  general: 'components.main.form.options.providerQuotaQueryGeneral',
  newapi: 'components.main.form.options.providerQuotaQueryNewApi',
  sub2api: 'components.main.form.options.providerQuotaQuerySub2Api',
  token_plan_glm: 'components.main.form.options.providerQuotaQueryTokenPlanGLM',
  token_plan_kimi: 'components.main.form.options.providerQuotaQueryTokenPlanKimi',
  token_plan_minimax: 'components.main.form.options.providerQuotaQueryTokenPlanMiniMax',
}

export const providerQuotaTemplateLabelKeyMap: Record<ProviderQuotaTemplateType, string> = {
  balance: 'components.main.form.options.providerQuotaQueryOfficial',
  custom: 'components.main.form.options.providerQuotaQueryCustom',
  general: 'components.main.form.options.providerQuotaQueryGeneral',
  newapi: 'components.main.form.options.providerQuotaQueryNewApi',
  sub2api: 'components.main.form.options.providerQuotaQuerySub2Api',
  token_plan: 'components.main.form.options.providerQuotaQueryTokenPlan',
}

export const providerQuotaTokenPlanProviderLabelKeyMap: Record<ProviderQuotaTokenPlanProvider, string> = {
  glm: 'components.main.form.options.providerQuotaQueryTokenPlanGLM',
  kimi: 'components.main.form.options.providerQuotaQueryTokenPlanKimi',
  minimax: 'components.main.form.options.providerQuotaQueryTokenPlanMiniMax',
}

export const providerQuotaTokenPlanProviderOptions: Array<{
  value: ProviderQuotaTokenPlanProvider
  matcher: RegExp
}> = [
  {
    value: 'glm',
    matcher: /bigmodel\.cn|api\.z\.ai/i,
  },
  {
    value: 'kimi',
    matcher: /api\.kimi\.com\/coding/i,
  },
  {
    value: 'minimax',
    matcher: /api\.minimaxi?\.com|api\.minimax\.io/i,
  },
]

export function buildProviderQuotaPresetCode(templateType: ProviderQuotaTemplateType): string {
  switch (templateType) {
    case 'custom':
      return `({
  request: {
    url: '',
    method: 'GET',
    headers: {},
  },
  extractor: function(response) {
    return {
      label: 'Quota',
      remaining: 0,
      unit: 'USD',
      valueMode: 'currency',
    };
  },
})`
    case 'general':
      return `({
  request: {
    url: '{{baseUrl}}/user/balance',
    method: 'GET',
    headers: {
      'Authorization': 'Bearer {{apiKey}}',
      'User-Agent': 'code-switch-R/1.0',
    },
  },
  extractor: function(response) {
    return {
      label: 'Balance',
      remaining: response.balance,
      unit: 'USD',
      valueMode: 'currency',
    };
  },
})`
    case 'newapi':
      return `({
  request: {
    url: '{{baseUrl}}/api/user/self',
    method: 'GET',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': 'Bearer {{accessToken}}',
      'User-Agent': 'code-switch-R/1.0',
      'New-Api-User': '{{userId}}',
    },
  },
  extractor: function(response) {
    if (response.success && response.data) {
      return {
        label: response.data.group || 'Default Plan',
        remaining: response.data.quota / 500000,
        used: response.data.used_quota / 500000,
        total: (response.data.quota + response.data.used_quota) / 500000,
        unit: 'USD',
        valueMode: 'currency',
      };
    }
    return {
      label: 'NewAPI',
      isValid: false,
      invalidMessage: response.message || 'Query failed',
    };
  },
})`
    case 'sub2api':
      return `({
  request: {
    url: '{{baseUrl}}/v1/usage',
    method: 'GET',
    headers: {
      'Authorization': 'Bearer {{apiKey}}',
    },
  },
  extractor: function(response) {
    const subscription = response?.subscription ?? {};
    const isValid = response?.is_active ?? response?.isValid ?? true;
    const unit = response?.unit ?? response?.quota?.unit ?? 'USD';
    const invalidMessage = isValid ? '' : (response?.message || 'Invalid subscription');
    const items = [];
    const limitValues = [
      subscription.daily_limit_usd,
      subscription.weekly_limit_usd,
      subscription.monthly_limit_usd,
    ];
    const isUnlimitedSubscription = limitValues.every((limit) => (
      limit !== null
      && limit !== undefined
      && String(limit).trim() !== ''
      && Number.isFinite(Number(limit))
      && Number(limit) === 0
    ));

    if (Number(subscription.daily_limit_usd) > 0) {
      const nextReset = new Date();
      nextReset.setHours(24, 0, 0, 0);
      items.push({
        key: 'daily',
        used: Number(subscription.daily_usage_usd) || 0,
        total: Number(subscription.daily_limit_usd),
        nextReset: nextReset.toISOString(),
        isValid,
        invalidMessage,
        unit,
        valueMode: 'currency',
      });
    }

    if (Number(subscription.weekly_limit_usd) > 0) {
      const weeklyWindowStart = subscription.weekly_window_start;
      const windowStart = weeklyWindowStart ? new Date(weeklyWindowStart) : null;
      const nextReset = windowStart && !Number.isNaN(windowStart.getTime())
        ? new Date(windowStart.getTime() + 7 * 24 * 60 * 60 * 1000).toISOString()
        : undefined;
      items.push({
        key: 'weekly',
        used: Number(subscription.weekly_usage_usd) || 0,
        total: Number(subscription.weekly_limit_usd),
        nextReset,
        isValid,
        invalidMessage,
        unit,
        valueMode: 'currency',
      });
    }

    if (Number(subscription.monthly_limit_usd) > 0) {
      items.push({
        key: 'monthly',
        used: Number(subscription.monthly_usage_usd) || 0,
        total: Number(subscription.monthly_limit_usd),
        nextReset: subscription.expires_at,
        isValid,
        invalidMessage,
        unit,
        valueMode: 'currency',
      });
    }

    if (items.length > 0) {
      return items;
    }

    const remaining = response?.remaining ?? response?.quota?.remaining ?? response?.balance;
    return {
      key: 'balance',
      label: response?.planName || 'Sub2API',
      isValid,
      invalidMessage,
      remaining,
      unlimited: isUnlimitedSubscription,
      unit,
      valueMode: 'currency',
    };
  },
})`
    default:
      return ''
  }
}

export const providerQuotaBalanceProviderOptions: Array<{
  value: ProviderQuotaBalanceProvider
  label: string
  matcher: RegExp
}> = [
  {
    value: 'deepseek',
    label: 'DeepSeek',
    matcher: /api\.deepseek\.com/i,
  },
  {
    value: 'stepfun',
    label: 'StepFun',
    matcher: /api\.stepfun\.(ai|com)/i,
  },
  {
    value: 'siliconflow',
    label: 'SiliconFlow',
    matcher: /api\.siliconflow\.(cn|com)/i,
  },
  {
    value: 'openrouter',
    label: 'OpenRouter',
    matcher: /openrouter\.ai/i,
  },
  {
    value: 'novita',
    label: 'Novita AI',
    matcher: /api\.novita\.ai/i,
  },
]

const providerQuotaQueryTypeSet = new Set<ProviderQuotaQueryType>(providerQuotaQueryTypes)
const providerQuotaTemplateTypeSet = new Set<ProviderQuotaTemplateType>(providerQuotaTemplateTypes)
const providerQuotaTokenPlanProviderSet = new Set<ProviderQuotaTokenPlanProvider>([
  'glm',
  'kimi',
  'minimax',
])

export function normalizeProviderQuotaQueryType(value: unknown): ProviderQuotaQueryType {
  const normalized = String(value ?? '').trim().toLowerCase()
  return providerQuotaQueryTypeSet.has(normalized as ProviderQuotaQueryType)
    ? normalized as ProviderQuotaQueryType
    : 'none'
}

export function normalizeProviderQuotaTemplateType(value: unknown): ProviderQuotaTemplateType | undefined {
  const normalized = String(value ?? '').trim().toLowerCase()
  return providerQuotaTemplateTypeSet.has(normalized as ProviderQuotaTemplateType)
    ? normalized as ProviderQuotaTemplateType
    : undefined
}

export function normalizeProviderQuotaTokenPlanProvider(value: unknown): ProviderQuotaTokenPlanProvider | undefined {
  const normalized = String(value ?? '').trim().toLowerCase()
  return providerQuotaTokenPlanProviderSet.has(normalized as ProviderQuotaTokenPlanProvider)
    ? normalized as ProviderQuotaTokenPlanProvider
    : undefined
}

export function queryTypeToTokenPlanProvider(queryType: unknown): ProviderQuotaTokenPlanProvider | undefined {
  switch (normalizeProviderQuotaQueryType(queryType)) {
    case 'token_plan_glm':
      return 'glm'
    case 'token_plan_kimi':
      return 'kimi'
    case 'token_plan_minimax':
      return 'minimax'
    default:
      return undefined
  }
}

export function tokenPlanProviderToQueryType(
  provider: unknown,
): Extract<ProviderQuotaQueryType, 'token_plan_glm' | 'token_plan_kimi' | 'token_plan_minimax'> {
  switch (normalizeProviderQuotaTokenPlanProvider(provider)) {
    case 'glm':
      return 'token_plan_glm'
    case 'minimax':
      return 'token_plan_minimax'
    case 'kimi':
    default:
      return 'token_plan_kimi'
  }
}

export function queryTypeToTemplateType(queryType: unknown): ProviderQuotaTemplateType | undefined {
  const normalized = normalizeProviderQuotaQueryType(queryType)
  switch (normalized) {
    case 'balance':
      return 'balance'
    case 'custom':
      return 'custom'
    case 'general':
      return 'general'
    case 'newapi':
      return 'newapi'
    case 'sub2api':
      return 'sub2api'
    case 'token_plan_glm':
    case 'token_plan_kimi':
    case 'token_plan_minimax':
      return 'token_plan'
    default:
      return undefined
  }
}

function toObjectRecord(value: unknown): Record<string, any> | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null
  return value as Record<string, any>
}

function normalizeOptionalTrimmedString(value: unknown): string | undefined {
  const normalized = `${value ?? ''}`.trim()
  return normalized ? normalized : undefined
}

function trimProviderQuotaText(value: unknown): string {
  return `${value ?? ''}`.trim()
}

function resolveEffectiveProviderQuotaBaseURL(
  value: ProviderQuotaQueryConfig,
  options: {
    fallbackBaseUrl?: string | null | undefined
  } = {},
): string {
  return trimProviderQuotaText(value.baseUrl) || trimProviderQuotaText(options.fallbackBaseUrl)
}

function resolveEffectiveProviderQuotaAPIKey(
  value: ProviderQuotaQueryConfig,
  options: {
    fallbackApiKey?: string | null | undefined
  } = {},
): string {
  return trimProviderQuotaText(value.apiKey) || trimProviderQuotaText(options.fallbackApiKey)
}

function normalizeOptionalInteger(
  value: unknown,
  {
    fallback,
    min,
    max,
  }: {
    fallback: number
    min: number
    max: number
  },
): number {
  const numericValue = Number(value)
  if (!Number.isFinite(numericValue)) return fallback
  const normalized = Math.floor(numericValue)
  return Math.min(Math.max(normalized, min), max)
}

function hasMeaningfulProviderQuotaQueryConfig(config: ProviderQuotaQueryConfig): boolean {
  return !!(
    config.enabled
    || config.templateType
    || config.code?.trim()
    || config.apiKey?.trim()
    || config.baseUrl?.trim()
    || config.accessToken?.trim()
    || config.userId?.trim()
    || config.tokenPlanProvider
  )
}

function inferTemplateTypeFromLegacyShape(
  objectValue: Record<string, any>,
  fallbackQueryType: ProviderQuotaQueryType,
): ProviderQuotaTemplateType | undefined {
  if (objectValue.accessToken || objectValue.userId) {
    return 'newapi'
  }
  if (objectValue.tokenPlanProvider || objectValue.codingPlanProvider) {
    return 'token_plan'
  }
  if (objectValue.apiKey || objectValue.baseUrl) {
    return 'general'
  }
  if (objectValue.code) {
    return 'custom'
  }
  return queryTypeToTemplateType(fallbackQueryType)
}

export function resolveProviderQuotaQueryType(
  value: unknown,
  fallbackQueryType?: unknown,
): ProviderQuotaQueryType {
  const objectValue = toObjectRecord(value)
  if (!objectValue) {
    return normalizeProviderQuotaQueryType(value)
  }

  const fallbackType = normalizeProviderQuotaQueryType(fallbackQueryType)
  const enabled = objectValue.enabled === undefined ? true : !!objectValue.enabled
  if (!enabled) return 'none'

  const explicitQueryType = normalizeProviderQuotaQueryType(objectValue.queryType ?? objectValue.providerQuotaQueryType)
  if (explicitQueryType !== 'none') {
    return explicitQueryType
  }

  const templateType = normalizeProviderQuotaTemplateType(objectValue.templateType)
    ?? inferTemplateTypeFromLegacyShape(objectValue, fallbackType)

  switch (templateType) {
    case 'balance':
      return 'balance'
    case 'custom':
      return 'custom'
    case 'general':
      return 'general'
    case 'newapi':
      return 'newapi'
    case 'sub2api':
      return 'sub2api'
    case 'token_plan':
      return tokenPlanProviderToQueryType(
        objectValue.tokenPlanProvider
          ?? objectValue.codingPlanProvider
          ?? queryTypeToTokenPlanProvider(fallbackType),
      )
    default:
      return fallbackType
  }
}

export function normalizeProviderQuotaQueryConfig(
  value: unknown,
  fallbackQueryType?: unknown,
): ProviderQuotaQueryConfig | undefined {
  const objectValue = toObjectRecord(value)
  const fallbackType = normalizeProviderQuotaQueryType(fallbackQueryType)

  if (!objectValue) {
    const inferredTemplateType = queryTypeToTemplateType(value)
    if (!inferredTemplateType) {
      return undefined
    }
    return {
      enabled: normalizeProviderQuotaQueryType(value) !== 'none',
      templateType: inferredTemplateType,
      tokenPlanProvider: queryTypeToTokenPlanProvider(value),
      timeout: 10,
      autoQueryInterval: 5,
    }
  }

  const resolvedQueryType = resolveProviderQuotaQueryType(objectValue, fallbackType)
  const templateType = normalizeProviderQuotaTemplateType(objectValue.templateType)
    ?? inferTemplateTypeFromLegacyShape(objectValue, fallbackType)
    ?? queryTypeToTemplateType(resolvedQueryType)

  const normalized: ProviderQuotaQueryConfig = {
    enabled: objectValue.enabled === undefined
      ? resolvedQueryType !== 'none'
      : !!objectValue.enabled,
    templateType,
    code: typeof objectValue.code === 'string' ? objectValue.code : '',
    timeout: normalizeOptionalInteger(objectValue.timeout, {
      fallback: 10,
      min: 2,
      max: 30,
    }),
    apiKey: normalizeOptionalTrimmedString(objectValue.apiKey),
    baseUrl: normalizeOptionalTrimmedString(objectValue.baseUrl),
    accessToken: normalizeOptionalTrimmedString(objectValue.accessToken),
    userId: normalizeOptionalTrimmedString(objectValue.userId),
    tokenPlanProvider: normalizeProviderQuotaTokenPlanProvider(
      objectValue.tokenPlanProvider ?? objectValue.codingPlanProvider,
    ) ?? queryTypeToTokenPlanProvider(resolvedQueryType),
    autoQueryInterval: normalizeOptionalInteger(
      objectValue.autoQueryInterval ?? objectValue.autoIntervalMinutes,
      {
        fallback: 5,
        min: 0,
        max: 1440,
      },
    ),
  }

  if (!hasMeaningfulProviderQuotaQueryConfig(normalized)) {
    return undefined
  }

  return normalized
}

export function serializeProviderQuotaQueryType(
  value: unknown,
  fallbackQueryType?: unknown,
): ProviderQuotaQueryType | undefined {
  const normalized = resolveProviderQuotaQueryType(value, fallbackQueryType)
  return normalized === 'none' ? undefined : normalized
}

export function serializeProviderQuotaQueryConfig(
  value: unknown,
  fallbackQueryType?: unknown,
): ProviderQuotaQueryConfig | undefined {
  const normalized = normalizeProviderQuotaQueryConfig(value, fallbackQueryType)
  if (!normalized) return undefined

  const serialized: ProviderQuotaQueryConfig = {
    enabled: normalized.enabled,
    templateType: normalized.templateType,
    code: normalized.code?.trim() ? normalized.code : '',
    timeout: normalizeOptionalInteger(normalized.timeout, {
      fallback: 10,
      min: 2,
      max: 30,
    }),
    autoQueryInterval: normalizeOptionalInteger(
      normalized.autoQueryInterval ?? normalized.autoIntervalMinutes,
      {
        fallback: 5,
        min: 0,
        max: 1440,
      },
    ),
  }

  if (normalized.apiKey?.trim()) serialized.apiKey = normalized.apiKey.trim()
  if (normalized.baseUrl?.trim()) serialized.baseUrl = normalized.baseUrl.trim()
  if (normalized.accessToken?.trim()) serialized.accessToken = normalized.accessToken.trim()
  if (normalized.userId?.trim()) serialized.userId = normalized.userId.trim()
  if (normalized.templateType === 'token_plan') {
    serialized.tokenPlanProvider = normalized.tokenPlanProvider ?? 'kimi'
  }

  return hasMeaningfulProviderQuotaQueryConfig(serialized)
    ? serialized
    : undefined
}

export function sanitizeProviderQuotaQueryConfigForSave(
  value: unknown,
  fallbackQueryType?: unknown,
): ProviderQuotaQueryConfig | undefined {
  const sanitized = serializeProviderQuotaQueryConfig(value, fallbackQueryType)
  if (!sanitized) return undefined

  switch (sanitized.templateType) {
    case 'balance':
      sanitized.code = ''
      delete sanitized.apiKey
      delete sanitized.baseUrl
      delete sanitized.accessToken
      delete sanitized.userId
      break
    case 'general':
    case 'sub2api':
      delete sanitized.accessToken
      delete sanitized.userId
      break
    case 'newapi':
      delete sanitized.apiKey
      break
    case 'token_plan':
      sanitized.code = ''
      delete sanitized.apiKey
      delete sanitized.baseUrl
      delete sanitized.accessToken
      delete sanitized.userId
      sanitized.tokenPlanProvider = sanitized.tokenPlanProvider ?? 'kimi'
      break
    default:
      break
  }

  return sanitized
}

export function resetProviderQuotaQueryConfigFieldsOnTemplateSwitch(
  value: ProviderQuotaQueryConfig,
  templateType: ProviderQuotaTemplateType,
  options: {
    defaultTokenPlanProvider?: ProviderQuotaTokenPlanProvider
  } = {},
): ProviderQuotaQueryConfig {
  const nextConfig: ProviderQuotaQueryConfig = {
    ...value,
    templateType,
  }

  if (normalizeProviderQuotaTemplateType(value.templateType) !== templateType) {
    nextConfig.code = ''
  }

  switch (templateType) {
    case 'custom':
      nextConfig.apiKey = ''
      nextConfig.baseUrl = ''
      nextConfig.accessToken = ''
      nextConfig.userId = ''
      break
    case 'general':
    case 'sub2api':
      nextConfig.accessToken = ''
      nextConfig.userId = ''
      break
    case 'newapi':
      nextConfig.apiKey = ''
      break
    case 'balance':
      nextConfig.code = ''
      nextConfig.apiKey = ''
      nextConfig.baseUrl = ''
      nextConfig.accessToken = ''
      nextConfig.userId = ''
      break
    case 'token_plan':
      nextConfig.code = ''
      nextConfig.apiKey = ''
      nextConfig.baseUrl = ''
      nextConfig.accessToken = ''
      nextConfig.userId = ''
      nextConfig.tokenPlanProvider = nextConfig.tokenPlanProvider
        ?? options.defaultTokenPlanProvider
        ?? 'kimi'
      break
  }

  return nextConfig
}

export function hasProviderQuotaQueryMissingCredentials(
  value: unknown,
  options: {
    fallbackQueryType?: unknown
    fallbackBaseUrl?: string | null | undefined
    fallbackApiKey?: string | null | undefined
  } = {},
): boolean {
  const normalized = normalizeProviderQuotaQueryConfig(value, options.fallbackQueryType)
  if (!normalized?.enabled) return false

  const templateType = normalized.templateType
    ?? queryTypeToTemplateType(options.fallbackQueryType)
  const effectiveBaseURL = resolveEffectiveProviderQuotaBaseURL(normalized, options)
  const effectiveAPIKey = resolveEffectiveProviderQuotaAPIKey(normalized, options)

  switch (templateType) {
    case 'balance':
    case 'general':
    case 'sub2api':
    case 'token_plan':
      return !effectiveBaseURL || !effectiveAPIKey
    case 'newapi':
      return !effectiveBaseURL
        || !trimProviderQuotaText(normalized.accessToken)
        || !trimProviderQuotaText(normalized.userId)
    default:
      return false
  }
}

export function validateProviderQuotaQueryConfigForSave(
  value: unknown,
  options: {
    fallbackQueryType?: unknown
    fallbackBaseUrl?: string | null | undefined
    fallbackApiKey?: string | null | undefined
  } = {},
): ProviderQuotaQuerySaveValidationIssue | null {
  const normalized = normalizeProviderQuotaQueryConfig(value, options.fallbackQueryType)
  if (!normalized?.enabled) return null

  const templateType = normalized.templateType
    ?? queryTypeToTemplateType(options.fallbackQueryType)
  const code = trimProviderQuotaText(normalized.code)

  if ((templateType === 'custom' || templateType === 'general' || templateType === 'newapi' || templateType === 'sub2api') && !code) {
    return 'missing_script'
  }

  if (templateType === 'newapi' && hasProviderQuotaQueryMissingCredentials(normalized, options)) {
    return 'missing_newapi_credentials'
  }

  if ((templateType === 'balance' || templateType === 'general' || templateType === 'sub2api' || templateType === 'token_plan')
    && hasProviderQuotaQueryMissingCredentials(normalized, options)) {
    return 'missing_provider_credentials'
  }

  if (templateType === 'balance'
    && !detectProviderQuotaBalanceProvider(resolveEffectiveProviderQuotaBaseURL(normalized, options))) {
    return 'unsupported_balance_provider'
  }

  return null
}

export function hasProviderQuotaQueryType(value: unknown, fallbackQueryType?: unknown): boolean {
  return resolveProviderQuotaQueryType(value, fallbackQueryType) !== 'none'
}

export function detectProviderQuotaTokenPlanProvider(baseUrl: string | null | undefined): ProviderQuotaTokenPlanProvider | undefined {
  const normalizedURL = `${baseUrl ?? ''}`.trim()
  if (!normalizedURL) return undefined
  return providerQuotaTokenPlanProviderOptions.find((option) => option.matcher.test(normalizedURL))?.value
}

export function detectProviderQuotaBalanceProvider(baseUrl: string | null | undefined): ProviderQuotaBalanceProvider | undefined {
  const normalizedURL = `${baseUrl ?? ''}`.trim()
  if (!normalizedURL) return undefined
  return providerQuotaBalanceProviderOptions.find((option) => option.matcher.test(normalizedURL))?.value
}

export function resolveProviderQuotaAutoQueryIntervalMinutes(
  config: unknown,
  fallbackQueryType?: unknown,
): number {
  const normalized = normalizeProviderQuotaQueryConfig(config, fallbackQueryType)
  return normalizeOptionalInteger(
    normalized?.autoQueryInterval ?? normalized?.autoIntervalMinutes,
    {
      fallback: 5,
      min: 0,
      max: 1440,
    },
  )
}

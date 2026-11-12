/**
 * @name: 供应商额度配置测试
 * @Descripttion: 验证额度配置兼容、保存校验与被动采样状态文案。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-11-06 16:35:58
 * @LastEditTime: 2026-11-06 16:35:58
 * @FilePath: frontend/src/utils/providerQuotaQuery.test.ts
 */
import { describe, expect, it } from 'vitest'
import {
  buildProviderQuotaPresetCode,
  detectProviderQuotaBalanceProvider,
  detectProviderQuotaTokenPlanProvider,
  formatPassiveQuotaEstimateNote,
  hasProviderQuotaQueryMissingCredentials,
  isPassiveProviderQuotaQueryType,
  normalizeProviderQuotaQueryConfig,
  localizeProviderQuotaMessage,
  providerQuotaTokenPlanProviderLabelKeyMap,
  providerQuotaTemplateTypes,
  resetProviderQuotaQueryConfigFieldsOnTemplateSwitch,
  resolveProviderQuotaQueryType,
  sanitizeProviderQuotaQueryConfigForSave,
  serializeProviderQuotaQueryConfig,
  validateProviderQuotaQueryConfigForSave,
} from './providerQuotaQuery'

describe('providerQuotaQuery utils', () => {
  it.each(['claude', 'codex'] as const)('round-trips %s passive sampling through Token Plan configuration', (provider) => {
    const tokenPlanProvider = `${provider}_passive` as const
    const queryType = `token_plan_${provider}_passive` as const
    const labelKey = provider === 'claude'
      ? 'components.main.form.options.providerQuotaQueryTokenPlanClaudePassive'
      : 'components.main.form.options.providerQuotaQueryTokenPlanCodexPassive'
    const config = sanitizeProviderQuotaQueryConfigForSave({
      enabled: true,
      templateType: 'token_plan',
      tokenPlanProvider,
      code: 'hidden-script',
      apiKey: 'hidden-key',
      baseUrl: 'https://hidden.example.com',
    })
    expect(config).toEqual({
      enabled: true,
      templateType: 'token_plan',
      tokenPlanProvider,
      code: '',
      timeout: 10,
      autoQueryInterval: 5,
    })
    expect(resolveProviderQuotaQueryType(config)).toBe(queryType)
    expect(normalizeProviderQuotaQueryConfig(queryType)?.tokenPlanProvider).toBe(tokenPlanProvider)
    expect(normalizeProviderQuotaQueryConfig(serializeProviderQuotaQueryConfig(config))?.tokenPlanProvider).toBe(tokenPlanProvider)
    expect(providerQuotaTokenPlanProviderLabelKeyMap[tokenPlanProvider]).toBe(labelKey)
    expect(isPassiveProviderQuotaQueryType(queryType)).toBe(true)
    expect(validateProviderQuotaQueryConfigForSave(config, {
      fallbackBaseUrl: 'https://example.com',
      fallbackApiKey: 'sample-key',
    })).toBeNull()
    expect(validateProviderQuotaQueryConfigForSave(config)).toBe('missing_provider_credentials')
  })

  it('localizes pending and stale samples while preserving other quota errors', () => {
    const t = (key: string) => `translated:${key}`
    expect(localizeProviderQuotaMessage('claude_passive_pending', t)).toBe('translated:components.main.providers.quotaClaudePassivePending')
    expect(localizeProviderQuotaMessage('claude_passive_stale', t)).toBe('translated:components.main.providers.quotaClaudePassiveStale')
    expect(localizeProviderQuotaMessage('codex_passive_pending', t)).toBe('translated:components.main.providers.quotaCodexPassivePending')
    expect(localizeProviderQuotaMessage('codex_passive_stale', t)).toBe('translated:components.main.providers.quotaCodexPassiveStale')
    expect(localizeProviderQuotaMessage(' Other error ', t)).toBe('Other error')
    expect(localizeProviderQuotaMessage(undefined, t)).toBe('')
  })

  it('describes aged and projected passive samples', () => {
    const t = (key: string, params?: Record<string, unknown>) => `${key}${params ? JSON.stringify(params) : ''}`
    const now = new Date('2026-11-12T12:00:00Z')
    const minutesAgo = (minutes: number) => now.getTime() - minutes * 60_000
    expect(formatPassiveQuotaEstimateNote({}, t, now)).toBe('')
    expect(formatPassiveQuotaEstimateNote({ estimate: 'reset', sampledAt: minutesAgo(400) }, t, now))
      .toBe('components.main.providers.quotaPassiveWindowReset')
    expect(formatPassiveQuotaEstimateNote({ estimate: 'projected', sampledAt: minutesAgo(400) }, t, now))
      .toBe('components.main.providers.quotaPassiveProjected')
    expect(formatPassiveQuotaEstimateNote({ estimate: 'aged', sampledAt: minutesAgo(6) }, t, now))
      .toBe('components.main.providers.quotaPassiveAged{"age":"6m"}')
    expect(formatPassiveQuotaEstimateNote({ estimate: 'aged', sampledAt: minutesAgo(150) }, t, now))
      .toBe('components.main.providers.quotaPassiveAged{"age":"2h"}')
    expect(formatPassiveQuotaEstimateNote({ estimate: 'aged', sampledAt: minutesAgo(3 * 24 * 60) }, t, now))
      .toBe('components.main.providers.quotaPassiveAged{"age":"3d"}')
    expect(formatPassiveQuotaEstimateNote({ estimate: 'aged' }, t, now))
      .toBe('components.main.providers.quotaPassiveAgedUnknown')
  })

  it('keeps active quota queries on the remote refresh path', () => {
    expect(isPassiveProviderQuotaQueryType('token_plan_glm')).toBe(false)
    expect(isPassiveProviderQuotaQueryType('sub2api')).toBe(false)
    expect(isPassiveProviderQuotaQueryType('none')).toBe(false)
  })

  it('normalizes legacy token plan type to config object', () => {
    expect(normalizeProviderQuotaQueryConfig('token_plan_glm')).toEqual({
      enabled: true,
      templateType: 'token_plan',
      tokenPlanProvider: 'glm',
      timeout: 10,
      autoQueryInterval: 5,
    })
  })

  it('resolves query type from structured config', () => {
    expect(resolveProviderQuotaQueryType({
      enabled: true,
      templateType: 'newapi',
      accessToken: 'token-1',
      userId: '1001',
    })).toBe('newapi')

    expect(resolveProviderQuotaQueryType({
      enabled: true,
      templateType: 'sub2api',
      apiKey: 'sub2api-key',
    })).toBe('sub2api')
  })

  it('places sub2api after newapi and preserves its optional dedicated credentials', () => {
    expect(providerQuotaTemplateTypes).toEqual([
      'balance',
      'custom',
      'general',
      'newapi',
      'sub2api',
      'token_plan',
    ])

    expect(sanitizeProviderQuotaQueryConfigForSave({
      enabled: true,
      templateType: 'sub2api',
      code: 'sub2api-code',
      apiKey: ' sub2api-key ',
      baseUrl: ' https://sub2api.example.com ',
    })).toEqual({
      enabled: true,
      templateType: 'sub2api',
      code: 'sub2api-code',
      timeout: 10,
      autoQueryInterval: 5,
      apiKey: 'sub2api-key',
      baseUrl: 'https://sub2api.example.com',
    })
  })

  it('serializes config with trimming and token plan defaults', () => {
    expect(serializeProviderQuotaQueryConfig({
      enabled: true,
      templateType: 'token_plan',
      tokenPlanProvider: 'minimax',
      apiKey: '  quota-key  ',
      timeout: 99,
      autoQueryInterval: -1,
    })).toEqual({
      enabled: true,
      templateType: 'token_plan',
      code: '',
      timeout: 30,
      autoQueryInterval: 0,
      apiKey: 'quota-key',
      tokenPlanProvider: 'minimax',
    })
  })

  it('sanitizes hidden provider quota fields before saving', () => {
    expect(sanitizeProviderQuotaQueryConfigForSave({
      enabled: true,
      templateType: 'token_plan',
      code: 'return 1',
      apiKey: ' quota-key ',
      baseUrl: ' https://quota.example.com ',
      accessToken: ' access-token ',
      userId: ' user-1 ',
      tokenPlanProvider: 'glm',
    })).toEqual({
      enabled: true,
      templateType: 'token_plan',
      code: '',
      timeout: 10,
      autoQueryInterval: 5,
      tokenPlanProvider: 'glm',
    })
  })

  it('resets stale credentials when switching templates', () => {
    expect(resetProviderQuotaQueryConfigFieldsOnTemplateSwitch({
      enabled: true,
      templateType: 'general',
      code: 'general-code',
      apiKey: 'general-key',
      baseUrl: 'https://general.example.com',
      accessToken: 'old-token',
      userId: 'old-user',
    }, 'newapi')).toEqual({
      enabled: true,
      templateType: 'newapi',
      code: '',
      apiKey: '',
      baseUrl: 'https://general.example.com',
      accessToken: 'old-token',
      userId: 'old-user',
    })

    expect(resetProviderQuotaQueryConfigFieldsOnTemplateSwitch({
      enabled: true,
      templateType: 'newapi',
      code: 'newapi-code',
      apiKey: 'old-key',
      baseUrl: 'https://newapi.example.com',
      accessToken: 'token-1',
      userId: 'user-1',
    }, 'balance')).toEqual({
      enabled: true,
      templateType: 'balance',
      code: '',
      apiKey: '',
      baseUrl: '',
      accessToken: '',
      userId: '',
    })

    const sub2apiConfig = resetProviderQuotaQueryConfigFieldsOnTemplateSwitch({
      enabled: true,
      templateType: 'general',
      code: 'general-code',
    }, 'sub2api')
    expect(sub2apiConfig.code).toBe('')
    expect(buildProviderQuotaPresetCode(sub2apiConfig.templateType!)).toContain("url: '{{baseUrl}}/v1/usage'")
  })

  it('builds sub2api quota items without creating epoch resets for empty weekly windows', () => {
    const preset = Function(`"use strict"; return ${buildProviderQuotaPresetCode('sub2api')}`)() as {
      request: { url: string }
      extractor: (response: Record<string, any>) => Record<string, any> | Array<Record<string, any>>
    }

    expect(preset.request.url).toBe('{{baseUrl}}/v1/usage')
    expect(preset.extractor({
      isValid: true,
      unit: 'USD',
      subscription: {
        daily_limit_usd: 10,
        daily_usage_usd: 1,
        weekly_limit_usd: 20,
        weekly_usage_usd: 2,
        weekly_window_start: null,
        monthly_limit_usd: 30,
        monthly_usage_usd: 3,
        expires_at: '2026-09-16T15:53:00.193234+08:00',
      },
    })).toEqual([
      expect.objectContaining({ key: 'daily', used: 1, total: 10 }),
      expect.objectContaining({ key: 'weekly', used: 2, total: 20, nextReset: undefined }),
      expect.objectContaining({ key: 'monthly', used: 3, total: 30 }),
    ])

    expect(preset.extractor({
      isValid: true,
      planName: 'Unlimited',
      remaining: 0,
      unit: 'USD',
      subscription: {
        daily_limit_usd: 0,
        weekly_limit_usd: 0,
        monthly_limit_usd: 0,
      },
    })).toEqual(expect.objectContaining({
      key: 'balance',
      unlimited: true,
      remaining: 0,
    }))

    expect(preset.extractor({
      isValid: true,
      remaining: -1,
      unit: 'USD',
      subscription: {},
    })).toEqual(expect.objectContaining({
      key: 'balance',
      unlimited: false,
      remaining: -1,
    }))
  })

  it('requires userId for newapi credentials', () => {
    expect(hasProviderQuotaQueryMissingCredentials({
      enabled: true,
      templateType: 'newapi',
      baseUrl: 'https://newapi.example.com',
      accessToken: 'access-token',
    })).toBe(true)

    expect(hasProviderQuotaQueryMissingCredentials({
      enabled: true,
      templateType: 'newapi',
      baseUrl: 'https://newapi.example.com',
      accessToken: 'access-token',
      userId: 'user-42',
    })).toBe(false)
  })

  it('validates save-time fatal config issues before persisting', () => {
    expect(validateProviderQuotaQueryConfigForSave({
      enabled: true,
      templateType: 'general',
      code: '',
    })).toBe('missing_script')

    expect(validateProviderQuotaQueryConfigForSave({
      enabled: true,
      templateType: 'newapi',
      code: '({ request: {}, extractor: function(response) { return response; } })',
      baseUrl: 'https://newapi.example.com',
      accessToken: 'access-token',
    })).toBe('missing_newapi_credentials')

    expect(validateProviderQuotaQueryConfigForSave({
      enabled: true,
      templateType: 'token_plan',
    }, {
      fallbackBaseUrl: '',
      fallbackApiKey: '',
    })).toBe('missing_provider_credentials')

    expect(validateProviderQuotaQueryConfigForSave({
      enabled: true,
      templateType: 'general',
      code: '({ request: {}, extractor: function(response) { return response; } })',
    }, {
      fallbackBaseUrl: 'https://quota.example.com',
      fallbackApiKey: 'quota-key',
    })).toBeNull()

    expect(validateProviderQuotaQueryConfigForSave({
      enabled: true,
      templateType: 'sub2api',
      code: '({ request: {}, extractor: function(response) { return response; } })',
    }, {
      fallbackBaseUrl: 'https://sub2api.example.com',
      fallbackApiKey: 'sub2api-key',
    })).toBeNull()

    expect(validateProviderQuotaQueryConfigForSave({
      enabled: true,
      templateType: 'balance',
    }, {
      fallbackBaseUrl: 'https://api.example.com/v1/chat/completions',
      fallbackApiKey: 'quota-key',
    })).toBe('unsupported_balance_provider')
  })

  it('detects built-in balance and token plan providers from base url', () => {
    expect(detectProviderQuotaBalanceProvider('https://openrouter.ai/api/v1/chat/completions')).toBe('openrouter')
    expect(detectProviderQuotaTokenPlanProvider('https://api.z.ai/api/paas/v4/chat/completions')).toBe('glm')
    expect(detectProviderQuotaTokenPlanProvider('https://api.kimi.com/coding/api/kat/chat/completions')).toBe('kimi')
    expect(detectProviderQuotaTokenPlanProvider('https://api.moonshot.cn/v1/chat/completions')).toBeUndefined()
  })
})

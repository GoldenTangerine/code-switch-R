import { describe, expect, it } from 'vitest'
import {
  detectProviderQuotaBalanceProvider,
  detectProviderQuotaTokenPlanProvider,
  hasProviderQuotaQueryMissingCredentials,
  normalizeProviderQuotaQueryConfig,
  resetProviderQuotaQueryConfigFieldsOnTemplateSwitch,
  resolveProviderQuotaQueryType,
  sanitizeProviderQuotaQueryConfigForSave,
  serializeProviderQuotaQueryConfig,
  validateProviderQuotaQueryConfigForSave,
} from './providerQuotaQuery'

describe('providerQuotaQuery utils', () => {
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
      code: 'general-code',
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

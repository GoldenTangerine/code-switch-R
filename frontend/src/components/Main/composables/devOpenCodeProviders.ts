import type { AutomationCard } from '../../../data/cards'
import type { OpenCodeProvider } from '../adapters/providerCardMappers'

const cloneRecord = <T>(value: T): T => JSON.parse(JSON.stringify(value))

const createCards = (): AutomationCard[] => ([
  {
    id: 400,
    providerRef: 'dev-opencode-kimi',
    name: 'Kimi',
    apiUrl: 'https://api.moonshot.cn/v1',
    apiKey: 'sk-dev-mock-kimi',
    officialSite: 'https://kimi.moonshot.cn',
    icon: 'kimi',
    tint: 'rgba(16, 185, 129, 0.16)',
    accent: '#10b981',
    enabled: false,
    sortOrder: 1,
    disabledSortOrder: 1,
    level: 1,
    requestBodyOverrides: {},
    opencodeNpm: '@ai-sdk/openai-compatible',
    opencodeSettingsConfig: {
      npm: '@ai-sdk/openai-compatible',
      name: 'Kimi',
      options: {
        baseURL: 'https://api.moonshot.cn/v1',
        apiKey: 'sk-dev-mock-kimi',
      },
      models: {
        'kimi-k2-0711-preview': { name: 'Kimi K2 Preview' },
        'moonshot-v1-32k': { name: 'Moonshot v1 32K' },
      },
    },
  },
  {
    id: 401,
    providerRef: 'dev-opencode-deepseek',
    name: 'Deepseek',
    apiUrl: 'https://api.deepseek.com/v1',
    apiKey: 'sk-dev-mock-deepseek',
    officialSite: 'https://www.deepseek.com',
    icon: 'deepseek',
    tint: 'rgba(251, 146, 60, 0.18)',
    accent: '#f97316',
    enabled: false,
    sortOrder: 2,
    disabledSortOrder: 2,
    level: 1,
    requestBodyOverrides: {},
    opencodeNpm: '@ai-sdk/openai-compatible',
    opencodeSettingsConfig: {
      npm: '@ai-sdk/openai-compatible',
      name: 'Deepseek',
      options: {
        baseURL: 'https://api.deepseek.com/v1',
        apiKey: 'sk-dev-mock-deepseek',
      },
      models: {
        'deepseek-chat': { name: 'DeepSeek Chat' },
        'deepseek-reasoner': { name: 'DeepSeek Reasoner' },
      },
    },
  },
] satisfies AutomationCard[])

export const createDevOpenCodeCards = (): AutomationCard[] => createCards().map((card) => ({
  ...card,
  opencodeSettingsConfig: cloneRecord(card.opencodeSettingsConfig || {}),
  requestBodyOverrides: cloneRecord(card.requestBodyOverrides || {}),
}))

export const createDevOpenCodeProviders = (): OpenCodeProvider[] => createDevOpenCodeCards().map((card) => ({
  id: card.providerRef || `dev-opencode-${card.id}`,
  name: card.name,
  websiteUrl: card.officialSite,
  baseUrl: card.apiUrl,
  apiKey: card.apiKey,
  npm: card.opencodeNpm || '@ai-sdk/openai-compatible',
  description: '仅用于本地 pnpm run dev 的 OpenCode 页面预览',
  category: 'dev-mock',
  enabled: card.enabled,
  sortOrder: card.sortOrder,
  enabledSortOrder: card.enabledSortOrder,
  disabledSortOrder: card.disabledSortOrder,
  level: card.level,
  settingsConfig: cloneRecord(card.opencodeSettingsConfig || {}),
  requestBodyOverrides: cloneRecord(card.requestBodyOverrides || {}),
}))

/**
 * @name: 供应商卡片展示测试
 * @Descripttion: 验证空数据成功率、黑名单计数提示和拉黑状态节点
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-07-23 16:35:00
 * @LastEditTime: 2026-07-23 16:35:00
 * @FilePath: frontend/src/components/Main/components/ProviderCard.test.ts
 */

import { createSSRApp, h } from 'vue'
import { createI18n } from 'vue-i18n'
import { renderToString } from 'vue/server-renderer'
import { describe, expect, it } from 'vitest'
import type { AutomationCard } from '../../../data/cards'
import zh from '../../../locales/zh.json'
import type { ProviderCardViewModel } from '../types'
import ProviderCard from './ProviderCard.vue'

const card: AutomationCard = {
  id: 102,
  name: 'kimi',
  apiUrl: 'https://api.example.com',
  apiKey: 'test-key',
  officialSite: 'https://example.com',
  icon: 'kimi',
  tint: 'rgba(16, 185, 129, 0.16)',
  accent: '#10b981',
  enabled: true,
}

const baseViewModel = (overrides: Partial<ProviderCardViewModel> = {}): ProviderCardViewModel => ({
  card,
  dragging: false,
  dragOver: false,
  isLastUsed: false,
  isDefaultHostedProvider: false,
  isHighlighted: false,
  isDirectApplied: false,
  blacklistStatus: null,
  blacklistCounters: {
    failureCount: 0,
    failureThreshold: null,
    healthFailureCount: 0,
    healthFailureThreshold: null,
  },
  connectivityClass: '',
  connectivityTooltip: '',
  stats: {
    state: 'empty',
    message: '暂无数据',
    unreadFailedRequests: 0,
    hasUnreadErrorLogs: false,
  },
  concurrencyStatus: undefined,
  concurrencyLimitEnabled: false,
  quotaDisplay: [],
  quotaRefreshing: false,
  formattedOfficialSite: 'example.com',
  iconSvg: '',
  vendorInitials: 'K',
  ...overrides,
})

const renderCard = async (viewModel: ProviderCardViewModel) => {
  const i18n = createI18n({
    legacy: false,
    locale: 'zh',
    missingWarn: false,
    fallbackWarn: false,
    messages: {
      zh,
    },
  })
  const app = createSSRApp({
    render: () => h(ProviderCard, {
      viewModel,
      activeTab: 'claude',
      activeProxyState: false,
      resolvedTheme: 'light',
      formatBlacklistCountdown: (seconds: number) => `${Math.floor(seconds / 60)}分${seconds % 60}秒`,
    }),
  })
  app.use(i18n)
  return renderToString(app)
}

describe('ProviderCard display states', () => {
  it('keeps a success-rate hover target when daily stats are empty', async () => {
    const html = await renderCard(baseViewModel())

    expect(html).toContain('成功率: —')
    expect(html).toContain('请求 0/—')
    expect(html).toContain('巡检 0/—')
    expect(html).not.toContain('blacklist-banner')
  })

  it('renders the compact active-blacklist trigger after the metrics', async () => {
    const html = await renderCard(baseViewModel({
      blacklistStatus: {
        platform: 'claude',
        providerName: 'kimi',
        failureCount: 2,
        failureThreshold: 5,
        healthFailureCount: 1,
        healthFailureThreshold: 4,
        isBlacklisted: true,
        remainingSeconds: 65,
        blacklistLevel: 1,
        blacklistTriggerSource: 'request',
        forgivenessRemaining: 0,
      },
      blacklistCounters: {
        failureCount: 2,
        failureThreshold: 5,
        healthFailureCount: 1,
        healthFailureThreshold: 4,
      },
      stats: {
        state: 'ready',
        requests: '请求数: 1',
        tokens: 'Tokens: 1',
        costLabel: '花费',
        costParts: [{ type: 'amount', value: '0' }],
        costFormatted: '0',
        costValue: 0,
        ttft: '—',
        tps: '—',
        performanceHint: '',
        successRateLabel: '成功率: 0%',
        successRateClass: 'success-bad',
        successRateHint: '成功 0 · 失败 1 · 已排除 0',
        failedRequests: 1,
        unreadFailedRequests: 0,
        hasUnreadErrorLogs: false,
      },
    }))

    expect(html).toContain('card-blacklist-trigger')
    expect(html).toContain('已拉黑 1分5秒')
    expect(html).not.toContain('blacklist-banner')
  })
})

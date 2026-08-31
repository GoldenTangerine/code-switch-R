/**
 * @name: 供应商卡片展示测试
 * @Descripttion: 验证空数据成功率、黑名单计数提示和拉黑状态节点
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-07-23 16:35:00
 * @LastEditTime: 2026-07-23 16:35:00
 * @FilePath: frontend/src/components/Main/components/ProviderCard.test.ts
 */

import { readFileSync } from 'node:fs'
import { createSSRApp, h } from 'vue'
import { createI18n } from 'vue-i18n'
import { renderToString } from 'vue/server-renderer'
import { describe, expect, it } from 'vitest'
import type { AutomationCard } from '../../../data/cards'
import zh from '../../../locales/zh.json'
import type { ProviderCardViewModel } from '../types'
import ProviderCard from './ProviderCard.vue'

const providerCardStyleSource = readFileSync(new URL('../styles/provider-card.css', import.meta.url), 'utf8')

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

const renderCard = async (
  viewModel: ProviderCardViewModel,
  props: Partial<{
    activeProxyState: boolean
    forcedPrioritySaving: boolean
  }> = {},
) => {
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
      activeProxyState: props.activeProxyState ?? false,
      forcedPrioritySaving: props.forcedPrioritySaving ?? false,
      resolvedTheme: 'light',
      formatBlacklistCountdown: (seconds: number) => `${Math.floor(seconds / 60)}分${seconds % 60}秒`,
    }),
  })
  app.use(i18n)
  return renderToString(app)
}

describe('ProviderCard display states', () => {
  it('preserves quota auto-disabled accents outside neutral disabled card styles', () => {
    expect(providerCardStyleSource).toContain(
      '.automation-card.is-disabled:not(.is-quota-auto-disabled):not(.drag-over):not(.is-highlighted)',
    )
    expect(providerCardStyleSource).toContain(
      '.automation-card.is-disabled:not(.is-quota-auto-disabled):hover:not(.drag-over):not(.is-highlighted)',
    )
    expect(providerCardStyleSource).toContain(
      '.automation-card.theme-dark.is-disabled:not(.is-quota-auto-disabled):hover:not(.drag-over):not(.is-highlighted)',
    )
  })

  it('renders mutually exclusive enabled and disabled card classes', async () => {
    const [enabledHtml, disabledHtml] = await Promise.all([
      renderCard(baseViewModel()),
      renderCard(baseViewModel({
        card: {
          ...card,
          enabled: false,
        },
      })),
    ])

    expect(enabledHtml).toContain('is-enabled')
    expect(enabledHtml).not.toContain('is-disabled')
    expect(disabledHtml).toContain('is-disabled')
    expect(disabledHtml).not.toContain('is-enabled')
  })

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

  it('replaces the normal switch with quota exhaustion actions when auto-disabled', async () => {
    const html = await renderCard(baseViewModel({
      card: {
        ...card,
        enabled: false,
        quotaAutoDisabled: true,
      },
    }))

    expect(html).toContain('额度用完')
    expect(html).toContain('临时启用')
    expect(html).toContain('is-disabled')
    expect(html).not.toContain('is-enabled')
    expect(html).not.toContain('mac-switch sm')
  })

  it('keeps the normal switch and shows resume automation while temporarily enabled', async () => {
    const html = await renderCard(baseViewModel({
      card: {
        ...card,
        quotaAutoDisablePaused: true,
      },
    }))

    expect(html).toContain('额度用完·临时启用')
    expect(html).toContain('恢复自动')
    expect(html).toContain('mac-switch sm')
  })

  it('keeps quota query errors behind one compact details trigger', async () => {
    const quotaDisplay = [
      {
        key: 'primary-error',
        label: 'Primary balance',
        used: 0,
        total: 0,
        progressRatio: 0,
        countdownLabel: '',
        nextReset: null,
        queriedAt: Date.now(),
        valueMode: 'currency' as const,
        invalidMessage: 'temporary upstream timeout',
      },
      {
        key: 'backup-error',
        label: 'Backup balance',
        used: 0,
        total: 0,
        progressRatio: 0,
        countdownLabel: '',
        nextReset: null,
        queriedAt: Date.now(),
        valueMode: 'currency' as const,
        invalidMessage: 'authentication failed with a long response body',
      },
    ]
    const readyStats: ProviderCardViewModel['stats'] = {
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
      successRateLabel: '成功率: 100%',
      successRateClass: 'success-good',
      successRateHint: '成功 1 · 失败 0 · 已排除 0',
      failedRequests: 0,
      unreadFailedRequests: 0,
      hasUnreadErrorLogs: false,
    }
    const renderedCards = await Promise.all([
      renderCard(baseViewModel({ quotaDisplay })),
      renderCard(baseViewModel({ quotaDisplay, stats: readyStats })),
    ])

    for (const html of renderedCards) {
      expect(html.match(/card-balance-quota__error-trigger/g)).toHaveLength(1)
      expect(html).toContain('aria-expanded="false"')
      expect(html).toContain('card-balance-quota__error-icon')
      expect(html).toContain('card-balance-quota-panel__updated')
      expect(html).toContain('card-balance-quota-panel__refresh')
      expect(html).not.toContain('temporary upstream timeout')
      expect(html).not.toContain('authentication failed with a long response body')
    }
  })

  it('exposes provider quota progress to assistive technology', async () => {
    const html = await renderCard(baseViewModel({
      quotaDisplay: [{
        key: 'weekly',
        label: '周',
        used: 50,
        total: 200,
        progressRatio: 0.25,
        countdownLabel: '6d23h',
        nextReset: new Date('2026-08-24T07:58:35.941Z'),
        queriedAt: Date.now(),
        valueMode: 'currency',
      }],
    }))

    expect(html).toContain('role="progressbar"')
    expect(html).toContain('aria-label="周"')
    expect(html).toContain('aria-valuemin="0"')
    expect(html).toContain('aria-valuemax="100"')
    expect(html).toContain('aria-valuenow="25"')
    expect(html).toContain('aria-valuetext="25%"')
  })

  it('highlights active concurrency without marking it as full', async () => {
    const html = await renderCard(baseViewModel({
      concurrencyStatus: {
        platform: 'claude',
        providerId: '102',
        providerName: 'kimi',
        activeRequests: 1,
        limit: 5,
      },
      concurrencyLimitEnabled: true,
    }))

    expect(html).toContain('1/5')
    expect(html).toContain('provider-concurrency-pill--active')
    expect(html).not.toContain('provider-concurrency-pill--overflow')
  })

  it('uses the full state when active concurrency reaches its limit', async () => {
    const html = await renderCard(baseViewModel({
      concurrencyStatus: {
        platform: 'claude',
        providerId: '102',
        providerName: 'kimi',
        activeRequests: 5,
        limit: 5,
      },
      concurrencyLimitEnabled: true,
    }))

    expect(html).toContain('provider-concurrency-pill--active')
    expect(html).toContain('provider-concurrency-pill--overflow')
  })

  it('renders zero capacity as a static full state when limiting is enabled', async () => {
    const html = await renderCard(baseViewModel({
      concurrencyStatus: {
        platform: 'claude',
        providerId: '102',
        providerName: 'kimi',
        activeRequests: 0,
        limit: 0,
      },
      concurrencyLimitEnabled: true,
    }))

    expect(html).toContain('0/0')
    expect(html).toContain('provider-concurrency-pill--overflow')
    expect(html).not.toContain('provider-concurrency-pill--active')
  })

  it('keeps zero capacity informational when limiting is disabled', async () => {
    const html = await renderCard(baseViewModel({
      concurrencyStatus: {
        platform: 'claude',
        providerId: '102',
        providerName: 'kimi',
        activeRequests: 1,
        limit: 0,
      },
      concurrencyLimitEnabled: false,
    }))

    expect(html).toContain('1/0')
    expect(html).toContain('provider-concurrency-pill--active')
    expect(html).not.toContain('provider-concurrency-pill--overflow')
  })

  it('uses infinity only when the concurrency limit is empty', async () => {
    const html = await renderCard(baseViewModel({
      concurrencyStatus: {
        platform: 'claude',
        providerId: '102',
        providerName: 'kimi',
        activeRequests: 0,
      },
      concurrencyLimitEnabled: true,
    }))

    expect(html).toContain('0/∞')
    expect(html).not.toContain('provider-concurrency-pill--active')
    expect(html).not.toContain('provider-concurrency-pill--overflow')
  })

  it('renders a visibly active forced-priority button and badge', async () => {
    const html = await renderCard(baseViewModel({
      card: { ...card, forcedPriority: true },
    }), { activeProxyState: true })

    expect(html).toContain('forced-priority-badge')
    expect(html).toContain('>强制</span>')
    expect(html).toContain('is-active ghost-icon forced-priority-btn')
    expect(html).toContain('aria-pressed="true"')
    expect(html).toContain('aria-label="取消强制优先"')
    expect(html).toContain('aria-busy="false"')
  })

  it('disables and labels the forced-priority button while saving', async () => {
    const html = await renderCard(baseViewModel({
      card: { ...card, forcedPriority: true },
    }), {
      activeProxyState: true,
      forcedPrioritySaving: true,
    })

    expect(html).toContain('is-active ghost-icon forced-priority-btn')
    expect(html).toContain('disabled')
    expect(html).toContain('aria-label="保存中"')
    expect(html).toContain('aria-busy="true"')
  })
})

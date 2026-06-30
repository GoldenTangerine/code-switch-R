import { reactive, ref } from 'vue'
import { Call, Events } from '@wailsio/runtime'
import { getBlacklistStatus, type BlacklistStatus } from '../../../services/blacklist'
import { showToast } from '../../../utils/toast'
import {
  blacklistStatusKeyFromCard,
  blacklistStatusKeyFromStatus,
  cardProviderRef,
  normalizeProviderRef,
} from '../adapters/providerCardMappers'
import { PROVIDER_TAB_IDS } from '../constants'
import type { LastUsedProvider, ProviderTab, TranslateFn } from '../types'
import type { AutomationCard } from '../../../data/cards'
import { normalizeLastUsedProvider, shouldUseLastUsedProviderForTool } from '../utils/lastUsedProvider'

type UseBlacklistStateOptions = {
  t: TranslateFn
  getActiveTab: () => ProviderTab
  getSelectedToolId: () => string | null
  switchToPlatform: (platform: ProviderTab) => void
}

const createBlacklistMap = (): Record<ProviderTab, Record<string, BlacklistStatus>> => ({
  claude: {},
  codex: {},
  gemini: {},
  opencode: {},
  others: {},
})

const createLastUsedMap = (): Record<ProviderTab, LastUsedProvider | null> => ({
  claude: null,
  codex: null,
  gemini: null,
  opencode: null,
  others: null,
})

const isProviderTab = (value: string): value is ProviderTab =>
  PROVIDER_TAB_IDS.includes(value as ProviderTab)

export function useBlacklistState(options: UseBlacklistStateOptions) {
  const { t, getActiveTab, getSelectedToolId, switchToPlatform } = options

  const blacklistStatusMap = reactive(createBlacklistMap())
  const lastUsedProviders = reactive(createLastUsedMap())
  const highlightedProviderRef = ref<string | null>(null)
  const highlightedProviderName = ref<string | null>(null)

  let blacklistTimer: number | undefined
  let blacklistPollingTimer: number | undefined
  let highlightTimer: number | undefined
  let unsubscribeSwitched: (() => void) | undefined
  let unsubscribeBlacklisted: (() => void) | undefined
  let unsubscribeRouted: (() => void) | undefined
  let handleWindowFocus: (() => void) | undefined

  const applyHighlightedProvider = (provider: LastUsedProvider) => {
    highlightedProviderRef.value = normalizeProviderRef(provider.provider_id)
    highlightedProviderName.value = provider.provider_name

    if (highlightTimer) {
      clearTimeout(highlightTimer)
    }
    highlightTimer = window.setTimeout(() => {
      highlightedProviderRef.value = null
      highlightedProviderName.value = null
    }, 3000)
  }

  const loadBlacklistStatus = async (tab: ProviderTab) => {
    if (tab === 'others' || tab === 'opencode') {
      blacklistStatusMap[tab] = {}
      return
    }

    try {
      const statuses = await getBlacklistStatus(tab)
      const map: Record<string, BlacklistStatus> = {}
      statuses.forEach((status) => {
        map[blacklistStatusKeyFromStatus(status)] = status
      })
      blacklistStatusMap[tab] = map
    } catch (error) {
      console.error(`加载 ${tab} 黑名单状态失败:`, error)
    }
  }

  const handleUnblockAndReset = async (card: AutomationCard) => {
    const providerRef = cardProviderRef(card)
    try {
      if (providerRef) {
        await Call.ByName(
          'codeswitch/services.BlacklistService.ManualUnblockAndResetByID',
          getActiveTab(),
          providerRef,
          card.name,
        )
      } else {
        await Call.ByName(
          'codeswitch/services.BlacklistService.ManualUnblockAndReset',
          getActiveTab(),
          card.name,
        )
      }
      showToast(t('components.main.blacklist.unblockSuccess', { name: card.name }), 'success')
      await loadBlacklistStatus(getActiveTab())
    } catch (error) {
      console.error('解除拉黑失败:', error)
      showToast(t('components.main.blacklist.unblockFailed'), 'error')
    }
  }

  const handleResetLevel = async (card: AutomationCard) => {
    const providerRef = cardProviderRef(card)
    try {
      if (providerRef) {
        await Call.ByName(
          'codeswitch/services.BlacklistService.ManualResetLevelByID',
          getActiveTab(),
          providerRef,
          card.name,
        )
      } else {
        await Call.ByName(
          'codeswitch/services.BlacklistService.ManualResetLevel',
          getActiveTab(),
          card.name,
        )
      }
      showToast(t('components.main.blacklist.resetLevelSuccess', { name: card.name }), 'success')
      await loadBlacklistStatus(getActiveTab())
    } catch (error) {
      console.error('清零等级失败:', error)
      showToast(t('components.main.blacklist.resetLevelFailed'), 'error')
    }
  }

  const formatBlacklistCountdown = (remainingSeconds: number): string => {
    const minutes = Math.floor(remainingSeconds / 60)
    const seconds = remainingSeconds % 60
    return `${minutes}${t('components.main.blacklist.minutes')}${seconds}${t('components.main.blacklist.seconds')}`
  }

  const getProviderBlacklistStatus = (card: AutomationCard): BlacklistStatus | null => {
    const map = blacklistStatusMap[getActiveTab()]
    const statusKey = blacklistStatusKeyFromCard(card)
    return map[statusKey] || map[card.name.trim().toLowerCase()] || null
  }

  const loadLastUsedProviders = async () => {
    try {
      PROVIDER_TAB_IDS.forEach((platform) => {
        lastUsedProviders[platform] = null
      })

      const result = await Call.ByName('codeswitch/services.ProviderRelayStateService.GetAllLastUsedProviders')
      if (!result) return

      Object.keys(result).forEach((platform) => {
        const normalized = normalizeLastUsedProvider(result[platform])
        if (normalized) {
          if (!shouldUseLastUsedProviderForTool(normalized, getSelectedToolId())) {
            return
          }
          lastUsedProviders[normalized.platform] = normalized
        }
      })
    } catch (error) {
      console.error('加载最后使用的供应商失败:', error)
    }
  }

  const switchToTabAndHighlight = (provider: LastUsedProvider) => {
    if (!shouldUseLastUsedProviderForTool(provider, getSelectedToolId())) return

    switchToPlatform(provider.platform as ProviderTab)
    lastUsedProviders[provider.platform as ProviderTab] = provider
    applyHighlightedProvider(provider)

    void loadBlacklistStatus(provider.platform as ProviderTab)
  }

  const handleProviderSwitched = (event: { data: { platform: string; toProvider: string; toProviderId?: string; timestamp?: number } }) => {
    const normalized = normalizeLastUsedProvider({
      platform: event.data.platform,
      providerId: event.data.toProviderId,
      providerName: event.data.toProvider,
      updatedAt: event.data.timestamp,
    })
    if (!normalized || !isProviderTab(normalized.platform)) return
    console.log('[Event] provider:switched', normalized.platform, normalized.provider_name, normalized.provider_id)
    switchToTabAndHighlight(normalized)
  }

  const handleProviderBlacklisted = (event: { data: { platform: string; providerName: string; providerId?: string; timestamp?: number } }) => {
    const normalized = normalizeLastUsedProvider({
      platform: event.data.platform,
      providerId: event.data.providerId,
      providerName: event.data.providerName,
      updatedAt: event.data.timestamp,
    })
    if (!normalized || !isProviderTab(normalized.platform)) return
    console.log('[Event] provider:blacklisted', normalized.platform, normalized.provider_name, normalized.provider_id)
    switchToTabAndHighlight(normalized)
  }

  const handleProviderRouted = (event: { data: { platform: string; providerName: string; providerId?: string; timestamp?: number } }) => {
    const normalized = normalizeLastUsedProvider(event.data)
    if (!normalized || !isProviderTab(normalized.platform)) return
    if (!shouldUseLastUsedProviderForTool(normalized, getSelectedToolId())) return

    lastUsedProviders[normalized.platform] = normalized

    if (normalized.platform === getActiveTab()) {
      applyHighlightedProvider(normalized)
    }
  }

  const isLastUsedProvider = (card: AutomationCard): boolean => {
    const lastUsed = lastUsedProviders[getActiveTab()]
    if (!lastUsed) return false
    if (!shouldUseLastUsedProviderForTool(lastUsed, getSelectedToolId())) return false
    const cardRef = cardProviderRef(card)
    if (cardRef && normalizeProviderRef(lastUsed.provider_id) !== '') {
      return normalizeProviderRef(lastUsed.provider_id) === cardRef
    }
    return lastUsed.provider_name === card.name
  }

  const isHighlightedCard = (card: AutomationCard): boolean => {
    const highlightedRef = normalizeProviderRef(highlightedProviderRef.value)
    const cardRef = cardProviderRef(card)
    if (cardRef && highlightedRef) {
      return highlightedRef === cardRef
    }
    return highlightedProviderName.value === card.name
  }

  const scrollToCard = (element: HTMLElement | null) => {
    if (element) {
      element.scrollIntoView({ behavior: 'smooth', block: 'center' })
    }
  }

  const startStatusSync = () => {
    stopStatusSync()

    // 每秒递减倒计时，避免只靠轮询导致 UI 卡顿。
    blacklistTimer = window.setInterval(() => {
      const tab = getActiveTab()
      Object.values(blacklistStatusMap[tab]).forEach((status) => {
        if (!status?.isBlacklisted || status.remainingSeconds <= 0) return

        status.remainingSeconds--
        if (status.remainingSeconds <= 0) {
          void loadBlacklistStatus(tab)
        }
      })
    }, 1000)

    handleWindowFocus = () => {
      void loadBlacklistStatus(getActiveTab())
      void loadLastUsedProviders()
    }
    window.addEventListener('focus', handleWindowFocus)

    blacklistPollingTimer = window.setInterval(() => {
      void loadBlacklistStatus(getActiveTab())
      void loadLastUsedProviders()
    }, 10_000)

    unsubscribeSwitched = Events.On('provider:switched', handleProviderSwitched as Events.Callback)
    unsubscribeBlacklisted = Events.On('provider:blacklisted', handleProviderBlacklisted as Events.Callback)
    unsubscribeRouted = Events.On('provider:routed', handleProviderRouted as Events.Callback)
  }

  const stopStatusSync = () => {
    if (blacklistTimer) {
      window.clearInterval(blacklistTimer)
      blacklistTimer = undefined
    }
    if (blacklistPollingTimer) {
      window.clearInterval(blacklistPollingTimer)
      blacklistPollingTimer = undefined
    }
    if (handleWindowFocus) {
      window.removeEventListener('focus', handleWindowFocus)
      handleWindowFocus = undefined
    }
    if (highlightTimer) {
      clearTimeout(highlightTimer)
      highlightTimer = undefined
    }
    if (unsubscribeSwitched) {
      unsubscribeSwitched()
      unsubscribeSwitched = undefined
    }
    if (unsubscribeBlacklisted) {
      unsubscribeBlacklisted()
      unsubscribeBlacklisted = undefined
    }
    if (unsubscribeRouted) {
      unsubscribeRouted()
      unsubscribeRouted = undefined
    }
  }

  return {
    blacklistStatusMap,
    lastUsedProviders,
    highlightedProviderRef,
    highlightedProviderName,
    loadBlacklistStatus,
    handleUnblockAndReset,
    handleResetLevel,
    formatBlacklistCountdown,
    getProviderBlacklistStatus,
    loadLastUsedProviders,
    isLastUsedProvider,
    isHighlightedCard,
    scrollToCard,
    startStatusSync,
    stopStatusSync,
  }
}

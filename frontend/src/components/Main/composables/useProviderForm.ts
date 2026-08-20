import { reactive, ref } from 'vue'
import { Call } from '@wailsio/runtime'
import type { AutomationCard } from '../../../data/cards'
import type { LogPlatform } from '../../../services/logs'
import { createGeminiProviderRef, createOpenCodeProviderRef, normalizeProviderRef } from '../adapters/providerCardMappers'
import { buildPersistedProviderFieldsFromForm } from '../adapters/providerFormMappers'
import { applyGrokSingleProvider } from '../../../services/grokSettings'
import { applyClaudeDesktopSingleProvider } from '../../../services/claudeDesktopSettings'
import { setCurrentOpenClawProvider } from '../../../services/openClaw'
import { setCurrentHermesProvider } from '../../../services/hermes'
import { setCurrentPiProvider } from '../../../services/pi'
import type { ProviderTab, TranslateFn, VendorForm } from '../types'
import { isDirectApplyBlockedForProvider } from '../utils/providerDirectApply'

type ToastType = 'success' | 'error' | 'warning'

type UseProviderFormOptions = {
  initialTab: ProviderTab
  t: TranslateFn
  showToast: (message: string, type?: ToastType) => void
  getActiveTab: () => ProviderTab
  getSelectedToolId?: () => string | null
  cards: Record<ProviderTab, AutomationCard[]>
  normalizeLevel: (level: number | string | undefined) => number
  persistProviders: (tabId: ProviderTab) => Promise<boolean>
  refreshDirectAppliedStatus: (tabId: ProviderTab) => Promise<void>
  removeProvider: (id: number, tabId: ProviderTab) => Promise<void>
  duplicateProvider: (card: AutomationCard) => Promise<boolean>
  reloadProviders: () => Promise<void>
  moveCardToStatusGroup: (tabId: ProviderTab, card: AutomationCard, enabled: boolean) => boolean
  appendCardToGroup: (tabId: ProviderTab, card: AutomationCard) => void
}

type ProviderModalState = {
  open: boolean
  tabId: ProviderTab
  card: AutomationCard | null
}

type ConfirmState = {
  open: boolean
  card: AutomationCard | null
  tabId: ProviderTab
}

const cloneAutomationCards = (source: AutomationCard[]): AutomationCard[] => (
  JSON.parse(JSON.stringify(source)) as AutomationCard[]
)

const restoreCards = (target: AutomationCard[], snapshot: AutomationCard[]) => {
  target.splice(0, target.length, ...snapshot)
}


export function useProviderForm(options: UseProviderFormOptions) {
  const {
    initialTab,
    t,
    showToast,
    getActiveTab,
    getSelectedToolId,
    cards,
    normalizeLevel,
    persistProviders,
    refreshDirectAppliedStatus,
    removeProvider,
    duplicateProvider,
    reloadProviders,
    moveCardToStatusGroup,
    appendCardToGroup,
  } = options

  const modelListModalOpen = ref(false)
  const modelListModalProvider = ref<AutomationCard | null>(null)
  const providerLogsModalOpen = ref(false)
  const providerLogsModalProvider = ref<AutomationCard | null>(null)
  const providerLogsModalPlatform = ref<LogPlatform | null>(null)
  const providerLogBadgeSaving = ref(false)
  const providerDataOverviewModalOpen = ref(false)
  const providerDataOverviewModalProvider = ref<AutomationCard | null>(null)
  const providerDataOverviewModalPlatform = ref<LogPlatform | null>(null)
  const providerCostTrendModalOpen = ref(false)
  const providerCostTrendModalProvider = ref<AutomationCard | null>(null)
  const providerCostTrendModalPlatform = ref<LogPlatform | null>(null)
  const providerModalState = reactive<ProviderModalState>({
    open: false,
    tabId: initialTab,
    card: null,
  })
  const confirmState = reactive<ConfirmState>({
    open: false,
    card: null,
    tabId: initialTab,
  })

  const openModelList = (card: AutomationCard) => {
    if (getActiveTab() === 'opencode') {
      return
    }
    if (!card.apiUrl || !card.apiKey) {
      showToast(t('components.main.modelList.apiKeyRequired'), 'error')
      return
    }
    modelListModalProvider.value = card
    modelListModalOpen.value = true
  }

  const closeModelListModal = () => {
    modelListModalOpen.value = false
    modelListModalProvider.value = null
  }

  const openProviderLogs = (card: AutomationCard) => {
    const activeTab = getActiveTab()
    if (activeTab === 'others' || activeTab === 'opencode' || activeTab === 'grokbuild' || activeTab === 'claude-desktop' || activeTab === 'openclaw' || activeTab === 'hermes' || activeTab === 'pi') {
      return
    }
    providerLogsModalProvider.value = card
    providerLogsModalPlatform.value = activeTab
    providerLogsModalOpen.value = true
  }

  const closeProviderLogsModal = () => {
    providerLogsModalOpen.value = false
    providerLogsModalProvider.value = null
    providerLogsModalPlatform.value = null
  }

  const updateProviderLogBadgeEnabled = async (enabled: boolean) => {
    const card = providerLogsModalProvider.value
    const platform = providerLogsModalPlatform.value
    if (!card || !platform || providerLogBadgeSaving.value) return

    const previousHideLogBadge = card.hideLogBadge
    card.hideLogBadge = !enabled
    providerLogBadgeSaving.value = true

    try {
      await persistProviders(platform)
    } catch {
      card.hideLogBadge = previousHideLogBadge
    } finally {
      providerLogBadgeSaving.value = false
    }
  }

  const openProviderDataOverview = (card: AutomationCard) => {
    const activeTab = getActiveTab()
    if (activeTab === 'others' || activeTab === 'opencode' || activeTab === 'grokbuild' || activeTab === 'claude-desktop' || activeTab === 'openclaw' || activeTab === 'hermes' || activeTab === 'pi') {
      return
    }
    providerDataOverviewModalProvider.value = card
    providerDataOverviewModalPlatform.value = activeTab
    providerDataOverviewModalOpen.value = true
  }

  const closeProviderDataOverviewModal = () => {
    providerDataOverviewModalOpen.value = false
    providerDataOverviewModalProvider.value = null
    providerDataOverviewModalPlatform.value = null
  }

  const openProviderCostTrend = (card: AutomationCard) => {
    const activeTab = getActiveTab()
    if (activeTab === 'others' || activeTab === 'opencode' || activeTab === 'grokbuild' || activeTab === 'claude-desktop' || activeTab === 'openclaw' || activeTab === 'hermes' || activeTab === 'pi') {
      return
    }
    providerCostTrendModalProvider.value = card
    providerCostTrendModalPlatform.value = activeTab
    providerCostTrendModalOpen.value = true
  }

  const closeProviderCostTrendModal = () => {
    providerCostTrendModalOpen.value = false
    providerCostTrendModalProvider.value = null
    providerCostTrendModalPlatform.value = null
  }

  const openCreateModal = () => {
    providerModalState.tabId = getActiveTab()
    providerModalState.card = null
    providerModalState.open = true
  }

  const openEditModal = (card: AutomationCard) => {
    providerModalState.tabId = getActiveTab()
    providerModalState.card = card
    providerModalState.open = true
  }

  const closeProviderModal = () => {
    providerModalState.open = false
  }

  const applySavedProvider = async (savedCard: AutomationCard, tabId: ProviderTab) => {
    if (isDirectApplyBlockedForProvider(tabId, savedCard)) {
      showToast(
        t(savedCard.quotaAutoDisabled
          ? 'components.main.providers.quotaAutoDisabledHint'
          : 'components.main.directApply.requiresHostedRouting'),
        'warning',
      )
      return
    }

    try {
      if (tabId === 'claude') {
        await Call.ByName('codeswitch/services.ClaudeSettingsService.ApplySingleProvider', savedCard.id)
      } else if (tabId === 'codex') {
        await Call.ByName('codeswitch/services.CodexSettingsService.ApplySingleProvider', savedCard.id)
      } else if (tabId === 'gemini') {
        const providerRef = normalizeProviderRef(savedCard.providerRef)
        if (providerRef) {
          await Call.ByName('codeswitch/services.GeminiService.ApplySingleProvider', providerRef)
        }
      } else if (tabId === 'grokbuild') {
        await applyGrokSingleProvider(savedCard.id)
      } else if (tabId === 'claude-desktop') {
        await applyClaudeDesktopSingleProvider(savedCard.id)
      } else if (tabId === 'openclaw') {
        const openClawId = `${normalizeProviderRef(savedCard.providerRef) || savedCard.id}`.trim()
        if (openClawId) {
          await setCurrentOpenClawProvider(openClawId)
        }
      } else if (tabId === 'hermes') {
        const hermesId = `${normalizeProviderRef(savedCard.providerRef) || savedCard.id}`.trim()
        if (hermesId) {
          await setCurrentHermesProvider(hermesId)
        }
      } else if (tabId === 'pi') {
        const piId = `${normalizeProviderRef(savedCard.providerRef) || savedCard.id}`.trim()
        if (piId) {
          await setCurrentPiProvider(piId)
        }
      }

      await refreshDirectAppliedStatus(tabId)
      showToast(t('components.main.directApply.success', { name: savedCard.name }), 'success')
    } catch (error) {
      console.error('Apply after save failed', error)
      showToast(t('components.main.directApply.failed'), 'error')
    }
  }

  const saveProviderModal = async (form: VendorForm, applyAfterSave = false) => {
    const tabId = providerModalState.tabId
    const list = cards[tabId]
    if (!list) return

    const editingCard = providerModalState.card
    let savedCard: AutomationCard | null = null
    const providerFields = buildPersistedProviderFieldsFromForm(form, tabId, normalizeLevel)

    if (editingCard) {
      const previousEnabled = editingCard.enabled
      const previousCards = cloneAutomationCards(list)

      Object.assign(editingCard, {
        name: form.name || editingCard.name,
        apiUrl: tabId === 'opencode' ? form.apiUrl : (form.apiUrl || editingCard.apiUrl),
        ...providerFields,
      })

      if (previousEnabled !== providerFields.enabled) {
        moveCardToStatusGroup(tabId, editingCard, providerFields.enabled)
      }

      savedCard = editingCard
      try {
        await persistProviders(tabId)
      } catch (error) {
        restoreCards(list, previousCards)
        providerModalState.card = list.find((card) => card.id === editingCard.id) ?? null
        throw error
      }
    } else {
      const newCardId = Date.now()
      const providerRef = tabId === 'gemini'
        ? createGeminiProviderRef()
        : tabId === 'opencode'
          ? (form.providerRef || createOpenCodeProviderRef())
          : `${newCardId}`
      const newCard: AutomationCard = {
        id: newCardId,
        name: form.name || 'Untitled vendor',
        apiUrl: form.apiUrl,
        accent: '#0a84ff',
        tint: 'rgba(15, 23, 42, 0.12)',
        ...providerFields,
        providerRef,
      }
      appendCardToGroup(tabId, newCard)
      savedCard = newCard
      try {
        await persistProviders(tabId)
      } catch (error) {
        const insertedIndex = list.findIndex((card) => card === newCard)
        if (insertedIndex >= 0) {
          list.splice(insertedIndex, 1)
        }
        throw error
      }
    }

    closeProviderModal()
    window.dispatchEvent(new CustomEvent('providers-updated'))

    // 保存后直连和卡片状态都依赖最新落盘结果，这里统一兜底，避免入口分叉。
    if (!applyAfterSave || !savedCard || tabId === 'others') return
    await applySavedProvider(savedCard, tabId)
  }

  const submitProviderModal = async (form: VendorForm) => {
    await saveProviderModal(form, false)
  }

  const submitAndApplyProviderModal = async (form: VendorForm) => {
    await saveProviderModal(form, true)
  }

  const configure = (card: AutomationCard) => {
    openEditModal(card)
  }

  const requestRemove = (card: AutomationCard) => {
    confirmState.card = card
    confirmState.tabId = getActiveTab()
    confirmState.open = true
  }

  const closeConfirm = () => {
    confirmState.open = false
    confirmState.card = null
  }

  const confirmRemove = async () => {
    if (!confirmState.card) return
    const card = confirmState.card
    const tabId = confirmState.tabId
    await removeProvider(confirmState.card.id, confirmState.tabId)
    closeConfirm()
  }

  const handleDuplicate = async (card: AutomationCard) => {
    const duplicated = await duplicateProvider(card)
    if (duplicated) {
      await reloadProviders()
    }
  }

  const handleProviderEnabledChange = async (card: AutomationCard, enabled: boolean) => {
    const tabId = getActiveTab()
    const previousCards = cloneAutomationCards(cards[tabId])
    card.quotaAutoDisabled = false
    card.quotaAutoDisablePaused = false
    moveCardToStatusGroup(tabId, card, enabled)
    if (tabId === 'opencode') {
      card.liveConfigManaged = enabled
      card.isInConfig = enabled
    }
    try {
      await persistProviders(tabId)
      if (!enabled) {
          }
    } catch (error) {
      restoreCards(cards[tabId], previousCards)
      throw error
    }
  }

  const persistModelMappingRuleEnabled = async (key: string, enabled: boolean) => {
    const card = providerModalState.card
    if (!card?.modelMapping || !Object.prototype.hasOwnProperty.call(card.modelMapping, key)) return

    const previousDisabled = { ...(card.modelMappingDisabled || {}) }
    const nextDisabled = { ...previousDisabled }
    if (enabled) {
      delete nextDisabled[key]
    } else {
      nextDisabled[key] = true
    }
    card.modelMappingDisabled = nextDisabled

    try {
      await persistProviders(providerModalState.tabId)
    } catch (error) {
      card.modelMappingDisabled = previousDisabled
      throw error
    }
  }

  return {
    modelListModalOpen,
    modelListModalProvider,
    providerLogsModalOpen,
    providerLogsModalProvider,
    providerLogsModalPlatform,
    providerLogBadgeSaving,
    providerDataOverviewModalOpen,
    providerDataOverviewModalProvider,
    providerDataOverviewModalPlatform,
    providerCostTrendModalOpen,
    providerCostTrendModalProvider,
    providerCostTrendModalPlatform,
    providerModalState,
    confirmState,
    openModelList,
    closeModelListModal,
    openProviderLogs,
    closeProviderLogsModal,
    updateProviderLogBadgeEnabled,
    openProviderDataOverview,
    closeProviderDataOverviewModal,
    openProviderCostTrend,
    closeProviderCostTrendModal,
    openCreateModal,
    openEditModal,
    closeProviderModal,
    submitProviderModal,
    submitAndApplyProviderModal,
    configure,
    requestRemove,
    closeConfirm,
    confirmRemove,
    handleDuplicate,
    handleProviderEnabledChange,
    persistModelMappingRuleEnabled,
  }
}

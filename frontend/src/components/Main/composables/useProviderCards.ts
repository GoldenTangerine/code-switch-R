import { reactive, ref } from 'vue'
import { Call } from '@wailsio/runtime'
import { automationCardGroups, createAutomationCards, type AutomationCard } from '../../../data/cards'
import { DuplicateProvider, LoadProviders, SaveProviders } from '../../../../bindings/codeswitch/services/providerservice'
import {
  AddProvider as AddGeminiProvider,
  DeleteProvider as DeleteGeminiProvider,
  GetProviders as GetGeminiProviders,
  ReorderProviders as ReorderGeminiProviders,
  UpdateProvider as UpdateGeminiProvider,
} from '../../../../bindings/codeswitch/services/geminiservice'
import { showToast } from '../../../utils/toast'
import {
  cardToGemini,
  createGeminiFromCard,
  createGeminiProviderRef,
  geminiToCard,
  normalizeProviderRef,
  serializeProviders,
  type GeminiProvider,
} from '../adapters/providerCardMappers'
import { PROVIDER_TAB_IDS } from '../constants'
import type { ProviderDragEndPayload, ProviderDragTarget, ProviderTab, TranslateFn } from '../types'
import {
  applyNormalizedProviderOrder,
  appendProviderToStatusGroup,
  commitProviderOrder,
  moveProviderToStatusGroup,
} from '../utils/providerOrder'

type UseProviderCardsOptions = {
  t: TranslateFn
  getActiveTab: () => ProviderTab
  isActiveProxyEnabled: () => boolean
  getSelectedToolId: () => string | null
}

const createCardRecord = (): Record<ProviderTab, AutomationCard[]> => ({
  claude: createAutomationCards(automationCardGroups.claude),
  codex: createAutomationCards(automationCardGroups.codex),
  gemini: [],
  others: [],
})

const createDirectAppliedIds = (): Record<ProviderTab, string | number | null> => ({
  claude: null,
  codex: null,
  gemini: null,
  others: null,
})

const getCustomProviderKind = (toolId: string): string => `custom:${toolId}`
const DRAG_FINALIZE_DELAY_MS = 24

export function useProviderCards(options: UseProviderCardsOptions) {
  const { t, getActiveTab, isActiveProxyEnabled, getSelectedToolId } = options

  const cards = reactive(createCardRecord())
  const draggingId = ref<number | null>(null)
  const dragOverId = ref<number | null>(null)
  const directAppliedIds = reactive(createDirectAppliedIds())
  const geminiProvidersCache = ref<GeminiProvider[]>([])
  const dragStartOrder = ref<number[]>([])
  const dragSourceTab = ref<ProviderTab | null>(null)
  const lastDragTarget = ref<ProviderDragTarget | null>(null)
  const dragSessionId = ref(0)
  const finalizedDragSessionId = ref(0)
  const droppedDragSessionId = ref(0)
  const dragWithinList = ref(false)

  const hasOrderChanged = (list: AutomationCard[], snapshot: number[]) =>
    snapshot.length === list.length && snapshot.some((id, index) => list[index]?.id !== id)

  const restoreOrder = (list: AutomationCard[], snapshot: number[]) => {
    if (!snapshot.length) return
    const orderMap = new Map(snapshot.map((id, index) => [id, index]))
    list.sort((left, right) => {
      const leftIndex = orderMap.get(left.id) ?? Number.MAX_SAFE_INTEGER
      const rightIndex = orderMap.get(right.id) ?? Number.MAX_SAFE_INTEGER
      return leftIndex - rightIndex
    })
  }

  const resetDragState = () => {
    draggingId.value = null
    dragOverId.value = null
    dragStartOrder.value = []
    dragSourceTab.value = null
    lastDragTarget.value = null
    dragWithinList.value = false
  }

  const finalizeDrag = async (
    sessionId: number,
    options: {
      dropEffect?: DataTransfer['dropEffect'] | 'none'
      endedInsideList?: boolean | null
      forcePersist?: boolean
    } = {},
  ) => {
    if (finalizedDragSessionId.value === sessionId) return
    if (dragSessionId.value !== sessionId) return

    const currentTab = dragSourceTab.value ?? getActiveTab()
    const list = cards[currentTab]
    const changed = list ? hasOrderChanged(list, dragStartOrder.value) : false
    const wasDropped = droppedDragSessionId.value === sessionId
    const endedInsideList = options.endedInsideList ?? null
    const shouldPersist =
      !!list &&
      changed &&
      (
        options.forcePersist === true ||
        wasDropped ||
        endedInsideList === true ||
        (
          endedInsideList === null &&
          (
            options.dropEffect === 'move' ||
            (dragWithinList.value && lastDragTarget.value !== null)
          )
        )
      )

    finalizedDragSessionId.value = sessionId

    if (list && changed && shouldPersist) {
      commitProviderOrder(list)
      resetDragState()
      await persistProviders(currentTab)
      return
    }

    if (list && changed) {
      restoreOrder(list, dragStartOrder.value)
    }
    resetDragState()
  }

  const reorderDraggingCard = (target: ProviderDragTarget) => {
    if (draggingId.value === null) return false

    const currentTab = dragSourceTab.value ?? getActiveTab()
    const list = cards[currentTab]
    if (!list?.length) return false

    const { id: targetId, position } = target
    dragOverId.value = targetId

    if (lastDragTarget.value?.id === targetId && lastDragTarget.value.position === position) {
      return false
    }

    if (draggingId.value === targetId) {
      lastDragTarget.value = target
      return false
    }

    const fromIndex = list.findIndex((card) => card.id === draggingId.value)
    const toIndex = list.findIndex((card) => card.id === targetId)
    if (fromIndex === -1 || toIndex === -1) return false

    if (list[fromIndex]?.enabled !== list[toIndex]?.enabled) {
      dragOverId.value = null
      lastDragTarget.value = null
      return false
    }

    let newIndex = position === 'after' ? toIndex + 1 : toIndex
    if (fromIndex < newIndex) {
      newIndex -= 1
    }

    lastDragTarget.value = target

    if (newIndex === fromIndex) return false

    const [moved] = list.splice(fromIndex, 1)
    list.splice(newIndex, 0, moved)
    return true
  }

  const normalizeLevel = (level: number | string | undefined): number => {
    const numeric = Number(level)
    if (!Number.isFinite(numeric) || numeric < 1) return 1
    if (numeric > 10) return 10
    return Math.floor(numeric)
  }

  const refreshDirectAppliedStatus = async (tab: ProviderTab = getActiveTab()) => {
    if (tab === 'others') return

    try {
      let id: string | number | null = null
      if (tab === 'claude') {
        id = await Call.ByName('codeswitch/services.ClaudeSettingsService.GetDirectAppliedProviderID')
      } else if (tab === 'codex') {
        id = await Call.ByName('codeswitch/services.CodexSettingsService.GetDirectAppliedProviderID')
      } else if (tab === 'gemini') {
        id = await Call.ByName('codeswitch/services.GeminiService.GetDirectAppliedProviderID')
      }
      directAppliedIds[tab] = id
    } catch (error) {
      console.error(`Failed to get direct applied status for ${tab}`, error)
    }
  }

  const handleDirectApply = async (card: AutomationCard) => {
    if (isActiveProxyEnabled()) return

    const tab = getActiveTab()
    try {
      if (tab === 'claude') {
        await Call.ByName('codeswitch/services.ClaudeSettingsService.ApplySingleProvider', card.id)
      } else if (tab === 'codex') {
        await Call.ByName('codeswitch/services.CodexSettingsService.ApplySingleProvider', card.id)
      } else if (tab === 'gemini') {
        const providerRef = normalizeProviderRef(card.providerRef)
        if (!providerRef) return
        await Call.ByName('codeswitch/services.GeminiService.ApplySingleProvider', providerRef)
      }
      await refreshDirectAppliedStatus(tab)
      showToast(t('components.main.directApply.success', { name: card.name }), 'success')
    } catch (error) {
      console.error('Direct apply failed', error)
      showToast(t('components.main.directApply.failed'), 'error')
    }
  }

  const isDirectApplied = (card: AutomationCard) => {
    const activeTab = getActiveTab()
    const appliedId = directAppliedIds[activeTab]
    if (appliedId === null) return false

    if (activeTab === 'gemini') {
      const providerRef = normalizeProviderRef(card.providerRef)
      if (!providerRef) return false
      return providerRef === appliedId
    }
    return card.id === appliedId
  }

  const replaceProviders = (tabId: ProviderTab, data: AutomationCard[]) => {
    const withRefs = createAutomationCards(data).map((card) => ({
      ...card,
      providerRef: normalizeProviderRef(card.providerRef) || `${card.id}`,
    }))
    cards[tabId].splice(0, cards[tabId].length, ...withRefs)
    applyNormalizedProviderOrder(cards[tabId])
  }

  const loadCustomCliProviders = async (toolId: string) => {
    if (!toolId) return

    try {
      const saved = await LoadProviders(getCustomProviderKind(toolId))
      if (Array.isArray(saved)) {
        cards.others.splice(0, cards.others.length, ...createAutomationCards(saved as AutomationCard[]))
        applyNormalizedProviderOrder(cards.others)
      } else {
        cards.others.splice(0, cards.others.length)
      }
    } catch (error) {
      console.error(`Failed to load providers for tool ${toolId}`, error)
      cards.others.splice(0, cards.others.length)
    }
  }

  const persistProviders = async (tabId: ProviderTab) => {
    try {
      commitProviderOrder(cards[tabId])

      if (tabId === 'others') {
        const selectedToolId = getSelectedToolId()
        if (!selectedToolId) {
          showToast(t('components.main.customCli.selectToolFirst'), 'error')
          return
        }
        await SaveProviders(getCustomProviderKind(selectedToolId), serializeProviders(cards.others))
        return
      }

      if (tabId === 'gemini') {
        const currentRefs = new Set(cards.gemini.map((card) => normalizeProviderRef(card.providerRef)).filter(Boolean))

        for (const cached of geminiProvidersCache.value) {
          if (!currentRefs.has(normalizeProviderRef(cached.id))) {
            await DeleteGeminiProvider(cached.id)
          }
        }

        for (const card of cards.gemini) {
          const providerRef = normalizeProviderRef(card.providerRef)
          const original = providerRef
            ? geminiProvidersCache.value.find((provider) => normalizeProviderRef(provider.id) === providerRef)
            : undefined

          if (original) {
            await UpdateGeminiProvider(cardToGemini(card, original))
          } else {
            const newProviderID = providerRef || createGeminiProviderRef()
            card.providerRef = newProviderID
            const newProvider = createGeminiFromCard(card, newProviderID)
            await AddGeminiProvider(newProvider)
          }
        }

        const updatedProviders = await GetGeminiProviders()
        geminiProvidersCache.value = updatedProviders

        const orderedIds: string[] = []
        for (const card of cards.gemini) {
          const providerRef = normalizeProviderRef(card.providerRef)
          const provider = providerRef
            ? updatedProviders.find((item) => normalizeProviderRef(item.id) === providerRef)
            : undefined
          if (provider) {
            orderedIds.push(provider.id)
          }
        }
        if (orderedIds.length > 0) {
          await ReorderGeminiProviders(orderedIds)
          geminiProvidersCache.value = await GetGeminiProviders()
        }
        return
      }

      await SaveProviders(tabId, serializeProviders(cards[tabId]))
    } catch (error) {
      console.error('Failed to save providers', error)
      showToast(t('components.main.form.saveFailed'), 'error')
    }
  }

  const loadProvidersFromDisk = async (loadCustomCliTools: () => Promise<void>) => {
    for (const tab of PROVIDER_TAB_IDS) {
      try {
        if (tab === 'others') {
          await loadCustomCliTools()
        } else if (tab === 'gemini') {
          const geminiProviders = await GetGeminiProviders()
          geminiProvidersCache.value = geminiProviders
          cards.gemini.splice(0, cards.gemini.length, ...geminiProviders.map(geminiToCard))
          applyNormalizedProviderOrder(cards.gemini)
        } else {
          const saved = await LoadProviders(tab)
          if (Array.isArray(saved)) {
            replaceProviders(tab, saved as AutomationCard[])
          } else {
            await persistProviders(tab)
          }
        }
      } catch (error) {
        console.error('Failed to load providers', error)
        showToast(t('components.main.errors.loadProvidersFailed', { tab }), 'error')
      }
    }
  }

  const removeProvider = async (id: number, tabId: ProviderTab = getActiveTab()) => {
    const list = cards[tabId]
    if (!list) return

    const index = list.findIndex((card) => card.id === id)
    if (index < 0) return

    list.splice(index, 1)
    await persistProviders(tabId)
  }

  const duplicateProvider = async (card: AutomationCard) => {
    try {
      const tab = getActiveTab()

      if (tab === 'gemini') {
        const providerRef = normalizeProviderRef(card.providerRef)
        if (!providerRef) {
          console.error('[Duplicate] 未找到 Gemini provider')
          return false
        }

        const newProvider = await Call.ByName(
          'codeswitch/services.GeminiService.DuplicateProvider',
          providerRef,
        )

        if (!newProvider) {
          console.warn('[Duplicate] DuplicateProvider 返回空结果，已跳过刷新')
          return false
        }
        console.log(`[Duplicate] Gemini Provider "${card.name}" duplicated`)
        return true
      }

      const newProvider = await DuplicateProvider(tab, card.id)
      if (!newProvider) {
        console.warn('[Duplicate] DuplicateProvider 返回空结果，已跳过刷新')
        return false
      }
      console.log(`[Duplicate] Provider "${card.name}" duplicated as "${newProvider.name}"`)
      return true
    } catch (error) {
      console.error('[Duplicate] Failed to duplicate provider:', error)
      return false
    }
  }

  const onDragStart = (id: number) => {
    const currentTab = getActiveTab()
    dragSessionId.value += 1
    finalizedDragSessionId.value = 0
    droppedDragSessionId.value = 0
    draggingId.value = id
    dragOverId.value = null
    dragSourceTab.value = currentTab
    dragStartOrder.value = cards[currentTab].map((card) => card.id)
    lastDragTarget.value = null
    dragWithinList.value = false
  }

  const onDragOverCard = (target: ProviderDragTarget) => {
    dragWithinList.value = true
    reorderDraggingCard(target)
  }

  const onDragLeaveList = () => {
    if (draggingId.value === null) return
    dragWithinList.value = false
    dragOverId.value = null
  }

  const onDrop = async (target: ProviderDragTarget) => {
    if (draggingId.value === null) return

    reorderDraggingCard(target)
    const sessionId = dragSessionId.value
    droppedDragSessionId.value = sessionId
    await finalizeDrag(sessionId, { forcePersist: true })
  }

  const onDragEnd = (payload: ProviderDragEndPayload = {
    dropEffect: 'none',
    clientX: null,
    clientY: null,
    endedInsideList: null,
  }) => {
    if (draggingId.value === null) return
    const sessionId = dragSessionId.value

    return new Promise<void>((resolve) => {
      setTimeout(() => {
        void finalizeDrag(sessionId, {
          dropEffect: payload.dropEffect,
          endedInsideList: payload.endedInsideList ?? null,
        }).finally(resolve)
      }, DRAG_FINALIZE_DELAY_MS)
    })
  }

  const moveCardToStatusGroup = (tabId: ProviderTab, card: AutomationCard, enabled: boolean) => {
    const list = cards[tabId]
    if (!list) return false
    return moveProviderToStatusGroup(list, card, enabled)
  }

  const appendCardToGroup = (tabId: ProviderTab, card: AutomationCard) => {
    const list = cards[tabId]
    if (!list) return
    appendProviderToStatusGroup(list, card)
  }

  return {
    cards,
    draggingId,
    dragOverId,
    directAppliedIds,
    normalizeLevel,
    refreshDirectAppliedStatus,
    handleDirectApply,
    isDirectApplied,
    loadCustomCliProviders,
    persistProviders,
    loadProvidersFromDisk,
    removeProvider,
    duplicateProvider,
    onDragStart,
    onDragOverCard,
    onDragLeaveList,
    onDrop,
    onDragEnd,
    moveCardToStatusGroup,
    appendCardToGroup,
  }
}

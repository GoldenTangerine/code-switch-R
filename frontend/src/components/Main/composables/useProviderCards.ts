import { reactive, ref } from 'vue'
import { Call } from '@wailsio/runtime'
import { automationCardGroups, createAutomationCards, type AutomationCard } from '../../../data/cards'
import { DuplicateProvider, LoadProviders, SaveProviders } from '../../../../bindings/codeswitch/services/providerservice'
import {
  AddProvider as AddGeminiProvider,
  DeleteProvider as DeleteGeminiProvider,
  GetProviders as GetGeminiProviders,
  ReorderProviders as ReorderGeminiProviders,
  SetForcedPriority as SetGeminiForcedPriority,
  UpdateProvider as UpdateGeminiProvider,
} from '../../../../bindings/codeswitch/services/geminiservice'
import { showToast } from '../../../utils/toast'
import { extractErrorMessage } from '../../../utils/error'
import {
  cardProviderRef,
  cardToGemini,
  cardToOpenCode,
  createGeminiFromCard,
  createGeminiProviderRef,
  createOpenCodeFromCard,
  createOpenCodeProviderRef,
  deserializeProviders,
  geminiToCard,
  normalizeProviderRef,
  opencodeToCard,
  serializeProviders,
  type GeminiProvider,
  type OpenCodeProvider,
  type PersistedProvider,
} from '../adapters/providerCardMappers'
import {
  duplicateOpenCodeProvider,
  getOpenCodeProviders,
  importOpenCodeProvidersFromLive,
  saveOpenCodeProviders,
} from '../../../services/opencode'
import { applyGrokSingleProvider, getGrokStatus } from '../../../services/grokSettings'
import {
  applyClaudeDesktopSingleProvider,
  getClaudeDesktopStatus,
} from '../../../services/claudeDesktopSettings'
import {
  addOpenClawProvider,
  cardToOpenClawProvider,
  deleteOpenClawProvider,
  duplicateOpenClawProvider,
  getOpenClawProviders,
  getOpenClawStatus,
  openClawToCard,
  setCurrentOpenClawProvider,
  updateOpenClawProvider,
  type OpenClawProvider,
} from '../../../services/openClaw'
import {
  addHermesProvider,
  cardToHermesProvider,
  deleteHermesProvider,
  duplicateHermesProvider,
  getHermesProviders,
  getHermesStatus,
  hermesToCard,
  setCurrentHermesProvider,
  updateHermesProvider,
  type HermesProvider,
} from '../../../services/hermes'
import {
  addPiProvider,
  cardToPiProvider,
  deletePiProvider,
  duplicatePiProvider,
  getPiProviders,
  getPiStatus,
  piToCard,
  setCurrentPiProvider,
  updatePiProvider,
  type PiProvider,
} from '../../../services/pi'
import { PROVIDER_TAB_IDS } from '../constants'
import type { ProviderDragEndPayload, ProviderDragTarget, ProviderTab, TranslateFn } from '../types'
import {
  applyNormalizedProviderOrder,
  insertProviderToStatusGroup,
  commitProviderOrder,
  moveProviderToStatusGroup,
} from '../utils/providerOrder'
import { isDirectApplyBlockedForProvider } from '../utils/providerDirectApply'

type UseProviderCardsOptions = {
  t: TranslateFn
  getActiveTab: () => ProviderTab
  isActiveProxyEnabled: () => boolean
  getSelectedToolId: () => string | null
}

type BrowserWindowWithWailsBridge = Window & {
  chrome?: {
    webview?: {
      postMessage?: (...args: any[]) => void
    }
  }
  webkit?: {
    messageHandlers?: {
      external?: {
        postMessage?: (...args: any[]) => void
      }
    }
  }
}

const DEV_CLAUDE_API_FORMAT_DEMO_BY_NAME: Partial<Record<string, NonNullable<AutomationCard['apiFormat']>>> = {
  Kimi: 'openai_chat',
  Deepseek: 'openai_responses',
}

const hasDesktopRuntimeBridge = () => {
  if (typeof window === 'undefined') {
    return false
  }
  const browserWindow = window as BrowserWindowWithWailsBridge
  return Boolean(
    browserWindow.chrome?.webview?.postMessage ||
    browserWindow.webkit?.messageHandlers?.external?.postMessage,
  )
}

const shouldApplyBrowserDevDemo = () => (
  import.meta.env.MODE !== 'production'
  && typeof window !== 'undefined'
  && !hasDesktopRuntimeBridge()
)

const shouldApplyDevClaudeApiFormatDemo = shouldApplyBrowserDevDemo

const shouldApplyDevOpenCodeProviders = () => (
  shouldApplyBrowserDevDemo()
)

const withDevClaudeApiFormatDemo = (cards: AutomationCard[]): AutomationCard[] => {
  if (!shouldApplyDevClaudeApiFormatDemo()) {
    return cards
  }

  return cards.map((card) => {
    const demoApiFormat = DEV_CLAUDE_API_FORMAT_DEMO_BY_NAME[card.name.trim()]
    if (demoApiFormat) {
      return {
        ...card,
        apiFormat: demoApiFormat,
      }
    }

    return card
  })
}

const loadDevOpenCodeProviders = async (): Promise<OpenCodeProvider[] | null> => {
  if (import.meta.env.MODE === 'production') {
    return null
  }

  if (!shouldApplyDevOpenCodeProviders()) {
    return null
  }

  const { createDevOpenCodeProviders } = await import('./devOpenCodeProviders')
  return createDevOpenCodeProviders()
}

const loadDevOpenCodeCards = async (): Promise<AutomationCard[] | null> => {
  if (import.meta.env.MODE === 'production') {
    return null
  }

  if (!shouldApplyDevOpenCodeProviders()) {
    return null
  }

  const { createDevOpenCodeCards } = await import('./devOpenCodeProviders')
  return createDevOpenCodeCards()
}

const loadOpenCodeProvidersForCards = async (): Promise<{
  providers: OpenCodeProvider[]
  isDevPreview: boolean
}> => {
  try {
    const loadedProviders = await getOpenCodeProviders()
    const providers = Array.isArray(loadedProviders) ? loadedProviders : []
    return { providers, isDevPreview: false }
  } catch (error) {
    if (!shouldApplyDevOpenCodeProviders()) {
      throw error
    }
    console.warn('[OpenCode] 本地 dev 预览无法连接桌面服务，已使用前端 mock provider。', error)
  }

  const devProviders = await loadDevOpenCodeProviders()
  if (devProviders) {
    return { providers: devProviders, isDevPreview: true }
  }

  return { providers: [], isDevPreview: false }
}

const cloneAutomationCards = (source: AutomationCard[]): AutomationCard[] => (
  JSON.parse(JSON.stringify(source)) as AutomationCard[]
)

const createCardRecord = (): Record<ProviderTab, AutomationCard[]> => ({
  claude: withDevClaudeApiFormatDemo(createAutomationCards(automationCardGroups.claude)),
  codex: createAutomationCards(automationCardGroups.codex),
  gemini: [],
  opencode: [],
  grokbuild: [],
  'claude-desktop': [],
  openclaw: [],
  hermes: [],
  pi: [],
  others: [],
})

const createDirectAppliedIds = (): Record<ProviderTab, string | number | null> => ({
  claude: null,
  codex: null,
  gemini: null,
  opencode: null,
  grokbuild: null,
  'claude-desktop': null,
  openclaw: null,
  hermes: null,
  pi: null,
  others: null,
})

const getCustomProviderKind = (toolId: string): string => `custom:${toolId}`
const DRAG_FINALIZE_DELAY_MS = 24

// additive 三平台（openclaw/hermes/pi）的持久化标记：进行中的 tab 集合
const persistingTabs = new Set<ProviderTab>()

export function useProviderCards(options: UseProviderCardsOptions) {
  const { t, getActiveTab, isActiveProxyEnabled, getSelectedToolId } = options

  const cards = reactive(createCardRecord())
  const draggingId = ref<number | null>(null)
  const dragOverId = ref<number | null>(null)
  const forcedPrioritySaving = ref(false)
  const directAppliedIds = reactive(createDirectAppliedIds())
  const geminiProvidersCache = ref<GeminiProvider[]>([])
  const opencodeProvidersCache = ref<OpenCodeProvider[]>([])
  const openclawProvidersCache = ref<OpenClawProvider[]>([])
  const hermesProvidersCache = ref<HermesProvider[]>([])
  const piProvidersCache = ref<PiProvider[]>([])
  const isOpenCodeDevPreview = ref(false)
  const dragStartOrder = ref<number[]>([])
  const dragSourceTab = ref<ProviderTab | null>(null)
  const lastDragTarget = ref<ProviderDragTarget | null>(null)
  const dragSessionId = ref(0)
  const finalizedDragSessionId = ref(0)
  const droppedDragSessionId = ref(0)
  const dragWithinList = ref(false)

  const hydrateDevOpenCodePreviewCards = async () => {
    const devCards = await loadDevOpenCodeCards()
    if (!devCards || cards.opencode.length > 0) {
      return
    }

    isOpenCodeDevPreview.value = true
    opencodeProvidersCache.value = []
    cards.opencode.splice(0, cards.opencode.length, ...devCards)
    applyNormalizedProviderOrder(cards.opencode)
  }

  void hydrateDevOpenCodePreviewCards()

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
    if (tab === 'others' || tab === 'opencode') {
      directAppliedIds[tab] = null
      return
    }

    try {
      let id: string | number | null = null
      if (tab === 'claude') {
        id = await Call.ByName('codeswitch/services.ClaudeSettingsService.GetDirectAppliedProviderID')
      } else if (tab === 'codex') {
        id = await Call.ByName('codeswitch/services.CodexSettingsService.GetDirectAppliedProviderID')
      } else if (tab === 'gemini') {
        id = await Call.ByName('codeswitch/services.GeminiService.GetDirectAppliedProviderID')
      } else if (tab === 'grokbuild') {
        // Grok 无独立 DirectApplied 接口，用 config.toml 当前 base_url 反查匹配的供应商卡片
        const status = await getGrokStatus()
        const liveBaseUrl = status.baseUrl.trim().replace(/\/+$/, '')
        const matched = liveBaseUrl
          ? cards.grokbuild.find((card) => card.apiUrl.trim().replace(/\/+$/, '') === liveBaseUrl)
          : undefined
        id = matched ? matched.id : null
      } else if (tab === 'claude-desktop') {
        // Claude Desktop 优先用 GetStatus 返回的后端统一存储数值 providerId 反查，缺省时退回 baseUrl 匹配
        const status = await getClaudeDesktopStatus()
        const numericProviderId = Number(status.providerId)
        if (Number.isFinite(numericProviderId) && numericProviderId > 0) {
          // 数值 providerId 对应统一存储的条目 ID：按 card.id 数值或 providerRef 字符串双路匹配
          const matched = cards['claude-desktop'].find((card) => (
            card.id === numericProviderId || normalizeProviderRef(card.providerRef) === status.providerId
          ))
          id = matched ? matched.id : null
        } else {
          const liveBaseUrl = status.baseUrl.trim().replace(/\/+$/, '')
          const matched = liveBaseUrl
            ? cards['claude-desktop'].find((card) => card.apiUrl.trim().replace(/\/+$/, '') === liveBaseUrl)
            : undefined
          id = matched ? matched.id : null
        }
      } else if (tab === 'openclaw') {
        // OpenClaw additive 模式：GetStatus 返回 settings 中当前生效条目，按 providerRef 反查卡片
        const status = await getOpenClawStatus()
        const currentRef = normalizeProviderRef(status.currentProviderId)
        const matchedByRef = currentRef
          ? cards.openclaw.find((card) => normalizeProviderRef(card.providerRef) === currentRef)
          : undefined
        if (matchedByRef) {
          id = matchedByRef.id
        } else if (status.currentProviderName) {
          const matchedByName = cards.openclaw.find((card) => card.name === status.currentProviderName)
          id = matchedByName ? matchedByName.id : null
        }
      } else if (tab === 'hermes') {
        // Hermes additive 模式：GetStatus 返回顶层 model 节指向的条目，按 providerRef 反查卡片
        const status = await getHermesStatus()
        const currentRef = normalizeProviderRef(status.currentProviderId)
        const matchedByRef = currentRef
          ? cards.hermes.find((card) => normalizeProviderRef(card.providerRef) === currentRef)
          : undefined
        if (matchedByRef) {
          id = matchedByRef.id
        } else if (status.currentProviderName) {
          const matchedByName = cards.hermes.find((card) => card.name === status.currentProviderName)
          id = matchedByName ? matchedByName.id : null
        }
      } else if (tab === 'pi') {
        // Pi additive 模式：GetStatus 返回启用标记指向的条目，按 providerRef 反查卡片
        const status = await getPiStatus()
        const currentRef = normalizeProviderRef(status.currentProviderId)
        const matchedByRef = currentRef
          ? cards.pi.find((card) => normalizeProviderRef(card.providerRef) === currentRef)
          : undefined
        if (matchedByRef) {
          id = matchedByRef.id
        } else if (status.currentProviderName) {
          const matchedByName = cards.pi.find((card) => card.name === status.currentProviderName)
          id = matchedByName ? matchedByName.id : null
        }
      }
      directAppliedIds[tab] = id
    } catch (error) {
      console.error(`Failed to get direct applied status for ${tab}`, error)
    }
  }

  const handleDirectApply = async (card: AutomationCard) => {
    const tab = getActiveTab()
    if (isActiveProxyEnabled() || isDirectApplyBlockedForProvider(tab, card)) return

    try {
      if (tab === 'claude') {
        await Call.ByName('codeswitch/services.ClaudeSettingsService.ApplySingleProvider', card.id)
      } else if (tab === 'codex') {
        await Call.ByName('codeswitch/services.CodexSettingsService.ApplySingleProvider', card.id)
      } else if (tab === 'gemini') {
        const providerRef = normalizeProviderRef(card.providerRef)
        if (!providerRef) return
        await Call.ByName('codeswitch/services.GeminiService.ApplySingleProvider', providerRef)
      } else if (tab === 'grokbuild') {
        await applyGrokSingleProvider(card.id)
      } else if (tab === 'claude-desktop') {
        await applyClaudeDesktopSingleProvider(card.id)
      } else if (tab === 'openclaw') {
        const openClawId = `${normalizeProviderRef(cardProviderRef(card)) || card.id}`.trim()
        if (!openClawId) return
        await setCurrentOpenClawProvider(openClawId)
      } else if (tab === 'hermes') {
        const hermesId = `${normalizeProviderRef(cardProviderRef(card)) || card.id}`.trim()
        if (!hermesId) return
        await setCurrentHermesProvider(hermesId)
      } else if (tab === 'pi') {
        const piId = `${normalizeProviderRef(cardProviderRef(card)) || card.id}`.trim()
        if (!piId) return
        await setCurrentPiProvider(piId)
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

  const replaceProviders = (tabId: ProviderTab, data: PersistedProvider[]) => {
    cards[tabId].splice(0, cards[tabId].length, ...deserializeProviders(data, tabId))
    applyNormalizedProviderOrder(cards[tabId])
  }

  const loadCustomCliProviders = async (toolId: string) => {
    if (!toolId) return

    try {
      const saved = await LoadProviders(getCustomProviderKind(toolId))
      if (Array.isArray(saved)) {
        cards.others.splice(0, cards.others.length, ...deserializeProviders(saved as PersistedProvider[], 'others'))
        applyNormalizedProviderOrder(cards.others)
      } else {
        cards.others.splice(0, cards.others.length)
      }
    } catch (error) {
      console.error(`Failed to load providers for tool ${toolId}`, error)
      cards.others.splice(0, cards.others.length)
    }
  }

  // 后端 Add/Update 返回的完整供应商携带最终 ID（新增时可能重新分配），直接精确回填卡片 providerRef
  const applyPersistedProviderRef = (card: AutomationCard, saved: { id?: string } | null) => {
    const savedRef = saved ? normalizeProviderRef(saved.id) : ''
    if (savedRef) {
      card.providerRef = savedRef
    }
  }

  // OpenClaw：后端若重新分配 ID，保存后按名称把最新落盘结果回填到卡片 providerRef（仅作后端返回值缺失时的兜底；
  // 同名条目无法消歧时跳过，避免顺序错配）
  const syncOpenClawCardRefs = (providers: OpenClawProvider[], knownRefs: Set<string>) => {
    const unmatched = providers.filter((provider) => !knownRefs.has(normalizeProviderRef(provider.id)))
    for (const card of cards.openclaw) {
      const ref = normalizeProviderRef(card.providerRef)
      if (ref && knownRefs.has(ref)) continue
      const sameName = unmatched.filter((provider) => provider.name === card.name)
      if (sameName.length !== 1) continue
      card.providerRef = normalizeProviderRef(sameName[0].id)
      unmatched.splice(unmatched.indexOf(sameName[0]), 1)
    }
  }

  // Hermes：后端若重新分配 ID（hermes-<unixnano>），保存后按名称把最新落盘结果回填到卡片 providerRef（同名跳过防错配）
  const syncHermesCardRefs = (providers: HermesProvider[], knownRefs: Set<string>) => {
    const unmatched = providers.filter((provider) => !knownRefs.has(normalizeProviderRef(provider.id)))
    for (const card of cards.hermes) {
      const ref = normalizeProviderRef(card.providerRef)
      if (ref && knownRefs.has(ref)) continue
      const sameName = unmatched.filter((provider) => provider.name === card.name)
      if (sameName.length !== 1) continue
      card.providerRef = normalizeProviderRef(sameName[0].id)
      unmatched.splice(unmatched.indexOf(sameName[0]), 1)
    }
  }

  // Pi：后端若重新分配 ID（pi-<unixnano>），保存后按名称把最新落盘结果回填到卡片 providerRef（同名跳过防错配）
  const syncPiCardRefs = (providers: PiProvider[], knownRefs: Set<string>) => {
    const unmatched = providers.filter((provider) => !knownRefs.has(normalizeProviderRef(provider.id)))
    for (const card of cards.pi) {
      const ref = normalizeProviderRef(card.providerRef)
      if (ref && knownRefs.has(ref)) continue
      const sameName = unmatched.filter((provider) => provider.name === card.name)
      if (sameName.length !== 1) continue
      card.providerRef = normalizeProviderRef(sameName[0].id)
      unmatched.splice(unmatched.indexOf(sameName[0]), 1)
    }
  }

  // additive 三平台保存失败时可能已部分落盘（逐条写入），从后端重拉缓存与卡片，避免 UI 快照与真实状态漂移
  const reloadAdditiveTab = async (tabId: ProviderTab) => {
    try {
      if (tabId === 'openclaw') {
        openclawProvidersCache.value = await getOpenClawProviders()
        cards.openclaw.splice(0, cards.openclaw.length, ...openclawProvidersCache.value.map(openClawToCard))
        applyNormalizedProviderOrder(cards.openclaw)
      } else if (tabId === 'hermes') {
        hermesProvidersCache.value = await getHermesProviders()
        cards.hermes.splice(0, cards.hermes.length, ...hermesProvidersCache.value.map(hermesToCard))
        applyNormalizedProviderOrder(cards.hermes)
      } else if (tabId === 'pi') {
        piProvidersCache.value = await getPiProviders()
        cards.pi.splice(0, cards.pi.length, ...piProvidersCache.value.map(piToCard))
        applyNormalizedProviderOrder(cards.pi)
      }
    } catch (reloadError) {
      console.error(`Failed to reload providers for ${tabId} after save failure`, reloadError)
    }
  }

  const persistProviders = async (tabId: ProviderTab): Promise<boolean> => {
    // additive 三平台逐条 Add/Update/Delete，同 tab 并发保存会基于同一缓存快照互相踩踏，加锁串行化
    if (persistingTabs.has(tabId)) {
      showToast(t('components.main.form.saveInProgress'), 'error')
      return false
    }
    persistingTabs.add(tabId)
    try {
      commitProviderOrder(cards[tabId])

      if (tabId === 'others') {
        const selectedToolId = getSelectedToolId()
        if (!selectedToolId) {
          showToast(t('components.main.customCli.selectToolFirst'), 'error')
          return false
        }
        await SaveProviders(getCustomProviderKind(selectedToolId), serializeProviders(cards.others, 'others'))
        return true
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
        return true
      }

      if (tabId === 'opencode') {
        if (isOpenCodeDevPreview.value) {
          return true
        }

        const nextProviders: OpenCodeProvider[] = []
        for (const card of cards.opencode) {
          const providerRef = normalizeProviderRef(card.providerRef)
          const original = providerRef
            ? opencodeProvidersCache.value.find((provider) => normalizeProviderRef(provider.id) === providerRef)
            : undefined

          if (original) {
            nextProviders.push(cardToOpenCode(card, original))
          } else {
            const newProviderID = providerRef || createOpenCodeProviderRef()
            card.providerRef = newProviderID
            nextProviders.push(createOpenCodeFromCard(card, newProviderID))
          }
        }

        await saveOpenCodeProviders(nextProviders)
        opencodeProvidersCache.value = await getOpenCodeProviders()
        return true
      }

      if (tabId === 'openclaw') {
        const currentRefs = new Set(cards.openclaw.map((card) => normalizeProviderRef(card.providerRef)).filter(Boolean))

        for (const cached of openclawProvidersCache.value) {
          if (!currentRefs.has(normalizeProviderRef(cached.id))) {
            await deleteOpenClawProvider(`${cached.id}`)
          }
        }

        for (const card of cards.openclaw) {
          const providerRef = normalizeProviderRef(card.providerRef)
          const original = providerRef
            ? openclawProvidersCache.value.find((provider) => normalizeProviderRef(provider.id) === providerRef)
            : undefined

          if (original) {
            applyPersistedProviderRef(card, await updateOpenClawProvider(cardToOpenClawProvider(card, original)))
          } else {
            applyPersistedProviderRef(card, await addOpenClawProvider(cardToOpenClawProvider(card)))
          }
        }

        const updatedProviders = await getOpenClawProviders()
        openclawProvidersCache.value = updatedProviders
        syncOpenClawCardRefs(updatedProviders, currentRefs)
        return true
      }

      if (tabId === 'hermes') {
        const currentRefs = new Set(cards.hermes.map((card) => normalizeProviderRef(card.providerRef)).filter(Boolean))

        for (const cached of hermesProvidersCache.value) {
          if (!currentRefs.has(normalizeProviderRef(cached.id))) {
            await deleteHermesProvider(`${cached.id}`)
          }
        }

        for (const card of cards.hermes) {
          const providerRef = normalizeProviderRef(card.providerRef)
          const original = providerRef
            ? hermesProvidersCache.value.find((provider) => normalizeProviderRef(provider.id) === providerRef)
            : undefined

          if (original) {
            applyPersistedProviderRef(card, await updateHermesProvider(cardToHermesProvider(card, original)))
          } else {
            applyPersistedProviderRef(card, await addHermesProvider(cardToHermesProvider(card)))
          }
        }

        const updatedProviders = await getHermesProviders()
        hermesProvidersCache.value = updatedProviders
        syncHermesCardRefs(updatedProviders, currentRefs)
        return true
      }

      if (tabId === 'pi') {
        const currentRefs = new Set(cards.pi.map((card) => normalizeProviderRef(card.providerRef)).filter(Boolean))

        for (const cached of piProvidersCache.value) {
          if (!currentRefs.has(normalizeProviderRef(cached.id))) {
            await deletePiProvider(`${cached.id}`)
          }
        }

        for (const card of cards.pi) {
          const providerRef = normalizeProviderRef(card.providerRef)
          const original = providerRef
            ? piProvidersCache.value.find((provider) => normalizeProviderRef(provider.id) === providerRef)
            : undefined

          if (original) {
            applyPersistedProviderRef(card, await updatePiProvider(cardToPiProvider(card, original)))
          } else {
            applyPersistedProviderRef(card, await addPiProvider(cardToPiProvider(card)))
          }
        }

        const updatedProviders = await getPiProviders()
        piProvidersCache.value = updatedProviders
        syncPiCardRefs(updatedProviders, currentRefs)
        return true
      }

      await SaveProviders(tabId, serializeProviders(cards[tabId], tabId))
      return true
    } catch (error) {
      console.error('Failed to save providers', error)
      showToast(t('components.main.form.saveFailed', { error: extractErrorMessage(error) }), 'error')
      await reloadAdditiveTab(tabId)
      throw error
    } finally {
      persistingTabs.delete(tabId)
    }
  }

  const importOpenCodeLiveProviders = async () => {
    try {
      const imported = await importOpenCodeProvidersFromLive()
      const opencodeProviders = await getOpenCodeProviders()
      isOpenCodeDevPreview.value = false
      opencodeProvidersCache.value = opencodeProviders
      cards.opencode.splice(0, cards.opencode.length, ...opencodeProviders.map(opencodeToCard))
      applyNormalizedProviderOrder(cards.opencode)
      return imported
    } catch (error) {
      console.error('Failed to import OpenCode live providers', error)
      showToast(t('components.main.importConfig.opencodeError'), 'error')
      return null
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
        } else if (tab === 'opencode') {
          const { providers: opencodeProviders, isDevPreview } = await loadOpenCodeProvidersForCards()
          isOpenCodeDevPreview.value = isDevPreview
          opencodeProvidersCache.value = opencodeProviders
          cards.opencode.splice(0, cards.opencode.length, ...opencodeProviders.map(opencodeToCard))
          applyNormalizedProviderOrder(cards.opencode)
        } else if (tab === 'openclaw') {
          const openclawProviders = await getOpenClawProviders()
          openclawProvidersCache.value = openclawProviders
          cards.openclaw.splice(0, cards.openclaw.length, ...openclawProviders.map(openClawToCard))
          applyNormalizedProviderOrder(cards.openclaw)
        } else if (tab === 'hermes') {
          const hermesProviders = await getHermesProviders()
          hermesProvidersCache.value = hermesProviders
          cards.hermes.splice(0, cards.hermes.length, ...hermesProviders.map(hermesToCard))
          applyNormalizedProviderOrder(cards.hermes)
        } else if (tab === 'pi') {
          const piProviders = await getPiProviders()
          piProvidersCache.value = piProviders
          cards.pi.splice(0, cards.pi.length, ...piProviders.map(piToCard))
          applyNormalizedProviderOrder(cards.pi)
        } else {
          const saved = await LoadProviders(tab)
          if (Array.isArray(saved)) {
            replaceProviders(tab, saved as PersistedProvider[])
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

    const previousCards = cloneAutomationCards(list)
    list.splice(index, 1)
    try {
      await persistProviders(tabId)
    } catch (error) {
      // additive 平台逐条写入可能部分成功，persistProviders 已从后端重拉真实状态，回滚 UI 快照仅用于整体保存的平台
      if (tabId !== 'openclaw' && tabId !== 'hermes' && tabId !== 'pi') {
        list.splice(0, list.length, ...previousCards)
      }
      throw error
    }
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

      if (tab === 'opencode') {
        if (isOpenCodeDevPreview.value) {
          console.info('[Duplicate] OpenCode dev mock provider 不支持复制到后端，已跳过')
          return false
        }

        const providerRef = normalizeProviderRef(card.providerRef)
        if (!providerRef) {
          console.error('[Duplicate] 未找到 OpenCode provider')
          return false
        }

        const newProvider = await duplicateOpenCodeProvider(providerRef)
        if (!newProvider) {
          console.warn('[Duplicate] OpenCode DuplicateProvider 返回空结果，已跳过刷新')
          return false
        }
        console.log(`[Duplicate] OpenCode Provider "${card.name}" duplicated`)
        return true
      }

      if (tab === 'openclaw') {
        const openClawId = `${normalizeProviderRef(card.providerRef) || card.id}`.trim()
        if (!openClawId) {
          console.error('[Duplicate] 未找到 OpenClaw provider')
          return false
        }

        const newProvider = await duplicateOpenClawProvider(openClawId)
        if (!newProvider) {
          console.warn('[Duplicate] OpenClaw DuplicateProvider 返回空结果，已跳过刷新')
          return false
        }
        console.log(`[Duplicate] OpenClaw Provider "${card.name}" duplicated`)
        return true
      }

      if (tab === 'hermes') {
        const hermesId = `${normalizeProviderRef(card.providerRef) || card.id}`.trim()
        if (!hermesId) {
          console.error('[Duplicate] 未找到 Hermes provider')
          return false
        }

        const newProvider = await duplicateHermesProvider(hermesId)
        if (!newProvider) {
          console.warn('[Duplicate] Hermes DuplicateProvider 返回空结果，已跳过刷新')
          return false
        }
        console.log(`[Duplicate] Hermes Provider "${card.name}" duplicated`)
        return true
      }

      if (tab === 'pi') {
        const piId = `${normalizeProviderRef(card.providerRef) || card.id}`.trim()
        if (!piId) {
          console.error('[Duplicate] 未找到 Pi provider')
          return false
        }

        const newProvider = await duplicatePiProvider(piId)
        if (!newProvider) {
          console.warn('[Duplicate] Pi DuplicateProvider 返回空结果，已跳过刷新')
          return false
        }
        console.log(`[Duplicate] Pi Provider "${card.name}" duplicated`)
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
    insertProviderToStatusGroup(list, card)
  }

  const setForcedPriority = async (card: AutomationCard) => {
    if (forcedPrioritySaving.value) {
      return false
    }
    const tabId = getActiveTab()
    if (!isActiveProxyEnabled()) {
      showToast(t('components.main.forcedPriority.proxyRequired'), 'warning')
      return false
    }
    if (!card.forcedPriority && (!card.enabled || card.quotaAutoDisabled)) {
      showToast(t('components.main.forcedPriority.providerUnavailable'), 'warning')
      return false
    }

    const list = cards[tabId]
    if (!list) return false
    const previous = cloneAutomationCards(list)
    const nextForcedPriority = !card.forcedPriority
    list.forEach((item) => {
      item.forcedPriority = item.id === card.id ? nextForcedPriority : false
    })
    forcedPrioritySaving.value = true
    try {
      if (tabId === 'gemini') {
        const providerRef = normalizeProviderRef(card.providerRef)
        if (!providerRef) {
          throw new Error('未找到 Gemini 供应商标识')
        }
        await SetGeminiForcedPriority(providerRef, nextForcedPriority)
        geminiProvidersCache.value = geminiProvidersCache.value.map((provider) => ({
          ...provider,
          forcedPriority: nextForcedPriority && normalizeProviderRef(provider.id) === providerRef,
        }))
      } else if (!await persistProviders(tabId)) {
        list.splice(0, list.length, ...previous)
        return false
      }
      showToast(
        nextForcedPriority
          ? t('components.main.forcedPriority.enabled', { name: card.name })
          : t('components.main.forcedPriority.cancelled'),
        'success',
      )
      return true
    } catch (error) {
      if (tabId === 'gemini') {
        try {
          const providers = await GetGeminiProviders()
          geminiProvidersCache.value = providers
          const forcedPriorityByID = new Map(providers.map((provider) => [
            normalizeProviderRef(provider.id),
            provider.forcedPriority === true,
          ]))
          list.forEach((item) => {
            item.forcedPriority = forcedPriorityByID.get(normalizeProviderRef(item.providerRef)) === true
          })
        } catch (reloadError) {
          console.error('Failed to reload Gemini forced priority after save failure', reloadError)
          list.splice(0, list.length, ...previous)
        }
        showToast(t('components.main.form.saveFailed', { error: extractErrorMessage(error) }), 'error')
      } else {
        list.splice(0, list.length, ...previous)
      }
      throw error
    } finally {
      forcedPrioritySaving.value = false
    }
  }

  return {
    cards,
    draggingId,
    dragOverId,
    forcedPrioritySaving,
    directAppliedIds,
    normalizeLevel,
    refreshDirectAppliedStatus,
    handleDirectApply,
    isDirectApplied,
    loadCustomCliProviders,
    persistProviders,
    loadProvidersFromDisk,
    importOpenCodeLiveProviders,
    removeProvider,
    duplicateProvider,
    onDragStart,
    onDragOverCard,
    onDragLeaveList,
    onDrop,
    onDragEnd,
    moveCardToStatusGroup,
    appendCardToGroup,
    setForcedPriority,
  }
}

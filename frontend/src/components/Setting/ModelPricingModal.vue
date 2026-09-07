<!--
@name: 模型价格与费用展示
@Descripttion: 维护模型价格规则及请求费用展示。
@version: 1.0.0
@Author: sm
@Date: 2026-09-07 11:13:10
@LastEditTime: 2026-09-07 11:13:10
@FilePath: frontend/src/components/Setting/ModelPricingModal.vue
-->
<script setup lang="ts">
import { Events } from '@wailsio/runtime'
import { computed, nextTick, onBeforeUnmount, onMounted, ref, shallowRef, watch, type ComponentPublicInstance } from 'vue'
import InlineModal from '../common/InlineModal.vue'
import BaseInput from '../common/BaseInput.vue'
import ModelPricingEditorModal from './ModelPricingEditorModal.vue'
import ClaudePricingPreviewModal from './ClaudePricingPreviewModal.vue'
import CloudPricingConflictModal from './CloudPricingConflictModal.vue'
import CloudPricingRulesSummary from './CloudPricingRulesSummary.vue'
import { useI18n } from 'vue-i18n'
import {
  MODEL_PRICING_CHANGED_EVENT,
  listModelPricing,
  previewCloudPriceTableSyncConflicts,
  previewClaudeOfficialPricing,
  syncCloudPriceTable,
  syncClaudeOfficialPricing,
  type ClaudeOfficialPricingPreviewRow,
  type CloudPriceTableSyncConflictRow,
  type ModelPricingRow,
} from '../../services/modelPricing'
import { extractErrorMessage } from '../../utils/error'
import { showToast } from '../../utils/toast'
import { buildVariableHeightVirtualList } from '../../utils/virtualList'
import {
  buildModelProviderTabs,
  collectModelProviderIconKeys,
  resolveModelProviderKey,
  resolveModelProviderMeta,
  type ModelProviderFilterKey,
  type ModelProviderKey,
} from '../../utils/modelProviders'
import {
  collectProviderDisplayIconKeys,
  getProviderDisplayIconSvg,
  preloadProviderDisplayIcons,
  resolveProviderDisplayIconKey,
} from '../../utils/providerIconAssets'
import {
  isManualModelPricingRow,
  matchesModelPricingSourceFilter,
  normalizeModelPricingSource,
  type ModelPricingSourceFilter,
} from '../../utils/modelPricingFilters'

const props = defineProps<{
  open: boolean
}>()

const emit = defineEmits<{
  (e: 'close'): void
}>()

const { t } = useI18n()

const MODEL_PRICING_ROW_ESTIMATED_HEIGHT = 120
const MODEL_PRICING_ROW_GAP = 10
const MODEL_PRICING_OVERSCAN = 720

type EditMode = 'edit' | 'new'
type PricingSource = 'builtin' | 'manual' | 'claude_sync' | 'cloud_sync'

interface DisplayCacheCreatePriceEntry {
  key: string
  label: string
  valueText: string
  hint: string
}

interface DisplayModelPricingRow {
  raw: ModelPricingRow
  model: string
  searchableText: string
  source: PricingSource
  isManualSource: boolean
  providerKey: ModelProviderKey
  providerLabel: string
  providerIconKey: string
  hasKnownProvider: boolean
  sourceClass: string
  sourceLabel: string
  sourceTooltip: string
  inputValueText: string
  outputValueText: string
  groupMultiplierText: string
  cacheCreateEntries: DisplayCacheCreatePriceEntry[]
  cacheReadValueText: string
  cacheReadHint: string
}

const loading = ref(false)
const error = ref('')

const rows = shallowRef<ModelPricingRow[]>([])
const search = ref('')
const selectedSourceFilter = ref<ModelPricingSourceFilter>('all')
const selectedProvider = ref<ModelProviderFilterKey>('all')
const selectedModel = ref<string>('')
const syncTask = ref<Promise<void> | null>(null)
const syncing = computed(() => syncTask.value !== null)
const syncMenuOpen = ref(false)
const syncMenuRef = ref<HTMLElement | null>(null)
const syncTriggerRef = ref<HTMLElement | null>(null)
const claudePreviewOpen = ref(false)
const claudePreviewRows = ref<ClaudeOfficialPricingPreviewRow[]>([])
const claudePreviewFetchedAt = ref('')
const previewRequestSeq = ref(0)
const cloudConflictOpen = ref(false)
const cloudConflictRows = ref<CloudPriceTableSyncConflictRow[]>([])
const cloudConflictFetchedAt = ref('')
const cloudConflictRequestSeq = ref(0)
const editorOpen = ref(false)
const editorMode = ref<EditMode>('edit')
const editorRow = ref<ModelPricingRow | null>(null)
const hasLoadedRows = ref(false)
const rowsStale = ref(true)
const listViewportRef = ref<HTMLElement | null>(null)
const listScrollTop = ref(0)
const listViewportHeight = ref(0)
const measuredItemHeights = shallowRef<Record<string, number>>({})

const visibleItemElements = new Map<string, HTMLElement>()
let loadRowsTask: Promise<void> | null = null
let measureFrameId = 0
let scrollFrameId = 0
let unsubscribeModelPricingChanged: (() => void) | null = null

function perTokenToPer1M(value: number) {
  return Number.isFinite(value) ? value * 1_000_000 : 0
}

function formatUsdPer1M(value: number) {
  if (!Number.isFinite(value)) return '—'
  const per1m = perTokenToPer1M(value)
  if (per1m === 0) return '$0'
  if (per1m < 0) return '—'
  if (per1m < 0.01) return `$${per1m.toFixed(6)}`
  if (per1m < 1) return `$${per1m.toFixed(4)}`
  return `$${per1m.toFixed(2)}`
}

function calculateMultiplier(value: number, input: number) {
  if (!Number.isFinite(value) || value < 0) return null
  if (!Number.isFinite(input) || input < 0) return null
  if (input === 0) return value === 0 ? 0 : null
  return value / input
}

function formatMultiplier(value: number | null | undefined) {
  if (typeof value !== 'number' || !Number.isFinite(value) || value < 0) return '—'
  if (Math.abs(value - Math.round(value)) < 1e-9) return `${Math.round(value)}x`
  if (value < 0.01) return `${value.toFixed(4)}x`
  return `${value.toFixed(2)}x`
}

function formatMultiplierHint(value: number | null | undefined) {
  const formatted = formatMultiplier(value)
  if (formatted === '—') return ''
  return `${t('components.general.modelPricing.columns.multiplier')} ${formatted}`
}

function resolveCacheCreateMultiplier(row: ModelPricingRow) {
  return calculateMultiplier(row.cache_creation_input_token_cost, row.input_cost_per_token)
}

function resolveCacheReadMultiplier(row: ModelPricingRow) {
  return calculateMultiplier(row.cache_read_input_token_cost, row.input_cost_per_token)
}

function resolveCacheCreateHint(row: ModelPricingRow) {
  return formatMultiplierHint(resolveCacheCreateMultiplier(row))
}

function resolveCacheReadHint(row: ModelPricingRow) {
  return formatMultiplierHint(resolveCacheReadMultiplier(row))
}

function resolveCacheCreatePriceEntries(row: ModelPricingRow) {
  const entries: Array<{ key: string; label: string; value: number; hint: string }> = []
  const hasGenericCache = Number.isFinite(row.cache_creation_input_token_cost) && row.cache_creation_input_token_cost >= 0
  const has1hCache = Number.isFinite(row.ephemeral_1h_cost_per_token) && row.ephemeral_1h_cost_per_token > 0
  const shouldShowBaseCache = hasGenericCache && (row.cache_creation_input_token_cost > 0 || !has1hCache)

  if (shouldShowBaseCache) {
    if (has1hCache) {
      entries.push({
        key: 'cache-create-5m',
        label: t('components.general.modelPricing.columns.cacheCreate5m'),
        value: row.cache_creation_input_token_cost,
        hint: resolveCacheCreateHint(row),
      })
    } else {
      entries.push({
        key: 'cache-create',
        label: t('components.general.modelPricing.columns.cacheCreate'),
        value: row.cache_creation_input_token_cost,
        hint: resolveCacheCreateHint(row),
      })
    }
  }

  if (has1hCache) {
    entries.push({
      key: 'cache-create-1h',
      label: t('components.general.modelPricing.columns.cacheCreate1h'),
      value: row.ephemeral_1h_cost_per_token,
      hint: formatMultiplierHint(calculateMultiplier(row.ephemeral_1h_cost_per_token, row.input_cost_per_token)),
    })
  }

  return entries
}

function resolvePricingSource(row: ModelPricingRow): PricingSource {
  const normalized = normalizeModelPricingSource(row.source)
  if (normalized) return normalized
  return row.is_override || row.is_custom ? 'manual' : 'builtin'
}

function resolvePricingSourceLabel(row: ModelPricingRow) {
  const source = resolvePricingSource(row)
  if (source === 'claude_sync') return t('components.general.modelPricing.badge.claudeSynced')
  if (source === 'cloud_sync') return t('components.general.modelPricing.badge.cloudSynced')
  if (source === 'builtin') return t('components.general.modelPricing.badge.builtin')
  return t('components.general.modelPricing.badge.manual')
}

function resolvePricingSourceClass(row: ModelPricingRow) {
  const source = resolvePricingSource(row)
  if (source === 'claude_sync') return 'tag-claude-synced'
  if (source === 'cloud_sync') return 'tag-cloud-synced'
  if (source === 'builtin') return 'tag-builtin'
  return 'tag-manual'
}

function formatDateTimeForTooltip(raw: string | undefined) {
  const value = (raw ?? '').trim()
  if (!value) return ''
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) return ''

  const pad = (num: number) => String(num).padStart(2, '0')
  const year = parsed.getFullYear()
  const month = pad(parsed.getMonth() + 1)
  const day = pad(parsed.getDate())
  const hour = pad(parsed.getHours())
  const minute = pad(parsed.getMinutes())
  const second = pad(parsed.getSeconds())
  return `${year}-${month}-${day} ${hour}:${minute}:${second}`
}

function resolvePricingSourceTooltip(row: ModelPricingRow) {
  const source = resolvePricingSource(row)
  if (source !== 'claude_sync' && source !== 'cloud_sync') return ''
  const formatted = formatDateTimeForTooltip(row.source_updated_at)
  if (!formatted) return ''
  return t('components.general.modelPricing.badge.syncedAtTooltip', {
    source: resolvePricingSourceLabel(row),
    time: formatted,
  })
}

function warmupProviderIcons(targetRows: ModelPricingRow[]) {
  const iconKeys = collectProviderDisplayIconKeys(
    collectModelProviderIconKeys(targetRows.map((row) => ({ model: row.model }))),
  )
  if (iconKeys.length > 0) {
    void preloadProviderDisplayIcons(iconKeys)
  }
}

function providerIconSvg(iconKey: string) {
  return getProviderDisplayIconSvg(resolveProviderDisplayIconKey(iconKey))
}

function buildDisplayModelPricingRow(row: ModelPricingRow): DisplayModelPricingRow {
  const source = resolvePricingSource(row)
  const providerMeta = resolveModelProviderMeta(row.model)
  return {
    raw: row,
    model: row.model,
    searchableText: [row.model, providerMeta?.label ?? '']
      .join(' ')
      .trim()
      .toLowerCase(),
    source,
    isManualSource: isManualModelPricingRow(row),
    providerKey: resolveModelProviderKey(row.model),
    providerLabel: providerMeta?.label ?? '',
    providerIconKey: providerMeta?.iconKey ?? '',
    hasKnownProvider: Boolean(providerMeta),
    sourceClass: resolvePricingSourceClass(row),
    sourceLabel: resolvePricingSourceLabel(row),
    sourceTooltip: resolvePricingSourceTooltip(row),
    inputValueText: `${formatUsdPer1M(row.input_cost_per_token)}/M`,
    outputValueText: `${formatUsdPer1M(row.output_cost_per_token)}/M`,
    groupMultiplierText: formatMultiplier(row.group_multiplier),
    cacheCreateEntries: resolveCacheCreatePriceEntries(row).map((entry) => ({
      key: entry.key,
      label: entry.label,
      valueText: `${formatUsdPer1M(entry.value)}/M`,
      hint: entry.hint,
    })),
    cacheReadValueText: `${formatUsdPer1M(row.cache_read_input_token_cost)}/M`,
    cacheReadHint: resolveCacheReadHint(row),
  }
}

const displayRows = computed<DisplayModelPricingRow[]>(() => rows.value.map((row) => buildDisplayModelPricingRow(row)))

const manualCount = computed(() => displayRows.value.filter((item) => item.isManualSource).length)

const sourceFilteredRows = computed(() => (
  displayRows.value.filter((item) => matchesModelPricingSourceFilter(selectedSourceFilter.value, item.raw))
))

const providerTabs = computed(() => buildModelProviderTabs(
  sourceFilteredRows.value.map((item) => ({ model: item.model })),
  {
    allLabel: t('components.general.modelPricing.vendorFilters.all'),
    unknownLabel: t('components.general.modelPricing.vendorFilters.unknown'),
  },
))

const filteredRows = computed(() => {
  const keyword = search.value.trim().toLowerCase()
  const base = sourceFilteredRows.value.filter((item) => (
    selectedProvider.value === 'all' || item.providerKey === selectedProvider.value
  ))
  if (!keyword) return base
  return base.filter((item) => item.searchableText.includes(keyword))
})

const virtualRowsState = computed(() => buildVariableHeightVirtualList({
  items: filteredRows.value,
  getItemKey: (item) => item.model,
  measuredHeights: measuredItemHeights.value,
  scrollTop: listScrollTop.value,
  viewportHeight: listViewportHeight.value,
  estimatedItemHeight: MODEL_PRICING_ROW_ESTIMATED_HEIGHT,
  overscan: MODEL_PRICING_OVERSCAN,
  gap: MODEL_PRICING_ROW_GAP,
}))

function syncListViewportMetrics() {
  const viewport = listViewportRef.value
  listScrollTop.value = viewport?.scrollTop ?? 0
  listViewportHeight.value = viewport?.clientHeight ?? 0
}

function scheduleVisibleItemMeasurement() {
  if (measureFrameId !== 0) return
  measureFrameId = window.requestAnimationFrame(() => {
    measureFrameId = 0
    if (visibleItemElements.size === 0) return

    const nextHeights = { ...measuredItemHeights.value }
    let changed = false

    for (const [model, element] of visibleItemElements.entries()) {
      const height = Math.ceil(element.getBoundingClientRect().height)
      if (height <= 0 || nextHeights[model] === height) continue
      nextHeights[model] = height
      changed = true
    }

    if (changed) {
      measuredItemHeights.value = nextHeights
    }
  })
}

function registerVisibleItemElement(model: string, element: Element | ComponentPublicInstance | null) {
  const htmlElement = element instanceof HTMLElement ? element : null
  if (htmlElement) {
    visibleItemElements.set(model, htmlElement)
    scheduleVisibleItemMeasurement()
    return
  }
  visibleItemElements.delete(model)
}

function resetListViewportPosition() {
  if (listViewportRef.value) {
    listViewportRef.value.scrollTop = 0
  }
  listScrollTop.value = 0
}

function reconcileMeasuredItemHeights(nextRows: ModelPricingRow[], resetAll = false) {
  if (resetAll) {
    measuredItemHeights.value = {}
    return
  }

  const activeModels = new Set(nextRows.map((row) => row.model))
  const currentHeights = measuredItemHeights.value
  let changed = false
  const nextHeights: Record<string, number> = {}

  for (const [model, height] of Object.entries(currentHeights)) {
    if (!activeModels.has(model)) {
      changed = true
      continue
    }
    nextHeights[model] = height
  }

  if (changed) {
    measuredItemHeights.value = nextHeights
  }
}

function handleListScroll() {
  if (scrollFrameId !== 0) return
  scrollFrameId = window.requestAnimationFrame(() => {
    scrollFrameId = 0
    syncListViewportMetrics()
    scheduleVisibleItemMeasurement()
  })
}

function findModelOffset(model: string) {
  let offset = 0
  for (let index = 0; index < filteredRows.value.length; index += 1) {
    const item = filteredRows.value[index]
    if (item.model === model) return offset
    const height = measuredItemHeights.value[item.model] ?? MODEL_PRICING_ROW_ESTIMATED_HEIGHT
    offset += height + (index < filteredRows.value.length - 1 ? MODEL_PRICING_ROW_GAP : 0)
  }
  return null
}

function scrollModelIntoView(model: string) {
  const viewport = listViewportRef.value
  if (!viewport || !model) return

  const targetOffset = findModelOffset(model)
  if (targetOffset == null) return

  const targetHeight = measuredItemHeights.value[model] ?? MODEL_PRICING_ROW_ESTIMATED_HEIGHT
  const currentTop = viewport.scrollTop
  const currentBottom = currentTop + viewport.clientHeight
  const targetBottom = targetOffset + targetHeight

  if (targetOffset >= currentTop && targetBottom <= currentBottom) return

  viewport.scrollTop = Math.max(0, targetOffset - MODEL_PRICING_ROW_GAP)
  syncListViewportMetrics()
  scheduleVisibleItemMeasurement()
}

async function loadRows(options: { force?: boolean; keepCurrentRows?: boolean } = {}) {
  const force = options.force === true
  const keepCurrentRows = options.keepCurrentRows === true

  if (!force && hasLoadedRows.value && !rowsStale.value) {
    await nextTick()
    syncListViewportMetrics()
    scheduleVisibleItemMeasurement()
    return
  }

  if (loadRowsTask) {
    await loadRowsTask
    return
  }

  loading.value = true
  if (!keepCurrentRows || rows.value.length === 0) {
    error.value = ''
  }

  loadRowsTask = (async () => {
    try {
      const nextRows = (await listModelPricing()) ?? []
      reconcileMeasuredItemHeights(nextRows, force)
      rows.value = nextRows
      warmupProviderIcons(nextRows)
      hasLoadedRows.value = true
      rowsStale.value = false
      error.value = ''
      await nextTick()
      syncListViewportMetrics()
      if (selectedModel.value) {
        scrollModelIntoView(selectedModel.value)
      }
      scheduleVisibleItemMeasurement()
    } catch (err) {
      const message = t('components.general.modelPricing.toast.loadFailed', { error: extractErrorMessage(err) })
      if (rows.value.length === 0) {
        error.value = message
      }
      showToast(message, 'error')
    } finally {
      loading.value = false
      loadRowsTask = null
    }
  })()

  await loadRowsTask
}

function openCreateModal() {
  editorMode.value = 'new'
  editorRow.value = null
  editorOpen.value = true
}

function openEditModal(row: ModelPricingRow) {
  selectedModel.value = row.model
  editorMode.value = 'edit'
  editorRow.value = row
  editorOpen.value = true
  syncMenuOpen.value = false
}

function resetClaudePreviewState() {
  previewRequestSeq.value += 1
  claudePreviewOpen.value = false
  claudePreviewRows.value = []
  claudePreviewFetchedAt.value = ''
}

function resetCloudConflictState() {
  cloudConflictRequestSeq.value += 1
  cloudConflictOpen.value = false
  cloudConflictRows.value = []
  cloudConflictFetchedAt.value = ''
}

function resetUIState() {
  search.value = ''
  selectedSourceFilter.value = 'all'
  selectedProvider.value = 'all'
  selectedModel.value = ''
  error.value = ''
  syncMenuOpen.value = false
  resetClaudePreviewState()
  resetCloudConflictState()
  resetListViewportPosition()
}

function closeModal() {
  editorOpen.value = false
  syncMenuOpen.value = false
  emit('close')
}

async function onSaved(model: string) {
  selectedModel.value = model
  editorOpen.value = false
  await loadRows({ force: true, keepCurrentRows: true })
}

async function onRemoved(model: string) {
  selectedModel.value = model
  editorOpen.value = false
  await loadRows({ force: true, keepCurrentRows: true })
}

function toggleSyncMenu() {
  if (syncing.value) return
  syncMenuOpen.value = !syncMenuOpen.value
}

async function runSyncTask(operation: () => Promise<void>) {
  if (syncing.value) return

  let task: Promise<void> | null = null
  task = (async () => {
    try {
      await operation()
    } finally {
      if (syncTask.value === task) {
        syncTask.value = null
      }
    }
  })()

  syncTask.value = task
  try {
    await task
  } catch {
    // 已在 task 内部统一 toast，这里吞掉避免重复报错链
  }
}

async function openClaudePreview() {
  if (syncing.value) return
  syncMenuOpen.value = false
  const requestSeq = previewRequestSeq.value + 1
  previewRequestSeq.value = requestSeq

  await runSyncTask(async () => {
    try {
      const result = await previewClaudeOfficialPricing()
      if (!props.open || requestSeq !== previewRequestSeq.value) return
      claudePreviewRows.value = result.rows ?? []
      claudePreviewFetchedAt.value = result.fetched_at ?? ''
      claudePreviewOpen.value = true
    } catch (err) {
      if (!props.open || requestSeq !== previewRequestSeq.value) return
      showToast(
        t('components.general.modelPricing.toast.previewFailed', {
          error: extractErrorMessage(err),
        }),
        'error',
      )
    }
  })
}

async function confirmClaudeSync() {
  if (syncing.value) return

  await runSyncTask(async () => {
    try {
      const result = await syncClaudeOfficialPricing()
      showToast(
        t('components.general.modelPricing.toast.syncSummary', {
          created: result.created_models,
          updated: result.updated_models,
          unchanged: result.unchanged_models,
        }),
      )
      if ((result.unrecognized_models ?? []).length > 0) {
        showToast(
          t('components.general.modelPricing.toast.syncUnrecognized', {
            models: (result.unrecognized_models ?? []).join(', '),
          }),
          'warning',
        )
      }
      claudePreviewOpen.value = false
      await loadRows({ force: true, keepCurrentRows: true })
    } catch (err) {
      showToast(
        t('components.general.modelPricing.toast.syncFailed', {
          error: extractErrorMessage(err),
        }),
        'error',
      )
    }
  })
}

async function executeCloudSync(overwriteManualModels: string[] = []) {
  const result = await syncCloudPriceTable(overwriteManualModels)
  showToast(
    t('components.general.modelPricing.toast.syncSummary', {
      created: result.created_models,
      updated: result.updated_models,
      unchanged: result.unchanged_models,
    }),
  )
  if ((result.skipped_manual_models ?? []).length > 0) {
    showToast(
      t('components.general.modelPricing.toast.syncSkippedManual', {
        count: result.skipped_manual_models?.length ?? 0,
      }),
      'warning',
    )
  }
  cloudConflictOpen.value = false
  await loadRows({ force: true, keepCurrentRows: true })
}

async function openCloudSync() {
  if (syncing.value) return
  syncMenuOpen.value = false
  const requestSeq = cloudConflictRequestSeq.value + 1
  cloudConflictRequestSeq.value = requestSeq

  await runSyncTask(async () => {
    try {
      const preview = await previewCloudPriceTableSyncConflicts()
      if (!props.open || requestSeq !== cloudConflictRequestSeq.value) return
      const conflicts = preview.conflicts ?? []
      if (conflicts.length === 0) {
        await executeCloudSync([])
        return
      }
      cloudConflictRows.value = conflicts
      cloudConflictFetchedAt.value = preview.fetched_at ?? ''
      cloudConflictOpen.value = true
    } catch (err) {
      if (!props.open || requestSeq !== cloudConflictRequestSeq.value) return
      showToast(
        t('components.general.modelPricing.toast.syncFailed', {
          error: extractErrorMessage(err),
        }),
        'error',
      )
    }
  })
}

async function confirmCloudConflictSync(models: string[]) {
  if (syncing.value) return

  await runSyncTask(async () => {
    try {
      await executeCloudSync(models)
    } catch (err) {
      showToast(
        t('components.general.modelPricing.toast.syncFailed', {
          error: extractErrorMessage(err),
        }),
        'error',
      )
    }
  })
}

function closeClaudePreview() {
  if (syncing.value) return
  claudePreviewOpen.value = false
}

function closeCloudConflict() {
  if (syncing.value) return
  cloudConflictOpen.value = false
}

function onGlobalPointerDown(event: PointerEvent) {
  if (!syncMenuOpen.value) return
  const target = event.target as Node | null
  if (!target) {
    syncMenuOpen.value = false
    return
  }
  if (syncMenuRef.value?.contains(target)) return
  if (syncTriggerRef.value?.contains(target)) return
  syncMenuOpen.value = false
}

function onWindowResize() {
  syncListViewportMetrics()
  scheduleVisibleItemMeasurement()
}

function handleModelPricingChanged() {
  rowsStale.value = true
  if (props.open) {
    void loadRows({ force: true, keepCurrentRows: true })
  }
}

onMounted(() => {
  document.addEventListener('pointerdown', onGlobalPointerDown)
  window.addEventListener('resize', onWindowResize)
  unsubscribeModelPricingChanged = Events.On(
    MODEL_PRICING_CHANGED_EVENT,
    handleModelPricingChanged as Events.WailsEventCallback,
  )
})

onBeforeUnmount(() => {
  document.removeEventListener('pointerdown', onGlobalPointerDown)
  window.removeEventListener('resize', onWindowResize)
  unsubscribeModelPricingChanged?.()
  unsubscribeModelPricingChanged = null
  visibleItemElements.clear()
  if (measureFrameId !== 0) {
    window.cancelAnimationFrame(measureFrameId)
    measureFrameId = 0
  }
  if (scrollFrameId !== 0) {
    window.cancelAnimationFrame(scrollFrameId)
    scrollFrameId = 0
  }
})

watch(
  () => props.open,
  (open) => {
    if (!open) {
      editorOpen.value = false
      resetUIState()
      return
    }

    resetUIState()
    void nextTick(() => {
      warmupProviderIcons(rows.value)
      syncListViewportMetrics()
      scheduleVisibleItemMeasurement()
    })

    if (!hasLoadedRows.value) {
      void loadRows()
      return
    }

    void loadRows({ force: true, keepCurrentRows: true })
  },
)

watch(
  providerTabs,
  (tabs) => {
    if (!tabs.some((tab) => tab.key === selectedProvider.value)) {
      selectedProvider.value = 'all'
    }
  },
)

watch(
  [search, selectedSourceFilter, selectedProvider],
  () => {
    resetListViewportPosition()
    void nextTick(() => {
      syncListViewportMetrics()
      scheduleVisibleItemMeasurement()
    })
  },
)
</script>

<template>
  <InlineModal
    :open="open"
    :body-scrollable="false"
    :title="$t('components.general.modelPricing.title')"
    :panel-width="'min(1280px, 98vw)'"
    @close="closeModal"
  >
    <div class="model-pricing-modal">
      <div class="model-pricing-toolbar">
        <div class="model-pricing-header">
          <div class="model-pricing-actions">
            <button type="button" class="action-btn" @click="openCreateModal">
              {{ $t('components.general.modelPricing.add') }}
            </button>
            <button
              type="button"
              class="action-btn"
              :disabled="loading"
              @click="loadRows({ force: true, keepCurrentRows: rows.length > 0 })"
            >
              {{ loading ? $t('components.general.modelPricing.loading') : $t('components.general.modelPricing.refresh') }}
            </button>
            <div class="sync-menu-anchor">
              <button
                ref="syncTriggerRef"
                type="button"
                class="action-btn"
                :disabled="syncing"
                @click.stop="toggleSyncMenu"
              >
                {{ syncing ? $t('components.general.modelPricing.syncing') : $t('components.general.modelPricing.sync') }}
              </button>
              <div v-if="syncMenuOpen" ref="syncMenuRef" class="sync-menu-card" @click.stop>
                <button
                  type="button"
                  class="sync-menu-btn"
                  :disabled="syncing"
                  @click="openClaudePreview"
                >
                  {{ $t('components.general.modelPricing.syncOptions.claude') }}
                </button>
                <button
                  type="button"
                  class="sync-menu-btn"
                  :disabled="syncing"
                  @click="openCloudSync"
                >
                  {{ $t('components.general.modelPricing.syncOptions.cloud') }}
                </button>
              </div>
            </div>
          </div>
        </div>

        <div class="model-pricing-search">
          <BaseInput
            v-model="search"
            type="text"
            :placeholder="$t('components.general.modelPricing.searchPlaceholder')"
          />
        </div>

        <div class="model-pricing-filters">
          <button
            type="button"
            class="vendor-pill"
            :class="{ active: selectedSourceFilter === 'all' }"
            @click="selectedSourceFilter = 'all'"
          >
            {{ $t('components.general.modelPricing.filterAll') }} ({{ rows.length }})
          </button>
          <button
            type="button"
            class="vendor-pill"
            :class="{ active: selectedSourceFilter === 'manual' }"
            @click="selectedSourceFilter = 'manual'"
          >
            <svg class="vendor-pill-icon vendor-pill-icon--stroke" viewBox="0 0 24 24" aria-hidden="true">
              <path
                d="M4.75 7.75 12 4.5l7.25 3.25V16.25L12 19.5 4.75 16.25V7.75Z"
                fill="none"
                stroke="currentColor"
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="1.6"
              />
              <path
                d="M4.75 7.75 12 11l7.25-3.25M12 11v8.5"
                fill="none"
                stroke="currentColor"
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="1.6"
              />
            </svg>
            <span>{{ $t('components.general.modelPricing.filterLocal') }} ({{ manualCount }})</span>
          </button>
        </div>

        <div v-if="providerTabs.length > 1" class="model-pricing-filters model-pricing-filters--vendors">
          <button
            v-for="tab in providerTabs"
            :key="tab.key"
            type="button"
            class="vendor-pill"
            :class="{ active: selectedProvider === tab.key }"
            @click="selectedProvider = tab.key"
          >
            <span
              v-if="tab.iconKey && providerIconSvg(tab.iconKey)"
              class="vendor-pill-icon"
              v-html="providerIconSvg(tab.iconKey)"
              aria-hidden="true"
            ></span>
            <span>{{ tab.label }} ({{ tab.count }})</span>
          </button>
        </div>
      </div>

      <div v-if="loading && rows.length === 0" class="model-pricing-state">
        {{ $t('components.general.modelPricing.loading') }}
      </div>
      <div v-else-if="error && rows.length === 0" class="model-pricing-state error">
        {{ error }}
      </div>
      <div v-else-if="filteredRows.length === 0" class="model-pricing-state">
        {{ $t('components.general.modelPricing.empty') }}
      </div>
      <div v-else class="model-pricing-list">
        <p class="pricing-hint">
          {{ $t('components.general.modelPricing.unitHint') }} · {{ $t('components.general.modelPricing.scrollHint') }}
        </p>

        <div
          ref="listViewportRef"
          class="model-pricing-viewport"
          @scroll="handleListScroll"
        >
          <div
            class="model-pricing-viewport-inner"
            :style="{ height: `${virtualRowsState.totalHeight}px` }"
          >
            <div
              v-for="virtualRow in virtualRowsState.items"
              :key="virtualRow.item.model"
              :ref="(element) => registerVisibleItemElement(virtualRow.item.model, element)"
              class="model-pricing-item model-pricing-item--virtual"
              :class="{ selected: virtualRow.item.model === selectedModel }"
              :style="{ top: `${virtualRow.top}px` }"
              @click="openEditModal(virtualRow.item.raw)"
            >
              <div class="model-main">
                <div class="model-name" :title="virtualRow.item.model">{{ virtualRow.item.model }}</div>
                <CloudPricingRulesSummary :rules="virtualRow.item.raw.cloud_pricing" @resize="scheduleVisibleItemMeasurement" />
                <div class="model-tags">
                  <span v-if="virtualRow.item.hasKnownProvider" class="tag tag-vendor">
                    <span
                      v-if="providerIconSvg(virtualRow.item.providerIconKey)"
                      class="tag-icon"
                      v-html="providerIconSvg(virtualRow.item.providerIconKey)"
                      aria-hidden="true"
                    ></span>
                    {{ virtualRow.item.providerLabel }}
                  </span>
                  <span
                    class="tag"
                    :class="virtualRow.item.sourceClass"
                    :title="virtualRow.item.sourceTooltip || undefined"
                  >
                    {{ virtualRow.item.sourceLabel }}
                  </span>
                </div>
              </div>

              <div class="pricing-inline-container" @pointerdown.stop @click.stop>
                <div class="model-pricing">
                  <div class="price-block">
                    <span class="price-label">{{ $t('components.general.modelPricing.columns.input') }}</span>
                    <span class="price-value input">{{ virtualRow.item.inputValueText }}</span>
                  </div>
                  <div class="price-block">
                    <span class="price-label">{{ $t('components.general.modelPricing.columns.output') }}</span>
                    <span class="price-value output">{{ virtualRow.item.outputValueText }}</span>
                  </div>
                  <div class="price-block">
                    <span class="price-label">{{ $t('components.general.modelPricing.columns.groupMultiplier') }}</span>
                    <span class="price-value">{{ virtualRow.item.groupMultiplierText }}</span>
                  </div>
                  <div
                    v-for="cacheItem in virtualRow.item.cacheCreateEntries"
                    :key="`${virtualRow.item.model}-${cacheItem.key}`"
                    class="price-block"
                  >
                    <span class="price-label">{{ cacheItem.label }}</span>
                    <span class="price-value cache-create">{{ cacheItem.valueText }}</span>
                    <span v-if="cacheItem.hint" class="price-note">
                      {{ cacheItem.hint }}
                    </span>
                  </div>
                  <div class="price-block">
                    <span class="price-label">{{ $t('components.general.modelPricing.columns.cacheRead') }}</span>
                    <span class="price-value cache-read">{{ virtualRow.item.cacheReadValueText }}</span>
                    <span v-if="virtualRow.item.cacheReadHint" class="price-note">
                      {{ virtualRow.item.cacheReadHint }}
                    </span>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <ModelPricingEditorModal
      :open="editorOpen"
      :mode="editorMode"
      :row="editorRow"
      :rows="rows"
      @close="editorOpen = false"
      @saved="onSaved"
      @removed="onRemoved"
    />

    <ClaudePricingPreviewModal
      :open="claudePreviewOpen"
      :rows="claudePreviewRows"
      :fetched-at="claudePreviewFetchedAt"
      :syncing="syncing"
      @close="closeClaudePreview"
      @confirm-sync="confirmClaudeSync"
    />

    <CloudPricingConflictModal
      :open="cloudConflictOpen"
      :rows="cloudConflictRows"
      :fetched-at="cloudConflictFetchedAt"
      :syncing="syncing"
      @close="closeCloudConflict"
      @confirm-sync="confirmCloudConflictSync"
    />
  </InlineModal>
</template>

<style scoped>
@import '../common/provider-model-list-shared.css';

.model-pricing-modal {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  gap: 16px;
}

.model-pricing-toolbar {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.model-pricing-header {
  display: flex;
  justify-content: flex-end;
  align-items: center;
}

.model-pricing-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}

.sync-menu-anchor {
  position: relative;
}

.sync-menu-card {
  position: absolute;
  right: 0;
  top: calc(100% + 6px);
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 150px;
  border: 1px solid var(--mac-border);
  border-radius: 10px;
  background: var(--mac-surface);
  box-shadow: 0 10px 24px rgba(15, 23, 42, 0.2);
  padding: 8px;
  z-index: 5;
}

.sync-menu-btn {
  display: flex !important;
  align-items: center !important;
  justify-content: center !important;
  width: 100%;
  min-width: 0 !important;
  border: 1px solid rgba(148, 163, 184, 0.32);
  border-radius: 8px;
  padding: 8px 12px !important;
  background: rgba(59, 130, 246, 0.1);
  color: var(--mac-text);
  cursor: pointer;
  text-align: center;
  line-height: 1.2 !important;
  transition: background 0.15s ease, border-color 0.15s ease;
}

.sync-menu-btn:hover:not(:disabled) {
  border-color: rgba(59, 130, 246, 0.35);
  background: rgba(59, 130, 246, 0.16);
}

.sync-menu-btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.model-pricing-filters {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.vendor-pill {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  border: 1px solid transparent;
  background: rgba(148, 163, 184, 0.12);
  color: var(--mac-text-secondary);
  padding: 6px 12px;
  border-radius: 999px;
  cursor: pointer;
  font-size: 0.85rem;
  transition: background 0.15s ease, border-color 0.15s ease, color 0.15s ease;
}

.vendor-pill:hover {
  background: rgba(148, 163, 184, 0.18);
}

.vendor-pill.active {
  border-color: rgba(59, 130, 246, 0.35);
  background: rgba(59, 130, 246, 0.14);
  color: var(--mac-text);
}

.vendor-pill-icon,
.tag-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: 0 0 auto;
}

.vendor-pill-icon {
  width: 16px;
  height: 16px;
}

.vendor-pill-icon--stroke {
  flex: 0 0 auto;
}

.tag-icon {
  width: 14px;
  height: 14px;
}

.vendor-pill-icon :deep(svg),
.tag-icon :deep(svg) {
  display: block;
  width: 100%;
  height: 100%;
}

.model-pricing-state {
  padding: 18px 12px;
  text-align: center;
  font-size: 0.92rem;
  color: var(--mac-text-secondary);
}

.model-pricing-state.error {
  color: #ef4444;
}

.model-pricing-list {
  display: flex;
  flex-direction: column;
  flex: 1 1 auto;
  gap: 10px;
  min-height: 0;
  overflow: hidden;
}

.model-pricing-viewport {
  flex: 1 1 auto;
  min-height: 0;
  overflow-y: auto;
  overflow-x: hidden;
  overscroll-behavior: contain;
  padding-right: 4px;
}

.model-pricing-viewport-inner {
  position: relative;
  min-height: 100%;
}

.pricing-hint {
  margin: 0;
  padding: 10px 12px;
  border-radius: 14px;
  border: 1px dashed rgba(148, 163, 184, 0.35);
  color: var(--mac-text-secondary);
  background: rgba(148, 163, 184, 0.08);
  font-size: 0.85rem;
  line-height: 1.5;
}

.model-pricing-item {
  --pricing-scroll-fade: var(--mac-surface-strong);
  border: 1px solid var(--mac-border);
  background: var(--mac-surface-strong);
  border-radius: 16px;
  padding: 14px 14px;
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 16px;
  align-items: start;
  cursor: pointer;
  transition: background 0.15s ease, border-color 0.15s ease;
  contain: layout paint;
}

.model-pricing-item--virtual {
  position: absolute;
  left: 0;
  right: 0;
}

.model-pricing-item:hover {
  --pricing-scroll-fade: var(--mac-surface-hover);
  background: var(--mac-surface-hover);
}

.model-pricing-item.selected {
  --pricing-scroll-fade: rgba(59, 130, 246, 0.12);
  border-color: rgba(59, 130, 246, 0.35);
  background: rgba(59, 130, 246, 0.12);
}

.model-main {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.model-name {
  font-weight: 600;
  color: var(--mac-text);
  font-size: 0.95rem;
  white-space: normal;
  overflow: visible;
  text-overflow: clip;
  word-break: break-word;
  overflow-wrap: anywhere;
  line-height: 1.4;
}

.model-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.tag {
  display: inline-flex;
  align-items: center;
  padding: 4px 10px;
  border-radius: 999px;
  font-size: 0.78rem;
  border: 1px solid transparent;
  white-space: nowrap;
}

.tag-vendor {
  gap: 6px;
  background: rgba(99, 102, 241, 0.1);
  color: var(--mac-text);
  border-color: rgba(99, 102, 241, 0.22);
}

.tag-builtin {
  background: rgba(148, 163, 184, 0.15);
  color: var(--mac-text-secondary);
  border-color: rgba(148, 163, 184, 0.32);
}

.tag-manual {
  background: rgba(245, 158, 11, 0.12);
  color: #f59e0b;
  border-color: rgba(245, 158, 11, 0.25);
}

.tag-claude-synced {
  background: rgba(16, 185, 129, 0.15);
  color: #10b981;
  border-color: rgba(16, 185, 129, 0.3);
}

.tag-cloud-synced {
  background: rgba(59, 130, 246, 0.14);
  color: #2563eb;
  border-color: rgba(59, 130, 246, 0.28);
}

.price-value.input {
  color: #2563eb;
}

.price-value.output {
  color: #16a34a;
}

.price-value.cache-create {
  color: #d97706;
}

.price-value.cache-read {
  color: #0f766e;
}

.model-pricing-item .pricing-inline-container {
  justify-self: end;
  width: fit-content;
  min-width: 0;
  max-width: 100%;
}

.model-pricing-item .pricing-inline-container::after {
  display: none;
}

.model-pricing-item .pricing-inline-container .model-pricing {
  width: fit-content;
  max-width: 100%;
  min-width: 0;
  justify-content: flex-end;
  overflow-x: auto;
  padding-right: 0;
  gap: 10px;
  overscroll-behavior-x: contain;
}

.model-pricing-item .pricing-inline-container .price-block {
  min-width: 88px;
}

@media (max-width: 720px) {
  .model-pricing-item {
    grid-template-columns: minmax(0, 1fr) auto;
  }
}

@media (max-width: 640px) {
  .model-pricing-item {
    grid-template-columns: 1fr;
  }

  .model-pricing {
    justify-content: flex-start;
  }
}
</style>

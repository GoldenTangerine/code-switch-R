<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import InlineModal from '../common/InlineModal.vue'
import BaseInput from '../common/BaseInput.vue'
import ModelPricingEditorModal from './ModelPricingEditorModal.vue'
import ClaudePricingPreviewModal from './ClaudePricingPreviewModal.vue'
import { useI18n } from 'vue-i18n'
import {
  listModelPricing,
  previewClaudeOfficialPricing,
  syncClaudeOfficialPricing,
  type ClaudeOfficialPricingPreviewRow,
  type ModelPricingRow,
} from '../../services/modelPricing'
import { extractErrorMessage } from '../../utils/error'
import { showToast } from '../../utils/toast'

const props = defineProps<{
  open: boolean
}>()

const emit = defineEmits<{
  (e: 'close'): void
}>()

const { t } = useI18n()

const loading = ref(false)
const error = ref('')

const rows = ref<ModelPricingRow[]>([])
const search = ref('')
const onlyOverrides = ref(false)
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

type EditMode = 'edit' | 'new'

const editorOpen = ref(false)
const editorMode = ref<EditMode>('edit')
const editorRow = ref<ModelPricingRow | null>(null)

const perTokenToPer1M = (value: number) => (Number.isFinite(value) ? value * 1_000_000 : 0)

const formatUsdPer1M = (value: number) => {
  if (!Number.isFinite(value)) return '—'
  const per1m = perTokenToPer1M(value)
  if (per1m === 0) return '$0'
  if (per1m < 0) return '—'
  if (per1m < 0.01) return `$${per1m.toFixed(6)}`
  if (per1m < 1) return `$${per1m.toFixed(4)}`
  return `$${per1m.toFixed(2)}`
}

const calculateMultiplier = (value: number, input: number) => {
  if (!Number.isFinite(value) || value < 0) return null
  if (!Number.isFinite(input) || input < 0) return null
  if (input === 0) return value === 0 ? 0 : null
  return value / input
}

const formatMultiplier = (value: number | null | undefined) => {
  if (typeof value !== 'number' || !Number.isFinite(value) || value < 0) return '—'
  if (Math.abs(value - Math.round(value)) < 1e-9) return `${Math.round(value)}x`
  if (value < 0.01) return `${value.toFixed(4)}x`
  return `${value.toFixed(2)}x`
}

const formatMultiplierHint = (value: number | null | undefined) => {
  const formatted = formatMultiplier(value)
  if (formatted === '—') return ''
  return `${t('components.general.modelPricing.columns.multiplier')} ${formatted}`
}

const resolveCacheCreateMultiplier = (row: ModelPricingRow) =>
  calculateMultiplier(row.cache_creation_input_token_cost, row.input_cost_per_token)

const resolveCacheReadMultiplier = (row: ModelPricingRow) =>
  calculateMultiplier(row.cache_read_input_token_cost, row.input_cost_per_token)

const resolveCacheCreateHint = (row: ModelPricingRow) =>
  formatMultiplierHint(resolveCacheCreateMultiplier(row))

const resolveCacheReadHint = (row: ModelPricingRow) =>
  formatMultiplierHint(resolveCacheReadMultiplier(row))

const overrideCount = computed(() => rows.value.filter((item) => item.is_override || item.is_custom).length)

const filteredRows = computed(() => {
  const keyword = search.value.trim().toLowerCase()
  const base = onlyOverrides.value ? rows.value.filter((item) => item.is_override || item.is_custom) : rows.value
  if (!keyword) return base
  return base.filter((item) => item.model.toLowerCase().includes(keyword))
})

const loadRows = async () => {
  loading.value = true
  error.value = ''
  try {
    rows.value = await listModelPricing()
  } catch (err) {
    const message = t('components.general.modelPricing.toast.loadFailed', { error: extractErrorMessage(err) })
    error.value = message
    showToast(message, 'error')
  } finally {
    loading.value = false
  }
}

const openCreateModal = () => {
  editorMode.value = 'new'
  editorRow.value = null
  editorOpen.value = true
}

const openEditModal = (row: ModelPricingRow) => {
  selectedModel.value = row.model
  editorMode.value = 'edit'
  editorRow.value = row
  editorOpen.value = true
  syncMenuOpen.value = false
}

const resetClaudePreviewState = () => {
  previewRequestSeq.value += 1
  claudePreviewOpen.value = false
  claudePreviewRows.value = []
  claudePreviewFetchedAt.value = ''
}

const resetUIState = () => {
  search.value = ''
  onlyOverrides.value = false
  selectedModel.value = ''
  error.value = ''
  syncMenuOpen.value = false
  resetClaudePreviewState()
}

const closeModal = () => {
  editorOpen.value = false
  syncMenuOpen.value = false
  emit('close')
}

const onSaved = async (model: string) => {
  selectedModel.value = model
  editorOpen.value = false
  await loadRows()
}

const onRemoved = async (model: string) => {
  selectedModel.value = model
  editorOpen.value = false
  await loadRows()
}

const resolvePricingSource = (row: ModelPricingRow) => {
  if (row.source === 'claude_sync') return 'claude_sync'
  if (row.source === 'manual') return 'manual'
  if (row.source === 'builtin') return 'builtin'
  return row.is_override || row.is_custom ? 'manual' : 'builtin'
}

const resolvePricingSourceLabel = (row: ModelPricingRow) => {
  const source = resolvePricingSource(row)
  if (source === 'claude_sync') return t('components.general.modelPricing.badge.synced')
  if (source === 'builtin') return t('components.general.modelPricing.badge.builtin')
  return t('components.general.modelPricing.badge.manual')
}

const resolvePricingSourceClass = (row: ModelPricingRow) => {
  const source = resolvePricingSource(row)
  if (source === 'claude_sync') return 'tag-synced'
  if (source === 'builtin') return 'tag-builtin'
  return 'tag-manual'
}

const formatDateTimeForTooltip = (raw: string | undefined) => {
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

const resolvePricingSourceTooltip = (row: ModelPricingRow) => {
  if (resolvePricingSource(row) !== 'claude_sync') return ''
  const formatted = formatDateTimeForTooltip(row.source_updated_at)
  if (!formatted) return ''
  return t('components.general.modelPricing.badge.syncedAtTooltip', { time: formatted })
}

const toggleSyncMenu = () => {
  if (syncing.value) return
  syncMenuOpen.value = !syncMenuOpen.value
}

const runSyncTask = async (operation: () => Promise<void>) => {
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

const openClaudePreview = async () => {
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

const confirmClaudeSync = async () => {
  if (syncing.value) return

  await runSyncTask(async () => {
    try {
      const result = await syncClaudeOfficialPricing()
      showToast(
        t('components.general.modelPricing.toast.syncSummary', {
          changed: result.changed_models,
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
      await loadRows()
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

const closeClaudePreview = () => {
  if (syncing.value) return
  claudePreviewOpen.value = false
}

const onGlobalPointerDown = (event: PointerEvent) => {
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

onMounted(() => {
  document.addEventListener('pointerdown', onGlobalPointerDown)
})

onBeforeUnmount(() => {
  document.removeEventListener('pointerdown', onGlobalPointerDown)
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
    // 每次打开都刷新一次，避免后端价格表更新后前端显示滞后
    void loadRows()
  },
)
</script>

<template>
  <InlineModal
    :open="open"
    :title="$t('components.general.modelPricing.title')"
    @close="closeModal"
  >
    <div class="model-pricing-modal">
      <div class="model-pricing-toolbar">
        <div class="model-pricing-header">
          <div class="model-pricing-actions">
            <button type="button" class="action-btn" @click="openCreateModal">
              {{ $t('components.general.modelPricing.add') }}
            </button>
            <button type="button" class="action-btn" :disabled="loading" @click="loadRows">
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
                  Claude
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
            :class="{ active: !onlyOverrides }"
            @click="onlyOverrides = false"
          >
            {{ $t('components.general.modelPricing.filterAll') }} ({{ rows.length }})
          </button>
          <button
            type="button"
            class="vendor-pill"
            :class="{ active: onlyOverrides }"
            @click="onlyOverrides = true"
          >
            {{ $t('components.general.modelPricing.onlyOverrides') }} ({{ overrideCount }})
          </button>
        </div>
      </div>

      <div v-if="loading && rows.length === 0" class="model-pricing-state">
        {{ $t('components.general.modelPricing.loading') }}
      </div>
      <div v-else-if="error" class="model-pricing-state error">
        {{ error }}
      </div>
      <div v-else-if="filteredRows.length === 0" class="model-pricing-state">
        {{ $t('components.general.modelPricing.empty') }}
      </div>
      <div v-else class="model-pricing-list">
        <p class="pricing-hint">
          {{ $t('components.general.modelPricing.unitHint') }}
        </p>

        <div
          v-for="item in filteredRows"
          :key="item.model"
          class="model-pricing-item"
          :class="{ selected: item.model === selectedModel }"
          @click="openEditModal(item)"
        >
          <div class="model-main">
            <div class="model-name">{{ item.model }}</div>
            <div class="model-tags">
              <span
                class="tag"
                :class="resolvePricingSourceClass(item)"
                :title="resolvePricingSourceTooltip(item) || undefined"
              >
                {{ resolvePricingSourceLabel(item) }}
              </span>
            </div>
          </div>

          <div class="model-pricing">
            <div class="price-block">
              <span class="price-label">{{ $t('components.general.modelPricing.columns.input') }}</span>
              <span class="price-value input">{{ formatUsdPer1M(item.input_cost_per_token) }}/M</span>
            </div>
            <div class="price-block">
              <span class="price-label">{{ $t('components.general.modelPricing.columns.output') }}</span>
              <span class="price-value output">{{ formatUsdPer1M(item.output_cost_per_token) }}/M</span>
            </div>
            <div class="price-block">
              <span class="price-label">{{ $t('components.general.modelPricing.columns.cacheCreate') }}</span>
              <span class="price-value cache-create">{{ formatUsdPer1M(item.cache_creation_input_token_cost) }}/M</span>
              <span v-if="resolveCacheCreateHint(item)" class="price-note">
                {{ resolveCacheCreateHint(item) }}
              </span>
            </div>
            <div class="price-block">
              <span class="price-label">{{ $t('components.general.modelPricing.columns.cacheRead') }}</span>
              <span class="price-value cache-read">{{ formatUsdPer1M(item.cache_read_input_token_cost) }}/M</span>
              <span v-if="resolveCacheReadHint(item)" class="price-note">
                {{ resolveCacheReadHint(item) }}
              </span>
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
  </InlineModal>
</template>

<style scoped>
.model-pricing-modal {
  display: flex;
  flex-direction: column;
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
  min-width: 150px;
  border: 1px solid var(--mac-border);
  border-radius: 10px;
  background: var(--mac-surface);
  box-shadow: 0 10px 24px rgba(15, 23, 42, 0.2);
  padding: 8px;
  z-index: 5;
}

.sync-menu-btn {
  width: 100%;
  border: 1px solid rgba(148, 163, 184, 0.32);
  border-radius: 8px;
  padding: 7px 10px;
  background: rgba(59, 130, 246, 0.1);
  color: var(--mac-text);
  cursor: pointer;
  text-align: left;
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
  gap: 10px;
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
  border: 1px solid var(--mac-border);
  background: var(--mac-surface-strong);
  border-radius: 16px;
  padding: 14px 14px;
  display: grid;
  grid-template-columns: minmax(150px, 1fr) minmax(0, 1.8fr);
  gap: 16px;
  align-items: start;
  cursor: pointer;
  transition: background 0.15s ease, border-color 0.15s ease;
}

.model-pricing-item:hover {
  background: var(--mac-surface-hover);
}

.model-pricing-item.selected {
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
  word-break: break-all;
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

.tag-synced {
  background: rgba(16, 185, 129, 0.15);
  color: #10b981;
  border-color: rgba(16, 185, 129, 0.3);
}

.model-pricing {
  display: flex;
  flex-wrap: wrap;
  width: 100%;
  gap: 12px;
  justify-content: flex-start;
  align-items: flex-start;
}

.price-block {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  justify-content: flex-start;
  gap: 6px;
  flex: 0 0 96px;
  min-height: 86px;
  padding: 10px 12px;
  border-radius: 14px;
  border: 1px solid rgba(148, 163, 184, 0.16);
  background: rgba(15, 23, 42, 0.18);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.02);
}

.price-label {
  font-size: 0.76rem;
  color: var(--mac-text-secondary);
  line-height: 1.2;
  white-space: normal;
}

.price-value {
  font-size: 1rem;
  font-weight: 700;
  line-height: 1.25;
  color: var(--mac-text);
  white-space: nowrap;
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

.price-note {
  font-size: 0.74rem;
  line-height: 1.35;
  color: var(--mac-text-secondary);
  white-space: normal;
}

@media (max-width: 860px) {
  .price-block {
    flex-basis: 92px;
    min-height: 82px;
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

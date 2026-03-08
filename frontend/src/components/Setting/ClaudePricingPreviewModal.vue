<template>
  <InlineModal
    :open="open"
    :body-scrollable="false"
    :title="t('components.general.modelPricing.preview.title')"
    :panel-width="'min(1280px, 98vw)'"
    @close="emit('close')"
  >
    <div class="provider-model-modal">
      <div class="preview-scroll">
        <div class="provider-model-toolbar">
          <div class="provider-model-meta">
            <span class="meta-pill">
              {{ t('components.general.modelPricing.preview.fetchedAt') }}：{{ formattedFetchedAt }}
            </span>
            <span class="meta-pill">
              {{ t('components.general.modelPricing.preview.total') }}：{{ rows.length }}
            </span>
            <span class="meta-pill">
              {{ t('components.general.modelPricing.preview.mapped') }}：{{ mappedCount }}
            </span>
            <span v-if="unmappedCount > 0" class="meta-pill">
              {{ t('components.general.modelPricing.preview.unmapped') }}：{{ unmappedCount }}
            </span>
          </div>

          <div class="provider-model-search">
            <BaseInput
              v-model="searchTerm"
              type="text"
              :placeholder="t('components.general.modelPricing.preview.searchPlaceholder')"
            />
          </div>

          <div class="provider-model-vendors">
            <button
              v-for="tab in previewTabs"
              :key="tab.key"
              type="button"
              class="vendor-pill"
              :class="{ active: selectedFilter === tab.key }"
              @click="selectedFilter = tab.key"
            >
              {{ tab.label }} ({{ tab.count }})
            </button>
          </div>
        </div>

        <div v-if="filteredRows.length === 0" class="provider-model-state">
          {{ t('components.general.modelPricing.preview.empty') }}
        </div>
        <div v-else class="provider-model-list">
          <p class="pricing-hint">
            {{ t('components.general.modelPricing.preview.hint') }}
          </p>

          <div
            v-for="(model, index) in filteredRows"
            :key="`${model.display_name}::${(model.target_models ?? []).join('|')}::${index}`"
            class="provider-model-item"
          >
            <div class="model-main">
              <div class="model-name" :title="model.display_name">{{ model.display_name }}</div>
              <div class="model-tags">
                <span class="tag" :class="model.is_recognized ? 'tag-token' : 'tag-neutral'">
                  {{ model.is_recognized ? t('components.general.modelPricing.preview.mapped') : t('components.general.modelPricing.preview.unmapped') }}
                </span>
                <span v-if="(model.target_models ?? []).length > 0" class="tag tag-neutral tag-wrap">
                  {{ t('components.general.modelPricing.preview.mappingHint', { models: summarizeTargetModels(model) }) }}
                </span>
                <span v-else class="tag tag-neutral">
                  {{ t('components.general.modelPricing.preview.mappingMissing') }}
                </span>
              </div>
            </div>

            <div class="model-pricing">
              <div class="price-block">
                <span class="price-label">{{ t('components.general.modelPricing.preview.input') }}</span>
                <span class="price-value input">{{ formatUsdPer1M(model.input_cost_per_token) }}/M</span>
              </div>
              <div class="price-block">
                <span class="price-label">{{ t('components.general.modelPricing.preview.output') }}</span>
                <span class="price-value output">{{ formatUsdPer1M(model.output_cost_per_token) }}/M</span>
              </div>
              <div class="price-block">
                <span class="price-label">{{ t('components.general.modelPricing.preview.cacheHit') }}</span>
                <span class="price-value cache-hit">{{ formatUsdPer1M(model.cache_read_input_token_cost) }}/M</span>
              </div>
              <div class="price-block">
                <span class="price-label">{{ t('components.general.modelPricing.preview.cacheWrite5m') }}</span>
                <span class="price-value cache-write">{{ formatUsdPer1M(model.cache_creation_input_token_cost) }}/M</span>
              </div>
              <div class="price-block">
                <span class="price-label">{{ t('components.general.modelPricing.preview.cacheWrite1h') }}</span>
                <span class="price-value cache-write-1h">{{ formatUsdPer1M(model.ephemeral_1h_cost_per_token) }}/M</span>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div class="preview-actions">
        <button
          type="button"
          class="action-btn secondary"
          :disabled="syncing"
          @click="emit('close')"
        >
          {{ t('components.general.modelPricing.preview.close') }}
        </button>
        <button
          type="button"
          class="action-btn sync-btn"
          :disabled="syncing || rows.length === 0"
          @click="emit('confirm-sync')"
        >
          {{ syncing ? t('components.general.modelPricing.syncing') : t('components.general.modelPricing.preview.applySync') }}
        </button>
      </div>
    </div>
  </InlineModal>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import InlineModal from '../common/InlineModal.vue'
import BaseInput from '../common/BaseInput.vue'
import type { ClaudeOfficialPricingPreviewRow } from '../../services/modelPricing'

type PreviewFilter = 'all' | 'mapped' | 'unmapped'

const props = defineProps<{
  open: boolean
  rows: ClaudeOfficialPricingPreviewRow[]
  fetchedAt: string
  syncing: boolean
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'confirm-sync'): void
}>()

const { t } = useI18n()

const searchTerm = ref('')
const selectedFilter = ref<PreviewFilter>('all')

const mappedCount = computed(() => props.rows.filter((row) => row.is_recognized).length)
const unmappedCount = computed(() => props.rows.length - mappedCount.value)

const previewTabs = computed(() => [
  {
    key: 'all' as const,
    label: t('components.general.modelPricing.preview.all'),
    count: props.rows.length,
  },
  {
    key: 'mapped' as const,
    label: t('components.general.modelPricing.preview.onlyMapped'),
    count: mappedCount.value,
  },
  {
    key: 'unmapped' as const,
    label: t('components.general.modelPricing.preview.onlyUnmapped'),
    count: unmappedCount.value,
  },
])

const filteredRows = computed(() => {
  const term = searchTerm.value.trim().toLowerCase()
  return props.rows.filter((row) => {
    if (selectedFilter.value === 'mapped' && !row.is_recognized) return false
    if (selectedFilter.value === 'unmapped' && row.is_recognized) return false

    if (!term) return true

    const targetModels = (row.target_models ?? []).join(' ').toLowerCase()
    return row.display_name.toLowerCase().includes(term) || targetModels.includes(term)
  })
})

const formattedFetchedAt = computed(() => {
  const raw = (props.fetchedAt || '').trim()
  if (!raw) return '-'

  const parsed = new Date(raw)
  if (Number.isNaN(parsed.getTime())) return raw
  return parsed.toLocaleString()
})

const formatUsdPer1M = (perToken: number) => {
  if (!Number.isFinite(perToken)) return '—'
  const per1M = perToken * 1_000_000
  if (per1M < 0) return '—'
  if (per1M === 0) return '$0'
  if (per1M < 0.01) return `$${per1M.toFixed(6)}`
  if (per1M < 1) return `$${per1M.toFixed(4)}`
  return `$${per1M.toFixed(2)}`
}

const summarizeTargetModels = (row: ClaudeOfficialPricingPreviewRow) => {
  const targets = row.target_models ?? []
  if (targets.length <= 2) return targets.join(', ')
  return `${targets.slice(0, 2).join(', ')} +${targets.length - 2}`
}

watch(
  () => props.open,
  (open) => {
    if (!open) return
    searchTerm.value = ''
    selectedFilter.value = 'all'
  },
)
</script>

<style scoped>
@import '../common/provider-model-list-shared.css';

.provider-model-modal {
  height: 100%;
  min-height: 0;
  gap: 12px;
}

.provider-model-item {
  grid-template-columns: minmax(320px, 1.35fr) minmax(0, 1.65fr);
  align-items: start;
  gap: 16px;
}

.provider-model-item .model-name {
  white-space: normal;
  overflow: visible;
  text-overflow: clip;
  word-break: break-word;
  overflow-wrap: anywhere;
  line-height: 1.4;
}

.preview-scroll {
  flex: 1 1 auto;
  min-height: 0;
  overflow-y: auto;
  padding-right: 2px;
}

.tag-wrap {
  white-space: normal;
  line-height: 1.4;
}

.price-value.cache-hit {
  color: #9333ea;
}

.price-value.cache-write {
  color: #d97706;
}

.price-value.cache-write-1h {
  color: #ea580c;
}

.preview-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 10px;
  margin-top: auto;
  padding-top: 12px;
  border-top: 1px solid var(--mac-border);
  background: var(--mac-surface);
}

@media (max-width: 720px) {
  .provider-model-item {
    grid-template-columns: minmax(240px, 1fr) minmax(0, 1.4fr);
  }
}

.action-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: max-content;
  max-width: 100%;
  min-width: 88px;
  border: 1px solid rgba(59, 130, 246, 0.35);
  background: rgba(59, 130, 246, 0.12);
  color: var(--mac-text);
  border-radius: 10px;
  padding: 8px 14px;
  min-height: 34px;
  font-size: 0.85rem;
  line-height: 1.2;
  white-space: nowrap;
  flex-shrink: 0;
  box-sizing: border-box;
  cursor: pointer;
  transition: background 0.15s ease, border-color 0.15s ease;
}

.sync-btn {
  min-width: 182px;
}

.action-btn:hover:not(:disabled) {
  background: rgba(59, 130, 246, 0.18);
  border-color: rgba(59, 130, 246, 0.45);
}

.action-btn.secondary {
  border-color: var(--mac-border);
  background: var(--mac-surface-strong);
}

.action-btn.secondary:hover:not(:disabled) {
  background: var(--mac-surface-hover);
}

.action-btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

@media (max-width: 640px) {
  .preview-actions {
    justify-content: flex-end;
  }

  .preview-actions .action-btn {
    flex: 0 0 auto;
  }

  .preview-actions .sync-btn {
    min-width: 168px;
  }
}
</style>

<template>
  <BaseModal :open="open" :title="t('components.logs.tokenDetail.title')" @close="emit('close')">
    <div class="token-detail-modal">
      <div class="token-detail-list">
        <div class="token-detail-item">
          <span class="token-detail-item__name">{{ t('components.logs.tokenLabels.input') }}</span>
          <span class="token-detail-item__value">{{ formatTokenNumber(stats?.input_tokens) }}</span>
        </div>
        <div class="token-detail-item">
          <span class="token-detail-item__name">{{ t('components.logs.tokenLabels.output') }}</span>
          <span class="token-detail-item__value">{{ formatTokenNumber(stats?.output_tokens) }}</span>
        </div>
        <div class="token-detail-item">
          <span class="token-detail-item__name">{{ t('components.logs.tokenLabels.cacheRead') }}</span>
          <span class="token-detail-item__value">{{ formatTokenNumber(stats?.cache_read_tokens) }}</span>
        </div>
      </div>
    </div>
  </BaseModal>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import BaseModal from '../../common/BaseModal.vue'
import type { LogStats } from '../../../services/logs'

defineProps<{
  open: boolean
  stats: LogStats | null
  formatTokenNumber: (value?: number) => string
}>()

const emit = defineEmits<{
  (event: 'close'): void
}>()

const { t } = useI18n()
</script>

<style scoped>
.token-detail-modal {
  min-height: 80px;
}

.token-detail-list {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.token-detail-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0.75rem 1rem;
  background: rgba(148, 163, 184, 0.08);
  border-radius: 8px;
  transition: background 0.15s ease;
}

.token-detail-item:hover {
  background: rgba(148, 163, 184, 0.12);
}

html.dark .token-detail-item {
  background: rgba(148, 163, 184, 0.12);
}

html.dark .token-detail-item:hover {
  background: rgba(148, 163, 184, 0.18);
}

.token-detail-item__name {
  font-weight: 500;
  color: #1e293b;
}

html.dark .token-detail-item__name {
  color: #f1f5f9;
}

.token-detail-item__value {
  font-weight: 600;
  color: #34d399;
  font-variant-numeric: tabular-nums;
}
</style>

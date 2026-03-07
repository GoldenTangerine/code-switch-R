<template>
  <BaseModal :open="open" :title="t('components.logs.costDetail.title')" @close="emit('close')">
    <div class="cost-detail-modal">
      <p v-if="loading" class="cost-detail-loading">
        {{ t('components.logs.loading') }}
      </p>
      <div v-else-if="data.length === 0" class="cost-detail-empty">
        {{ t('components.logs.costDetail.empty') }}
      </div>
      <ul v-else class="cost-detail-list">
        <li v-for="item in data" :key="item.provider" class="cost-detail-item">
          <span class="cost-detail-item__name">{{ item.provider }}</span>
          <span class="cost-detail-item__value">{{ formatCurrency(item.cost_total) }}</span>
        </li>
      </ul>
    </div>
  </BaseModal>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import BaseModal from '../../common/BaseModal.vue'
import type { ProviderDailyStat } from '../../../services/logs'

defineProps<{
  open: boolean
  loading: boolean
  data: ProviderDailyStat[]
  formatCurrency: (value?: number) => string
}>()

const emit = defineEmits<{
  (event: 'close'): void
}>()

const { t } = useI18n()
</script>

<style scoped>
.cost-detail-modal {
  min-height: 120px;
}

.cost-detail-loading,
.cost-detail-empty {
  text-align: center;
  color: #64748b;
  padding: 2rem 0;
}

html.dark .cost-detail-loading,
html.dark .cost-detail-empty {
  color: #94a3b8;
}

.cost-detail-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.cost-detail-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0.75rem 1rem;
  background: rgba(148, 163, 184, 0.08);
  border-radius: 8px;
  transition: background 0.15s ease;
}

.cost-detail-item:hover {
  background: rgba(148, 163, 184, 0.12);
}

html.dark .cost-detail-item {
  background: rgba(148, 163, 184, 0.12);
}

html.dark .cost-detail-item:hover {
  background: rgba(148, 163, 184, 0.18);
}

.cost-detail-item__name {
  font-weight: 500;
  color: #1e293b;
}

html.dark .cost-detail-item__name {
  color: #f1f5f9;
}

.cost-detail-item__value {
  font-weight: 600;
  color: #f97316;
  font-variant-numeric: tabular-nums;
}
</style>

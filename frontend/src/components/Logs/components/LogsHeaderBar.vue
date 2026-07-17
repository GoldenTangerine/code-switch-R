<template>
  <div class="logs-header-bar">
    <div class="logs-header">
      <BaseButton variant="outline" type="button" @click="emit('back')">
        {{ t('components.logs.back') }}
      </BaseButton>
      <div class="refresh-indicator">
        <BaseButton variant="outline" size="sm" :disabled="storageLoading" @click="emit('open-storage')">
          {{ t('components.logs.storage.title') }}
        </BaseButton>
        <label class="refresh-interval-control">
          <span class="sr-only">{{ t('components.logs.refreshInterval') }}</span>
          <select
            :value="refreshInterval"
            :aria-label="t('components.logs.refreshInterval')"
            :title="t('components.logs.refreshInterval')"
            :disabled="refreshSaving"
            @change="handleRefreshIntervalChange"
          >
            <option v-for="seconds in refreshOptions" :key="seconds" :value="seconds">
              {{ seconds === 0 ? t('components.logs.refreshOff') : t('components.logs.refreshSeconds', { seconds }) }}
            </option>
          </select>
        </label>
        <span v-if="refreshInterval > 0">{{ t('components.logs.nextRefresh', { seconds: countdown }) }}</span>
        <span v-else>{{ t('components.logs.refreshPaused') }}</span>
        <BaseButton size="sm" :disabled="loading" @click="emit('refresh')">
          {{ t('components.logs.refresh') }}
        </BaseButton>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import BaseButton from '../../common/BaseButton.vue'
import type { LogsRefreshIntervalSeconds } from '../../../services/appSettings'

const props = defineProps<{
  countdown: number
  loading: boolean
  storageLoading: boolean
  refreshInterval: LogsRefreshIntervalSeconds
  refreshOptions: readonly LogsRefreshIntervalSeconds[]
  refreshSaving: boolean
}>()

const emit = defineEmits<{
  (event: 'back'): void
  (event: 'open-storage'): void
  (event: 'refresh'): void
  (event: 'update:refresh-interval', value: LogsRefreshIntervalSeconds): void
}>()

const { t } = useI18n()

const handleRefreshIntervalChange = (event: Event) => {
  const value = Number((event.target as HTMLSelectElement).value)
  if (!props.refreshOptions.includes(value as LogsRefreshIntervalSeconds)) return
  emit('update:refresh-interval', value as LogsRefreshIntervalSeconds)
}
</script>

<style scoped>
.refresh-indicator {
  flex-wrap: wrap;
  justify-content: flex-end;
}

.refresh-interval-control select {
  min-height: 34px;
  border: 1px solid var(--mac-border);
  border-radius: 7px;
  padding: 0 28px 0 10px;
  background: var(--mac-surface);
  color: var(--mac-text);
  font: inherit;
  cursor: pointer;
}

.refresh-interval-control select:focus-visible {
  outline: 2px solid rgba(59, 130, 246, 0.55);
  outline-offset: 2px;
}

.refresh-interval-control select:disabled {
  cursor: wait;
  opacity: 0.6;
}

.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border: 0;
}

@media (max-width: 640px) {
  .refresh-indicator {
    width: 100%;
    justify-content: flex-start;
  }

  .refresh-interval-control,
  .refresh-interval-control select {
    width: 100%;
  }
}
</style>

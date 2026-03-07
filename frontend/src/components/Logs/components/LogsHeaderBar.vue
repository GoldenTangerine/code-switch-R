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
        <span>{{ t('components.logs.nextRefresh', { seconds: countdown }) }}</span>
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

defineProps<{
  countdown: number
  loading: boolean
  storageLoading: boolean
}>()

const emit = defineEmits<{
  (event: 'back'): void
  (event: 'open-storage'): void
  (event: 'refresh'): void
}>()

const { t } = useI18n()
</script>

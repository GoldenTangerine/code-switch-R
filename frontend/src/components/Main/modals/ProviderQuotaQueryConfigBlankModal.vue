<template>
  <InlineModal
    :open="open"
    :title="t('components.main.form.labels.providerQuotaQueryConfig')"
    :body-scrollable="true"
    :panel-width="'min(560px, 94vw)'"
    @close="$emit('close')"
  />
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import InlineModal from '../../common/InlineModal.vue'
import type { ProviderQuotaQueryConfig } from '../../../utils/providerQuotaQuery'

defineProps<{
  open: boolean
  modelValue?: ProviderQuotaQueryConfig
  providerApiUrl?: string
  providerApiKey?: string
}>()

defineEmits<{
  close: []
  save: [config: ProviderQuotaQueryConfig]
}>()

const { t } = useI18n()
</script>

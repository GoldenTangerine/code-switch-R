<template>
  <BaseModal
    :open="open"
    :title="t('components.main.customCli.deleteTitle')"
    variant="confirm"
    @close="$emit('close')"
  >
    <div class="confirm-body">
      <p>{{ t('components.main.customCli.deleteMessage', { name: tool?.name ?? '' }) }}</p>
    </div>
    <footer class="form-actions confirm-actions">
      <BaseButton variant="outline" type="button" @click="$emit('close')">
        {{ t('components.main.form.actions.cancel') }}
      </BaseButton>
      <BaseButton variant="danger" type="button" @click="$emit('confirm')">
        {{ t('components.main.form.actions.delete') }}
      </BaseButton>
    </footer>
  </BaseModal>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import BaseButton from '../../common/BaseButton.vue'
import BaseModal from '../../common/BaseModal.vue'
import type { CustomCliTool } from '../../../services/customCliService'

defineProps<{
  open: boolean
  tool: CustomCliTool | null
}>()

defineEmits<{
  close: []
  confirm: []
}>()

const { t } = useI18n()
</script>

<!--
 * @name: 未保存修改确认弹窗
 * @Descripttion: 离开有未保存修改的面板前弹出的统一确认弹窗，确认后由父组件执行放弃修改并离开
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-17 00:00:00
 * @LastEditTime: 2026-08-17 00:00:00
 * @FilePath: frontend/src/components/common/UnsavedConfirmModal.vue
-->
<template>
  <BaseModal :open="open" :title="title" variant="confirm" @close="emit('close')">
    <div class="confirm-body">
      <p>{{ message }}</p>
    </div>
    <footer class="form-actions confirm-actions">
      <BaseButton variant="outline" type="button" @click="emit('close')">
        {{ stayLabel }}
      </BaseButton>
      <BaseButton variant="danger" type="button" @click="emit('confirm')">
        {{ leaveLabel }}
      </BaseButton>
    </footer>
  </BaseModal>
</template>

<script setup lang="ts">
import BaseButton from './BaseButton.vue'
import BaseModal from './BaseModal.vue'

withDefaults(
  defineProps<{
    open: boolean
    title: string
    message: string
    stayLabel?: string
    leaveLabel?: string
  }>(),
  { stayLabel: '', leaveLabel: '' },
)

const emit = defineEmits<{ (e: 'close'): void; (e: 'confirm'): void }>()
</script>

<style scoped>
.confirm-body {
  color: var(--mac-text-secondary);
  font-size: 13px;
  line-height: 1.6;
}

.confirm-body p {
  margin: 0;
}
</style>

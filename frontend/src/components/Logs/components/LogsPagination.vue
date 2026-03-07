<template>
  <div class="logs-pagination">
    <span>{{ page }} / {{ totalPages }}</span>
    <div class="pagination-actions">
      <BaseButton variant="outline" size="sm" :disabled="page === 1 || loading" @click="emit('prev')">
        ‹
      </BaseButton>
      <BaseButton variant="outline" size="sm" :disabled="page >= totalPages || loading" @click="emit('next')">
        ›
      </BaseButton>
    </div>
  </div>
</template>

<script setup lang="ts">
import BaseButton from '../../common/BaseButton.vue'

defineProps<{
  page: number
  totalPages: number
  loading: boolean
}>()

const emit = defineEmits<{
  (event: 'prev'): void
  (event: 'next'): void
}>()
</script>

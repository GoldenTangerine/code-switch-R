<script setup lang="ts">
defineProps<{
   label: string
   subLabel?: string
 }>()
</script>

<template>
  <div class="mac-list-row">
    <div class="mac-list-text">
      <div class="mac-list-label">{{ label }}</div>
      <div v-if="subLabel" class="mac-list-sublabel">{{ subLabel }}</div>
    </div>
    <div class="mac-list-control">
      <slot />
    </div>
  </div>
</template>

<style scoped>
.mac-list-row {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-start;
  justify-content: space-between;
  padding: 14px 18px;
  gap: 16px;
}

.mac-list-text {
  text-align: left;
  flex: 1 1 auto;
  min-width: 160px;
}

.mac-list-label {
  font-size: 0.95rem;
  font-weight: 600;
  color: var(--mac-text);
  overflow-wrap: anywhere;
}

.mac-list-sublabel {
  font-size: 0.8rem;
  color: var(--mac-text-secondary);
  margin-top: 4px;
}

.mac-list-control {
  display: inline-flex;
  align-items: center;
  justify-content: flex-end;
  max-width: 100%;
  min-width: 0;
  margin-left: auto;
}
</style>

<template>
  <div class="cli-tool-selector">
    <div class="tool-selector-row">
      <select
        :value="selectedToolId ?? ''"
        class="tool-select"
        @change="handleChange"
      >
        <option v-if="customCliTools.length === 0" value="" disabled>
          {{ t('components.main.customCli.noTools') }}
        </option>
        <option
          v-for="tool in customCliTools"
          :key="tool.id"
          :value="tool.id"
        >
          {{ tool.name }}
        </option>
      </select>
      <button
        class="ghost-icon add-tool-btn"
        :data-tooltip="t('components.main.customCli.addTool')"
        type="button"
        @click="$emit('create')"
      >
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <path
            d="M12 5v14M5 12h14"
            stroke="currentColor"
            stroke-width="1.5"
            stroke-linecap="round"
            stroke-linejoin="round"
            fill="none"
          />
        </svg>
      </button>
      <button
        v-if="selectedToolId"
        class="ghost-icon"
        :data-tooltip="t('components.main.form.editTitle')"
        type="button"
        @click="$emit('edit')"
      >
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <path
            d="M11.983 2.25a1.125 1.125 0 011.077.81l.563 2.101a7.482 7.482 0 012.326 1.343l2.08-.621a1.125 1.125 0 011.356.651l1.313 3.207a1.125 1.125 0 01-.442 1.339l-1.86 1.205a7.418 7.418 0 010 2.686l1.86 1.205a1.125 1.125 0 01.442 1.339l-1.313 3.207a1.125 1.125 0 01-1.356.651l-2.08-.621a7.482 7.482 0 01-2.326 1.343l-.563 2.101a1.125 1.125 0 01-1.077.81h-2.634a1.125 1.125 0 01-1.077-.81l-.563-2.101a7.482 7.482 0 01-2.326-1.343l-2.08.621a1.125 1.125 0 01-1.356-.651l-1.313-3.207a1.125 1.125 0 01.442-1.339l1.86-1.205a7.418 7.418 0 010-2.686l-1.86-1.205a1.125 1.125 0 01-.442-1.339l1.313-3.207a1.125 1.125 0 011.356-.651l2.08.621a7.482 7.482 0 012.326-1.343l.563-2.101a1.125 1.125 0 011.077-.81h2.634z"
            fill="none"
            stroke="currentColor"
            stroke-width="1.5"
            stroke-linecap="round"
            stroke-linejoin="round"
          />
          <path d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
        </svg>
      </button>
      <button
        v-if="selectedToolId"
        class="ghost-icon"
        :data-tooltip="t('components.main.form.actions.delete')"
        type="button"
        @click="$emit('delete')"
      >
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <path
            d="M9 3h6m-7 4h8m-6 0v11m4-11v11M5 7h14l-.867 12.138A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.862L5 7z"
            fill="none"
            stroke="currentColor"
            stroke-width="1.5"
            stroke-linecap="round"
            stroke-linejoin="round"
          />
        </svg>
      </button>
    </div>
    <p v-if="customCliTools.length === 0" class="no-tools-hint">
      {{ t('components.main.customCli.noTools') }} - {{ t('components.main.customCli.addTool') }}
    </p>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { CustomCliTool } from '../../../services/customCliService'

const props = defineProps<{
  customCliTools: CustomCliTool[]
  selectedToolId: string | null
}>()

const emit = defineEmits<{
  'update:selectedToolId': [value: string | null]
  change: []
  create: []
  edit: []
  delete: []
}>()

const { t } = useI18n()

const handleChange = (event: Event) => {
  const value = (event.target as HTMLSelectElement).value || null
  emit('update:selectedToolId', value)
  emit('change')
}
</script>

<style scoped>
.cli-tool-selector {
  padding: 12px 16px;
  background: var(--mac-surface);
  border-radius: 8px;
  margin-bottom: 16px;
  border: 1px solid var(--mac-border);
}

.tool-selector-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.tool-select {
  flex: 1;
  padding: 8px 12px;
  background: var(--color-bg-secondary);
  border: 1px solid var(--color-border);
  border-radius: 6px;
  font-size: 14px;
  color: var(--color-text-primary);
  cursor: pointer;
  transition: all 0.2s ease;
}

.tool-select:hover {
  border-color: var(--color-border-hover);
}

.tool-select:focus {
  outline: 2px solid var(--color-accent);
  outline-offset: 2px;
}

.add-tool-btn {
  flex-shrink: 0;
}

.no-tools-hint {
  margin-top: 8px;
  font-size: 13px;
  color: var(--mac-text-secondary);
  text-align: center;
}

:global(.dark) .cli-tool-selector {
  background: var(--mac-surface);
  border-color: var(--mac-border);
}

:global(.dark) .tool-select {
  background: rgba(255, 255, 255, 0.05);
  border-color: rgba(255, 255, 255, 0.1);
  color: var(--mac-text);
}

:global(.dark) .tool-select:hover {
  border-color: rgba(255, 255, 255, 0.2);
}
</style>

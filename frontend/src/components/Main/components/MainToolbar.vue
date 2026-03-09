<template>
  <div class="global-actions">
    <p class="global-eyebrow">{{ t('components.main.hero.eyebrow') }}</p>
    <button
      class="ghost-icon github-icon"
      :class="{
        'github-upgrade': hasUpdateAvailable && !updateReady,
        'update-ready': updateReady,
      }"
      :data-tooltip="githubTooltip"
      type="button"
      @click="$emit('github-click')"
    >
      <svg viewBox="0 0 24 24" aria-hidden="true">
        <path
          d="M9 19c-4.5 1.5-4.5-2.5-6-3m12 5v-3.87a3.37 3.37 0 00-.94-2.61c3.14-.35 6.44-1.54 6.44-7A5.44 5.44 0 0018 3.77 5.07 5.07 0 0017.91 1S16.73.65 14 2.48a13.38 13.38 0 00-5 0C6.27.65 5.09 1 5.09 1A5.07 5.07 0 005 3.77a5.44 5.44 0 00-1.5 3.76c0 5.42 3.3 6.61 6.44 7A3.37 3.37 0 009 18.13V22"
          fill="none"
          stroke="currentColor"
          stroke-width="1.5"
          stroke-linecap="round"
          stroke-linejoin="round"
        />
      </svg>
      <span v-if="updateReady" class="update-badge pulse">Ready</span>
      <span v-else-if="downloadProgress > 0 && downloadProgress < 100" class="update-badge downloading">
        {{ Math.round(downloadProgress) }}%
      </span>
      <span v-else-if="hasUpdateAvailable" class="update-badge">New</span>
    </button>
    <button
      class="ghost-icon"
      :data-tooltip="t('components.main.controls.theme')"
      type="button"
      @click="$emit('toggle-theme')"
    >
      <svg v-if="themeIcon === 'sun'" viewBox="0 0 24 24" aria-hidden="true">
        <circle cx="12" cy="12" r="4" stroke="currentColor" stroke-width="1.5" fill="none" />
        <path
          d="M12 3v2m0 14v2m9-9h-2M5 12H3m14.95 6.95-1.41-1.41M7.46 7.46 6.05 6.05m12.9 0-1.41 1.41M7.46 16.54l-1.41 1.41"
          stroke="currentColor"
          stroke-width="1.5"
          stroke-linecap="round"
        />
      </svg>
      <svg v-else viewBox="0 0 24 24" aria-hidden="true">
        <path
          d="M21 12.79A9 9 0 1111.21 3a7 7 0 109.79 9.79z"
          fill="none"
          stroke="currentColor"
          stroke-width="1.5"
          stroke-linecap="round"
          stroke-linejoin="round"
        />
      </svg>
    </button>
    <button
      v-if="showImportButton"
      class="ghost-icon"
      :data-tooltip="importButtonTooltip"
      :disabled="importBusy"
      type="button"
      @click="$emit('import')"
    >
      <svg viewBox="0 0 24 24" aria-hidden="true" :class="{ rotating: importBusy }">
        <path
          d="M12 4v9"
          stroke="currentColor"
          stroke-width="1.5"
          stroke-linecap="round"
          stroke-linejoin="round"
          fill="none"
        />
        <path
          d="M8.5 10.5l3.5 3.5 3.5-3.5"
          stroke="currentColor"
          stroke-width="1.5"
          stroke-linecap="round"
          stroke-linejoin="round"
          fill="none"
        />
        <path
          d="M5 19h14"
          stroke="currentColor"
          stroke-width="1.5"
          stroke-linecap="round"
          stroke-linejoin="round"
          fill="none"
        />
      </svg>
    </button>
    <button
      class="ghost-icon"
      :data-tooltip="t('components.main.controls.settings')"
      type="button"
      @click="$emit('settings')"
    >
      <svg viewBox="0 0 24 24" aria-hidden="true">
        <path
          d="M12 15a3 3 0 100-6 3 3 0 000 6z"
          stroke="currentColor"
          stroke-width="1.5"
          stroke-linecap="round"
          stroke-linejoin="round"
          fill="none"
        />
        <path
          d="M19.4 15a1.65 1.65 0 00.33 1.82l.06.06a2 2 0 01-2.83 2.83l-.06-.06a1.65 1.65 0 00-1.82-.33 1.65 1.65 0 00-1 1.51V21a2 2 0 01-4 0v-.09a1.65 1.65 0 00-1-1.51 1.65 1.65 0 00-1.82.33l-.06.06a2 2 0 01-2.83-2.83l.06-.06a1.65 1.65 0 00.33-1.82 1.65 1.65 0 00-1.51-1H3a2 2 0 010-4h.09a1.65 1.65 0 001.51-1 1.65 1.65 0 00-.33-1.82l-.06-.06a2 2 0 012.83-2.83l.06.06a1.65 1.65 0 001.82.33H9a1.65 1.65 0 001-1.51V3a2 2 0 014 0v.09a1.65 1.65 0 001 1.51 1.65 1.65 0 001.82-.33l.06-.06a2 2 0 012.83 2.83l-.06.06a1.65 1.65 0 00-.33 1.82V9a1.65 1.65 0 001.51 1H21a2 2 0 010 4h-.09a1.65 1.65 0 00-1.51 1z"
          stroke="currentColor"
          stroke-width="1.5"
          stroke-linecap="round"
          stroke-linejoin="round"
          fill="none"
        />
      </svg>
    </button>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'

defineProps<{
  hasUpdateAvailable: boolean
  updateReady: boolean
  downloadProgress: number
  githubTooltip: string
  themeIcon: 'sun' | 'moon'
  showImportButton: boolean
  importButtonTooltip: string
  importBusy: boolean
}>()

defineEmits<{
  'github-click': []
  'toggle-theme': []
  'import': []
  'settings': []
}>()

const { t } = useI18n()
</script>

<style scoped>
.global-actions .ghost-icon svg.rotating {
  animation: import-spin 0.9s linear infinite;
}

@keyframes import-spin {
  from {
    transform: rotate(0deg);
  }

  to {
    transform: rotate(360deg);
  }
}
</style>

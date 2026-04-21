<template>
  <div class="section-header main-platform-tabs">
    <div class="tab-group" role="tablist" :aria-label="t('components.main.tabs.ariaLabel')">
      <button
        v-for="(tab, idx) in tabs"
        :key="tab.id"
        class="tab-pill"
        :class="{ active: selectedIndex === idx }"
        role="tab"
        :aria-selected="selectedIndex === idx"
        type="button"
        @click="$emit('change', idx)"
      >
        {{ tab.label }}
      </button>
    </div>

    <div class="section-controls">
      <div class="relay-toggle" :aria-label="currentProxyLabel">
        <div class="relay-switch">
          <button
            type="button"
            class="relay-toggle-switch"
            :class="{ 'is-active': activeProxyState, 'is-busy': activeProxyBusy }"
            role="switch"
            :aria-checked="activeProxyState"
            :aria-label="`${currentProxyLabel} · ${activeProxyState ? t('components.main.relayToggle.statusOn') : t('components.main.relayToggle.statusOff')}`"
            :disabled="activeProxyBusy"
            @click="$emit('toggle-proxy')"
          >
            <span class="relay-toggle-switch__thumb" aria-hidden="true">
              <svg
                v-if="activeProxyState"
                viewBox="0 0 24 24"
                class="relay-toggle-switch__icon"
                aria-hidden="true"
              >
                <path
                  d="M13 2L5.5 12.2h4.6L9.4 22 18.5 10.8h-4.9L13 2z"
                  fill="currentColor"
                  stroke="currentColor"
                  stroke-width="0.6"
                  stroke-linejoin="round"
                />
              </svg>
              <span v-else class="relay-toggle-switch__dot"></span>
            </span>
            <span class="sr-only">
              {{ activeProxyState ? t('components.main.relayToggle.statusOn') : t('components.main.relayToggle.statusOff') }}
            </span>
          </button>
          <span class="relay-tooltip-content">
            {{ currentProxyLabel }} · {{ t('components.main.relayToggle.tooltip') }}
          </span>
        </div>
      </div>

      <button
        class="ghost-icon"
        :data-tooltip="t('components.main.tabs.addCard')"
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
        class="ghost-icon"
        :class="{ rotating: refreshing }"
        :data-tooltip="t('components.main.tabs.refresh')"
        :disabled="refreshing"
        type="button"
        @click="$emit('refresh')"
      >
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <path
            d="M21.5 2v6h-6M2.5 22v-6h6M2 11.5a10 10 0 0118.8-4.3M22 12.5a10 10 0 01-18.8 4.2"
            stroke="currentColor"
            stroke-width="1.5"
            stroke-linecap="round"
            stroke-linejoin="round"
            fill="none"
          />
        </svg>
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { MainTabOption } from '../types'

defineProps<{
  tabs: readonly MainTabOption[]
  selectedIndex: number
  currentProxyLabel: string
  activeProxyState: boolean
  activeProxyBusy: boolean
  refreshing: boolean
}>()

defineEmits<{
  change: [index: number]
  'toggle-proxy': []
  create: []
  refresh: []
}>()

const { t } = useI18n()
</script>

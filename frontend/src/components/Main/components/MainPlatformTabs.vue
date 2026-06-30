<template>
  <div class="section-header main-platform-tabs">
    <div
      ref="tabGroupRef"
      class="tab-group"
      :style="tabGroupStyle"
      role="tablist"
      :aria-label="t('components.main.tabs.ariaLabel')"
    >
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
        <span class="tab-pill__icon" aria-hidden="true">
          <span
            v-if="getTabIconSvg(tab.icon)"
            class="tab-pill__icon-svg"
            v-html="getTabIconSvg(tab.icon)"
          ></span>
          <span v-else class="tab-pill__icon-fallback">
            {{ getTabInitials(tab.label) }}
          </span>
        </span>
        <span class="tab-pill__label">{{ tab.label }}</span>
      </button>
    </div>

    <div class="section-controls">
      <div v-if="showProxyToggle" class="relay-toggle" :aria-label="currentProxyLabel">
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

      <div v-if="showSessionAffinityToggle" class="relay-toggle" :aria-label="t('components.main.sessionAffinity.label')">
        <div class="relay-switch">
          <button
            type="button"
            class="relay-toggle-switch session-affinity-toggle"
            :class="{ 'is-active': activeSessionAffinityState, 'is-busy': sessionAffinityBusy }"
            role="switch"
            :aria-checked="activeSessionAffinityState"
            :aria-label="`${t('components.main.sessionAffinity.label')} · ${activeSessionAffinityState ? t('components.main.relayToggle.statusOn') : t('components.main.relayToggle.statusOff')}`"
            :disabled="sessionAffinityBusy"
            @click="$emit('toggle-session-affinity')"
          >
            <span class="relay-toggle-switch__thumb" aria-hidden="true">
              <span class="relay-toggle-switch__dot"></span>
            </span>
            <span class="sr-only">
              {{ activeSessionAffinityState ? t('components.main.relayToggle.statusOn') : t('components.main.relayToggle.statusOff') }}
            </span>
          </button>
          <span class="relay-tooltip-content">
            {{ activeSessionAffinityState ? t('components.main.sessionAffinity.tooltipOn') : t('components.main.sessionAffinity.tooltipOff') }}
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
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  getProviderDisplayIconSvg,
  preloadProviderDisplayIcons,
} from '../../../utils/providerIconAssets'
import type { MainTabOption } from '../types'

const props = withDefaults(defineProps<{
  tabs: readonly MainTabOption[]
  selectedIndex: number
  currentProxyLabel: string
  showProxyToggle?: boolean
  activeProxyState: boolean
  activeProxyBusy: boolean
  activeSessionAffinityState: boolean
  sessionAffinityBusy: boolean
  showSessionAffinityToggle?: boolean
  refreshing: boolean
}>(), {
  showProxyToggle: true,
  showSessionAffinityToggle: true,
})

defineEmits<{
  change: [index: number]
  'toggle-proxy': []
  'toggle-session-affinity': []
  create: []
  refresh: []
}>()

const { t } = useI18n()
const tabGroupRef = ref<HTMLDivElement | null>(null)
const measuredTabGroupWidth = ref<number | null>(null)

const getTabIconSvg = (icon: string) => getProviderDisplayIconSvg(icon)

const getTabInitials = (label: string) => {
  return label
    .split(/\s+/)
    .filter(Boolean)
    .map((word) => word[0])
    .join('')
    .slice(0, 2)
    .toUpperCase()
}

let tabGroupResizeObserver: ResizeObserver | null = null
let tabGroupMeasureFrameId: number | null = null

function parsePixelValue(value: string | null | undefined) {
  return Number.parseFloat(value ?? '0') || 0
}

function cleanupMeasureFrame() {
  if (typeof window === 'undefined' || tabGroupMeasureFrameId === null) return
  window.cancelAnimationFrame(tabGroupMeasureFrameId)
  tabGroupMeasureFrameId = null
}

function measureTabGroupWidth() {
  if (typeof window === 'undefined') return

  const tabGroupElement = tabGroupRef.value
  if (!tabGroupElement) return

  const tabButtons = Array.from(tabGroupElement.querySelectorAll<HTMLButtonElement>('.tab-pill'))
  if (!tabButtons.length) return

  const styles = window.getComputedStyle(tabGroupElement)
  const gap = parsePixelValue(styles.columnGap || styles.gap)
  const paddingLeft = parsePixelValue(styles.paddingLeft)
  const paddingRight = parsePixelValue(styles.paddingRight)
  const borderLeft = parsePixelValue(styles.borderLeftWidth)
  const borderRight = parsePixelValue(styles.borderRightWidth)

  const buttonsWidth = tabButtons.reduce((total, button) => total + button.getBoundingClientRect().width, 0)
  const nextWidth = Math.ceil(
    buttonsWidth +
      gap * Math.max(tabButtons.length - 1, 0) +
      paddingLeft +
      paddingRight +
      borderLeft +
      borderRight,
  )

  if (nextWidth > 0 && nextWidth !== measuredTabGroupWidth.value) {
    measuredTabGroupWidth.value = nextWidth
  }
}

function scheduleTabGroupMeasure() {
  if (typeof window === 'undefined') return

  cleanupMeasureFrame()
  nextTick(() => {
    tabGroupMeasureFrameId = window.requestAnimationFrame(() => {
      tabGroupMeasureFrameId = null
      measureTabGroupWidth()
    })
  })
}

function bindResizeObserver() {
  if (typeof window === 'undefined' || typeof ResizeObserver === 'undefined') return

  tabGroupResizeObserver?.disconnect()

  const tabGroupElement = tabGroupRef.value
  if (!tabGroupElement) return

  tabGroupResizeObserver = new ResizeObserver(() => {
    scheduleTabGroupMeasure()
  })

  tabGroupResizeObserver.observe(tabGroupElement)
  Array.from(tabGroupElement.children).forEach((child) => {
    if (child instanceof HTMLElement) {
      tabGroupResizeObserver?.observe(child)
    }
  })
}

const tabGroupStyle = computed(() => {
  if (!measuredTabGroupWidth.value) return undefined
  return {
    '--main-platform-tab-group-width': `${measuredTabGroupWidth.value}px`,
  }
})

watch(
  () => props.tabs.map((tab) => `${tab.id}:${tab.label}:${tab.icon}`).join('|'),
  () => {
    void preloadProviderDisplayIcons(props.tabs.map((tab) => tab.icon))
    scheduleTabGroupMeasure()
    bindResizeObserver()
  },
  { immediate: true },
)

onMounted(() => {
  scheduleTabGroupMeasure()
  bindResizeObserver()

  window.addEventListener('resize', scheduleTabGroupMeasure)

  if (typeof document !== 'undefined' && 'fonts' in document && document.fonts?.ready) {
    void document.fonts.ready.then(() => {
      scheduleTabGroupMeasure()
      bindResizeObserver()
    })
  }
})

onBeforeUnmount(() => {
  cleanupMeasureFrame()
  tabGroupResizeObserver?.disconnect()
  tabGroupResizeObserver = null
  if (typeof window !== 'undefined') {
    window.removeEventListener('resize', scheduleTabGroupMeasure)
  }
})
</script>

<style scoped>
.main-platform-tabs {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: nowrap;
}

.main-platform-tabs .tab-group {
  width: var(--main-platform-tab-group-width, auto);
  min-width: 0;
  max-width: none;
  flex: 0 0 auto;
  overflow: visible;
}

.main-platform-tabs :deep(.tab-pill) {
  gap: 7px;
}

.tab-pill__icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 16px;
  height: 16px;
  flex-shrink: 0;
  color: currentColor;
}

.tab-pill__icon-svg,
.tab-pill__icon-svg :deep(svg) {
  display: block;
  width: 16px;
  height: 16px;
}

.tab-pill__icon-fallback {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 16px;
  height: 16px;
  border-radius: 5px;
  background: rgba(148, 163, 184, 0.16);
  font-size: 8px;
  font-weight: 700;
}

.tab-pill__label {
  line-height: 1.1;
}

.main-platform-tabs .section-controls {
  min-width: max-content;
  margin-left: auto;
  justify-self: auto;
}

@media (max-width: 700px) {
  .main-platform-tabs {
    display: flex;
  }

  .main-platform-tabs .tab-group {
    width: 100%;
    max-width: 100%;
    min-width: 0;
    overflow-x: auto;
    overflow-y: hidden;
  }

  .main-platform-tabs .section-controls {
    justify-self: auto;
  }
}
</style>

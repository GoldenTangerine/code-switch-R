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
        :class="{ active: selectedIndex === idx, 'tab-pill--icon-only': !showTabLabels }"
        role="tab"
        :aria-selected="selectedIndex === idx"
        :aria-label="showTabLabels ? undefined : tab.label"
        :title="showTabLabels ? undefined : tab.label"
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
        <span v-if="showTabLabels" class="tab-pill__label">{{ tab.label }}</span>
        <span
          class="tab-pill__status-group"
          :aria-label="getTabStatusLabel(tab.id)"
          :title="getTabStatusLabel(tab.id)"
        >
          <span
            class="tab-pill__status-dot tab-pill__status-dot--proxy"
            :class="{
              'is-active': getTabStatus(tab.id).proxyEnabled,
              'is-disabled': !getTabStatus(tab.id).proxySupported,
            }"
            aria-hidden="true"
          ></span>
          <span
            class="tab-pill__status-dot tab-pill__status-dot--concurrency"
            :class="{ 'is-active': getTabStatus(tab.id).concurrencyLimited }"
            aria-hidden="true"
          ></span>
        </span>
      </button>
    </div>

    <div class="section-controls">
      <div v-if="showProxyToggle" class="relay-toggle" :aria-label="currentProxyLabel">
        <div class="relay-switch">
          <span class="relay-toggle-caption">
            {{ t('components.main.relayToggle.label') }}
          </span>
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

      <div v-if="showProxyToggle" class="relay-toggle relay-toggle--concurrency" :aria-label="t('components.main.concurrencyLimitToggle.label')">
        <div class="relay-switch">
          <span class="relay-toggle-caption">
            {{ t('components.main.concurrencyLimitToggle.label') }}
          </span>
          <button
            type="button"
            class="relay-toggle-switch concurrency-limit-toggle"
            :class="{ 'is-active': activeProviderConcurrencyLimitState, 'is-busy': providerConcurrencyLimitBusy }"
            role="switch"
            :aria-checked="activeProviderConcurrencyLimitState"
            :aria-label="`${t('components.main.concurrencyLimitToggle.label')} · ${activeProviderConcurrencyLimitState ? t('components.main.concurrencyLimitToggle.statusOn') : t('components.main.concurrencyLimitToggle.statusOff')}`"
            :disabled="providerConcurrencyLimitBusy"
            @click="$emit('toggle-provider-concurrency-limit')"
          >
            <span class="relay-toggle-switch__thumb" aria-hidden="true">
              <span class="relay-toggle-switch__dot"></span>
            </span>
            <span class="sr-only">
              {{ activeProviderConcurrencyLimitState ? t('components.main.concurrencyLimitToggle.statusOn') : t('components.main.concurrencyLimitToggle.statusOff') }}
            </span>
          </button>
          <span class="relay-tooltip-content">
            {{ activeProviderConcurrencyLimitState ? t('components.main.concurrencyLimitToggle.tooltipOn') : t('components.main.concurrencyLimitToggle.tooltipOff') }}
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
import type { MainTabOption, MainTabStatus, ProviderTab } from '../types'

const props = withDefaults(defineProps<{
  tabs: readonly MainTabOption[]
  selectedIndex: number
  currentProxyLabel: string
  showProxyToggle?: boolean
  activeProxyState: boolean
  activeProxyBusy: boolean
  activeProviderConcurrencyLimitState: boolean
  providerConcurrencyLimitBusy: boolean
  refreshing: boolean
  tabStatuses?: Partial<Record<ProviderTab, MainTabStatus>>
}>(), {
  showProxyToggle: true,
  tabStatuses: () => ({}),
})

defineEmits<{
  change: [index: number]
  'toggle-proxy': []
  'toggle-provider-concurrency-limit': []
  create: []
  refresh: []
}>()

const { t } = useI18n()
const tabGroupRef = ref<HTMLDivElement | null>(null)
const measuredTabGroupWidth = ref<number | null>(null)

// 可见 tab 不超过 4 个时展示「图标+名称」，超过后整体切换为纯图标（title 提示名称）
const TAB_LABEL_MAX_COUNT = 4
const showTabLabels = computed(() => props.tabs.length <= TAB_LABEL_MAX_COUNT)

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

function getTabStatus(tab: ProviderTab): MainTabStatus {
  return props.tabStatuses[tab] ?? {
    proxyEnabled: false,
    concurrencyLimited: false,
    proxySupported: false,
  }
}

function getTabStatusLabel(tab: ProviderTab) {
  const status = getTabStatus(tab)
  const proxyLabel = status.proxySupported
    ? status.proxyEnabled
      ? t('components.main.tabs.status.proxyOn')
      : t('components.main.tabs.status.proxyOff')
    : t('components.main.tabs.status.proxyUnsupported')
  const concurrencyLabel = status.concurrencyLimited
    ? t('components.main.tabs.status.concurrencyOn')
    : t('components.main.tabs.status.concurrencyOff')

  return `${proxyLabel} · ${concurrencyLabel}`
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

.main-platform-tabs :deep(.tab-pill--icon-only) {
  padding-left: 13px;
  padding-right: 13px;
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

.tab-pill__status-group {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  margin-left: 2px;
  flex-shrink: 0;
}

.tab-pill__status-dot {
  width: 6px;
  height: 6px;
  border-radius: 999px;
  background: rgba(100, 116, 139, 0.72);
  box-shadow: inset 0 0 0 1px rgba(148, 163, 184, 0.12);
  opacity: 0.78;
  transition: background 0.22s ease, box-shadow 0.22s ease, opacity 0.22s ease, transform 0.22s ease;
}

.tab-pill__status-dot.is-active {
  opacity: 1;
  transform: scale(1.08);
  animation: tab-status-breathe 1.8s ease-in-out infinite;
}

.tab-pill__status-dot--proxy.is-active {
  background: #34d399;
  box-shadow:
    0 0 0 1px rgba(52, 211, 153, 0.28),
    0 0 8px rgba(16, 185, 129, 0.82);
}

.tab-pill__status-dot--concurrency.is-active {
  background: #a78bfa;
  box-shadow:
    0 0 0 1px rgba(167, 139, 250, 0.28),
    0 0 8px rgba(139, 92, 246, 0.82);
}

.tab-pill__status-dot.is-disabled {
  opacity: 0.38;
}

@keyframes tab-status-breathe {
  0%,
  100% {
    filter: saturate(1);
  }

  50% {
    filter: saturate(1.25);
  }
}

@media (prefers-reduced-motion: reduce) {
  .tab-pill__status-dot.is-active {
    animation: none;
    transform: none;
  }
}

.main-platform-tabs .section-controls {
  min-width: max-content;
  margin-left: auto;
  justify-self: auto;
}

.main-platform-tabs .relay-toggle {
  gap: 0;
}

.main-platform-tabs .relay-switch {
  gap: 8px;
  min-height: 40px;
  padding: 7px 12px;
  border: 1px solid var(--main-home-action-border);
  border-radius: 12px;
  background: var(--main-home-action-bg);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.08);
  box-sizing: border-box;
  backdrop-filter: blur(14px);
  -webkit-backdrop-filter: blur(14px);
}

.main-platform-tabs .relay-switch:hover,
.main-platform-tabs .relay-switch:focus-within {
  border-color: var(--main-home-action-hover-border);
  background: var(--main-home-action-hover-bg);
}

.main-platform-tabs .relay-toggle-switch {
  width: 44px;
  height: 24px;
  transform: none;
  transition:
    background 0.3s ease,
    box-shadow 0.3s ease,
    opacity 0.2s ease;
}

.main-platform-tabs .relay-toggle-switch:hover:not(:disabled) {
  transform: none;
}

.main-platform-tabs .relay-toggle-switch__thumb {
  top: 2px;
  left: 2px;
  width: 20px;
  height: 20px;
  transition: transform 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.main-platform-tabs .relay-toggle-switch.is-active .relay-toggle-switch__thumb {
  transform: translateX(20px);
}

.main-platform-tabs .relay-toggle-switch__icon {
  width: 11px;
  height: 11px;
}

.main-platform-tabs .relay-toggle-switch__dot {
  width: 5px;
  height: 5px;
}

.relay-toggle-caption {
  color: var(--mac-text-secondary);
  font-size: 0.72rem;
  font-weight: 600;
  line-height: 1;
  white-space: nowrap;
}

@media (max-width: 1120px) {
  .main-platform-tabs {
    display: flex;
    flex-wrap: wrap;
  }

  .main-platform-tabs .tab-group {
    width: 100%;
    max-width: 100%;
    min-width: 0;
    flex: 1 1 100%;
    overflow-x: auto;
    overflow-y: hidden;
  }

  .main-platform-tabs .section-controls {
    width: 100%;
    min-width: 0;
    margin-left: 0;
    flex-wrap: wrap;
    justify-content: flex-end;
    justify-self: auto;
  }
}

@media (max-width: 700px) {
  .main-platform-tabs .section-controls {
    justify-content: center;
  }
}
</style>

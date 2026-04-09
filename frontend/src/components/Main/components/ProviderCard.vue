<template>
  <article
    :ref="bindCardRef"
    :data-provider-id="viewModel.card.id"
    :class="[
      'automation-card',
      { dragging: viewModel.dragging },
      { 'drag-over': viewModel.dragOver },
      { 'is-last-used': viewModel.isLastUsed },
      { 'is-hosted-active': hostedSelectionActive },
      { 'is-highlighted': viewModel.isHighlighted },
    ]"
    draggable="true"
    @click="$emit('card-click')"
    @dragstart="handleDragStart"
    @dragend="handleDragEnd"
  >
    <div class="card-leading">
      <div class="card-icon" :style="{ backgroundColor: viewModel.card.tint, color: viewModel.card.accent }">
        <span
          v-if="!viewModel.iconSvg"
          class="icon-fallback"
        >
          {{ viewModel.vendorInitials }}
        </span>
        <span
          v-else
          class="icon-svg"
          v-html="viewModel.iconSvg"
          aria-hidden="true"
        ></span>
      </div>

      <div class="card-text">
        <div class="card-title-row">
          <p class="card-title">{{ viewModel.card.name }}</p>
          <span
            v-if="viewModel.isDirectApplied && !activeProxyState"
            class="current-use-badge"
          >
            {{ t('components.main.directApply.currentBadge') }}
          </span>
          <span
            v-if="viewModel.card.availabilityMonitorEnabled"
            class="connectivity-dot"
            :class="viewModel.connectivityClass"
            :title="viewModel.connectivityTooltip"
          ></span>
          <span v-if="viewModel.card.level" class="level-badge scheduling-level" :class="`level-${viewModel.card.level}`">
            L{{ viewModel.card.level }}
          </span>
          <span
            v-if="viewModel.blacklistStatus"
            :class="[
              'blacklist-level-badge',
              `bl-level-${viewModel.blacklistStatus.blacklistLevel}`,
              { dark: resolvedTheme === 'dark' },
            ]"
            :title="t('components.main.blacklist.levelTitle', { level: viewModel.blacklistStatus.blacklistLevel })"
          >
            BL{{ viewModel.blacklistStatus.blacklistLevel }}
          </span>
          <span
            v-if="showHostedStateBadges"
            class="provider-state-inline"
            :title="relayStatusTitle"
            :aria-label="relayStatusTitle"
            role="group"
          >
            <span
              v-if="showHostedModeBadge"
              class="provider-state-pill provider-state-pill--hosted"
              :title="relayStatusTitle"
              :aria-hidden="true"
            >
              <span
                class="provider-state-indicator"
                :class="hostedSelectionActive ? 'provider-state-pulse' : 'provider-state-pulse provider-state-pulse--idle'"
                aria-hidden="true"
              ></span>
              {{ t('components.main.providers.hostedLive') }}
            </span>
            <span
              class="provider-state-pill"
              :class="statePillClass"
              :title="relayStatusTitle"
              :aria-hidden="true"
            >
              {{ relayStatusLabel }}
            </span>
          </span>
          <button
            v-if="viewModel.card.officialSite"
            class="card-site"
            type="button"
            @click.stop="$emit('open-site')"
          >
            {{ viewModel.formattedOfficialSite }}
          </button>
        </div>

        <div
          v-for="stats in [viewModel.stats]"
          :key="`metrics-${viewModel.card.id}`"
          class="card-metrics"
        >
          <template v-if="stats.state !== 'ready'">
            {{ stats.message }}
          </template>
          <template v-else>
            <div class="card-metrics-line">
              <span
                v-if="stats.successRateLabel"
                class="card-success-rate"
                :class="stats.successRateClass"
              >
                {{ stats.successRateLabel }}
              </span>
              <span
                v-if="stats.successRateLabel"
                class="card-metric-separator"
                aria-hidden="true"
              >
                ·
              </span>
              <span>{{ stats.requests }}</span>
              <span class="card-metric-separator" aria-hidden="true">·</span>
              <span>{{ stats.tokens }}</span>
              <span class="card-metric-separator" aria-hidden="true">·</span>
              <span class="card-cost-inline">
                <span class="card-cost-label">{{ stats.costLabel }}:</span>
                <button
                  type="button"
                  class="card-cost-trigger"
                  :class="{ 'card-cost-trigger--zero': stats.costValue <= 0 }"
                  :title="t('components.main.providerCostTrend.buttonTooltip')"
                  :aria-label="t('components.main.providerCostTrend.buttonAriaLabel', {
                    name: viewModel.card.name,
                    amount: stats.costFormatted,
                  })"
                  @click.stop="$emit('open-provider-cost-trend')"
                >
                  <span
                    v-for="(part, index) in stats.costParts"
                    :key="`${part.type}-${index}`"
                    :class="['card-cost-trigger__part', `card-cost-trigger__part--${part.type}`]"
                  >
                    {{ part.value }}
                  </span>
                </button>
              </span>
            </div>
            <div
              class="card-metrics-line card-metrics-line-performance"
              :title="stats.performanceHint"
            >
              <span class="card-performance-item">
                <span class="performance-badge performance-badge--ttft">首</span>
                <span>{{ stats.ttft }}</span>
              </span>
              <span class="card-performance-item">
                <span class="performance-badge performance-badge--tps">速</span>
                <span>{{ stats.tps }}</span>
              </span>
              <div
                v-if="viewModel.quotaDisplay.length > 0"
                class="card-performance-quotas"
              >
                <span
                  v-for="item in viewModel.quotaDisplay"
                  :key="item.key"
                  class="card-quota-item"
                  :title="quotaTooltip(item)"
                >
                  <span class="quota-badge" :class="`quota-badge--${item.key}`">{{ item.label }}</span>
                  <span class="quota-progress-bar">
                    <span
                      class="quota-progress-fill"
                      :class="quotaProgressClass(item)"
                      :style="{ width: `${quotaProgressWidth(item)}%` }"
                    ></span>
                  </span>
                  <span class="quota-usage-percent">{{ quotaUsagePercent(item) }}</span>
                  <span class="quota-countdown">{{ item.countdownLabel }}</span>
                </span>
              </div>
            </div>
          </template>
        </div>

        <div
          v-if="viewModel.blacklistStatus?.isBlacklisted"
          :class="['blacklist-banner', { dark: resolvedTheme === 'dark' }]"
        >
          <div class="blacklist-info">
            <span class="blacklist-icon">⛔</span>
            <span
              v-if="viewModel.blacklistStatus.blacklistLevel > 0"
              :class="[
                'level-badge',
                `level-${viewModel.blacklistStatus.blacklistLevel}`,
                { dark: resolvedTheme === 'dark' },
              ]"
            >
              L{{ viewModel.blacklistStatus.blacklistLevel }}
            </span>
            <span class="blacklist-text">
              {{ t('components.main.blacklist.blocked') }} |
              {{ t('components.main.blacklist.remaining') }}:
              {{ formatBlacklistCountdown(viewModel.blacklistStatus.remainingSeconds) }}
            </span>
          </div>
          <div class="blacklist-actions">
            <button
              class="unblock-btn primary"
              type="button"
              :title="t('components.main.blacklist.unblockAndResetHint')"
              @click.stop="$emit('unblock-and-reset')"
            >
              {{ t('components.main.blacklist.unblockAndReset') }}
            </button>
            <button
              class="unblock-btn secondary"
              type="button"
              :title="t('components.main.blacklist.resetLevelHint')"
              @click.stop="$emit('reset-level')"
            >
              {{ t('components.main.blacklist.resetLevel') }}
            </button>
          </div>
        </div>
        <div
          v-else-if="viewModel.blacklistStatus && viewModel.blacklistStatus.blacklistLevel > 0"
          class="level-badge-standalone"
        >
          <span
            :class="[
              'level-badge',
              `level-${viewModel.blacklistStatus.blacklistLevel}`,
              { dark: resolvedTheme === 'dark' },
            ]"
          >
            L{{ viewModel.blacklistStatus.blacklistLevel }}
          </span>
          <span class="level-hint">{{ t('components.main.blacklist.levelHint') }}</span>
          <button
            class="reset-level-mini"
            type="button"
            :title="t('components.main.blacklist.resetLevelHint')"
            @click.stop="$emit('reset-level')"
          >
            ✕
          </button>
        </div>
      </div>
    </div>

    <div class="card-actions" @click.stop>
      <label class="mac-switch sm">
        <input
          type="checkbox"
          :checked="viewModel.card.enabled"
          @change="handleToggleEnabled"
        />
        <span></span>
      </label>

      <button
        v-if="activeTab !== 'others'"
        class="ghost-icon direct-apply-btn"
        :class="{ 'is-active': viewModel.isDirectApplied && !activeProxyState }"
        :disabled="activeProxyState"
        :data-tooltip="directApplyTooltip"
        type="button"
        @click.stop="!viewModel.isDirectApplied && $emit('direct-apply')"
      >
        <span v-if="viewModel.isDirectApplied && !activeProxyState" class="apply-text">
          {{ t('components.main.directApply.inUse') }}
        </span>
        <svg v-else viewBox="0 0 24 24" aria-hidden="true" class="lightning-icon">
          <path
            d="M13 2L3 14h9l-1 8 10-12h-9l1-8z"
            stroke="currentColor"
            stroke-width="1.5"
            fill="none"
            stroke-linecap="round"
            stroke-linejoin="round"
          />
        </svg>
      </button>

      <button
        class="ghost-icon"
        :data-tooltip="t('components.main.form.editTitle')"
        type="button"
        @click="$emit('configure')"
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
        v-if="activeTab !== 'others'"
        class="ghost-icon provider-data-btn"
        :data-tooltip="t('components.main.providerDataOverview.buttonTooltip')"
        :aria-label="t('components.main.providerDataOverview.buttonAriaLabel', { name: viewModel.card.name })"
        type="button"
        @click="$emit('open-provider-data')"
      >
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <path
            d="M5.5 18.5V11.5M12 18.5V6.5M18.5 18.5V9.5"
            fill="none"
            stroke="currentColor"
            stroke-width="1.7"
            stroke-linecap="round"
          />
          <path
            d="M4.5 19.5h15"
            fill="none"
            stroke="currentColor"
            stroke-width="1.5"
            stroke-linecap="round"
          />
          <circle cx="5.5" cy="9.5" r="1.2" fill="currentColor" />
          <circle cx="12" cy="4.5" r="1.2" fill="currentColor" />
          <circle cx="18.5" cy="7.5" r="1.2" fill="currentColor" />
        </svg>
      </button>

      <button
        class="ghost-icon"
        :disabled="!viewModel.card.apiUrl || !viewModel.card.apiKey"
        :data-tooltip="(!viewModel.card.apiUrl || !viewModel.card.apiKey)
          ? t('components.main.modelList.buttonDisabledTooltip')
          : t('components.main.modelList.buttonTooltip')"
        type="button"
        @click="$emit('open-model-list')"
      >
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <path
            d="M8 6h13M8 12h13M8 18h13"
            fill="none"
            stroke="currentColor"
            stroke-width="1.5"
            stroke-linecap="round"
            stroke-linejoin="round"
          />
          <path
            d="M3.5 6.5h1v-1h-1v1zm0 6h1v-1h-1v1zm0 6h1v-1h-1v1z"
            fill="currentColor"
          />
        </svg>
      </button>

      <button
        v-if="activeTab !== 'others'"
        class="ghost-icon provider-log-btn"
        :data-tooltip="t('components.main.providerLogs.buttonTooltip')"
        type="button"
        @click="$emit('open-provider-logs')"
      >
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <path
            d="M7.25 4.75h6.5l3 3v11a1.75 1.75 0 01-1.75 1.75h-7.75A1.75 1.75 0 015.5 18.75V6.5a1.75 1.75 0 011.75-1.75z"
            fill="none"
            stroke="currentColor"
            stroke-width="1.5"
            stroke-linecap="round"
            stroke-linejoin="round"
          />
          <path
            d="M13.75 4.75V8h3.25"
            fill="none"
            stroke="currentColor"
            stroke-width="1.5"
            stroke-linecap="round"
            stroke-linejoin="round"
          />
          <path
            d="M11.5 10v4"
            fill="none"
            stroke="currentColor"
            stroke-width="1.7"
            stroke-linecap="round"
          />
          <circle cx="11.5" cy="17" r="0.9" fill="currentColor" />
        </svg>
      </button>

      <button
        class="ghost-icon"
        :data-tooltip="t('components.main.controls.duplicate')"
        type="button"
        @click="$emit('duplicate')"
      >
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <path
            d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z"
            fill="none"
            stroke="currentColor"
            stroke-width="1.5"
            stroke-linecap="round"
            stroke-linejoin="round"
          />
        </svg>
      </button>

      <button
        class="ghost-icon"
        :data-tooltip="t('components.main.form.actions.delete')"
        type="button"
        @click="$emit('remove')"
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
  </article>
</template>

<script setup lang="ts">
import { computed, type ComponentPublicInstance } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ProviderCardViewModel, ProviderDragEndPayload, ProviderQuotaDisplayItem, ProviderTab, ResolvedTheme } from '../types'
import { formatQuotaUsagePercent, getQuotaProgressClass, getQuotaProgressPercent } from '../utils/providerQuotaDisplay'
import { isHostedRouteActive } from '../utils/providerRoutingState'

const props = defineProps<{
  viewModel: ProviderCardViewModel
  activeTab: ProviderTab
  activeProxyState: boolean
  resolvedTheme: ResolvedTheme
  formatBlacklistCountdown: (remainingSeconds: number) => string
  bindCardRef?: (element: Element | ComponentPublicInstance | null) => void
}>()

const emit = defineEmits<{
  'card-click': []
  dragstart: []
  dragend: [payload: ProviderDragEndPayload]
  'open-site': []
  'unblock-and-reset': []
  'reset-level': []
  'toggle-enabled': [enabled: boolean]
  'direct-apply': []
  configure: []
  'open-provider-data': []
  'open-model-list': []
  'open-provider-logs': []
  'open-provider-cost-trend': []
  duplicate: []
  remove: []
}>()

const { t } = useI18n()

const quotaProgressClass = (item: ProviderQuotaDisplayItem) => getQuotaProgressClass(item)

const quotaProgressWidth = (item: ProviderQuotaDisplayItem) => getQuotaProgressPercent(item)

const quotaUsagePercent = (item: ProviderQuotaDisplayItem) => formatQuotaUsagePercent(item)

const quotaTooltip = (item: ProviderQuotaDisplayItem) => {
  const used = item.used.toFixed(2)
  const total = item.total.toFixed(2)
  if (item.nextReset) {
    return t('components.main.providers.quotaTooltip', { label: item.label, used, total, countdown: item.countdownLabel })
  }
  return t('components.main.providers.quotaTooltipNoCountdown', { label: item.label, used, total })
}

const directApplyTooltip = computed(() => {
  if (props.activeProxyState) {
    return t('components.main.directApply.proxyEnabled')
  }
  if (props.viewModel.isDirectApplied) {
    return t('components.main.directApply.inUse')
  }
  return t('components.main.directApply.title')
})

const hostedSelectionActive = computed(() => isHostedRouteActive({
  activeProxyState: props.activeProxyState,
  isLastUsed: props.viewModel.isLastUsed,
  enabled: props.viewModel.card.enabled,
  apiUrl: props.viewModel.card.apiUrl,
  apiKey: props.viewModel.card.apiKey,
  isBlacklisted: props.viewModel.blacklistStatus?.isBlacklisted === true,
}))

const showHostedStateBadges = computed(() => props.viewModel.isLastUsed || props.viewModel.isDefaultHostedProvider)

const showHostedModeBadge = computed(() => hostedSelectionActive.value || props.viewModel.isDefaultHostedProvider)

const statePillClass = computed(() => {
  if (hostedSelectionActive.value) {
    return 'provider-state-pill--active'
  }
  if (props.viewModel.isDefaultHostedProvider) {
    return 'provider-state-pill--default'
  }
  return 'provider-state-pill--recent'
})

const relayStatusLabel = computed(() => (
  hostedSelectionActive.value
    ? t('components.main.providers.currentRouted')
    : props.viewModel.isDefaultHostedProvider
      ? t('components.main.providers.defaultRouted')
    : t('components.main.providers.recentRouted')
))

const relayStatusTitle = computed(() => (
  hostedSelectionActive.value
    ? t('components.main.providers.currentRoutedHint')
    : props.viewModel.isDefaultHostedProvider
      ? t('components.main.providers.defaultRoutedHint')
    : t('components.main.providers.recentRoutedHint')
))

const handleToggleEnabled = (event: Event) => {
  emit('toggle-enabled', (event.target as HTMLInputElement).checked)
}

const handleDragStart = (event: DragEvent) => {
  if (event.dataTransfer) {
    event.dataTransfer.effectAllowed = 'move'
    event.dataTransfer.setData('text/plain', `${props.viewModel.card.id}`)
  }
  emit('dragstart')
}

const handleDragEnd = (event: DragEvent) => {
  emit('dragend', {
    dropEffect: event.dataTransfer?.dropEffect ?? 'none',
    clientX: Number.isFinite(event.clientX) ? event.clientX : null,
    clientY: Number.isFinite(event.clientY) ? event.clientY : null,
  })
}
</script>

<style scoped src="../styles/provider-card.css"></style>

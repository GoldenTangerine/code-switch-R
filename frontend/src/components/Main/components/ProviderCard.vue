<template>
  <article
    :ref="bindCardRef"
    :data-provider-id="viewModel.card.id"
    :class="[
      'automation-card',
      { 'theme-dark': resolvedTheme === 'dark' },
      { 'theme-light': resolvedTheme === 'light' },
      { dragging: viewModel.dragging },
      { 'drag-over': viewModel.dragOver },
      { 'is-enabled': viewModel.card.enabled },
      { 'is-quota-auto-disabled': viewModel.card.quotaAutoDisabled },
      { 'is-currently-active': isCurrentlyActive },
      { 'is-last-used': viewModel.isLastUsed },
      { 'is-hosted-active': hostedSelectionActive },
      { 'is-highlighted': viewModel.isHighlighted },
      { 'has-open-popover': hasOpenCardPopover },
    ]"
    :draggable="!viewModel.card.quotaAutoDisabled && !hasOpenCardPopover"
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
            v-if="apiFormatBadge"
            class="api-format-badge"
            :class="`api-format-badge--${apiFormatBadge.format}`"
            :data-tooltip="apiFormatBadge.title"
            :title="apiFormatBadge.title"
            :aria-label="apiFormatBadge.title"
          >
            <svg
              v-if="apiFormatBadge.format === 'openai_chat'"
              viewBox="0 0 24 24"
              aria-hidden="true"
              class="api-format-badge__icon"
            >
              <path
                d="M7.5 16.5L5 19v-4.25A4.75 4.75 0 016.75 5.6 4.72 4.72 0 019.5 4.75h5a4.75 4.75 0 014.75 4.75v.5a4.75 4.75 0 01-4.75 4.75H7.5z"
                fill="none"
                stroke="currentColor"
                stroke-width="1.6"
                stroke-linecap="round"
                stroke-linejoin="round"
              />
              <path
                d="M9 10h6M9 13h3.5"
                fill="none"
                stroke="currentColor"
                stroke-width="1.6"
                stroke-linecap="round"
              />
            </svg>
            <svg
              v-else
              viewBox="0 0 24 24"
              aria-hidden="true"
              class="api-format-badge__icon"
            >
              <path
                d="M6.75 6.75h10.5M6.75 12h10.5M6.75 17.25h7"
                fill="none"
                stroke="currentColor"
                stroke-width="1.7"
                stroke-linecap="round"
              />
              <path
                d="M16.25 14.75l1.25 2.5 2.5 1.25-2.5 1.25-1.25 2.5-1.25-2.5-2.5-1.25 2.5-1.25 1.25-2.5z"
                fill="currentColor"
              />
            </svg>
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
            v-if="anthropicCacheTTLBadge"
            class="anthropic-cache-ttl-badge"
            :class="`anthropic-cache-ttl-badge--${anthropicCacheTTLBadge.ttl}`"
            :data-tooltip="anthropicCacheTTLBadge.title"
            :title="anthropicCacheTTLBadge.title"
            :aria-label="anthropicCacheTTLBadge.title"
          >
            {{ anthropicCacheTTLBadge.label }}
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
            v-if="concurrencyStatusBadge"
            type="button"
            class="provider-state-pill provider-concurrency-pill"
            :class="{
              'provider-concurrency-pill--active': concurrencyStatusBadge.active,
              'provider-concurrency-pill--overflow': concurrencyStatusBadge.overflow,
            }"
            :title="concurrencyStatusBadge.title"
            @click.stop="$emit('open-concurrency-details')"
          >
            {{ concurrencyStatusBadge.label }}
          </button>
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
          <div class="card-metrics-line">
            <template v-if="stats.state === 'loading'">
              {{ stats.message }}
            </template>
            <template v-else-if="stats.state === 'empty'">
              <template v-if="activeTab !== 'others' && activeTab !== 'opencode'">
                <span
                  class="card-success-rate"
                  :title="successRateTooltip"
                  :aria-label="successRateTooltip"
                  tabindex="0"
                >
                  {{ t('components.main.providers.successRate') }}: —
                </span>
                <span class="card-metric-separator" aria-hidden="true">·</span>
              </template>
              <span>{{ stats.message }}</span>
            </template>
            <template v-else-if="stats.state === 'ready'">
              <span
                class="card-success-rate"
                :class="stats.successRateClass"
                :title="successRateTooltip"
                :aria-label="successRateTooltip"
                tabindex="0"
              >
                {{ stats.successRateLabel }}
              </span>
              <span class="card-metric-separator" aria-hidden="true">·</span>
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
            </template>
            <template v-if="viewModel.blacklistStatus?.isBlacklisted">
              <span class="card-metric-separator" aria-hidden="true">·</span>
              <span ref="blacklistPopoverRef" class="card-blacklist-inline">
                <button
                  type="button"
                  class="card-blacklist-trigger"
                  :aria-expanded="blacklistPopoverOpen"
                  :aria-controls="`blacklist-actions-${viewModel.card.id}`"
                  aria-haspopup="dialog"
                  @click.stop="toggleBlacklistPopover"
                >
                  {{ t('components.main.blacklist.blocked') }}
                  {{ formatBlacklistCountdown(viewModel.blacklistStatus.remainingSeconds) }}
                </button>
                <div
                  v-if="blacklistPopoverOpen"
                  :id="`blacklist-actions-${viewModel.card.id}`"
                  class="card-blacklist-popover"
                  role="dialog"
                  :aria-label="t('components.main.blacklist.detailsTitle')"
                  @click.stop
                >
                  <div class="card-blacklist-popover__row">
                    <span>{{ t('components.main.blacklist.remaining') }}</span>
                    <strong>{{ formatBlacklistCountdown(viewModel.blacklistStatus.remainingSeconds) }}</strong>
                  </div>
                  <div class="card-blacklist-popover__row">
                    <span>{{ t('components.main.blacklist.triggerLabel') }}</span>
                    <strong>{{ blacklistTriggerLabel(viewModel.blacklistStatus.blacklistTriggerSource) }}</strong>
                  </div>
                  <div
                    v-if="viewModel.blacklistStatus.blacklistReason"
                    class="card-blacklist-popover__reason"
                  >
                    {{ t('components.main.blacklist.reason', { reason: viewModel.blacklistStatus.blacklistReason }) }}
                  </div>
                  <div class="card-blacklist-popover__actions">
                    <button
                      type="button"
                      class="unblock-btn primary"
                      :title="t('components.main.blacklist.unblockAndResetHint')"
                      @click.stop="handleUnblockAndReset"
                    >
                      {{ t('components.main.blacklist.unblockAndReset') }}
                    </button>
                    <button
                      type="button"
                      class="unblock-btn secondary"
                      :title="t('components.main.blacklist.resetLevelHint')"
                      @click.stop="handleResetLevel"
                    >
                      {{ t('components.main.blacklist.resetLevel') }}
                    </button>
                  </div>
                </div>
              </span>
            </template>
          </div>
          <template v-if="stats.state === 'ready'">
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
                v-if="hasBalanceQuotaItems"
                class="card-balance-quota-panel"
              >
                <div class="card-balance-quota-list">
                  <div
                    v-for="item in balanceQuotaItems"
                    :key="`balance-${item.key}`"
                    class="card-balance-quota"
                    :title="quotaTooltip(item)"
                  >
                    <span
                      v-if="showBalanceItemLabel(item)"
                      class="card-balance-quota__item-label"
                    >
                      {{ item.label }}
                    </span>
                    <span class="card-balance-quota__value-row">
                      <span class="card-balance-quota__label">
                        {{ t('components.main.providers.quotaRemainingLabel') }}
                      </span>
                      <span
                        class="card-balance-quota__amount"
                        :class="balanceQuotaAmountClass(item)"
                      >
                        {{ formatBalanceRemainingValue(item) }}
                      </span>
                    </span>
                    <span
                      v-if="quotaItemNote(item)"
                      class="card-balance-quota__note"
                    >
                      {{ quotaItemNote(item) }}
                    </span>
                  </div>
                </div>
                <div class="card-balance-quota-panel__meta">
                  <span class="card-balance-quota-panel__updated">
                    <svg viewBox="0 0 24 24" aria-hidden="true">
                      <path
                        d="M12 7.25v5.15l3.1 1.85M20 12a8 8 0 11-2.34-5.66"
                        fill="none"
                        stroke="currentColor"
                        stroke-width="1.7"
                        stroke-linecap="round"
                        stroke-linejoin="round"
                      />
                    </svg>
                    {{ balanceQuotaUpdatedAtLabel }}
                  </span>
                  <button
                    class="card-balance-quota-panel__refresh"
                    type="button"
                    :disabled="viewModel.quotaRefreshing"
                    :title="t('components.main.providers.quotaRefresh')"
                    :aria-label="t('components.main.providers.quotaRefreshAriaLabel', { name: viewModel.card.name })"
                    @click.stop="$emit('refresh-provider-quota')"
                  >
                    <svg
                      viewBox="0 0 24 24"
                      aria-hidden="true"
                      :class="{ 'is-spinning': viewModel.quotaRefreshing }"
                    >
                      <path
                        d="M20 12a8 8 0 10-2.34 5.66M20 12v5m0-5h-5"
                        fill="none"
                        stroke="currentColor"
                        stroke-width="1.7"
                        stroke-linecap="round"
                        stroke-linejoin="round"
                      />
                    </svg>
                  </button>
                </div>
              </div>
              <div
                v-if="hasErrorQuotaItems"
                class="card-balance-quota-panel card-balance-quota-panel--error"
              >
                <div ref="quotaErrorPopoverRef" class="card-balance-quota-error">
                  <span class="sr-only" role="status" aria-live="polite">
                    {{ t('components.main.providers.quotaQueryFailed') }}
                  </span>
                  <button
                    ref="quotaErrorTriggerRef"
                    type="button"
                    class="card-balance-quota__error-trigger"
                    :title="quotaErrorDetailsAriaLabel"
                    :aria-label="quotaErrorDetailsAriaLabel"
                    :aria-expanded="quotaErrorPopoverOpen"
                    :aria-controls="quotaErrorDetailsId"
                    aria-haspopup="dialog"
                    @click.stop="toggleQuotaErrorPopover"
                  >
                    <span
                      class="card-balance-quota__error-icon"
                      aria-hidden="true"
                    >
                      !
                    </span>
                  </button>
                  <Teleport to="body">
                    <div
                      v-if="quotaErrorPopoverOpen"
                      :id="quotaErrorDetailsId"
                      ref="quotaErrorPopoverPanelRef"
                      class="card-balance-quota__error-popover"
                      :class="[
                        `card-balance-quota__error-popover--${quotaErrorPopoverPlacement}`,
                        { 'theme-dark': resolvedTheme === 'dark' },
                      ]"
                      :style="quotaErrorPopoverStyle"
                      role="dialog"
                      tabindex="-1"
                      :aria-label="t('components.main.providers.quotaErrorDetails')"
                      @click.stop
                      @pointerdown.stop
                      @dragstart.stop.prevent
                    >
                      <strong class="card-balance-quota__error-popover-title">
                        {{ t('components.main.providers.quotaErrorDetails') }}
                      </strong>
                      <div
                        v-for="item in errorQuotaItems"
                        :key="`error-detail-${item.key}`"
                        class="card-balance-quota__error-detail"
                      >
                        <span
                          v-if="showErrorItemLabel(item)"
                          class="card-balance-quota__item-label card-balance-quota__item-label--error"
                        >
                          {{ item.label }}
                        </span>
                        <span class="card-balance-quota__error-text">
                          {{ quotaItemNote(item) || t('components.main.providers.quotaQueryFailed') }}
                        </span>
                      </div>
                    </div>
                  </Teleport>
                </div>
                <div class="card-balance-quota-panel__meta">
                  <span class="card-balance-quota-panel__updated">
                    <svg viewBox="0 0 24 24" aria-hidden="true">
                      <path
                        d="M12 7.25v5.15l3.1 1.85M20 12a8 8 0 11-2.34-5.66"
                        fill="none"
                        stroke="currentColor"
                        stroke-width="1.7"
                        stroke-linecap="round"
                        stroke-linejoin="round"
                      />
                    </svg>
                    {{ errorQuotaUpdatedAtLabel }}
                  </span>
                  <button
                    class="card-balance-quota-panel__refresh"
                    type="button"
                    :disabled="viewModel.quotaRefreshing"
                    :title="t('components.main.providers.quotaRefresh')"
                    :aria-label="t('components.main.providers.quotaRefreshAriaLabel', { name: viewModel.card.name })"
                    @click.stop="$emit('refresh-provider-quota')"
                  >
                    <svg
                      viewBox="0 0 24 24"
                      aria-hidden="true"
                      :class="{ 'is-spinning': viewModel.quotaRefreshing }"
                    >
                      <path
                        d="M20 12a8 8 0 10-2.34 5.66M20 12v5m0-5h-5"
                        fill="none"
                        stroke="currentColor"
                        stroke-width="1.7"
                        stroke-linecap="round"
                        stroke-linejoin="round"
                      />
                    </svg>
                  </button>
                </div>
              </div>
              <div
                v-if="progressQuotaSectionMode === 'inline-with-performance'"
                class="card-performance-quotas"
              >
                <span
                  v-for="item in progressQuotaItems"
                  :key="item.key"
                  :class="['card-quota-item', `card-quota-item--${item.key}`, quotaProgressClass(item)]"
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
                  <span v-if="showQuotaCountdown(item)" class="quota-countdown">{{ item.countdownLabel }}</span>
                </span>
              </div>
            </div>
          </template>
          <div
            v-if="showStandaloneQuotaStatusRow"
            class="card-metrics-line card-metrics-line-performance"
          >
            <div
              v-if="hasBalanceQuotaItems"
              class="card-balance-quota-panel card-balance-quota-panel--standalone"
            >
              <div class="card-balance-quota-list">
                <div
                  v-for="item in balanceQuotaItems"
                  :key="`balance-standalone-${item.key}`"
                  class="card-balance-quota"
                  :title="quotaTooltip(item)"
                >
                  <span
                    v-if="showBalanceItemLabel(item)"
                    class="card-balance-quota__item-label"
                  >
                    {{ item.label }}
                  </span>
                  <span class="card-balance-quota__value-row">
                    <span class="card-balance-quota__label">
                      {{ t('components.main.providers.quotaRemainingLabel') }}
                    </span>
                    <span
                      class="card-balance-quota__amount"
                      :class="balanceQuotaAmountClass(item)"
                    >
                      {{ formatBalanceRemainingValue(item) }}
                    </span>
                  </span>
                  <span
                    v-if="quotaItemNote(item)"
                    class="card-balance-quota__note"
                  >
                    {{ quotaItemNote(item) }}
                  </span>
                </div>
              </div>
              <div class="card-balance-quota-panel__meta">
                <span class="card-balance-quota-panel__updated">
                  <svg viewBox="0 0 24 24" aria-hidden="true">
                    <path
                      d="M12 7.25v5.15l3.1 1.85M20 12a8 8 0 11-2.34-5.66"
                      fill="none"
                      stroke="currentColor"
                      stroke-width="1.7"
                      stroke-linecap="round"
                      stroke-linejoin="round"
                    />
                  </svg>
                  {{ balanceQuotaUpdatedAtLabel }}
                </span>
                <button
                  class="card-balance-quota-panel__refresh"
                  type="button"
                  :disabled="viewModel.quotaRefreshing"
                  :title="t('components.main.providers.quotaRefresh')"
                  :aria-label="t('components.main.providers.quotaRefreshAriaLabel', { name: viewModel.card.name })"
                  @click.stop="$emit('refresh-provider-quota')"
                >
                  <svg
                    viewBox="0 0 24 24"
                    aria-hidden="true"
                    :class="{ 'is-spinning': viewModel.quotaRefreshing }"
                  >
                    <path
                      d="M20 12a8 8 0 10-2.34 5.66M20 12v5m0-5h-5"
                      fill="none"
                      stroke="currentColor"
                      stroke-width="1.7"
                      stroke-linecap="round"
                      stroke-linejoin="round"
                    />
                  </svg>
                </button>
              </div>
            </div>
            <div
              v-if="hasErrorQuotaItems"
              class="card-balance-quota-panel card-balance-quota-panel--standalone card-balance-quota-panel--error"
            >
              <div ref="quotaErrorPopoverRef" class="card-balance-quota-error">
                <span class="sr-only" role="status" aria-live="polite">
                  {{ t('components.main.providers.quotaQueryFailed') }}
                </span>
                <button
                  ref="quotaErrorTriggerRef"
                  type="button"
                  class="card-balance-quota__error-trigger"
                  :title="quotaErrorDetailsAriaLabel"
                  :aria-label="quotaErrorDetailsAriaLabel"
                  :aria-expanded="quotaErrorPopoverOpen"
                  :aria-controls="quotaErrorDetailsId"
                  aria-haspopup="dialog"
                  @click.stop="toggleQuotaErrorPopover"
                >
                  <span
                    class="card-balance-quota__error-icon"
                    aria-hidden="true"
                  >
                    !
                  </span>
                </button>
                <Teleport to="body">
                  <div
                    v-if="quotaErrorPopoverOpen"
                    :id="quotaErrorDetailsId"
                    ref="quotaErrorPopoverPanelRef"
                    class="card-balance-quota__error-popover"
                    :class="[
                      `card-balance-quota__error-popover--${quotaErrorPopoverPlacement}`,
                      { 'theme-dark': resolvedTheme === 'dark' },
                    ]"
                    :style="quotaErrorPopoverStyle"
                    role="dialog"
                    tabindex="-1"
                    :aria-label="t('components.main.providers.quotaErrorDetails')"
                    @click.stop
                    @pointerdown.stop
                    @dragstart.stop.prevent
                  >
                    <strong class="card-balance-quota__error-popover-title">
                      {{ t('components.main.providers.quotaErrorDetails') }}
                    </strong>
                    <div
                      v-for="item in errorQuotaItems"
                      :key="`error-detail-standalone-${item.key}`"
                      class="card-balance-quota__error-detail"
                    >
                      <span
                        v-if="showErrorItemLabel(item)"
                        class="card-balance-quota__item-label card-balance-quota__item-label--error"
                      >
                        {{ item.label }}
                      </span>
                      <span class="card-balance-quota__error-text">
                        {{ quotaItemNote(item) || t('components.main.providers.quotaQueryFailed') }}
                      </span>
                    </div>
                  </div>
                </Teleport>
              </div>
              <div class="card-balance-quota-panel__meta">
                <span class="card-balance-quota-panel__updated">
                  <svg viewBox="0 0 24 24" aria-hidden="true">
                    <path
                      d="M12 7.25v5.15l3.1 1.85M20 12a8 8 0 11-2.34-5.66"
                      fill="none"
                      stroke="currentColor"
                      stroke-width="1.7"
                      stroke-linecap="round"
                      stroke-linejoin="round"
                    />
                  </svg>
                  {{ errorQuotaUpdatedAtLabel }}
                </span>
                <button
                  class="card-balance-quota-panel__refresh"
                  type="button"
                  :disabled="viewModel.quotaRefreshing"
                  :title="t('components.main.providers.quotaRefresh')"
                  :aria-label="t('components.main.providers.quotaRefreshAriaLabel', { name: viewModel.card.name })"
                  @click.stop="$emit('refresh-provider-quota')"
                >
                  <svg
                    viewBox="0 0 24 24"
                    aria-hidden="true"
                    :class="{ 'is-spinning': viewModel.quotaRefreshing }"
                  >
                    <path
                      d="M20 12a8 8 0 10-2.34 5.66M20 12v5m0-5h-5"
                      fill="none"
                      stroke="currentColor"
                      stroke-width="1.7"
                      stroke-linecap="round"
                      stroke-linejoin="round"
                    />
                  </svg>
                </button>
              </div>
            </div>
          </div>
          <div
            v-if="progressQuotaSectionMode === 'standalone'"
            class="card-metrics-line card-metrics-line-performance"
          >
            <div class="card-performance-quotas">
              <span
                v-for="item in progressQuotaItems"
                :key="item.key"
                :class="['card-quota-item', `card-quota-item--${item.key}`, quotaProgressClass(item)]"
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
                <span v-if="showQuotaCountdown(item)" class="quota-countdown">{{ item.countdownLabel }}</span>
              </span>
            </div>
          </div>
        </div>

        <div
          v-if="viewModel.blacklistStatus && !viewModel.blacklistStatus.isBlacklisted && viewModel.blacklistStatus.blacklistLevel > 0"
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
      <div
        v-if="viewModel.card.quotaAutoDisabled"
        class="quota-auto-action"
        :title="t('components.main.providers.quotaAutoDisabledHint')"
      >
        <span class="quota-auto-status">{{ t('components.main.providers.quotaAutoDisabled') }}</span>
        <button type="button" class="quota-auto-button" @click.stop="$emit('temporarily-enable-quota-provider')">
          {{ t('components.main.providers.quotaTemporarilyEnable') }}
        </button>
      </div>
      <template v-else>
        <label class="mac-switch sm">
          <input
            type="checkbox"
            :checked="viewModel.card.enabled"
            @change="handleToggleEnabled"
          />
          <span></span>
        </label>
        <div v-if="viewModel.card.quotaAutoDisablePaused" class="quota-auto-action quota-auto-action--paused">
          <span class="quota-auto-status">{{ t('components.main.providers.quotaTemporarilyEnabled') }}</span>
          <button type="button" class="quota-auto-button" @click.stop="$emit('resume-quota-automation')">
            {{ t('components.main.providers.quotaResumeAutomation') }}
          </button>
        </div>
      </template>

      <span class="card-actions-divider" aria-hidden="true"></span>

      <div class="card-actions-toolbar">
        <button
          v-if="activeTab !== 'others' && activeTab !== 'opencode'"
          class="ghost-icon direct-apply-btn"
          :class="{ 'is-active': viewModel.isDirectApplied && !activeProxyState }"
          :disabled="directApplyDisabled"
          :data-tooltip="directApplyTooltip"
          type="button"
          @click.stop="!viewModel.isDirectApplied && !directApplyDisabled && $emit('direct-apply')"
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
          v-if="activeTab !== 'others' && activeTab !== 'opencode'"
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
          v-if="activeTab !== 'opencode'"
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
          v-if="activeTab !== 'others' && activeTab !== 'opencode'"
          :class="['ghost-icon', 'provider-log-btn', { 'ghost-icon-alert': hasUnreadErrorLogs }]"
          :data-tooltip="providerLogsButtonTooltip"
          type="button"
          @click="handleProviderLogsClick"
          @dblclick="handleProviderLogsDoubleClick"
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
          class="ghost-icon ghost-icon-danger ghost-icon-tooltip-end"
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
    </div>
  </article>
</template>

<script setup lang="ts">
import { computed, nextTick, onUnmounted, ref, watch, type ComponentPublicInstance, type CSSProperties } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ProviderCardViewModel, ProviderDragEndPayload, ProviderQuotaDisplayItem, ProviderTab, ResolvedTheme } from '../types'
import { formatQuotaUsagePercent, getQuotaProgressClass, getQuotaProgressPercent } from '../utils/providerQuotaDisplay'
import { resolveProviderCardQuotaSectionMode } from '../utils/providerCardQuotaVisibility'
import {
  calculateProviderQuotaErrorPopoverLayout,
  type ProviderQuotaErrorPopoverPlacement,
} from '../utils/providerQuotaErrorPopover'
import { resolveProviderQuotaCurrencyCode } from '../utils/providerQuotaValueFormat'
import {
  formatProviderQuotaRelativeUpdatedAt,
  getProviderQuotaBalanceTone,
  getProviderQuotaRemainingValue,
  getProviderQuotaVisibleNote,
  isProviderQuotaBalanceItem,
  isProviderQuotaErrorItem,
} from '../utils/providerQuotaCardDisplay'
import { isDirectApplyBlockedForProvider } from '../utils/providerDirectApply'
import { isHostedRouteActive } from '../utils/providerRoutingState'
import { shouldShowProviderLogBadge } from '../utils/providerLogBadge'
import { buildProviderSuccessRateTooltip } from '../utils/providerBlacklistDisplay'

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
  'temporarily-enable-quota-provider': []
  'resume-quota-automation': []
  'direct-apply': []
  configure: []
  'open-provider-data': []
  'open-model-list': []
  'open-provider-logs': []
  'mark-provider-logs-read': []
  'open-provider-cost-trend': []
  'open-concurrency-details': []
  'refresh-provider-quota': []
  duplicate: []
  remove: []
}>()

const { t, locale } = useI18n()

const resolveQuotaCurrencyFormatter = (unit?: string) => {
  const currencyCode = resolveProviderQuotaCurrencyCode(unit)
  if (currencyCode) {
    return new Intl.NumberFormat(locale.value || 'en', {
      style: 'currency',
      currency: currencyCode,
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    })
  }
  return null
}

const quotaProgressClass = (item: ProviderQuotaDisplayItem) => getQuotaProgressClass(item)

const quotaProgressWidth = (item: ProviderQuotaDisplayItem) => getQuotaProgressPercent(item)

const quotaUsagePercent = (item: ProviderQuotaDisplayItem) => formatQuotaUsagePercent(item)

const showQuotaCountdown = (item: ProviderQuotaDisplayItem) => Boolean(item.countdownLabel)

const blacklistTriggerLabel = (source?: string) => {
  if (source === 'request' || source === 'health' || source === 'legacy') {
    return t(`components.main.blacklist.trigger.${source}`)
  }
  return t('components.main.blacklist.trigger.unknown')
}

const successRateTooltip = computed(() => {
  const stats = props.viewModel.stats
  const counters = props.viewModel.blacklistCounters
  const summary = stats.state === 'ready'
    ? stats.successRateHint
    : stats.state === 'empty'
      ? t('components.main.providers.noData')
      : ''

  return buildProviderSuccessRateTooltip(summary, counters, t)
})

const blacklistPopoverOpen = ref(false)
const blacklistPopoverRef = ref<HTMLElement | null>(null)
const quotaErrorPopoverOpen = ref(false)
const quotaErrorPopoverRef = ref<HTMLElement | null>(null)
const quotaErrorTriggerRef = ref<HTMLButtonElement | null>(null)
const quotaErrorPopoverPanelRef = ref<HTMLElement | null>(null)
const quotaErrorPopoverPlacement = ref<ProviderQuotaErrorPopoverPlacement>('below')
const quotaErrorPopoverStyle = ref<CSSProperties>({ visibility: 'hidden' })
const hasOpenCardPopover = computed(() => blacklistPopoverOpen.value || quotaErrorPopoverOpen.value)

const quotaErrorDetailsId = computed(() => (
  `provider-quota-error-${props.activeTab}-${props.viewModel.card.id}`
))

const quotaErrorDetailsAriaLabel = computed(() => t(
  'components.main.providers.quotaErrorDetailsAriaLabel',
  { name: props.viewModel.card.name },
))

const closeBlacklistPopover = () => {
  blacklistPopoverOpen.value = false
}

const closeQuotaErrorPopover = (restoreFocus = false) => {
  const wasOpen = quotaErrorPopoverOpen.value
  quotaErrorPopoverOpen.value = false
  quotaErrorPopoverStyle.value = { visibility: 'hidden' }
  if (restoreFocus && wasOpen) {
    void nextTick(() => quotaErrorTriggerRef.value?.focus({ preventScroll: true }))
  }
}

const toggleBlacklistPopover = () => {
  closeQuotaErrorPopover()
  blacklistPopoverOpen.value = !blacklistPopoverOpen.value
}

const toggleQuotaErrorPopover = () => {
  closeBlacklistPopover()
  quotaErrorPopoverOpen.value = !quotaErrorPopoverOpen.value
}

const handleBlacklistPopoverPointerDown = (event: PointerEvent) => {
  if (!blacklistPopoverRef.value?.contains(event.target as Node)) {
    closeBlacklistPopover()
  }
}

const handleBlacklistPopoverKeydown = (event: KeyboardEvent) => {
  if (event.key === 'Escape') {
    closeBlacklistPopover()
  }
}

const stopBlacklistPopoverListeners = () => {
  document.removeEventListener('pointerdown', handleBlacklistPopoverPointerDown)
  document.removeEventListener('keydown', handleBlacklistPopoverKeydown)
}

const handleQuotaErrorPopoverPointerDown = (event: PointerEvent) => {
  const target = event.target as Node
  if (
    !quotaErrorPopoverRef.value?.contains(target)
    && !quotaErrorPopoverPanelRef.value?.contains(target)
  ) {
    closeQuotaErrorPopover()
  }
}

const handleQuotaErrorPopoverKeydown = (event: KeyboardEvent) => {
  if (event.key === 'Escape') {
    closeQuotaErrorPopover(true)
  }
}

const updateQuotaErrorPopoverPosition = () => {
  if (!quotaErrorPopoverOpen.value || typeof window === 'undefined') return
  const anchor = quotaErrorTriggerRef.value
  const panel = quotaErrorPopoverPanelRef.value
  if (!anchor?.isConnected || !panel?.isConnected) {
    closeQuotaErrorPopover()
    return
  }

  const anchorRect = anchor.getBoundingClientRect()
  const panelRect = panel.getBoundingClientRect()
  const layout = calculateProviderQuotaErrorPopoverLayout(
    {
      top: anchorRect.top,
      bottom: anchorRect.bottom,
      left: anchorRect.left,
      width: anchorRect.width,
      height: anchorRect.height,
    },
    {
      width: panelRect.width,
      height: panelRect.height,
    },
    {
      width: window.innerWidth,
      height: window.innerHeight,
    },
  )

  quotaErrorPopoverPlacement.value = layout.placement
  quotaErrorPopoverStyle.value = {
    top: `${layout.top}px`,
    left: `${layout.left}px`,
    width: `${layout.width}px`,
    maxHeight: `${layout.maxHeight}px`,
    visibility: 'visible',
  }
}

const stopQuotaErrorPopoverListeners = () => {
  document.removeEventListener('pointerdown', handleQuotaErrorPopoverPointerDown)
  document.removeEventListener('keydown', handleQuotaErrorPopoverKeydown)
  window.removeEventListener('scroll', updateQuotaErrorPopoverPosition, true)
  window.removeEventListener('resize', updateQuotaErrorPopoverPosition)
}

watch(blacklistPopoverOpen, (open) => {
  stopBlacklistPopoverListeners()
  if (open) {
    document.addEventListener('pointerdown', handleBlacklistPopoverPointerDown)
    document.addEventListener('keydown', handleBlacklistPopoverKeydown)
  }
})

watch(quotaErrorPopoverOpen, async (open) => {
  stopQuotaErrorPopoverListeners()
  if (open) {
    document.addEventListener('pointerdown', handleQuotaErrorPopoverPointerDown)
    document.addEventListener('keydown', handleQuotaErrorPopoverKeydown)
    window.addEventListener('scroll', updateQuotaErrorPopoverPosition, true)
    window.addEventListener('resize', updateQuotaErrorPopoverPosition)
    quotaErrorPopoverStyle.value = { visibility: 'hidden' }
    await nextTick()
    if (!quotaErrorPopoverOpen.value) return
    updateQuotaErrorPopoverPosition()
    quotaErrorPopoverPanelRef.value?.focus({ preventScroll: true })
  }
})

watch(
  () => [
    props.activeTab,
    props.viewModel.card.id,
    props.viewModel.card.providerRef,
    props.viewModel.card.name,
    props.viewModel.card.apiUrl,
  ],
  () => {
    closeBlacklistPopover()
    closeQuotaErrorPopover()
  },
)

watch(() => props.viewModel.blacklistStatus?.isBlacklisted, (isBlacklisted) => {
  if (!isBlacklisted) {
    closeBlacklistPopover()
  }
})

const handleUnblockAndReset = () => {
  closeBlacklistPopover()
  emit('unblock-and-reset')
}

const handleResetLevel = () => {
  closeBlacklistPopover()
  emit('reset-level')
}

const progressQuotaItems = computed(() => (
  props.viewModel.quotaDisplay.filter((item) => (
    !isProviderQuotaBalanceItem(item)
    && !isProviderQuotaErrorItem(item)
  ))
))

const balanceQuotaItems = computed(() => (
  props.viewModel.quotaDisplay.filter((item) => isProviderQuotaBalanceItem(item))
))

const errorQuotaItems = computed(() => (
  props.viewModel.quotaDisplay.filter((item) => isProviderQuotaErrorItem(item))
))

const progressQuotaSectionMode = computed(() => resolveProviderCardQuotaSectionMode(
  props.viewModel.stats,
  progressQuotaItems.value,
))

const hasBalanceQuotaItems = computed(() => balanceQuotaItems.value.length > 0)
const hasErrorQuotaItems = computed(() => errorQuotaItems.value.length > 0)
const hasQuotaStatusPanelItems = computed(() => (
  hasBalanceQuotaItems.value || hasErrorQuotaItems.value
))
const showStandaloneQuotaStatusRow = computed(() => (
  props.viewModel.stats.state !== 'ready' && hasQuotaStatusPanelItems.value
))

watch(hasErrorQuotaItems, (hasErrors) => {
  if (!hasErrors) {
    closeQuotaErrorPopover()
  }
})

const formatQuotaValue = (item: ProviderQuotaDisplayItem, value: number) => {
  const normalized = Number.isFinite(value) ? value : 0
  if (item.valueMode === 'count') {
    const formatted = new Intl.NumberFormat(locale.value || 'en', {
      maximumFractionDigits: Number.isInteger(normalized) ? 0 : 2,
    }).format(normalized)
    return item.unit?.trim() ? `${formatted} ${item.unit.trim()}` : formatted
  }

  const currencyFormatter = resolveQuotaCurrencyFormatter(item.unit)
  if (currencyFormatter) {
    return currencyFormatter.format(normalized)
  }

  const fallbackFormatted = new Intl.NumberFormat(locale.value || 'en', {
    maximumFractionDigits: Number.isInteger(normalized) ? 0 : 2,
  }).format(normalized)
  return item.unit?.trim() ? `${fallbackFormatted} ${item.unit.trim()}` : fallbackFormatted
}

const formatBalanceRemainingValue = (item: ProviderQuotaDisplayItem) => {
  if (item.unlimited === true) {
    return t('components.main.providers.quotaUnlimited')
  }
  const remaining = getProviderQuotaRemainingValue(item)
  const formatted = new Intl.NumberFormat(locale.value || 'en', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(remaining)
  return item.unit?.trim() ? `${formatted} ${item.unit.trim()}` : formatted
}

const showBalanceItemLabel = (item: ProviderQuotaDisplayItem) => {
  const normalizedLabel = `${item.label ?? ''}`.trim()
  const normalizedProviderName = `${props.viewModel.card.name ?? ''}`.trim()
  if (!normalizedLabel) return false
  if (balanceQuotaItems.value.length > 1) return true
  return normalizedLabel.toLowerCase() !== normalizedProviderName.toLowerCase()
}

const showErrorItemLabel = (item: ProviderQuotaDisplayItem) => {
  const normalizedLabel = `${item.label ?? ''}`.trim()
  if (!normalizedLabel) return false
  return errorQuotaItems.value.length > 1
}

const balanceQuotaAmountClass = (item: ProviderQuotaDisplayItem) => (
  `card-balance-quota__amount--${getProviderQuotaBalanceTone(item)}`
)

const quotaItemNote = (item: ProviderQuotaDisplayItem) => getProviderQuotaVisibleNote(item)

watch(
  () => errorQuotaItems.value.map((item) => `${item.key}:${quotaItemNote(item)}`).join('\u0000'),
  () => {
    if (quotaErrorPopoverOpen.value) {
      void nextTick(updateQuotaErrorPopoverPosition)
    }
  },
)

const balanceQuotaNow = ref(Date.now())
let balanceQuotaTimeTicker: ReturnType<typeof globalThis.setInterval> | undefined

const stopBalanceQuotaTimeTicker = () => {
  if (balanceQuotaTimeTicker !== undefined) {
    globalThis.clearInterval(balanceQuotaTimeTicker)
    balanceQuotaTimeTicker = undefined
  }
}

const startBalanceQuotaTimeTicker = () => {
  stopBalanceQuotaTimeTicker()
  balanceQuotaNow.value = Date.now()
  balanceQuotaTimeTicker = globalThis.setInterval(() => {
    balanceQuotaNow.value = Date.now()
  }, 30_000)
}

const balanceQuotaUpdatedAt = computed(() => (
  balanceQuotaItems.value.find((item) => Number.isFinite(item.queriedAt))?.queriedAt
))

const balanceQuotaUpdatedAtLabel = computed(() => formatProviderQuotaRelativeUpdatedAt(
  balanceQuotaUpdatedAt.value,
  balanceQuotaNow.value,
  t,
))

const errorQuotaUpdatedAt = computed(() => (
  errorQuotaItems.value.find((item) => Number.isFinite(item.queriedAt))?.queriedAt
))

const errorQuotaUpdatedAtLabel = computed(() => formatProviderQuotaRelativeUpdatedAt(
  errorQuotaUpdatedAt.value,
  balanceQuotaNow.value,
  t,
))

watch(hasQuotaStatusPanelItems, (enabled) => {
  if (enabled) {
    startBalanceQuotaTimeTicker()
    return
  }
  stopBalanceQuotaTimeTicker()
}, { immediate: true })

onUnmounted(() => {
  stopBlacklistPopoverListeners()
  stopQuotaErrorPopoverListeners()
  stopBalanceQuotaTimeTicker()
  clearProviderLogsClickTimer()
})

const quotaTooltip = (item: ProviderQuotaDisplayItem) => {
  const invalidMessage = `${item.invalidMessage ?? ''}`.trim()
  const extra = `${item.extra ?? ''}`.trim()
  const refreshErrorMessage = `${item.refreshErrorMessage ?? ''}`.trim()
  if (invalidMessage && item.total <= 0 && item.used <= 0) {
    return [item.label, refreshErrorMessage, invalidMessage, extra].filter(Boolean).join('\n')
  }

  if (isProviderQuotaBalanceItem(item)) {
    const remaining = formatBalanceRemainingValue(item)
    const baseTooltip = t('components.main.providers.quotaBalanceTooltip', {
      label: item.label,
      remaining,
    })
    return [baseTooltip, refreshErrorMessage, invalidMessage, extra].filter(Boolean).join('\n')
  }

  const used = formatQuotaValue(item, item.used)
  const total = formatQuotaValue(item, item.total)
  const baseTooltip = item.key === 'total'
    ? t('components.main.providers.quotaTooltipNoReset', { label: item.label, used, total })
    : item.nextReset && item.countdownLabel
      ? t('components.main.providers.quotaTooltip', { label: item.label, used, total, countdown: item.countdownLabel })
      : t('components.main.providers.quotaTooltipNoCountdown', { label: item.label, used, total })

  return [baseTooltip, refreshErrorMessage, invalidMessage, extra].filter(Boolean).join('\n')
}

type ApiFormatBadgeMeta = {
  format: 'openai_chat' | 'openai_responses'
  title: string
}

type AnthropicCacheTTLBadgeMeta = {
  ttl: '5m' | '1h'
  label: string
  title: string
}

const directApplyTooltip = computed(() => {
  if (props.viewModel.card.quotaAutoDisabled) {
    return t('components.main.providers.quotaAutoDisabledHint')
  }
  if (props.activeProxyState) {
    return t('components.main.directApply.proxyEnabled')
  }
  if (directApplyBlockedByProvider.value) {
    return t('components.main.directApply.requiresHostedRouting')
  }
  if (props.viewModel.isDirectApplied) {
    return t('components.main.directApply.inUse')
  }
  return t('components.main.directApply.title')
})

const directApplyBlockedByProvider = computed(() => (
  isDirectApplyBlockedForProvider(props.activeTab, props.viewModel.card)
))

const directApplyDisabled = computed(() => (
  props.activeProxyState || directApplyBlockedByProvider.value
))

const apiFormatBadge = computed<ApiFormatBadgeMeta | null>(() => {
  if (props.activeTab !== 'claude') return null

  switch (props.viewModel.card.apiFormat || 'anthropic') {
    case 'openai_chat':
      return {
        format: 'openai_chat',
        title: t('components.main.providers.apiFormatOpenAIChatHint'),
      }
    case 'openai_responses':
      return {
        format: 'openai_responses',
        title: t('components.main.providers.apiFormatOpenAIResponsesHint'),
      }
    default:
      return null
  }
})

const anthropicCacheTTLBadge = computed<AnthropicCacheTTLBadgeMeta | null>(() => {
  if (props.activeTab !== 'claude') return null
  if ((props.viewModel.card.apiFormat || 'anthropic') !== 'anthropic') return null
  const ttl = props.viewModel.card.anthropicCacheTTL
  if (ttl !== '5m' && ttl !== '1h') return null

  return {
    ttl,
    label: `TTL ${ttl}`,
    title: t('components.main.providers.anthropicCacheTTLHint', { ttl }),
  }
})

const hostedSelectionActive = computed(() => isHostedRouteActive({
  activeProxyState: props.activeProxyState,
  isLastUsed: props.viewModel.isLastUsed,
  enabled: props.viewModel.card.enabled,
  apiUrl: props.viewModel.card.apiUrl,
  apiKey: props.viewModel.card.apiKey,
  isBlacklisted: props.viewModel.blacklistStatus?.isBlacklisted === true,
}))

const isCurrentlyActive = computed(() => (
  hostedSelectionActive.value || (!props.activeProxyState && props.viewModel.isDirectApplied)
))

const hasUnreadErrorLogs = computed(() => (
  shouldShowProviderLogBadge(
    props.viewModel.card.hideLogBadge,
    props.viewModel.stats.hasUnreadErrorLogs,
  )
))

const providerLogsButtonTooltip = computed(() => (
  hasUnreadErrorLogs.value
    ? t('components.main.providerLogs.buttonTooltipWithUnread')
    : t('components.main.providerLogs.buttonTooltip')
))

let providerLogsClickTimer: number | undefined

const clearProviderLogsClickTimer = () => {
  if (providerLogsClickTimer === undefined) return
  window.clearTimeout(providerLogsClickTimer)
  providerLogsClickTimer = undefined
}

const handleProviderLogsClick = () => {
  clearProviderLogsClickTimer()
  providerLogsClickTimer = window.setTimeout(() => {
    providerLogsClickTimer = undefined
    emit('open-provider-logs')
  }, 220)
}

const handleProviderLogsDoubleClick = () => {
  clearProviderLogsClickTimer()
  if (!hasUnreadErrorLogs.value) {
    emit('open-provider-logs')
    return
  }
  emit('mark-provider-logs-read')
}

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

const concurrencyStatusBadge = computed(() => {
  const status = props.viewModel.concurrencyStatus
  if (!status) return null
  const activeRequests = status.activeRequests || 0
  const limit = typeof status.limit === 'number' && Number.isFinite(status.limit)
    ? Math.max(0, status.limit)
    : undefined
  const maxLabel = limit === undefined ? '∞' : `${limit}`
  const limited = props.viewModel.concurrencyLimitEnabled && limit !== undefined
  return {
    label: t('components.main.concurrency.badge', {
      current: activeRequests,
      max: maxLabel,
    }),
    title: t('components.main.concurrency.badgeHint', {
      current: activeRequests,
      max: maxLabel,
    }),
    active: activeRequests > 0,
    overflow: limited && activeRequests >= limit,
  }
})

const handleToggleEnabled = (event: Event) => {
  emit('toggle-enabled', (event.target as HTMLInputElement).checked)
}

const handleDragStart = (event: DragEvent) => {
  if (props.viewModel.card.quotaAutoDisabled) {
    event.preventDefault()
    return
  }
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

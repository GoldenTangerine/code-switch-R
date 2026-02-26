<template>
  <div class="logs-page">
    <div class="logs-header-bar">
      <div class="logs-header">
        <BaseButton variant="outline" type="button" @click="backToHome">
          {{ t('components.logs.back') }}
        </BaseButton>
        <div class="refresh-indicator">
          <BaseButton variant="outline" size="sm" :disabled="storageLoading" @click="openStorageModal">
            {{ t('components.logs.storage.title') }}
          </BaseButton>
          <span>{{ t('components.logs.nextRefresh', { seconds: countdown }) }}</span>
          <BaseButton size="sm" :disabled="loading" @click="manualRefresh">
            {{ t('components.logs.refresh') }}
          </BaseButton>
        </div>
      </div>
    </div>

    <div class="logs-content">
      <div class="logs-controls">
        <form class="logs-filter-row" @submit.prevent="applyFilters">
            <div class="filter-fields">
              <label class="filter-field">
                <span>{{ t('components.logs.filters.platform') }}</span>
                <select v-model="filters.platform" class="mac-select">
                  <option value="">{{ t('components.logs.filters.allPlatforms') }}</option>
                  <option value="claude">Claude</option>
                  <option value="codex">Codex</option>
                  <option value="gemini">Gemini</option>
                </select>
              </label>
              <label class="filter-field">
                <span>{{ t('components.logs.filters.provider') }}</span>
                <select v-model="filters.provider" class="mac-select">
                  <option value="">{{ t('components.logs.filters.allProviders') }}</option>
                  <option v-for="provider in providerOptions" :key="provider.value" :value="provider.value">
                    {{ provider.label }}
                  </option>
                </select>
              </label>
              <label class="filter-field">
                <span>{{ t('components.logs.filters.dateType') }}</span>
                <select v-model="filters.dateType" class="mac-select">
                  <option value="all">{{ t('components.logs.filters.dateTypeAll') }}</option>
                  <option value="today">{{ t('components.logs.filters.dateTypeToday') }}</option>
                  <option value="year">{{ t('components.logs.filters.dateTypeYear') }}</option>
                  <option value="month">{{ t('components.logs.filters.dateTypeMonth') }}</option>
                  <option value="day">{{ t('components.logs.filters.dateTypeDay') }}</option>
                  <option value="range">{{ t('components.logs.filters.dateTypeRange') }}</option>
                </select>
              </label>
              <div class="filter-query-cell">
                <BaseButton
                  size="sm"
                  type="submit"
                  :disabled="loading || !isFilterValid"
                >
                  {{ t('components.logs.query') }}
                </BaseButton>
              </div>

              <label v-if="filters.dateType === 'year'" class="filter-field">
                <span>{{ t('components.logs.filters.year') }}</span>
                <VueDatePicker
                  v-model="yearPickerValue"
                  class="logs-date-picker"
                  :dark="isDarkTheme"
                  :locale="dateFnsLocale"
                  year-picker
                  auto-apply
                  :text-input="false"
                  :year-range="yearPickerRange"
                  :input-attrs="{ hideInputIcon: true }"
                  :ui="datePickerUi"
                  :formats="{ input: 'yyyy' }"
                  placeholder="YYYY"
                />
              </label>
              <label v-else-if="filters.dateType === 'month'" class="filter-field">
                <span>{{ t('components.logs.filters.month') }}</span>
                <VueDatePicker
                  v-model="monthPickerValue"
                  class="logs-date-picker"
                  :dark="isDarkTheme"
                  :locale="dateFnsLocale"
                  month-picker
                  auto-apply
                  :text-input="false"
                  :year-range="yearPickerRange"
                  :input-attrs="{ hideInputIcon: true }"
                  :ui="datePickerUi"
                  :formats="{ input: 'yyyy-MM' }"
                  placeholder="YYYY-MM"
                />
              </label>
              <label v-else-if="filters.dateType === 'day'" class="filter-field">
                <span>{{ t('components.logs.filters.day') }}</span>
                <VueDatePicker
                  v-model="dayPickerValue"
                  class="logs-date-picker"
                  :dark="isDarkTheme"
                  :locale="dateFnsLocale"
                  auto-apply
                  :text-input="false"
                  :input-attrs="{ hideInputIcon: true }"
                  :ui="datePickerUi"
                  :formats="{ input: 'yyyy-MM-dd' }"
                  placeholder="YYYY-MM-DD"
                />
              </label>
              <label v-else-if="filters.dateType === 'range'" class="filter-field">
                <span>{{ t('components.logs.filters.range') }}</span>
                <VueDatePicker
                  v-model="rangePickerValue"
                  class="logs-date-picker"
                  :dark="isDarkTheme"
                  :locale="dateFnsLocale"
                  :range="rangePickerConfig"
                  :multi-calendars="2"
                  auto-apply
                  :text-input="false"
                  :input-attrs="{ hideInputIcon: true }"
                  :ui="datePickerUi"
                  :formats="{ input: formatRangeInput }"
                  :placeholder="t('components.logs.filters.range')"
                />
              </label>
            </div>
          </form>
      </div>

        <section class="logs-summary" v-if="statsCards.length">
      <article
        v-for="card in statsCards"
        :key="card.key"
        :class="['summary-card', { 'summary-card--clickable': card.key === 'cost' || card.key === 'tokens' }]"
        @click="handleCardClick(card.key)"
      >
        <div class="summary-card__label">{{ card.label }}</div>
        <div class="summary-card__value">
          {{ card.value }}
          <span v-if="card.subValue" class="summary-card__sub-value">({{ card.subValue }})</span>
        </div>
        <div class="summary-card__hint">{{ card.hint }}</div>
      </article>
    </section>

        <section class="logs-model-share">
          <div class="logs-model-share__title">{{ t('components.logs.modelShare.title') }}</div>
          <div v-if="modelShareRows.length" class="logs-model-share__body">
            <div class="logs-model-share__chart-wrap">
              <Doughnut :data="modelShareChartData" :options="modelShareChartOptions" />
            </div>

            <div class="logs-model-share__table">
              <div class="logs-model-share__table-head">
                <span>{{ t('components.logs.modelShare.model') }}</span>
                <span>{{ t('components.logs.modelShare.requests') }}</span>
                <span>{{ t('components.logs.modelShare.tokens') }}</span>
                <span>{{ t('components.logs.modelShare.cost') }}</span>
              </div>
              <div class="logs-model-share__table-body">
                <div v-for="item in modelShareRows" :key="item.model" class="logs-model-share__table-row">
                  <span class="logs-model-share__model">
                    <span class="logs-model-share__dot" :style="{ backgroundColor: item.color }"></span>
                    <span class="logs-model-share__model-name">{{ item.model }}</span>
                  </span>
                  <span>{{ formatNumber(item.requests) }}</span>
                  <span>{{ formatTokenNumber(item.tokens) }}</span>
                  <span class="logs-model-share__cost">{{ formatCurrency(item.cost) }}</span>
                </div>
              </div>
            </div>
          </div>
          <div v-else class="logs-model-share__empty">
            {{ t('components.logs.modelShare.empty') }}
          </div>
        </section>

        <section class="logs-chart">
          <Line :data="chartData" :options="chartOptions" />
        </section>

        <section class="logs-table-wrapper">
          <table class="logs-table">
            <thead>
              <tr>
                <th class="col-time">{{ t('components.logs.table.time') }}</th>
                <th class="col-platform">{{ t('components.logs.table.platform') }}</th>
                <th class="col-provider">{{ t('components.logs.table.provider') }}</th>
                <th class="col-model">{{ t('components.logs.table.model') }}</th>
                <th class="col-verify">{{ t('components.logs.table.verify') }}</th>
                <th class="col-http">{{ t('components.logs.table.httpCode') }}</th>
                <th class="col-stream">{{ t('components.logs.table.stream') }}</th>
                <th class="col-duration">{{ t('components.logs.table.duration') }}</th>
                <th class="col-cost">{{ t('components.logs.table.cost') }}</th>
                <th class="col-tokens">{{ t('components.logs.table.tokens') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="item in pagedLogs" :key="item.id">
                <td>{{ formatTime(item.created_at) }}</td>
                <td>{{ item.platform || '—' }}</td>
                <td>{{ item.provider || '—' }}</td>
                <td class="model-cell">
                  <span
                    class="model-name model-meta-trigger"
                    tabindex="0"
                    aria-haspopup="true"
                    :aria-label="formatModelInfoAriaLabel(item)"
                    :aria-describedby="logInfoTooltip.visible ? 'logs-table-info-tooltip' : undefined"
                    @mouseenter="scheduleShowModelInfoTooltip(item, $event)"
                    @mousemove="moveLogInfoTooltip($event)"
                    @mouseleave="hideLogInfoTooltip"
                    @focus="showModelInfoTooltip(item, $event)"
                    @blur="hideLogInfoTooltip"
                    @keydown.esc="hideLogInfoTooltipImmediately"
                  >
                    {{ item.model || '—' }}
                  </span>
                </td>
                <td class="verify-cell">
                  <span
                    :class="['verify-tag', `verify-${resolveModelVerifyStatus(item)}`, 'verify-meta-trigger']"
                    tabindex="0"
                    aria-haspopup="true"
                    :aria-label="formatVerifyInfoAriaLabel(item)"
                    :aria-describedby="logInfoTooltip.visible ? 'logs-table-info-tooltip' : undefined"
                    @mouseenter="scheduleShowVerifyInfoTooltip(item, $event)"
                    @mousemove="moveLogInfoTooltip($event)"
                    @mouseleave="hideLogInfoTooltip"
                    @focus="showVerifyInfoTooltip(item, $event)"
                    @blur="hideLogInfoTooltip"
                    @keydown.esc="hideLogInfoTooltipImmediately"
                  >
                    {{ formatModelVerifyStatus(item) }}
                  </span>
                </td>
                <td :class="['code', httpCodeClass(item.http_code)]">{{ item.http_code }}</td>
                <td><span :class="['stream-tag', item.is_stream ? 'on' : 'off']">{{ formatStream(item.is_stream) }}</span></td>
                <td><span :class="['duration-tag', durationColor(item.duration_sec)]">{{ formatDuration(item.duration_sec) }}</span></td>
                <td class="cost-cell">
                  <span
                    class="cost-cell__value"
                    tabindex="0"
                    aria-haspopup="true"
                    :aria-label="formatCostAriaLabel(item)"
                    :aria-describedby="costTooltip.visible ? 'logs-table-cost-tooltip' : undefined"
                    @mouseenter="scheduleShowCostTooltip(item, $event)"
                    @mousemove="moveCostTooltip($event)"
                    @mouseleave="hideCostTooltip"
                    @focus="showCostTooltip(item, $event)"
                    @blur="hideCostTooltip"
                    @keydown.esc="hideCostTooltipImmediately"
                  >
                    {{ formatCurrency(item.total_cost) }}
                  </span>
                </td>
                <td class="token-cell">
                  <div>
                    <span class="token-label">{{ t('components.logs.tokenLabels.input') }}</span>
                    <span class="token-value">{{ formatTokenNumber(item.input_tokens) }}</span>
                  </div>
                  <div>
                    <span class="token-label">{{ t('components.logs.tokenLabels.output') }}</span>
                    <span class="token-value">{{ formatTokenNumber(item.output_tokens) }}</span>
                  </div>
                  <div>
                    <span class="token-label">{{ t('components.logs.tokenLabels.reasoning') }}</span>
                    <span class="token-value">{{ formatTokenNumber(item.reasoning_tokens) }}</span>
                  </div>
                  <div class="token-breakdown-row">
                    <span class="token-label">{{ t('components.logs.tokenLabels.cacheWrite') }}</span>
                    <span class="token-value">{{ formatTokenNumber(item.cache_create_tokens) }}</span>
                    <span v-if="hasCacheCreateDetail(item)" class="cache-create-badges">
                      <span v-if="resolveEphemeral5mTokens(item) > 0" class="cache-create-badge cache-create-badge--5m">
                        {{ t('components.logs.tokenLabels.cacheWrite5m') }} {{ formatTokenNumber(resolveEphemeral5mTokens(item)) }}
                      </span>
                      <span v-if="resolveEphemeral1hTokens(item) > 0" class="cache-create-badge cache-create-badge--1h">
                        {{ t('components.logs.tokenLabels.cacheWrite1h') }} {{ formatTokenNumber(resolveEphemeral1hTokens(item)) }}
                      </span>
                    </span>
                  </div>
                  <div>
                    <span class="token-label">{{ t('components.logs.tokenLabels.cacheRead') }}</span>
                    <span class="token-value">{{ formatTokenNumber(item.cache_read_tokens) }}</span>
                  </div>
                </td>
              </tr>
              <tr v-if="!pagedLogs.length && !loading">
                <td colspan="10" class="empty">{{ t('components.logs.empty') }}</td>
              </tr>
            </tbody>
          </table>
          <p v-if="loading" class="empty">{{ t('components.logs.loading') }}</p>
        </section>

    <Teleport to="body">
      <div
        v-if="logInfoTooltip.visible && logInfoTooltip.detail"
        ref="logInfoTooltipRef"
        id="logs-table-info-tooltip"
        class="log-info-tooltip"
        :class="[`is-${logInfoTooltip.placement}`, `log-info-tooltip--${logInfoTooltip.detail.variant}`]"
        :style="{ left: `${logInfoTooltip.left}px`, top: `${logInfoTooltip.top}px` }"
        role="tooltip"
        @mouseenter="handleLogInfoTooltipMouseEnter"
        @mouseleave="handleLogInfoTooltipMouseLeave"
      >
        <p class="log-info-tooltip__title">{{ logInfoTooltip.detail.title }}</p>
        <div class="log-info-tooltip__rows">
          <div
            v-for="row in logInfoTooltip.detail.rows"
            :key="row.key"
            class="log-info-tooltip__row"
          >
            <span class="log-info-tooltip__label">{{ row.label }}</span>
            <span
              class="log-info-tooltip__value"
              :class="row.tone ? `tone-${row.tone}` : ''"
            >
              {{ row.value }}
            </span>
          </div>
        </div>
      </div>
    </Teleport>

    <Teleport to="body">
      <div
        v-if="costTooltip.visible && costTooltip.detail"
        ref="costTooltipRef"
        id="logs-table-cost-tooltip"
        class="cost-breakdown-tooltip"
        :class="`is-${costTooltip.placement}`"
        :style="{ left: `${costTooltip.left}px`, top: `${costTooltip.top}px` }"
        role="tooltip"
        @mouseenter="handleCostTooltipMouseEnter"
        @mouseleave="handleCostTooltipMouseLeave"
      >
        <p class="cost-breakdown-tooltip__model">
          {{ t('components.logs.costTooltip.pricingModel', { model: costTooltip.detail.pricingModel || '—' }) }}
        </p>

        <div v-if="costTooltip.detail.hasPricing" class="cost-breakdown-tooltip__prices">
          <div
            v-for="line in costTooltip.detail.priceLines"
            :key="line.key"
            class="cost-breakdown-tooltip__price-row"
          >
            <span class="cost-breakdown-tooltip__price-label">{{ line.label }}</span>
            <span class="cost-breakdown-tooltip__price-value">{{ line.value }}</span>
          </div>
        </div>

        <p class="cost-breakdown-tooltip__formula">{{ costTooltip.detail.formula }}</p>

        <p v-if="costTooltip.detail.note" class="cost-breakdown-tooltip__note">
          {{ costTooltip.detail.note }}
        </p>
        <p v-if="costTooltip.detail.recordedCostHint" class="cost-breakdown-tooltip__note">
          {{ costTooltip.detail.recordedCostHint }}
        </p>
      </div>
    </Teleport>

        <div class="logs-pagination">
          <span>{{ page }} / {{ totalPages }}</span>
          <div class="pagination-actions">
            <BaseButton variant="outline" size="sm" :disabled="page === 1 || loading" @click="prevPage">
              ‹
            </BaseButton>
            <BaseButton variant="outline" size="sm" :disabled="page >= totalPages || loading" @click="nextPage">
              ›
            </BaseButton>
          </div>
        </div>

        <!-- 清理确认弹窗 -->
        <BaseModal
          :open="storageClearConfirm.open"
          :title="t('components.logs.storage.confirmTitle')"
          variant="confirm"
          @close="closeStorageClearConfirm"
        >
          <div class="confirm-body">
            <p>{{ storageClearConfirmMessage }}</p>
          </div>
          <footer class="form-actions confirm-actions">
            <BaseButton variant="outline" type="button" :disabled="storageClearing" @click="closeStorageClearConfirm">
              {{ t('common.cancel') }}
            </BaseButton>
            <BaseButton variant="danger" type="button" :disabled="storageClearing" @click="confirmStorageClear">
              {{ storageClearing ? t('components.logs.storage.clearing') : storageClearConfirmActionLabel }}
            </BaseButton>
          </footer>
        </BaseModal>

        <!-- 日志存储弹窗 -->
        <BaseModal
          :open="storageModal.open"
          :title="t('components.logs.storage.title')"
          @close="closeStorageModal"
        >
          <div class="logs-storage-modal">
            <div class="logs-storage-actions logs-storage-actions--modal">
              <BaseButton variant="outline" size="sm" :disabled="storageLoading" @click="loadStorageStats">
                {{ t('components.logs.storage.refresh') }}
              </BaseButton>
            </div>
            <p v-if="storageLoading && !storageStats" class="cost-detail-loading">
              {{ t('components.logs.loading') }}
            </p>
            <div v-else-if="storageStats" class="mac-panel logs-storage-panel">
              <div class="logs-storage-db">
                <div class="logs-storage-db-line">
                  {{ t('components.logs.storage.db') }}：
                  {{ t('components.logs.storage.used') }} {{ formatBytes(storageStats.database.used_bytes) }}
                  /
                  {{ formatBytes(storageStats.database.total_bytes || storageStats.database.file_bytes) }}
                  <span v-if="storageStats.database.free_bytes">
                    （{{ t('components.logs.storage.free') }} {{ formatBytes(storageStats.database.free_bytes) }}）
                  </span>
                  <span v-if="storageStats.database.wal_bytes">
                    · {{ t('components.logs.storage.wal') }} {{ formatBytes(storageStats.database.wal_bytes) }}
                  </span>
                </div>
              </div>

              <div class="logs-storage-rows">
                <div class="logs-storage-row">
                  <div class="logs-storage-name">{{ t('components.logs.storage.requestLog') }}</div>
                  <div class="logs-storage-meta">
                    {{ t('components.logs.storage.rows', { count: storageStats.request_log.rows }) }}
                    · {{ formatBytes(storageStats.request_log.bytes, storageStats.request_log.rows) }}
                  </div>
                  <BaseButton
                    variant="outline"
                    size="sm"
                    :disabled="storageClearing"
                    @click="handleClearRequestLogs"
                  >
                    {{ storageClearing ? t('components.logs.storage.clearing') : t('components.logs.storage.clearRequestLog') }}
                  </BaseButton>
                </div>

                <div class="logs-storage-row">
                  <div class="logs-storage-name">{{ t('components.logs.storage.stats') }}</div>
                  <div class="logs-storage-meta">
                    {{ t('components.logs.storage.rows', { count: storageStats.stats_hour.rows + storageStats.stats_day.rows }) }}
                    · {{ formatBytes(
                      storageStats.stats_hour.bytes + storageStats.stats_day.bytes,
                      storageStats.stats_hour.rows + storageStats.stats_day.rows,
                    ) }}
                  </div>
                  <BaseButton
                    variant="outline"
                    size="sm"
                    :disabled="storageClearing"
                    @click="handleClearStats"
                  >
                    {{ storageClearing ? t('components.logs.storage.clearing') : t('components.logs.storage.clearStats') }}
                  </BaseButton>
                </div>
              </div>
            </div>
            <p v-else class="cost-detail-empty">
              {{ t('components.logs.storage.empty') }}
            </p>
          </div>
        </BaseModal>

        <!-- 金额明细弹窗 -->
        <BaseModal
          :open="costDetailModal.open"
          :title="t('components.logs.costDetail.title')"
          @close="closeCostDetailModal"
        >
          <div class="cost-detail-modal">
            <p v-if="costDetailModal.loading" class="cost-detail-loading">
              {{ t('components.logs.loading') }}
            </p>
            <div v-else-if="costDetailModal.data.length === 0" class="cost-detail-empty">
              {{ t('components.logs.costDetail.empty') }}
            </div>
            <ul v-else class="cost-detail-list">
              <li v-for="item in costDetailModal.data" :key="item.provider" class="cost-detail-item">
                <span class="cost-detail-item__name">{{ item.provider }}</span>
                <span class="cost-detail-item__value">{{ formatCurrency(item.cost_total) }}</span>
              </li>
            </ul>
          </div>
        </BaseModal>

        <!-- Token 明细弹窗 -->
        <BaseModal
          :open="tokenDetailModal.open"
          :title="t('components.logs.tokenDetail.title')"
          @close="closeTokenDetailModal"
        >
          <div class="token-detail-modal">
            <div class="token-detail-list">
              <div class="token-detail-item">
                <span class="token-detail-item__name">{{ t('components.logs.tokenLabels.input') }}</span>
                <span class="token-detail-item__value">{{ formatTokenNumber(stats?.input_tokens) }}</span>
              </div>
              <div class="token-detail-item">
                <span class="token-detail-item__name">{{ t('components.logs.tokenLabels.output') }}</span>
                <span class="token-detail-item__value">{{ formatTokenNumber(stats?.output_tokens) }}</span>
              </div>
              <div class="token-detail-item">
                <span class="token-detail-item__name">{{ t('components.logs.tokenLabels.cacheRead') }}</span>
                <span class="token-detail-item__value">{{ formatTokenNumber(stats?.cache_read_tokens) }}</span>
              </div>
            </div>
          </div>
        </BaseModal>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, reactive, ref, onMounted, watch, onUnmounted, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { VueDatePicker, type MonthModel } from '@vuepic/vue-datepicker'
import { enUS, zhCN } from 'date-fns/locale'
import BaseButton from '../common/BaseButton.vue'
import BaseModal from '../common/BaseModal.vue'
import { LoadProviders } from '../../../bindings/codeswitch/services/providerservice'
import { GetProviders as GetGeminiProviders } from '../../../bindings/codeswitch/services/geminiservice'
import {
  fetchRequestLogs,
  fetchLogProviderRefs,
  fetchLogStatsV2,
  fetchModelStatsV2,
  fetchProviderStatsV2,
  fetchLogStorageStats,
  clearRequestLogs,
  clearLogStats,
  type RequestLog,
  type LogStats,
  type LogStatsSeries,
  type LogPlatform,
  type ProviderDailyStat,
  type ModelUsageStat,
  type LogStorageStats,
  type LogProviderRef,
} from '../../services/logs'
import { listModelPricing, type ModelPricingRow } from '../../services/modelPricing'
import {
  Chart,
  CategoryScale,
  LinearScale,
  ArcElement,
  PointElement,
  LineElement,
  Tooltip,
  Legend,
} from 'chart.js'
import type { ChartOptions } from 'chart.js'
import { Doughnut, Line } from 'vue-chartjs'
import { showToast } from '../../utils/toast'
import { extractErrorMessage } from '../../utils/error'

Chart.register(CategoryScale, LinearScale, ArcElement, PointElement, LineElement, Tooltip, Legend)

const { t, locale } = useI18n()
const router = useRouter()

const dateFnsLocale = computed(() => (locale.value === 'zh' ? zhCN : enUS))

const isDarkTheme = ref(document.documentElement.classList.contains('dark'))
let themeObserver: MutationObserver | null = null

const startThemeObserver = () => {
  themeObserver?.disconnect()
  themeObserver = new MutationObserver(() => {
    isDarkTheme.value = document.documentElement.classList.contains('dark')
  })
  themeObserver.observe(document.documentElement, {
    attributes: true,
    attributeFilter: ['class'],
  })
}

const datePickerUi = {
  input: 'mac-input logs-date-picker-input',
  menu: 'mac-panel logs-date-picker-menu',
}

const yearPickerRange = computed<[number, number]>(() => {
  const currentYear = new Date().getFullYear()
  return [1970, Math.max(currentYear + 1, 1971)]
})

const rangePickerConfig = { partialRange: false } as const

const pad2 = (num: number) => num.toString().padStart(2, '0')

const formatDateYmd = (date: Date) =>
  `${date.getFullYear()}-${pad2(date.getMonth() + 1)}-${pad2(date.getDate())}`

const formatRangeInput = (dates: Array<Date | null>) => {
  const [start, end] = dates ?? []
  if (!start) return ''
  if (!end) return formatDateYmd(start)
  return `${formatDateYmd(start)} ~ ${formatDateYmd(end)}`
}

type LogDateFilterType = 'all' | 'today' | 'year' | 'month' | 'day' | 'range'

const logs = ref<RequestLog[]>([])
const stats = ref<LogStats | null>(null)
const modelStats = ref<ModelUsageStat[]>([])
const loading = ref(false)
const storageStats = ref<LogStorageStats | null>(null)
const storageLoading = ref(false)
const storageClearing = ref(false)
const storageModal = reactive({
  open: false,
})
const filters = reactive<{
  platform: LogPlatform | ''
  provider: string
  dateType: LogDateFilterType
  year: string
  month: string
  day: string
  rangeStart: string
  rangeEnd: string
}>({
  platform: '',
  provider: '',
  dateType: 'all',
  year: '',
  month: '',
  day: '',
  rangeStart: '',
  rangeEnd: '',
})
const page = ref(1)
const PAGE_SIZE = 15
type LogProviderOption = {
  value: string
  label: string
  providerId?: string
  providerName: string
}
const providerOptions = ref<LogProviderOption[]>([])
const PROVIDER_CONFIG_CACHE_TTL_MS = 60_000
const providerConfigCache = new Map<string, { loadedAt: number; options: LogProviderOption[] }>()
const statsSeries = computed<LogStatsSeries[]>(() => stats.value?.series ?? [])

type CostTooltipPlacement = 'above' | 'below'

type LogInfoTooltipTone = 'muted' | 'source-provider-api' | 'source-builtin' | 'source-none'
type LogInfoTooltipVariant = 'model' | 'verify'

type LogInfoTooltipRow = {
  key: string
  label: string
  value: string
  tone?: LogInfoTooltipTone
}

type LogInfoTooltipDetail = {
  title: string
  variant: LogInfoTooltipVariant
  rows: LogInfoTooltipRow[]
}

type CostTooltipPriceLine = {
  key: string
  label: string
  value: string
}

type TokenRatePriceLineOptions = {
  inputPerToken: number
  outputPerToken: number
  reasoningPerToken: number
  cacheCreatePerToken: number
  cacheReadPerToken: number
  includeCacheRead: boolean
  includeReasoning: boolean
  suffix?: string
  includeCacheMultiplierHint?: boolean
}

type CostTooltipDetail = {
  pricingModel: string
  hasPricing: boolean
  priceLines: CostTooltipPriceLine[]
  formula: string
  note: string
  recordedCostHint: string
}

const modelPricingRows = ref<ModelPricingRow[]>([])
const modelPricingLoading = ref(false)
const modelPricingLoaded = ref(false)
let modelPricingLoadingTask: Promise<void> | null = null

const logInfoTooltipRef = ref<HTMLElement | null>(null)
const logInfoTooltipAnchorRef = ref<HTMLElement | null>(null)
const logInfoTooltipRequestId = ref(0)
let logInfoTooltipHideTimer: number | null = null
let logInfoTooltipShowTimer: number | null = null
const logInfoTooltip = reactive<{
  visible: boolean
  left: number
  top: number
  placement: CostTooltipPlacement
  detail: LogInfoTooltipDetail | null
}>({
  visible: false,
  left: 0,
  top: 0,
  placement: 'above',
  detail: null,
})

const costTooltipRef = ref<HTMLElement | null>(null)
const costTooltipAnchorRef = ref<HTMLElement | null>(null)
const costTooltipRequestId = ref(0)
let costTooltipHideTimer: number | null = null
let costTooltipShowTimer: number | null = null
const costTooltip = reactive<{
  visible: boolean
  left: number
  top: number
  placement: CostTooltipPlacement
  detail: CostTooltipDetail | null
}>({
  visible: false,
  left: 0,
  top: 0,
  placement: 'above',
  detail: null,
})

const PER_MILLION_TOKENS = 1_000_000
const COST_TOOLTIP_DEFAULT_WIDTH = 460
const COST_TOOLTIP_DEFAULT_HEIGHT = 236
const COST_TOOLTIP_VERTICAL_OFFSET = 12
const COST_TOOLTIP_HORIZONTAL_MARGIN = 14
const COST_TOOLTIP_VERTICAL_MARGIN = 20
const COST_TOOLTIP_DIFF_EPSILON = 0.000001
const LOG_INFO_TOOLTIP_DEFAULT_WIDTH = 340
const LOG_INFO_TOOLTIP_DEFAULT_HEIGHT = 136
const LOG_INFO_TOOLTIP_VERTICAL_OFFSET = 10
const LOG_TOOLTIP_SHOW_DELAY_MS = 100

const formatBytes = (bytes?: number, rows?: number) => {
  const value = Number(bytes ?? 0)
  const count = Number(rows ?? 0)
  if (!Number.isFinite(value) || value < 0) return '—'
  if (value === 0 && Number.isFinite(count) && count > 0) return '—'
  if (value === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  let current = value
  let idx = 0
  while (current >= 1024 && idx < units.length - 1) {
    current /= 1024
    idx++
  }
  const digits = idx === 0 ? 0 : current >= 10 ? 1 : 2
  return `${current.toFixed(digits)} ${units[idx]}`
}

const toTimeLayout = (date: Date) => {
  const pad = (num: number) => num.toString().padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
}

const toDateParts = (value: string) => {
  const [y, m, d] = value.split('-').map((item) => Number(item))
  if (!Number.isFinite(y) || !Number.isFinite(m) || !Number.isFinite(d)) return null
  if (y <= 0 || m <= 0 || d <= 0) return null
  return { y, m, d }
}

const yearPickerValue = computed<number | null>({
  get() {
    const year = Number(filters.year)
    if (!Number.isFinite(year) || year < 1970 || year > 9999) return null
    return year
  },
  set(value) {
    filters.year = value == null ? '' : String(value)
  },
})

const monthPickerValue = computed<MonthModel | null>({
  get() {
    const match = String(filters.month || '').match(/^(\d{4})-(\d{2})$/)
    if (!match) return null
    const year = Number(match[1])
    const month = Number(match[2])
    if (!Number.isFinite(year) || !Number.isFinite(month) || month < 1 || month > 12) return null
    return { year, month: month - 1 }
  },
  set(value) {
    if (!value) {
      filters.month = ''
      return
    }
    const year = Number(value.year)
    const monthIndex = Number(value.month)
    if (!Number.isFinite(year) || !Number.isFinite(monthIndex) || monthIndex < 0 || monthIndex > 11) {
      filters.month = ''
      return
    }
    filters.month = `${year}-${pad2(monthIndex + 1)}`
  },
})

const dayPickerValue = computed<Date | null>({
  get() {
    if (!filters.day) return null
    const parts = toDateParts(filters.day)
    if (!parts) return null
    return new Date(parts.y, parts.m - 1, parts.d, 0, 0, 0, 0)
  },
  set(value) {
    filters.day = value ? formatDateYmd(value) : ''
  },
})

const rangePickerValue = computed<Date[] | null>({
  get() {
    if (!filters.rangeStart || !filters.rangeEnd) return null
    const startParts = toDateParts(filters.rangeStart)
    const endParts = toDateParts(filters.rangeEnd)
    if (!startParts || !endParts) return null
    const start = new Date(startParts.y, startParts.m - 1, startParts.d, 0, 0, 0, 0)
    const end = new Date(endParts.y, endParts.m - 1, endParts.d, 0, 0, 0, 0)
    return [start, end]
  },
  set(value) {
    if (!value || value.length < 2 || !value[0] || !value[1]) {
      filters.rangeStart = ''
      filters.rangeEnd = ''
      return
    }
    filters.rangeStart = formatDateYmd(value[0])
    filters.rangeEnd = formatDateYmd(value[1])
  },
})

const computeDateRange = () => {
  switch (filters.dateType) {
    case 'all':
      return { startAt: '', endAt: '' }
    case 'today': {
      const start = startOfTodayLocal()
      const end = new Date(start.getTime())
      end.setDate(end.getDate() + 1)
      return { startAt: toTimeLayout(start), endAt: toTimeLayout(end) }
    }
    case 'year': {
      const year = Number(filters.year)
      if (!Number.isFinite(year) || year < 1970 || year > 9999) return null
      const start = new Date(year, 0, 1, 0, 0, 0, 0)
      const end = new Date(year + 1, 0, 1, 0, 0, 0, 0)
      return { startAt: toTimeLayout(start), endAt: toTimeLayout(end) }
    }
    case 'month': {
      const match = String(filters.month || '').match(/^(\d{4})-(\d{2})$/)
      if (!match) return null
      const year = Number(match[1])
      const month = Number(match[2])
      if (!Number.isFinite(year) || !Number.isFinite(month) || month < 1 || month > 12) return null
      const start = new Date(year, month - 1, 1, 0, 0, 0, 0)
      const end = new Date(year, month, 1, 0, 0, 0, 0)
      return { startAt: toTimeLayout(start), endAt: toTimeLayout(end) }
    }
    case 'day': {
      if (!filters.day) return null
      const parts = toDateParts(filters.day)
      if (!parts) return null
      const start = new Date(parts.y, parts.m - 1, parts.d, 0, 0, 0, 0)
      const end = new Date(parts.y, parts.m - 1, parts.d + 1, 0, 0, 0, 0)
      return { startAt: toTimeLayout(start), endAt: toTimeLayout(end) }
    }
    case 'range': {
      if (!filters.rangeStart || !filters.rangeEnd) return null
      const startParts = toDateParts(filters.rangeStart)
      const endParts = toDateParts(filters.rangeEnd)
      if (!startParts || !endParts) return null
      const start = new Date(startParts.y, startParts.m - 1, startParts.d, 0, 0, 0, 0)
      const inclusiveEnd = new Date(endParts.y, endParts.m - 1, endParts.d, 0, 0, 0, 0)
      if (start.getTime() > inclusiveEnd.getTime()) return null
      const endExclusive = new Date(endParts.y, endParts.m - 1, endParts.d + 1, 0, 0, 0, 0)
      return { startAt: toTimeLayout(start), endAt: toTimeLayout(endExclusive) }
    }
    default:
      return null
  }
}

const isFilterValid = computed(() => {
  if (filters.dateType === 'all') return true
  return computeDateRange() != null
})

// 金额明细弹窗状态
const costDetailModal = reactive<{
  open: boolean
  loading: boolean
  data: ProviderDailyStat[]
}>({
  open: false,
  loading: false,
  data: [],
})

// Token 明细弹窗状态
const tokenDetailModal = reactive<{
  open: boolean
}>({
  open: false,
})

// 打开金额明细弹窗
const openCostDetailModal = async () => {
  costDetailModal.open = true
  costDetailModal.loading = true
  costDetailModal.data = []

  try {
    const range = computeDateRange()
    if (range == null) return
    const stats = await fetchProviderStatsV2({
      platform: filters.platform,
      provider: filters.provider,
      startAt: range.startAt,
      endAt: range.endAt,
    })
    // 按金额降序排序，过滤掉金额为 0 的
    costDetailModal.data = (stats ?? [])
      .filter(item => item.cost_total > 0)
      .sort((a, b) => b.cost_total - a.cost_total)
  } catch (error) {
    console.error('failed to load provider daily stats', error)
  } finally {
    costDetailModal.loading = false
  }
}

// 关闭金额明细弹窗
const closeCostDetailModal = () => {
  costDetailModal.open = false
}

// 处理卡片点击
const handleCardClick = (key: string) => {
  if (key === 'cost') {
    openCostDetailModal()
  } else if (key === 'tokens') {
    openTokenDetailModal()
  }
}

// 打开 Token 明细弹窗
const openTokenDetailModal = () => {
  tokenDetailModal.open = true
}

// 关闭 Token 明细弹窗
const closeTokenDetailModal = () => {
  tokenDetailModal.open = false
}

type ModelShareRow = {
  model: string
  requests: number
  tokens: number
  cost: number
  color: string
}

const MODEL_SHARE_COLORS = [
  '#4f7ee3',
  '#6990e8',
  '#7fb2ff',
  '#22c55e',
  '#38bdf8',
  '#f59e0b',
  '#a78bfa',
  '#f472b6',
  '#fb7185',
  '#94a3b8',
]

const normalizeModelShareKey = (value?: string) => String(value ?? '').trim().toLowerCase()

const modelShareRows = computed<ModelShareRow[]>(() => {
  const grouped = new Map<string, { model: string; requests: number; tokens: number; cost: number }>()
  for (const item of modelStats.value) {
    const model = String(item.model ?? '').trim() || '—'
    const normalizedKey = normalizeModelShareKey(model) || '—'
    const requests = Math.max(0, Math.round(safeNumber(item.total_requests)))
    const fallbackTokens = safeNumber(item.input_tokens) + safeNumber(item.output_tokens) + safeNumber(item.cache_read_tokens)
    const tokens = Math.max(0, Math.round(safeNumber(item.total_tokens) || fallbackTokens))
    const cost = safeNumber(item.cost_total)

    const current = grouped.get(normalizedKey) ?? { model, requests: 0, tokens: 0, cost: 0 }
    if (current.model === '—' && model !== '—') {
      current.model = model
    }
    current.requests += requests
    current.tokens += tokens
    current.cost += cost
    grouped.set(normalizedKey, current)
  }

  const rows = Array.from(grouped.values()).sort((a, b) => {
    if (b.tokens !== a.tokens) return b.tokens - a.tokens
    if (b.requests !== a.requests) return b.requests - a.requests
    return b.cost - a.cost
  })

  return rows.map((item, index) => ({
    ...item,
    color: MODEL_SHARE_COLORS[index % MODEL_SHARE_COLORS.length],
  }))
})

const modelShareTotalTokens = computed(() =>
  modelShareRows.value.reduce((sum, item) => sum + item.tokens, 0)
)

const modelShareChartData = computed(() => ({
  labels: modelShareRows.value.map(item => item.model),
  datasets: [
    {
      data: modelShareRows.value.map(item => item.tokens),
      backgroundColor: modelShareRows.value.map(item => item.color),
      borderWidth: 0,
      hoverOffset: 6,
    },
  ],
}))

const modelShareChartOptions: ChartOptions<'doughnut'> = {
  responsive: true,
  maintainAspectRatio: false,
  cutout: '50%',
  plugins: {
    legend: {
      display: false,
    },
    tooltip: {
      callbacks: {
        label: (context) => {
          const tokenValue = Number(context.raw ?? 0)
          const total = modelShareTotalTokens.value
          const ratio = total > 0 ? (tokenValue / total) * 100 : 0
          return `${context.label}: ${formatTokenNumber(tokenValue)} (${ratio.toFixed(1)}%)`
        },
      },
    },
  },
}

const parseLogDate = (value?: string) => {
  if (!value) return null
  const normalize = value.replace(' ', 'T')
  const attempts = [value, `${normalize}`, `${normalize}Z`]
  for (const candidate of attempts) {
    const parsed = new Date(candidate)
    if (!Number.isNaN(parsed.getTime())) {
      return parsed
    }
  }
  const match = value.match(/^(\d{4}-\d{2}-\d{2}) (\d{2}:\d{2}:\d{2}) ([+-]\d{4}) UTC$/)
  if (match) {
    const [, day, time, zone] = match
    const zoneFormatted = `${zone.slice(0, 3)}:${zone.slice(3)}`
    const parsed = new Date(`${day}T${time}${zoneFormatted}`)
    if (!Number.isNaN(parsed.getTime())) {
      return parsed
    }
  }
  return null
}

const hexToRgb = (hexColor: string) => {
  const normalized = String(hexColor ?? '').trim().replace('#', '')
  if (normalized.length !== 6) return null
  const red = Number.parseInt(normalized.slice(0, 2), 16)
  const green = Number.parseInt(normalized.slice(2, 4), 16)
  const blue = Number.parseInt(normalized.slice(4, 6), 16)
  if ([red, green, blue].some(channel => Number.isNaN(channel))) return null
  return { red, green, blue }
}

const buildAlphaColor = (hexColor: string, alpha: number) => {
  const normalizedAlpha = Number.isFinite(alpha)
    ? Math.max(0, Math.min(0.5, alpha))
    : 0
  const rgb = hexToRgb(hexColor)
  if (!rgb) return `rgba(148, 163, 184, ${normalizedAlpha})`
  return `rgba(${rgb.red}, ${rgb.green}, ${rgb.blue}, ${normalizedAlpha})`
}

const buildLineAreaGradient = (chart: Chart<'line'>, hexColor: string, alpha = 0.28) => {
  const area = chart.chartArea
  if (!area) return buildAlphaColor(hexColor, alpha)
  const gradient = chart.ctx.createLinearGradient(0, area.top, 0, area.bottom)
  gradient.addColorStop(0, buildAlphaColor(hexColor, alpha))
  gradient.addColorStop(1, buildAlphaColor(hexColor, 0))
  return gradient
}

const chartData = computed(() => {
  const series = statsSeries.value
  return {
    labels: series.map((item) => formatSeriesLabel(item.day)),
    datasets: [
      {
        label: t('components.logs.tokenLabels.cost'),
        data: series.map((item) => Number(((item.total_cost ?? 0)).toFixed(4))),
        borderColor: '#f97316',
        backgroundColor: (context) => buildLineAreaGradient(context.chart, '#f97316', 0.22),
        tension: 0.3,
        fill: 'origin',
        yAxisID: 'yCost',
      },
      {
        label: t('components.logs.tokenLabels.input'),
        data: series.map((item) => item.input_tokens ?? 0),
        borderColor: '#34d399',
        backgroundColor: (context) => buildLineAreaGradient(context.chart, '#34d399', 0.34),
        tension: 0.35,
        fill: 'origin',
      },
      {
        label: t('components.logs.tokenLabels.output'),
        data: series.map((item) => item.output_tokens ?? 0),
        borderColor: '#60a5fa',
        backgroundColor: (context) => buildLineAreaGradient(context.chart, '#60a5fa', 0.3),
        tension: 0.35,
        fill: 'origin',
      },
      {
        label: t('components.logs.tokenLabels.reasoning'),
        data: series.map((item) => item.reasoning_tokens ?? 0),
        borderColor: '#f472b6',
        backgroundColor: (context) => buildLineAreaGradient(context.chart, '#f472b6', 0.3),
        tension: 0.35,
        fill: 'origin',
      },
      {
        label: t('components.logs.tokenLabels.cacheWrite'),
        data: series.map((item) => item.cache_create_tokens ?? 0),
        borderColor: '#fbbf24',
        backgroundColor: (context) => buildLineAreaGradient(context.chart, '#fbbf24', 0.28),
        tension: 0.35,
        fill: 'origin',
      },
      {
        label: t('components.logs.tokenLabels.cacheRead'),
        data: series.map((item) => item.cache_read_tokens ?? 0),
        borderColor: '#38bdf8',
        backgroundColor: (context) => buildLineAreaGradient(context.chart, '#38bdf8', 0.26),
        tension: 0.35,
        fill: 'origin',
      },
    ],
  }
})

const resolveChartLegendColor = () => (isDarkTheme.value ? '#e2e8f0' : '#0f172a')
const resolveChartTickColor = () => (isDarkTheme.value ? '#94a3b8' : '#64748b')

const chartOptions = computed<ChartOptions<'line'>>(() => ({
  responsive: true,
  maintainAspectRatio: false,
  interaction: {
    mode: 'index',
    intersect: false,
  },
  plugins: {
    legend: {
      position: 'top',
      align: 'start',
      labels: {
        color: resolveChartLegendColor(),
        usePointStyle: true,
        pointStyle: 'circle',
        boxWidth: 8,
        boxHeight: 8,
        padding: 14,
        font: {
          size: 12,
          weight: 500,
        },
        generateLabels: (chart) => {
          const base = Chart.defaults.plugins.legend.labels.generateLabels(chart)
          return base.map((item) => {
            const datasetIndex = typeof item.datasetIndex === 'number' ? item.datasetIndex : -1
            const dataset =
              datasetIndex >= 0
                ? (chart.data.datasets[datasetIndex] as { borderColor?: unknown } | undefined)
                : undefined
            const borderColorValue = dataset?.borderColor
            const color = Array.isArray(borderColorValue)
              ? String(borderColorValue[0] ?? '#94a3b8')
              : String(borderColorValue ?? '#94a3b8')
            return {
              ...item,
              fillStyle: color,
              strokeStyle: color,
              lineWidth: 0,
              hidden: datasetIndex >= 0 ? !chart.isDatasetVisible(datasetIndex) : true,
            }
          })
        },
      },
    },
    tooltip: {
      callbacks: {
        labelColor: (context) => {
          const borderColorValue = context.dataset?.borderColor
          const color = Array.isArray(borderColorValue)
            ? String(borderColorValue[0] ?? '#94a3b8')
            : String(borderColorValue ?? '#94a3b8')
          return {
            borderColor: color,
            backgroundColor: color,
            borderWidth: 0,
          }
        },
        label: (context) => {
          const labelPrefix = context.dataset.label ? `${context.dataset.label}: ` : ''
          const rawValue = Number(context.parsed?.y ?? context.raw ?? 0)
          if (context.dataset.yAxisID === 'yCost') {
            if (!Number.isFinite(rawValue)) return `${labelPrefix}$0`
            return `${labelPrefix}${rawValue >= 1 ? `$${rawValue.toFixed(2)}` : `$${rawValue.toFixed(4)}`}`
          }
          return `${labelPrefix}${formatTokenNumber(rawValue)}`
        },
      },
    },
  },
  elements: {
    point: {
      radius: 0,
      hoverRadius: 0,
      hitRadius: 10,
      borderWidth: 0,
    },
  },
  scales: {
    x: {
      grid: { display: false },
      ticks: { color: resolveChartTickColor() },
    },
    y: {
      beginAtZero: true,
      ticks: {
        color: resolveChartTickColor(),
        callback: (value: string | number) => {
          const numeric = typeof value === 'number' ? value : Number(value)
          if (!Number.isFinite(numeric)) return '0'
          return formatTokenNumber(numeric)
        },
      },
      grid: { color: 'rgba(148, 163, 184, 0.2)' },
    },
    yCost: {
      position: 'right',
      beginAtZero: true,
      grid: { drawOnChartArea: false },
      ticks: {
        color: resolveChartTickColor(),
        callback: (value: string | number) => {
          const numeric = typeof value === 'number' ? value : Number(value)
          if (Number.isNaN(numeric)) return '$0'
          if (numeric >= 1) return `$${numeric.toFixed(2)}`
          return `$${numeric.toFixed(4)}`
        },
      },
    },
  },
}))

const seriesGranularity = computed<'hour' | 'day'>(() => {
  const range = computeDateRange()
  if (!range || (!range.startAt && !range.endAt)) {
    return 'hour'
  }
  const start = parseLogDate(range.startAt)
  const end = parseLogDate(range.endAt)
  if (!start || !end) {
    return filters.dateType === 'day' ? 'hour' : 'day'
  }
  const duration = end.getTime() - start.getTime()
  return duration > 48 * 60 * 60 * 1000 ? 'day' : 'hour'
})

const formatSeriesLabel = (value?: string) => {
  if (!value) return ''
  const parsed = parseLogDate(value)
  if (parsed) {
    if (seriesGranularity.value === 'day') {
      return `${padHour(parsed.getMonth() + 1)}-${padHour(parsed.getDate())}`
    }
    return `${padHour(parsed.getHours())}:00`
  }
  const match = value.match(/(\d{2}):(\d{2})/)
  if (match) {
    return `${match[1]}:${match[2]}`
  }
  return value
}

const REFRESH_INTERVAL = 30
const countdown = ref(REFRESH_INTERVAL)
let timer: number | undefined

const resetTimer = () => {
  countdown.value = REFRESH_INTERVAL
}

const startCountdown = () => {
  stopCountdown()
  timer = window.setInterval(() => {
    if (countdown.value <= 1) {
      countdown.value = REFRESH_INTERVAL
      void loadDashboard()
    } else {
      countdown.value -= 1
    }
  }, 1000)
}

const stopCountdown = () => {
  if (timer) {
    clearInterval(timer)
    timer = undefined
  }
}

const normalizeProviderName = (value: string) => value.trim()
const normalizeProviderRef = (value: string | number | null | undefined) => `${value ?? ''}`.trim()
const providerNameKey = (value: string | null | undefined) => normalizeProviderName(value ?? '').toLowerCase()

const buildProviderOption = (
  providerIdRaw: string | number | null | undefined,
  providerNameRaw: string | null | undefined,
): LogProviderOption | null => {
  const providerName = normalizeProviderName(providerNameRaw ?? '')
  const providerId = normalizeProviderRef(providerIdRaw)
  if (!providerName && !providerId) return null
  const value = providerId || providerName
  if (!value) return null
  const displayName = providerName || providerId
  const label = providerId && providerName ? `${displayName} (${providerId})` : displayName
  return {
    value,
    label,
    providerId: providerId || undefined,
    providerName: displayName,
  }
}

const mergeProviderOptions = (options: LogProviderOption[]): LogProviderOption[] => {
  const idRefsByName = new Map<string, Set<string>>()
  for (const option of options) {
    const nameKey = providerNameKey(option.providerName || option.label || option.value)
    const providerId = normalizeProviderRef(option.providerId)
    if (!nameKey || !providerId) continue
    const refs = idRefsByName.get(nameKey) ?? new Set<string>()
    refs.add(providerId)
    idRefsByName.set(nameKey, refs)
  }

  const merged = new Map<string, LogProviderOption>()
  for (const option of options) {
    let value = normalizeProviderRef(option.value)
    let providerId = normalizeProviderRef(option.providerId)
    const nameKey = providerNameKey(option.providerName || option.label || option.value)
    if (!providerId && nameKey) {
      const refs = idRefsByName.get(nameKey)
      if (refs && refs.size === 1) {
        const [resolvedId] = Array.from(refs)
        if (resolvedId) {
          providerId = resolvedId
          value = resolvedId
        }
      }
    }
    if (!value) continue
    const normalized: LogProviderOption = {
      ...option,
      value,
      providerId: providerId || undefined,
    }
    const current = merged.get(value)
    if (!current) {
      merged.set(value, normalized)
      continue
    }
    const currentHasId = normalizeProviderRef(current.providerId) !== ''
    const normalizedHasId = normalizeProviderRef(normalized.providerId) !== ''
    if (
      (normalizedHasId && !currentHasId) ||
      (normalizedHasId === currentHasId && normalized.label.length >= current.label.length)
    ) {
      merged.set(value, normalized)
    }
  }
  const result = Array.from(merged.values())
  result.sort((a, b) => {
    const left = normalizeProviderName(a.providerName || a.label || a.value)
    const right = normalizeProviderName(b.providerName || b.label || b.value)
    if (left === right) {
      return normalizeProviderRef(a.value).localeCompare(normalizeProviderRef(b.value))
    }
    return left.localeCompare(right)
  })
  return result
}

const loadProviderNamesFromConfig = async (platform: LogPlatform | ''): Promise<LogProviderOption[]> => {
  const cacheKey = platform
  const now = Date.now()
  const cached = providerConfigCache.get(cacheKey)
  if (cached && now - cached.loadedAt < PROVIDER_CONFIG_CACHE_TTL_MS) {
    return cached.options
  }

  const options: LogProviderOption[] = []
  const pushProvider = (providerIdRaw: string | number | null | undefined, providerNameRaw: string | null | undefined) => {
    const option = buildProviderOption(providerIdRaw, providerNameRaw)
    if (option) {
      options.push(option)
    }
  }

  const includeClaude = platform === '' || platform === 'claude'
  const includeCodex = platform === '' || platform === 'codex'
  const includeGemini = platform === '' || platform === 'gemini'

  if (includeClaude) {
    try {
      const providers = await LoadProviders('claude')
      for (const provider of providers ?? []) {
        pushProvider(provider?.id, provider?.name)
      }
    } catch (error) {
      console.error('failed to load claude providers from config', error)
    }
  }

  if (includeCodex) {
    try {
      const providers = await LoadProviders('codex')
      for (const provider of providers ?? []) {
        pushProvider(provider?.id, provider?.name)
      }
    } catch (error) {
      console.error('failed to load codex providers from config', error)
    }
  }

  if (includeGemini) {
    try {
      const providers = await GetGeminiProviders()
      for (const provider of providers ?? []) {
        pushProvider(provider?.id, provider?.name)
      }
    } catch (error) {
      console.error('failed to load gemini providers from config', error)
    }
  }

  const result = mergeProviderOptions(options)
  providerConfigCache.set(cacheKey, { loadedAt: now, options: result })
  return result
}

const buildProviderOptionsFromRefs = (refs: LogProviderRef[]): LogProviderOption[] => {
  const options: LogProviderOption[] = []
  for (const ref of refs ?? []) {
    const option = buildProviderOption(ref.provider_id, ref.provider)
    if (option) {
      options.push(option)
    }
  }
  return mergeProviderOptions(options)
}

const syncProviderOptionsFromLogs = (items: RequestLog[]) => {
  if (!items.length) return
  const nextOptions = [...providerOptions.value]
  for (const item of items) {
    const option = buildProviderOption(item.provider_id, item.provider)
    if (option) {
      nextOptions.push(option)
    }
  }
  providerOptions.value = mergeProviderOptions(nextOptions)
}

const loadLogs = async () => {
  const range = computeDateRange()
  if (range == null) {
    return
  }
  loading.value = true
  try {
    const data = await fetchRequestLogs({
      platform: filters.platform,
      provider: filters.provider,
      limit: 200,
      startAt: range.startAt,
      endAt: range.endAt,
    })
    logs.value = data ?? []
    page.value = Math.min(page.value, totalPages.value)
  } catch (error) {
    console.error('failed to load request logs', error)
  } finally {
    loading.value = false
  }
}

const loadStats = async () => {
  try {
    const range = computeDateRange()
    if (range == null) return
    const data = await fetchLogStatsV2({
      platform: filters.platform,
      provider: filters.provider,
      startAt: range.startAt,
      endAt: range.endAt,
    })
    stats.value = data ?? null
  } catch (error) {
    console.error('failed to load log stats', error)
  }
}

const loadModelStats = async () => {
  try {
    const range = computeDateRange()
    if (range == null) return
    const data = await fetchModelStatsV2({
      platform: filters.platform,
      provider: filters.provider,
      startAt: range.startAt,
      endAt: range.endAt,
    })
    modelStats.value = data ?? []
  } catch (error) {
    console.error('failed to load model stats', error)
    modelStats.value = []
  }
}

const loadDashboard = async () => {
  await Promise.all([loadLogs(), loadStats(), loadModelStats(), loadProviderOptions()])
  syncProviderOptionsFromLogs(logs.value)
}

const loadStorageStats = async () => {
  storageLoading.value = true
  try {
    storageStats.value = await fetchLogStorageStats()
  } catch (error) {
    console.error('failed to load log storage stats', error)
  } finally {
    storageLoading.value = false
  }
}

const openStorageModal = () => {
  storageModal.open = true
  void loadStorageStats()
}

const closeStorageModal = () => {
  if (storageClearing.value) return
  storageModal.open = false
}

type StorageClearTarget = 'requestLogs' | 'stats'

const storageClearConfirm = reactive<{
  open: boolean
  target: StorageClearTarget | null
}>({
  open: false,
  target: null,
})

const resetStorageClearConfirm = () => {
  storageClearConfirm.open = false
  storageClearConfirm.target = null
}

const closeStorageClearConfirm = () => {
  if (storageClearing.value) return
  resetStorageClearConfirm()
}

const storageClearConfirmMessage = computed(() => {
  switch (storageClearConfirm.target) {
    case 'requestLogs':
      return t('components.logs.storage.confirmClearRequestLog')
    case 'stats':
      return t('components.logs.storage.confirmClearStats')
    default:
      return ''
  }
})

const storageClearConfirmActionLabel = computed(() => {
  switch (storageClearConfirm.target) {
    case 'requestLogs':
      return t('components.logs.storage.clearRequestLog')
    case 'stats':
      return t('components.logs.storage.clearStats')
    default:
      return t('components.logs.storage.clearRequestLog')
  }
})

const handleClearRequestLogs = () => {
  if (storageClearing.value) return
  storageClearConfirm.target = 'requestLogs'
  storageClearConfirm.open = true
}

const handleClearStats = () => {
  if (storageClearing.value) return
  storageClearConfirm.target = 'stats'
  storageClearConfirm.open = true
}

const confirmStorageClear = async () => {
  if (storageClearing.value || !storageClearConfirm.target) return
  const target = storageClearConfirm.target
  storageClearing.value = true
  try {
    if (target === 'requestLogs') {
      await clearRequestLogs()
    } else {
      await clearLogStats()
    }
    showToast(t('components.logs.storage.success'), 'success')
    await Promise.all([loadStorageStats(), loadDashboard()])
    resetStorageClearConfirm()
  } catch (error) {
    console.error('failed to clear log storage', error)
    showToast(t('components.logs.storage.failed', { error: extractErrorMessage(error) }), 'error')
    resetStorageClearConfirm()
  } finally {
    storageClearing.value = false
  }
}

const pagedLogs = computed(() => {
  const start = (page.value - 1) * PAGE_SIZE
  return logs.value.slice(start, start + PAGE_SIZE)
})

const totalPages = computed(() => Math.max(1, Math.ceil(logs.value.length / PAGE_SIZE)))

const applyFilters = async () => {
  if (!isFilterValid.value) {
    return
  }
  page.value = 1
  await loadDashboard()
  resetTimer()
}

const refreshLogs = () => {
  void loadDashboard()
}

const manualRefresh = () => {
  resetTimer()
  void loadDashboard()
}

const nextPage = () => {
  if (page.value < totalPages.value) {
    page.value += 1
  }
}

const prevPage = () => {
  if (page.value > 1) {
    page.value -= 1
  }
}

const backToHome = () => {
  router.push('/')
}

const padHour = (num: number) => num.toString().padStart(2, '0')

const formatTime = (value?: string) => {
  const date = parseLogDate(value)
  if (!date) return value || '—'
  return `${date.getFullYear()}-${padHour(date.getMonth() + 1)}-${padHour(date.getDate())} ${padHour(date.getHours())}:${padHour(date.getMinutes())}:${padHour(date.getSeconds())}`
}

const formatStream = (value?: boolean | number) => {
  const isOn = value === true || value === 1
  return isOn ? t('components.logs.streamOn') : t('components.logs.streamOff')
}

const formatDuration = (value?: number) => {
  if (!value || Number.isNaN(value)) return '—'
  return `${value.toFixed(2)}s`
}

type ModelVerifyStatus = 'match' | 'mismatch' | 'unknown'

const normalizeModelName = (value?: string) => String(value ?? '').trim().toLowerCase()

const resolveModelVerifyStatus = (item: RequestLog): ModelVerifyStatus => {
  const requestedModel = normalizeModelName(item.requested_model)
  const responseModel = normalizeModelName(item.response_model)
  if (!requestedModel || !responseModel) return 'unknown'
  return requestedModel === responseModel ? 'match' : 'mismatch'
}

const formatModelVerifyStatus = (item: RequestLog) =>
  t(`components.logs.table.verifyValues.${resolveModelVerifyStatus(item)}`)

const formatModelInfoAriaLabel = (item: RequestLog) =>
  `${t('components.logs.table.model')}: ${item.model || '—'}`

const formatVerifyInfoAriaLabel = (item: RequestLog) =>
  `${t('components.logs.table.verify')}: ${formatModelVerifyStatus(item)}`

const resolveTooltipModelValue = (value?: string): { value: string; tone?: LogInfoTooltipTone } => {
  const normalized = String(value ?? '').trim()
  if (!normalized) {
    return {
      value: t('components.logs.table.tooltipValues.missing'),
      tone: 'muted',
    }
  }
  return { value: normalized }
}

const httpCodeClass = (code: number) => {
  if (code >= 500) return 'http-server-error'
  if (code >= 400) return 'http-client-error'
  if (code >= 300) return 'http-redirect'
  if (code >= 200) return 'http-success'
  return 'http-info'
}

const durationColor = (value?: number) => {
  if (!value || Number.isNaN(value)) return 'neutral'
  if (value < 2) return 'fast'
  if (value < 5) return 'medium'
  return 'slow'
}

const clampToRange = (value: number, min: number, max: number) => {
  if (max <= min) return min
  return Math.min(Math.max(value, min), max)
}

const normalizePricingModelKey = (value: string) =>
  value
    .trim()
    .toLowerCase()
    .replace(/[-_.:/\s]/g, '')

const commonPrefixLength = (a: string, b: string) => {
  const limit = Math.min(a.length, b.length)
  let cursor = 0
  while (cursor < limit && a[cursor] === b[cursor]) {
    cursor += 1
  }
  return cursor
}

const stripPricingRegionPrefix = (value: string) => {
  const trimmed = value.trim()
  const lowered = trimmed.toLowerCase()
  for (const prefix of ['us.', 'eu.', 'apac.']) {
    if (lowered.startsWith(prefix)) {
      return trimmed.slice(prefix.length)
    }
  }
  return trimmed
}

const stripPricingProviderPrefix = (value: string) => {
  const trimmed = value.trim()
  const lowered = trimmed.toLowerCase()
  if (lowered.startsWith('anthropic.')) {
    return trimmed.slice('anthropic.'.length)
  }
  return trimmed
}

const pricingAliasCandidates = (value: string) => {
  const lowered = value.trim().toLowerCase()
  if (lowered === 'gpt-5-codex') {
    return ['gpt-5']
  }
  return [] as string[]
}

const buildPricingModelCandidates = (value: string) => {
  const base = value.trim()
  if (!base) return [] as string[]
  const result = new Set<string>()
  const collect = (candidate: string) => {
    const normalized = candidate.trim()
    if (normalized) {
      result.add(normalized)
    }
  }
  const collectWithVariants = (candidate: string) => {
    const trimmed = candidate.trim()
    if (!trimmed) return
    collect(trimmed)
    collect(stripPricingRegionPrefix(trimmed))
    collect(stripPricingProviderPrefix(trimmed))
    collect(stripPricingProviderPrefix(stripPricingRegionPrefix(trimmed)))
    for (const alias of pricingAliasCandidates(trimmed)) {
      collect(alias)
      collect(stripPricingRegionPrefix(alias))
      collect(stripPricingProviderPrefix(alias))
      collect(stripPricingProviderPrefix(stripPricingRegionPrefix(alias)))
    }
  }
  collectWithVariants(base)

  const noLongContextSuffix = base.replace(/\[1m\]/gi, '').trim()
  if (noLongContextSuffix && noLongContextSuffix !== base) {
    collectWithVariants(noLongContextSuffix)
  }
  return Array.from(result)
}

const modelPricingIndex = computed(() => {
  const byExact = new Map<string, ModelPricingRow>()
  const byLower = new Map<string, ModelPricingRow>()
  const byNormalized = new Map<string, ModelPricingRow>()
  for (const row of modelPricingRows.value) {
    const model = String(row.model ?? '').trim()
    if (!model) continue
    byExact.set(model, row)
    byLower.set(model.toLowerCase(), row)
    byNormalized.set(normalizePricingModelKey(model), row)
  }
  return { byExact, byLower, byNormalized }
})

const safeNumber = (value?: number) => {
  const numeric = Number(value ?? 0)
  return Number.isFinite(numeric) ? numeric : 0
}

type LogPriceSource = 'provider_api' | 'builtin' | 'none'

const resolvePriceSource = (item: RequestLog): LogPriceSource => {
  const source = String(item.price_source ?? '').trim().toLowerCase()
  if (source === 'provider_api') return 'provider_api'
  if (source === 'builtin') return 'builtin'
  if (source === 'none') return 'none'
  if (safeNumber(item.total_cost) > 0) return 'builtin'
  return 'none'
}

const formatPriceSource = (item: RequestLog) => {
  const source = resolvePriceSource(item)
  if (source === 'provider_api') return t('components.logs.table.priceSourceValues.providerApi')
  if (source === 'builtin') return t('components.logs.table.priceSourceValues.builtin')
  return t('components.logs.table.priceSourceValues.none')
}

const priceSourceClass = (item: RequestLog) => {
  const source = resolvePriceSource(item)
  if (source === 'provider_api') return 'provider-api'
  return source
}

const buildModelInfoTooltipDetail = (item: RequestLog): LogInfoTooltipDetail => {
  const rows: LogInfoTooltipRow[] = [
    {
      key: 'price-source',
      label: t('components.logs.table.tooltipLabels.pricingSource'),
      value: formatPriceSource(item),
      tone: `source-${priceSourceClass(item)}` as LogInfoTooltipTone,
    },
  ]
  const costDetail = buildCostTooltipDetail(item)
  const matchedModel = String(costDetail.pricingModel ?? item.matched_pricing_model ?? '').trim()
  const currentModel = String(item.model ?? '').trim()
  if (
    matchedModel &&
    normalizeModelName(matchedModel) !== normalizeModelName(currentModel)
  ) {
    rows.push({
      key: 'pricing-model',
      label: t('components.logs.table.tooltipLabels.pricingModel'),
      value: matchedModel,
    })
  }

  if (costDetail.priceLines.length > 0) {
    rows.push(
      ...costDetail.priceLines.map((line) => ({
        key: `pricing-line-${line.key}`,
        label: line.label,
        value: line.value,
      }))
    )
  } else {
    rows.push({
      key: 'pricing-line-empty',
      label: t('components.logs.table.tooltipLabels.pricingDetail'),
      value: t('components.logs.table.tooltipValues.pricingUnavailable'),
      tone: 'muted',
    })
  }

  rows.push({
    key: 'pricing-formula',
    label: t('components.logs.table.tooltipLabels.pricingFormula'),
    value: costDetail.formula,
    tone: costDetail.priceLines.length > 0 ? undefined : 'muted',
  })

  if (costDetail.note) {
    rows.push({
      key: 'pricing-note',
      label: t('components.logs.table.tooltipLabels.pricingHint'),
      value: costDetail.note,
      tone: 'muted',
    })
  }

  rows.push({
    key: 'pricing-recorded-cost',
    label: t('components.logs.table.tooltipLabels.recordedCost'),
    value: formatUsdPrecise(safeNumber(item.total_cost)),
  })

  return {
    title: t('components.logs.table.model'),
    variant: 'model',
    rows,
  }
}

const buildVerifyInfoTooltipDetail = (item: RequestLog): LogInfoTooltipDetail => {
  const requested = resolveTooltipModelValue(item.requested_model)
  const response = resolveTooltipModelValue(item.response_model)
  return {
    title: t('components.logs.table.verify'),
    variant: 'verify',
    rows: [
      {
        key: 'requested-model',
        label: t('components.logs.table.tooltipLabels.requestedModel'),
        value: requested.value,
        tone: requested.tone,
      },
      {
        key: 'response-model',
        label: t('components.logs.table.tooltipLabels.responseModel'),
        value: response.value,
        tone: response.tone,
      },
    ],
  }
}

const formatUsdPrecise = (value: number) => `$${safeNumber(value).toFixed(6)}`

const formatUsdPerMillion = (perTokenPrice: number) =>
  formatUsdPrecise(safeNumber(perTokenPrice) * PER_MILLION_TOKENS)

const formatMultiplierValue = (value: number) => {
  const numeric = safeNumber(value)
  if (numeric <= 0) return '1'
  const rounded = Number(numeric.toFixed(4))
  if (Number.isInteger(rounded)) return String(rounded)
  return rounded.toString()
}

const loadModelPricingRows = async () => {
  if (modelPricingLoaded.value) return
  if (modelPricingLoadingTask) {
    await modelPricingLoadingTask
    return
  }

  modelPricingLoading.value = true
  modelPricingLoadingTask = (async () => {
    try {
      modelPricingRows.value = (await listModelPricing()) ?? []
      modelPricingLoaded.value = true
    } catch (error) {
      console.error('failed to load model pricing rows', error)
    } finally {
      modelPricingLoading.value = false
      modelPricingLoadingTask = null
    }
  })()

  await modelPricingLoadingTask
}

const resolvePricingRow = (item: RequestLog) => {
  const lookup = modelPricingIndex.value
  const candidates = [item.matched_pricing_model, item.model]
  for (const modelName of candidates) {
    const name = String(modelName ?? '').trim()
    if (!name) continue
    for (const candidate of buildPricingModelCandidates(name)) {
      const exact = lookup.byExact.get(candidate)
      if (exact) return exact
      const lower = lookup.byLower.get(candidate.toLowerCase())
      if (lower) return lower
      const normalized = lookup.byNormalized.get(normalizePricingModelKey(candidate))
      if (normalized) return normalized
    }
  }

  const rows = modelPricingRows.value
  for (const modelName of candidates) {
    const name = String(modelName ?? '').trim()
    if (!name) continue
    const targetNorm = normalizePricingModelKey(name)
    if (!targetNorm) continue
    let bestRow: ModelPricingRow | null = null
    let bestScore = -1
    for (const row of rows) {
      const rowNorm = normalizePricingModelKey(String(row.model ?? ''))
      if (!rowNorm) continue
      if (!(rowNorm.includes(targetNorm) || targetNorm.includes(rowNorm))) continue
      const maxLen = Math.max(rowNorm.length, targetNorm.length)
      if (maxLen <= 0) continue
      const prefixScore = commonPrefixLength(rowNorm, targetNorm) / maxLen
      const overlapScore = Math.min(rowNorm.length, targetNorm.length) / maxLen
      const score = overlapScore * 0.8 + prefixScore * 0.2
      if (score > bestScore) {
        bestScore = score
        bestRow = row
      }
    }
    if (bestRow) return bestRow
  }
  return null
}

const resolveGroupMultiplier = (item: RequestLog) => {
  const candidate = safeNumber((item as RequestLog & { group_multiplier?: number }).group_multiplier)
  if (candidate <= 0) return 1
  return candidate
}

const hasBreakdownCostPayload = (item: RequestLog) =>
  [item.input_cost, item.output_cost, item.reasoning_cost, item.cache_create_cost, item.cache_read_cost]
    .some(value => safeNumber(value) > 0)

const isTrueFlag = (value: unknown) => value === true || value === 1

const hasProviderPricingSnapshot = (item: RequestLog) =>
  isTrueFlag(item.provider_pricing_available)

const isProviderPerCallValueSet = (value?: number, setFlag?: boolean) => {
  if (isTrueFlag(setFlag)) return true
  if (setFlag === false) return false
  return safeNumber(value) > 0
}

const withPriceSuffix = (value: string, suffix?: string) => {
  const normalized = String(suffix ?? '').trim()
  return normalized ? `${value} ${normalized}` : value
}

const buildTokenRatePriceLines = ({
  inputPerToken,
  outputPerToken,
  reasoningPerToken,
  cacheCreatePerToken,
  cacheReadPerToken,
  includeCacheRead,
  includeReasoning,
  suffix = '',
  includeCacheMultiplierHint = false,
}: TokenRatePriceLineOptions): CostTooltipPriceLine[] => {
  const completionMultiplier = inputPerToken > 0 ? outputPerToken / inputPerToken : 0
  const cacheCreateMultiplier = inputPerToken > 0 ? cacheCreatePerToken / inputPerToken : 0
  const cacheReadMultiplier = inputPerToken > 0 ? cacheReadPerToken / inputPerToken : 0
  const tokensUnit = '/ 1M tokens'
  const priceLines: CostTooltipPriceLine[] = [
    {
      key: 'prompt',
      label: t('components.logs.costTooltip.promptPrice'),
      value: withPriceSuffix(`${formatUsdPerMillion(inputPerToken)} ${tokensUnit}`, suffix),
    },
  ]

  const completionValue =
    completionMultiplier > 0 && inputPerToken > 0
      ? `${formatUsdPerMillion(inputPerToken)} * ${formatMultiplierValue(completionMultiplier)} = ${formatUsdPerMillion(outputPerToken)} ${tokensUnit}`
      : `${formatUsdPerMillion(outputPerToken)} ${tokensUnit}`
  priceLines.push({
    key: 'completion',
    label: t('components.logs.costTooltip.completionPrice'),
    value: withPriceSuffix(completionValue, suffix),
  })

  const cacheCreateHint = includeCacheMultiplierHint
    ? ` (${t('components.logs.costTooltip.cacheCreateMultiplierLabel', { multiplier: formatMultiplierValue(cacheCreateMultiplier) })})`
    : ''
  const cacheCreateValue =
    cacheCreateMultiplier > 0 && inputPerToken > 0
      ? `${formatUsdPerMillion(inputPerToken)} * ${formatMultiplierValue(cacheCreateMultiplier)} = ${formatUsdPerMillion(cacheCreatePerToken)} ${tokensUnit}${cacheCreateHint}`
      : `${formatUsdPerMillion(cacheCreatePerToken)} ${tokensUnit}`
  priceLines.push({
    key: 'cacheCreate',
    label: t('components.logs.costTooltip.cacheCreatePrice'),
    value: withPriceSuffix(cacheCreateValue, suffix),
  })

  if (includeCacheRead) {
    const cacheReadHint = includeCacheMultiplierHint
      ? ` (${t('components.logs.costTooltip.cacheReadMultiplierLabel', { multiplier: formatMultiplierValue(cacheReadMultiplier) })})`
      : ''
    const cacheReadValue =
      cacheReadMultiplier > 0 && inputPerToken > 0
        ? `${formatUsdPerMillion(inputPerToken)} * ${formatMultiplierValue(cacheReadMultiplier)} = ${formatUsdPerMillion(cacheReadPerToken)} ${tokensUnit}${cacheReadHint}`
        : `${formatUsdPerMillion(cacheReadPerToken)} ${tokensUnit}`
    priceLines.push({
      key: 'cacheRead',
      label: t('components.logs.costTooltip.cacheReadPrice'),
      value: withPriceSuffix(cacheReadValue, suffix),
    })
  }

  if (includeReasoning) {
    priceLines.push({
      key: 'reasoning',
      label: t('components.logs.costTooltip.reasoningPrice'),
      value: withPriceSuffix(`${formatUsdPerMillion(reasoningPerToken)} ${tokensUnit}`, suffix),
    })
  }

  return priceLines
}

const buildObservedCostPriceLines = (item: RequestLog): CostTooltipPriceLine[] => {
  if (!hasBreakdownCostPayload(item)) return []

  const inputTokens = Math.max(0, Math.round(safeNumber(item.input_tokens)))
  const outputTokens = Math.max(0, Math.round(safeNumber(item.output_tokens)))
  const reasoningTokens = Math.max(0, Math.round(safeNumber(item.reasoning_tokens)))
  const cacheCreateTokens = Math.max(0, Math.round(safeNumber(item.cache_create_tokens)))
  const cacheReadTokens = Math.max(0, Math.round(safeNumber(item.cache_read_tokens)))

  const inputCost = Math.max(0, safeNumber(item.input_cost))
  const outputCost = Math.max(0, safeNumber(item.output_cost))
  const reasoningCost = Math.max(0, safeNumber(item.reasoning_cost))
  const cacheCreateCost = Math.max(0, safeNumber(item.cache_create_cost))
  const cacheReadCost = Math.max(0, safeNumber(item.cache_read_cost))

  const inputPerToken = inputTokens > 0 ? inputCost / inputTokens : 0
  const outputPerToken = outputTokens > 0 ? outputCost / outputTokens : 0
  const reasoningPerToken = reasoningTokens > 0 ? reasoningCost / reasoningTokens : 0
  const cacheCreatePerToken = cacheCreateTokens > 0 ? cacheCreateCost / cacheCreateTokens : 0
  const cacheReadPerToken = cacheReadTokens > 0 ? cacheReadCost / cacheReadTokens : 0

  return buildTokenRatePriceLines({
    inputPerToken,
    outputPerToken,
    reasoningPerToken,
    cacheCreatePerToken,
    cacheReadPerToken,
    includeCacheRead: cacheReadTokens > 0,
    includeReasoning: reasoningTokens > 0,
    suffix: t('components.logs.costTooltip.observedPriceSuffix'),
  })
}

const buildProviderAPITokenTooltipDetail = (
  item: RequestLog,
  fallbackModelName: string,
  recordedCost: number,
): CostTooltipDetail | null => {
  if (!hasProviderPricingSnapshot(item)) return null
  if (safeNumber(item.provider_quota_type) !== 0) return null

  const inputTokens = Math.max(0, Math.round(safeNumber(item.input_tokens)))
  const outputTokens = Math.max(0, Math.round(safeNumber(item.output_tokens)))
  const reasoningTokens = Math.max(0, Math.round(safeNumber(item.reasoning_tokens)))
  const cacheCreateTokens = Math.max(0, Math.round(safeNumber(item.cache_create_tokens)))
  const cacheReadTokens = Math.max(0, Math.round(safeNumber(item.cache_read_tokens)))

  const breakdownInputCost = Math.max(0, safeNumber(item.input_cost))
  const breakdownOutputCost = Math.max(0, safeNumber(item.output_cost))
  const breakdownReasoningCost = Math.max(0, safeNumber(item.reasoning_cost))
  const breakdownCacheCreateCost = Math.max(0, safeNumber(item.cache_create_cost))
  const breakdownCacheReadCost = Math.max(0, safeNumber(item.cache_read_cost))

  const inputPerTokenSnapshot = Math.max(0, safeNumber(item.provider_input_usd_per_m)) / PER_MILLION_TOKENS
  const outputPerTokenSnapshot = Math.max(0, safeNumber(item.provider_output_usd_per_m)) / PER_MILLION_TOKENS

  const inputPerToken =
    inputPerTokenSnapshot > 0
      ? inputPerTokenSnapshot
      : inputTokens > 0 && breakdownInputCost > 0
        ? breakdownInputCost / inputTokens
        : 0
  const outputPerToken =
    outputPerTokenSnapshot > 0
      ? outputPerTokenSnapshot
      : outputTokens > 0 && breakdownOutputCost > 0
        ? breakdownOutputCost / outputTokens
        : 0
  const reasoningPerToken =
    reasoningTokens > 0 && breakdownReasoningCost > 0
      ? breakdownReasoningCost / reasoningTokens
      : outputPerToken
  const cacheCreatePerToken =
    cacheCreateTokens > 0 && breakdownCacheCreateCost > 0
      ? breakdownCacheCreateCost / cacheCreateTokens
      : inputPerToken
  const cacheReadPerToken =
    cacheReadTokens > 0 && breakdownCacheReadCost > 0
      ? breakdownCacheReadCost / cacheReadTokens
      : inputPerToken

  const hasAnyTokenRate =
    inputPerToken > 0 ||
    outputPerToken > 0 ||
    reasoningPerToken > 0 ||
    cacheCreatePerToken > 0 ||
    (cacheReadTokens > 0 && cacheReadPerToken > 0)
  if (!hasAnyTokenRate) return null

  const inputCost = inputTokens > 0 && breakdownInputCost > 0 ? breakdownInputCost : inputTokens * inputPerToken
  const outputCost = outputTokens > 0 && breakdownOutputCost > 0 ? breakdownOutputCost : outputTokens * outputPerToken
  const reasoningCost = reasoningTokens > 0 && breakdownReasoningCost > 0
    ? breakdownReasoningCost
    : reasoningTokens * reasoningPerToken
  const cacheCreateCost = cacheCreateTokens > 0 && breakdownCacheCreateCost > 0
    ? breakdownCacheCreateCost
    : cacheCreateTokens * cacheCreatePerToken
  const cacheReadCost = cacheReadTokens > 0 && breakdownCacheReadCost > 0
    ? breakdownCacheReadCost
    : cacheReadTokens * cacheReadPerToken

  const calculatedTotal = inputCost + outputCost + reasoningCost + cacheCreateCost + cacheReadCost
  const cacheCreateMultiplier = inputPerToken > 0 ? cacheCreatePerToken / inputPerToken : 0
  const cacheReadMultiplier = inputPerToken > 0 ? cacheReadPerToken / inputPerToken : 0

  const priceLines = buildTokenRatePriceLines({
    inputPerToken,
    outputPerToken,
    reasoningPerToken,
    cacheCreatePerToken,
    cacheReadPerToken,
    includeCacheRead: cacheReadTokens > 0,
    includeReasoning: reasoningTokens > 0,
    includeCacheMultiplierHint: true,
  })

  const formulaParts: string[] = []
  if (inputTokens > 0 && inputPerToken > 0) {
    formulaParts.push(
      `${t('components.logs.costTooltip.usagePrompt')} ${formatTokenFormulaValue(inputTokens)} tokens / 1M tokens * ${formatUsdPerMillion(inputPerToken)}`
    )
  }
  if (cacheCreateTokens > 0 && cacheCreatePerToken > 0) {
    const multiplierSuffix = cacheCreateMultiplier > 0
      ? ` (${t('components.logs.costTooltip.cacheCreateMultiplierLabel', { multiplier: formatMultiplierValue(cacheCreateMultiplier) })})`
      : ''
    formulaParts.push(
      `${t('components.logs.costTooltip.usageCacheCreate')} ${formatTokenFormulaValue(cacheCreateTokens)} tokens / 1M tokens * ${formatUsdPerMillion(cacheCreatePerToken)}${multiplierSuffix}`
    )
  }
  if (cacheReadTokens > 0 && cacheReadPerToken > 0) {
    const multiplierSuffix = cacheReadMultiplier > 0
      ? ` (${t('components.logs.costTooltip.cacheReadMultiplierLabel', { multiplier: formatMultiplierValue(cacheReadMultiplier) })})`
      : ''
    formulaParts.push(
      `${t('components.logs.costTooltip.usageCacheRead')} ${formatTokenFormulaValue(cacheReadTokens)} tokens / 1M tokens * ${formatUsdPerMillion(cacheReadPerToken)}${multiplierSuffix}`
    )
  }
  if (outputTokens > 0 && outputPerToken > 0) {
    formulaParts.push(
      `${t('components.logs.costTooltip.usageCompletion')} ${formatTokenFormulaValue(outputTokens)} tokens / 1M tokens * ${formatUsdPerMillion(outputPerToken)}`
    )
  }
  if (reasoningTokens > 0 && reasoningPerToken > 0) {
    formulaParts.push(
      `${t('components.logs.costTooltip.usageReasoning')} ${formatTokenFormulaValue(reasoningTokens)} tokens / 1M tokens * ${formatUsdPerMillion(reasoningPerToken)}`
    )
  }

  const formula = formulaParts.length > 0
    ? `${formulaParts.join(' + ')} = ${formatUsdPrecise(calculatedTotal)}`
    : t('components.logs.costTooltip.providerApiFormula')

  const recordedCostHint =
    Math.abs(calculatedTotal - recordedCost) > COST_TOOLTIP_DIFF_EPSILON
      ? t('components.logs.costTooltip.recordedCostHint', {
        cost: formatUsdPrecise(recordedCost),
      })
      : ''

  return {
    pricingModel: fallbackModelName,
    hasPricing: true,
    priceLines,
    formula,
    note: t('components.logs.costTooltip.providerApiHint'),
    recordedCostHint,
  }
}

const buildProviderAPIPerCallPriceLines = (item: RequestLog): CostTooltipPriceLine[] => {
  if (!hasProviderPricingSnapshot(item)) return []
  if (safeNumber(item.provider_quota_type) !== 1) return []

  const hasUnified = isProviderPerCallValueSet(item.provider_per_call_unified, item.provider_per_call_unified_set)
  const hasInput = isProviderPerCallValueSet(item.provider_per_call_input, item.provider_per_call_input_set)
  const hasOutput = isProviderPerCallValueSet(item.provider_per_call_output, item.provider_per_call_output_set)
  const lines: CostTooltipPriceLine[] = []

  if (hasUnified) {
    lines.push({
      key: 'per-call-unified',
      label: t('components.logs.costTooltip.perCallUnifiedPrice'),
      value: `${formatUsdPrecise(safeNumber(item.provider_per_call_unified))} ${t('components.logs.costTooltip.perRequestSuffix')}`,
    })
  }

  if (hasInput) {
    lines.push({
      key: 'per-call-input',
      label: t('components.logs.costTooltip.perCallInputPrice'),
      value: `${formatUsdPrecise(safeNumber(item.provider_per_call_input))} ${t('components.logs.costTooltip.perRequestSuffix')}`,
    })
  }

  if (hasOutput) {
    lines.push({
      key: 'per-call-output',
      label: t('components.logs.costTooltip.perCallOutputPrice'),
      value: `${formatUsdPrecise(safeNumber(item.provider_per_call_output))} ${t('components.logs.costTooltip.perRequestSuffix')}`,
    })
  }

  return lines
}

const mergeCostTooltipNotes = (...notes: Array<string | undefined>) =>
  notes
    .map(note => String(note ?? '').trim())
    .filter(note => note.length > 0)
    .join(' ')

const buildBuiltinCostTooltipDetail = (
  item: RequestLog,
  fallbackModelName: string,
  recordedCost: number,
): CostTooltipDetail => {
  const pricingRow = resolvePricingRow(item)
  const modelName = String(pricingRow?.model ?? fallbackModelName).trim() || '—'

  if (!pricingRow) {
    return {
      pricingModel: modelName,
      hasPricing: false,
      priceLines: [],
      formula: t('components.logs.costTooltip.noPricingFormula'),
      note: t('components.logs.costTooltip.noPricingHint'),
      recordedCostHint: t('components.logs.costTooltip.recordedCostHint', {
        cost: formatUsdPrecise(recordedCost),
      }),
    }
  }

  const inputTokens = Math.max(0, Math.round(safeNumber(item.input_tokens)))
  const outputTokens = Math.max(0, Math.round(safeNumber(item.output_tokens)))
  const reasoningTokens = Math.max(0, Math.round(safeNumber(item.reasoning_tokens)))
  const cacheCreateTokens = Math.max(0, Math.round(safeNumber(item.cache_create_tokens)))
  const cacheReadTokens = Math.max(0, Math.round(safeNumber(item.cache_read_tokens)))

  const inputPerTokenBase = Math.max(0, safeNumber(pricingRow.input_cost_per_token))
  const outputPerTokenBase = Math.max(0, safeNumber(pricingRow.output_cost_per_token))
  const reasoningPerTokenBase = Math.max(0, safeNumber(pricingRow.output_cost_per_reasoning_token))
  const cacheCreateRaw = Math.max(0, safeNumber(pricingRow.cache_creation_input_token_cost))
  const cacheReadRaw = Math.max(0, safeNumber(pricingRow.cache_read_input_token_cost))

  const cacheCreatePerTokenBase = cacheCreateRaw > 0 ? cacheCreateRaw : inputPerTokenBase * 1.25
  const cacheReadPerTokenBase = cacheReadRaw > 0 ? cacheReadRaw : inputPerTokenBase * 0.1

  const breakdownPayload = hasBreakdownCostPayload(item)
  const breakdownInputCost = Math.max(0, safeNumber(item.input_cost))
  const breakdownOutputCost = Math.max(0, safeNumber(item.output_cost))
  const breakdownReasoningCost = Math.max(0, safeNumber(item.reasoning_cost))
  const breakdownCacheCreateCost = Math.max(0, safeNumber(item.cache_create_cost))
  const breakdownCacheReadCost = Math.max(0, safeNumber(item.cache_read_cost))

  const inputCost = breakdownPayload ? breakdownInputCost : inputTokens * inputPerTokenBase
  const outputCost = breakdownPayload ? breakdownOutputCost : outputTokens * outputPerTokenBase
  const reasoningCost = breakdownPayload ? breakdownReasoningCost : reasoningTokens * reasoningPerTokenBase
  const cacheCreateCost = breakdownPayload ? breakdownCacheCreateCost : cacheCreateTokens * cacheCreatePerTokenBase
  const cacheReadCost = breakdownPayload ? breakdownCacheReadCost : cacheReadTokens * cacheReadPerTokenBase

  const inputPerToken = inputTokens > 0 ? inputCost / inputTokens : inputPerTokenBase
  const outputPerToken = outputTokens > 0 ? outputCost / outputTokens : outputPerTokenBase
  const reasoningPerToken = reasoningTokens > 0 ? reasoningCost / reasoningTokens : reasoningPerTokenBase
  const cacheCreatePerToken = cacheCreateTokens > 0 ? cacheCreateCost / cacheCreateTokens : cacheCreatePerTokenBase
  const cacheReadPerToken = cacheReadTokens > 0 ? cacheReadCost / cacheReadTokens : cacheReadPerTokenBase

  const cacheCreateMultiplier = inputPerToken > 0 ? cacheCreatePerToken / inputPerToken : 0
  const cacheReadMultiplier = inputPerToken > 0 ? cacheReadPerToken / inputPerToken : 0
  const groupMultiplier = resolveGroupMultiplier(item)
  const calculatedTotal = inputCost + cacheCreateCost + cacheReadCost + outputCost + reasoningCost

  const priceLines = buildTokenRatePriceLines({
    inputPerToken,
    outputPerToken,
    reasoningPerToken,
    cacheCreatePerToken,
    cacheReadPerToken,
    includeCacheRead: cacheReadTokens > 0,
    includeReasoning: reasoningTokens > 0,
    includeCacheMultiplierHint: true,
  })

  const formulaParts: string[] = []
  if (inputTokens > 0 && inputPerToken > 0) {
    formulaParts.push(
      `${t('components.logs.costTooltip.usagePrompt')} ${formatTokenFormulaValue(inputTokens)} tokens / 1M tokens * ${formatUsdPerMillion(inputPerToken)}`
    )
  }
  if (cacheCreateTokens > 0 && cacheCreatePerToken > 0) {
    formulaParts.push(
      `${t('components.logs.costTooltip.usageCacheCreate')} ${formatTokenFormulaValue(cacheCreateTokens)} tokens / 1M tokens * ${formatUsdPerMillion(cacheCreatePerToken)} (${t('components.logs.costTooltip.cacheCreateMultiplierLabel', { multiplier: formatMultiplierValue(cacheCreateMultiplier) })})`
    )
  }
  if (cacheReadTokens > 0 && cacheReadPerToken > 0) {
    formulaParts.push(
      `${t('components.logs.costTooltip.usageCacheRead')} ${formatTokenFormulaValue(cacheReadTokens)} tokens / 1M tokens * ${formatUsdPerMillion(cacheReadPerToken)} (${t('components.logs.costTooltip.cacheReadMultiplierLabel', { multiplier: formatMultiplierValue(cacheReadMultiplier) })})`
    )
  }
  if (outputTokens > 0 && outputPerToken > 0) {
    formulaParts.push(
      `${t('components.logs.costTooltip.usageCompletion')} ${formatTokenFormulaValue(outputTokens)} tokens / 1M tokens * ${formatUsdPerMillion(outputPerToken)} * ${t('components.logs.costTooltip.groupMultiplierLabel', { multiplier: formatMultiplierValue(groupMultiplier) })}`
    )
  }
  if (reasoningTokens > 0 && reasoningPerToken > 0) {
    formulaParts.push(
      `${t('components.logs.costTooltip.usageReasoning')} ${formatTokenFormulaValue(reasoningTokens)} tokens / 1M tokens * ${formatUsdPerMillion(reasoningPerToken)} * ${t('components.logs.costTooltip.groupMultiplierLabel', { multiplier: formatMultiplierValue(groupMultiplier) })}`
    )
  }

  const formula =
    formulaParts.length > 0
      ? `${formulaParts.join(' + ')} = ${formatUsdPrecise(calculatedTotal)}`
      : t('components.logs.costTooltip.formulaEmpty')

  const rowModel = String(pricingRow.model ?? '').trim().toLowerCase()
  const logModel = String(item.model ?? '').trim().toLowerCase()
  const note = rowModel && logModel && rowModel !== logModel
    ? t('components.logs.costTooltip.matchedModelHint', { model: pricingRow.model })
    : ''

  const recordedCostHint =
    Math.abs(calculatedTotal - recordedCost) > COST_TOOLTIP_DIFF_EPSILON
      ? t('components.logs.costTooltip.recordedCostHint', {
        cost: formatUsdPrecise(recordedCost),
      })
      : ''

  return {
    pricingModel: modelName,
    hasPricing: true,
    priceLines,
    formula,
    note,
    recordedCostHint,
  }
}

const buildCostTooltipDetail = (item: RequestLog): CostTooltipDetail => {
  const source = resolvePriceSource(item)
  const fallbackModelName = String(item.matched_pricing_model ?? item.model ?? '').trim() || '—'
  const recordedCost = safeNumber(item.total_cost)
  const providerSnapshotAvailable = hasProviderPricingSnapshot(item)
  const shouldAvoidFallbackEstimate =
    !providerSnapshotAvailable && recordedCost <= COST_TOOLTIP_DIFF_EPSILON
  const recordedCostHint = t('components.logs.costTooltip.recordedCostHint', {
    cost: formatUsdPrecise(recordedCost),
  })

  if (source === 'provider_api') {
    const providerTokenDetail = buildProviderAPITokenTooltipDetail(item, fallbackModelName, recordedCost)
    if (providerTokenDetail) {
      return providerTokenDetail
    }

    const providerPerCallLines = buildProviderAPIPerCallPriceLines(item)
    if (providerPerCallLines.length > 0) {
      return {
        pricingModel: fallbackModelName,
        hasPricing: true,
        priceLines: providerPerCallLines,
        formula: t('components.logs.costTooltip.providerApiPerCallFormula'),
        note: t('components.logs.costTooltip.providerApiHint'),
        recordedCostHint,
      }
    }

    if (shouldAvoidFallbackEstimate) {
      return {
        pricingModel: fallbackModelName,
        hasPricing: false,
        priceLines: [],
        formula: t('components.logs.costTooltip.providerApiFormula'),
        note: mergeCostTooltipNotes(
          t('components.logs.costTooltip.providerApiHint'),
          t('components.logs.costTooltip.providerApiZeroCostHint'),
        ),
        recordedCostHint,
      }
    }

    const observedPriceLines = buildObservedCostPriceLines(item)
    if (observedPriceLines.length > 0) {
      return {
        pricingModel: fallbackModelName,
        hasPricing: true,
        priceLines: observedPriceLines,
        formula: t('components.logs.costTooltip.providerApiFormula'),
        note: providerSnapshotAvailable
          ? t('components.logs.costTooltip.providerApiHint')
          : mergeCostTooltipNotes(
            t('components.logs.costTooltip.providerApiHint'),
            t('components.logs.costTooltip.providerApiFallbackHint'),
          ),
        recordedCostHint,
      }
    }

    const builtinFallbackDetail = buildBuiltinCostTooltipDetail(item, fallbackModelName, recordedCost)
    if (builtinFallbackDetail.hasPricing) {
      return {
        ...builtinFallbackDetail,
        note: mergeCostTooltipNotes(
          t('components.logs.costTooltip.providerApiFallbackHint'),
          builtinFallbackDetail.note,
        ),
        recordedCostHint,
      }
    }

    return {
      pricingModel: fallbackModelName,
      hasPricing: false,
      priceLines: [],
      formula: t('components.logs.costTooltip.providerApiFormula'),
      note: t('components.logs.costTooltip.providerApiHint'),
      recordedCostHint,
    }
  }

  return buildBuiltinCostTooltipDetail(item, fallbackModelName, recordedCost)
}

const getCostTooltipSize = () => {
  const rect = costTooltipRef.value?.getBoundingClientRect()
  return {
    width: rect?.width ?? COST_TOOLTIP_DEFAULT_WIDTH,
    height: rect?.height ?? COST_TOOLTIP_DEFAULT_HEIGHT,
  }
}

const getViewportSize = () => {
  if (typeof window !== 'undefined') {
    return { width: window.innerWidth, height: window.innerHeight }
  }
  if (typeof document !== 'undefined' && document.documentElement) {
    return {
      width: document.documentElement.clientWidth,
      height: document.documentElement.clientHeight,
    }
  }
  return { width: 0, height: 0 }
}

const getLogInfoTooltipSize = () => {
  const rect = logInfoTooltipRef.value?.getBoundingClientRect()
  return {
    width: rect?.width ?? LOG_INFO_TOOLTIP_DEFAULT_WIDTH,
    height: rect?.height ?? LOG_INFO_TOOLTIP_DEFAULT_HEIGHT,
  }
}

const resolveTooltipAnchor = (event: MouseEvent | FocusEvent) =>
  event.currentTarget as HTMLElement | null

const clearLogInfoTooltipShowTimer = () => {
  if (logInfoTooltipShowTimer != null) {
    window.clearTimeout(logInfoTooltipShowTimer)
    logInfoTooltipShowTimer = null
  }
}

const clearLogInfoTooltipHideTimer = () => {
  if (logInfoTooltipHideTimer != null) {
    window.clearTimeout(logInfoTooltipHideTimer)
    logInfoTooltipHideTimer = null
  }
}

const hideLogInfoTooltipImmediately = () => {
  clearLogInfoTooltipShowTimer()
  clearLogInfoTooltipHideTimer()
  logInfoTooltipRequestId.value += 1
  logInfoTooltipAnchorRef.value = null
  logInfoTooltip.visible = false
  logInfoTooltip.detail = null
}

const scheduleHideLogInfoTooltip = () => {
  clearLogInfoTooltipShowTimer()
  clearLogInfoTooltipHideTimer()
  logInfoTooltipHideTimer = window.setTimeout(() => {
    hideLogInfoTooltipImmediately()
  }, 90)
}

const updateLogInfoTooltipPosition = (anchor: HTMLElement | null) => {
  if (!anchor) return
  const anchorRect = anchor.getBoundingClientRect()
  const { width: tooltipWidth, height: tooltipHeight } = getLogInfoTooltipSize()
  const { width: viewportWidth, height: viewportHeight } = getViewportSize()

  const centerX = anchorRect.left + anchorRect.width / 2
  const minLeft = COST_TOOLTIP_HORIZONTAL_MARGIN + tooltipWidth / 2
  const maxLeft =
    viewportWidth > 0 ? viewportWidth - tooltipWidth / 2 - COST_TOOLTIP_HORIZONTAL_MARGIN : centerX
  logInfoTooltip.left = clampToRange(centerX, minLeft, maxLeft)

  const canShowAbove =
    anchorRect.top - tooltipHeight - LOG_INFO_TOOLTIP_VERTICAL_OFFSET >= COST_TOOLTIP_VERTICAL_MARGIN
  const shouldPlaceBelow = !canShowAbove
  logInfoTooltip.placement = shouldPlaceBelow ? 'below' : 'above'

  const desiredTop = shouldPlaceBelow
    ? anchorRect.bottom + LOG_INFO_TOOLTIP_VERTICAL_OFFSET
    : anchorRect.top - tooltipHeight - LOG_INFO_TOOLTIP_VERTICAL_OFFSET
  const maxTop =
    viewportHeight > 0 ? viewportHeight - tooltipHeight - COST_TOOLTIP_VERTICAL_MARGIN : desiredTop
  logInfoTooltip.top = clampToRange(desiredTop, COST_TOOLTIP_VERTICAL_MARGIN, maxTop)
}

const showLogInfoTooltip = async (
  detail: LogInfoTooltipDetail,
  target: HTMLElement | null,
) => {
  if (!target) return
  if (costTooltip.visible) {
    hideCostTooltipImmediately()
  }
  clearLogInfoTooltipShowTimer()
  clearLogInfoTooltipHideTimer()
  logInfoTooltipAnchorRef.value = target
  logInfoTooltip.detail = detail
  logInfoTooltip.visible = true
  updateLogInfoTooltipPosition(target)
  await nextTick()
  if (logInfoTooltipAnchorRef.value !== target) return
  updateLogInfoTooltipPosition(target)
}

const refreshModelInfoTooltipAfterPricingLoad = async (
  item: RequestLog,
  target: HTMLElement,
  requestId: number,
) => {
  if (modelPricingLoaded.value) return
  await loadModelPricingRows()
  if (requestId !== logInfoTooltipRequestId.value) return
  if (!logInfoTooltip.visible) return
  if (logInfoTooltipAnchorRef.value !== target) return
  logInfoTooltip.detail = buildModelInfoTooltipDetail(item)
  updateLogInfoTooltipPosition(target)
  await nextTick()
  if (requestId !== logInfoTooltipRequestId.value) return
  if (logInfoTooltipAnchorRef.value !== target) return
  updateLogInfoTooltipPosition(target)
}

const showModelInfoTooltip = (item: RequestLog, event: MouseEvent | FocusEvent) => {
  const target = resolveTooltipAnchor(event)
  if (!target) return
  const requestId = ++logInfoTooltipRequestId.value
  void (async () => {
    await showLogInfoTooltip(buildModelInfoTooltipDetail(item), target)
    if (requestId !== logInfoTooltipRequestId.value) return
    await refreshModelInfoTooltipAfterPricingLoad(item, target, requestId)
  })()
}

const showVerifyInfoTooltip = (item: RequestLog, event: MouseEvent | FocusEvent) => {
  const target = resolveTooltipAnchor(event)
  logInfoTooltipRequestId.value += 1
  void showLogInfoTooltip(buildVerifyInfoTooltipDetail(item), target)
}

const scheduleShowModelInfoTooltip = (item: RequestLog, event: MouseEvent) => {
  const target = resolveTooltipAnchor(event)
  if (!target) return
  clearLogInfoTooltipHideTimer()
  clearLogInfoTooltipShowTimer()
  logInfoTooltipShowTimer = window.setTimeout(() => {
    logInfoTooltipShowTimer = null
    const requestId = ++logInfoTooltipRequestId.value
    void (async () => {
      await showLogInfoTooltip(buildModelInfoTooltipDetail(item), target)
      if (requestId !== logInfoTooltipRequestId.value) return
      await refreshModelInfoTooltipAfterPricingLoad(item, target, requestId)
    })()
  }, LOG_TOOLTIP_SHOW_DELAY_MS)
}

const scheduleShowVerifyInfoTooltip = (item: RequestLog, event: MouseEvent) => {
  const target = resolveTooltipAnchor(event)
  if (!target) return
  clearLogInfoTooltipHideTimer()
  clearLogInfoTooltipShowTimer()
  logInfoTooltipShowTimer = window.setTimeout(() => {
    logInfoTooltipShowTimer = null
    logInfoTooltipRequestId.value += 1
    void showLogInfoTooltip(buildVerifyInfoTooltipDetail(item), target)
  }, LOG_TOOLTIP_SHOW_DELAY_MS)
}

const moveLogInfoTooltip = (event: MouseEvent) => {
  if (!logInfoTooltip.visible) return
  clearLogInfoTooltipHideTimer()
  const target = event.currentTarget as HTMLElement | null
  if (!target) return
  logInfoTooltipAnchorRef.value = target
  updateLogInfoTooltipPosition(target)
}

const hideLogInfoTooltip = () => {
  scheduleHideLogInfoTooltip()
}

const handleLogInfoTooltipMouseEnter = () => {
  clearLogInfoTooltipHideTimer()
}

const handleLogInfoTooltipMouseLeave = () => {
  scheduleHideLogInfoTooltip()
}

const clearCostTooltipShowTimer = () => {
  if (costTooltipShowTimer != null) {
    window.clearTimeout(costTooltipShowTimer)
    costTooltipShowTimer = null
  }
}

const clearCostTooltipHideTimer = () => {
  if (costTooltipHideTimer != null) {
    window.clearTimeout(costTooltipHideTimer)
    costTooltipHideTimer = null
  }
}

const hideCostTooltipImmediately = () => {
  clearCostTooltipShowTimer()
  clearCostTooltipHideTimer()
  costTooltipRequestId.value += 1
  costTooltipAnchorRef.value = null
  costTooltip.visible = false
  costTooltip.detail = null
}

const scheduleHideCostTooltip = () => {
  clearCostTooltipShowTimer()
  clearCostTooltipHideTimer()
  costTooltipHideTimer = window.setTimeout(() => {
    hideCostTooltipImmediately()
  }, 80)
}

const updateCostTooltipPosition = (anchor: HTMLElement | null) => {
  if (!anchor) return
  const anchorRect = anchor.getBoundingClientRect()
  const { width: tooltipWidth, height: tooltipHeight } = getCostTooltipSize()
  const { width: viewportWidth, height: viewportHeight } = getViewportSize()

  const centerX = anchorRect.left + anchorRect.width / 2
  const minLeft = COST_TOOLTIP_HORIZONTAL_MARGIN + tooltipWidth / 2
  const maxLeft =
    viewportWidth > 0 ? viewportWidth - tooltipWidth / 2 - COST_TOOLTIP_HORIZONTAL_MARGIN : centerX
  costTooltip.left = clampToRange(centerX, minLeft, maxLeft)

  const canShowAbove =
    anchorRect.top - tooltipHeight - COST_TOOLTIP_VERTICAL_OFFSET >= COST_TOOLTIP_VERTICAL_MARGIN
  const shouldPlaceBelow = !canShowAbove
  costTooltip.placement = shouldPlaceBelow ? 'below' : 'above'

  const desiredTop = shouldPlaceBelow
    ? anchorRect.bottom + COST_TOOLTIP_VERTICAL_OFFSET
    : anchorRect.top - tooltipHeight - COST_TOOLTIP_VERTICAL_OFFSET
  const maxTop =
    viewportHeight > 0 ? viewportHeight - tooltipHeight - COST_TOOLTIP_VERTICAL_MARGIN : desiredTop
  costTooltip.top = clampToRange(desiredTop, COST_TOOLTIP_VERTICAL_MARGIN, maxTop)
}

const showCostTooltipByAnchor = async (item: RequestLog, target: HTMLElement | null) => {
  if (!target) return
  if (logInfoTooltip.visible) {
    hideLogInfoTooltipImmediately()
  }
  clearCostTooltipShowTimer()
  clearCostTooltipHideTimer()
  costTooltipAnchorRef.value = target
  const requestId = ++costTooltipRequestId.value
  costTooltip.detail = buildCostTooltipDetail(item)
  costTooltip.visible = true
  updateCostTooltipPosition(target)
  await nextTick()
  if (requestId !== costTooltipRequestId.value) return
  if (costTooltipAnchorRef.value !== target) return
  updateCostTooltipPosition(target)
  if (modelPricingLoaded.value) return
  await loadModelPricingRows()
  if (requestId !== costTooltipRequestId.value) return
  if (costTooltipAnchorRef.value !== target) return
  costTooltip.detail = buildCostTooltipDetail(item)
  updateCostTooltipPosition(target)
  await nextTick()
  if (requestId !== costTooltipRequestId.value) return
  if (costTooltipAnchorRef.value !== target) return
  updateCostTooltipPosition(target)
}

const showCostTooltip = (item: RequestLog, event: MouseEvent | FocusEvent) => {
  const target = resolveTooltipAnchor(event)
  void showCostTooltipByAnchor(item, target)
}

const scheduleShowCostTooltip = (item: RequestLog, event: MouseEvent) => {
  const target = resolveTooltipAnchor(event)
  if (!target) return
  clearCostTooltipHideTimer()
  clearCostTooltipShowTimer()
  costTooltipShowTimer = window.setTimeout(() => {
    costTooltipShowTimer = null
    void showCostTooltipByAnchor(item, target)
  }, LOG_TOOLTIP_SHOW_DELAY_MS)
}

const moveCostTooltip = (event: MouseEvent) => {
  if (!costTooltip.visible) return
  clearCostTooltipHideTimer()
  const target = event.currentTarget as HTMLElement | null
  if (!target) return
  costTooltipAnchorRef.value = target
  updateCostTooltipPosition(target)
}

const hideCostTooltip = () => {
  scheduleHideCostTooltip()
}

const handleCostTooltipMouseEnter = () => {
  clearCostTooltipHideTimer()
}

const handleCostTooltipMouseLeave = () => {
  scheduleHideCostTooltip()
}

const handleViewportChange = () => {
  if (logInfoTooltip.visible) {
    hideLogInfoTooltipImmediately()
  }
  if (costTooltip.visible) {
    hideCostTooltipImmediately()
  }
}

const formatNumber = (value?: number) => {
  if (value === undefined || value === null) return '—'
  return value.toLocaleString()
}

/**
 * 格式化 token 数值，支持 k/M/B 单位换算
 * @author sm
 */
const formatTokenNumber = (value?: number) => {
  if (value === undefined || value === null) return '—'

  if (value >= 1_000_000_000) {
    return `${(value / 1_000_000_000).toFixed(2)}B`
  }
  if (value >= 1_000_000) {
    return `${(value / 1_000_000).toFixed(2)}M`
  }
  if (value >= 1_000) {
    return `${(value / 1_000).toFixed(2)}k`
  }

  return value.toLocaleString()
}

const formatTokenFormulaValue = (value?: number) => {
  const compact = formatTokenNumber(value)
  if (compact === '—') return '0'
  const numeric = Number(value ?? 0)
  if (!Number.isFinite(numeric) || Math.abs(numeric) < 1_000) return compact
  const exact = Number.isInteger(numeric)
    ? numeric.toLocaleString()
    : numeric.toLocaleString(undefined, { maximumFractionDigits: 2 })
  return `${compact} (${exact})`
}

const normalizeTokenCount = (value?: number) => {
  const normalized = Number(value ?? 0)
  if (!Number.isFinite(normalized)) return 0
  return Math.max(0, Math.round(normalized))
}

const resolveEphemeral1hTokens = (item: RequestLog) =>
  normalizeTokenCount(item.ephemeral_1h_tokens)

const resolveEphemeral5mTokens = (item: RequestLog) => {
  const total = normalizeTokenCount(item.cache_create_tokens)
  const explicit5m = normalizeTokenCount(item.ephemeral_5m_tokens)
  const ephemeral1h = resolveEphemeral1hTokens(item)
  if (explicit5m > 0) return explicit5m
  if (total <= 0) return 0
  const fallback5m = total - ephemeral1h
  return fallback5m > 0 ? fallback5m : 0
}

const hasCacheCreateDetail = (item: RequestLog) => {
  const total = normalizeTokenCount(item.cache_create_tokens)
  if (total <= 0) return false
  return resolveEphemeral5mTokens(item) > 0 || resolveEphemeral1hTokens(item) > 0
}

/**
 * 计算缓存命中率
 * @param cacheRead 缓存读取 token 数
 * @param inputTokens 输入 token 数
 * @returns 命中率百分比字符串
 * @author sm
 */
const formatCacheHitRate = (cacheRead?: number, inputTokens?: number) => {
  const read = cacheRead ?? 0
  const input = inputTokens ?? 0
  const total = read + input

  if (total === 0) return '0%'

  const rate = (read / total) * 100
  return `${rate.toFixed(1)}%`
}

const formatCurrency = (value?: number) => {
  if (value === undefined || value === null || Number.isNaN(value)) {
    return '$0.0000'
  }
  if (value >= 1) {
    return `$${value.toFixed(2)}`
  }
  if (value >= 0.01) {
    return `$${value.toFixed(3)}`
  }
  return `$${value.toFixed(4)}`
}

const formatCostAriaLabel = (item: RequestLog) =>
  `${t('components.logs.table.cost')}: ${formatCurrency(item.total_cost)}`

const startOfTodayLocal = () => {
  const now = new Date()
  now.setHours(0, 0, 0, 0)
  return now
}

const statsCards = computed(() => {
  const data = stats.value
  const scopeHint = summaryScopeHint.value
  const totalTokens =
    (data?.input_tokens ?? 0) + (data?.output_tokens ?? 0) + (data?.cache_read_tokens ?? 0)
  return [
    {
      key: 'requests',
      label: t('components.logs.summary.total'),
      hint: t('components.logs.summary.requests'),
      value: data ? formatNumber(data.total_requests) : '—',
    },
    {
      key: 'tokens',
      label: t('components.logs.summary.tokens'),
      hint: t('components.logs.summary.tokenHint'),
      value: data ? formatTokenNumber(totalTokens) : '—',
    },
    {
      key: 'cacheReads',
      label: t('components.logs.summary.cache'),
      hint: t('components.logs.summary.cacheHint'),
      value: data ? formatTokenNumber(data.cache_read_tokens) : '—',
      subValue: data ? formatCacheHitRate(data.cache_read_tokens, data.input_tokens) : '',
    },
    {
      key: 'cost',
      label: t('components.logs.tokenLabels.cost'),
      hint: scopeHint,
      value: formatCurrency(data?.cost_total ?? 0),
    },
  ]
})

const summaryScopeHint = computed(() => {
  switch (filters.dateType) {
    case 'all': {
      const today = startOfTodayLocal()
      const date = `${today.getFullYear()}-${padHour(today.getMonth() + 1)}-${padHour(today.getDate())}`
      return t('components.logs.summary.todayScope', { date })
    }
    case 'today': {
      const today = startOfTodayLocal()
      const date = `${today.getFullYear()}-${padHour(today.getMonth() + 1)}-${padHour(today.getDate())}`
      return t('components.logs.summary.todayScope', { date })
    }
    case 'year': {
      const year = filters.year?.trim()
      return year ? t('components.logs.summary.yearScope', { year }) : ''
    }
    case 'month': {
      const month = filters.month?.trim()
      return month ? t('components.logs.summary.monthScope', { month }) : ''
    }
    case 'day': {
      const day = filters.day?.trim()
      return day ? t('components.logs.summary.dayScope', { date: day }) : ''
    }
    case 'range': {
      const start = filters.rangeStart?.trim()
      const end = filters.rangeEnd?.trim()
      if (!start || !end) return ''
      return t('components.logs.summary.rangeScope', { start, end })
    }
    default:
      return ''
  }
})

const loadProviderOptions = async () => {
  const [fromLogs, fromConfig] = await Promise.all([
    fetchLogProviderRefs(filters.platform).catch((error) => {
      console.error('failed to load provider refs from request logs', error)
      return [] as LogProviderRef[]
    }),
    loadProviderNamesFromConfig(filters.platform).catch((error) => {
      console.error('failed to load providers from config', error)
      return [] as LogProviderOption[]
    }),
  ])

  providerOptions.value = mergeProviderOptions([
    ...buildProviderOptionsFromRefs(fromLogs ?? []),
    ...(fromConfig ?? []),
  ])
}

watch(
  () => filters.platform,
  async () => {
    await loadProviderOptions()
    if (filters.provider && !providerOptions.value.some((option) => option.value === filters.provider)) {
      filters.provider = ''
    }
  },
)

watch(
  [page, () => logs.value.length],
  () => {
    if (logInfoTooltip.visible) {
      hideLogInfoTooltipImmediately()
    }
    if (costTooltip.visible) {
      hideCostTooltipImmediately()
    }
  },
)

onMounted(async () => {
  startThemeObserver()
  window.addEventListener('scroll', handleViewportChange, true)
  window.addEventListener('resize', handleViewportChange)
  await Promise.all([loadDashboard(), loadModelPricingRows()])
  startCountdown()
})

onUnmounted(() => {
  hideLogInfoTooltipImmediately()
  hideCostTooltipImmediately()
  window.removeEventListener('scroll', handleViewportChange, true)
  window.removeEventListener('resize', handleViewportChange)
  stopCountdown()
  themeObserver?.disconnect()
  themeObserver = null
})
</script>

<style scoped>
.logs-summary {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(190px, 1fr));
  gap: 1rem;
  margin-bottom: 0.75rem;
}

.logs-model-share {
  border: 1px solid rgba(15, 23, 42, 0.08);
  border-radius: 16px;
  padding: 1rem 1.2rem;
  background: radial-gradient(circle at top left, rgba(79, 126, 227, 0.08), rgba(15, 23, 42, 0));
}

.logs-model-share__title {
  font-size: 1rem;
  font-weight: 600;
  color: #0f172a;
  margin-bottom: 0.9rem;
}

.logs-model-share__body {
  display: grid;
  grid-template-columns: minmax(170px, 220px) minmax(0, 1fr);
  gap: 1.25rem;
  align-items: center;
}

.logs-model-share__chart-wrap {
  width: min(100%, 190px);
  height: 190px;
  margin: 0 auto;
}

.logs-model-share__table {
  min-width: 0;
}

.logs-model-share__table-head,
.logs-model-share__table-row {
  display: grid;
  grid-template-columns: minmax(0, 2.2fr) minmax(70px, 0.8fr) minmax(90px, 1fr) minmax(90px, 0.9fr);
  gap: 0.75rem;
  align-items: center;
}

.logs-model-share__table-head {
  padding: 0.35rem 0.25rem 0.55rem;
  font-size: 0.83rem;
  font-weight: 600;
  color: #64748b;
}

.logs-model-share__table-body {
  max-height: 210px;
  overflow-y: auto;
}

.logs-model-share__table-row {
  padding: 0.7rem 0.25rem;
  border-top: 1px solid rgba(148, 163, 184, 0.2);
  font-size: 0.92rem;
  color: #334155;
  font-variant-numeric: tabular-nums;
}

.logs-model-share__model {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  min-width: 0;
}

.logs-model-share__dot {
  width: 0.6rem;
  height: 0.6rem;
  border-radius: 999px;
  flex-shrink: 0;
}

.logs-model-share__model-name {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.logs-model-share__cost {
  color: #0f766e;
  font-weight: 600;
}

.logs-model-share__empty {
  font-size: 0.9rem;
  color: #64748b;
  padding: 0.35rem 0.1rem;
}

.summary-meta {
  grid-column: 1 / -1;
  font-size: 0.85rem;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: #64748b;
}

.summary-card {
  border: 1px solid rgba(15, 23, 42, 0.08);
  border-radius: 16px;
  padding: 1rem 1.25rem;
  background: radial-gradient(circle at top, rgba(148, 163, 184, 0.1), rgba(15, 23, 42, 0));
  backdrop-filter: blur(6px);
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
}

.summary-card__label {
  font-size: 0.85rem;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: #475569;
}

.summary-card__value {
  font-size: 1.85rem;
  font-weight: 600;
  color: #0f172a;
}

.summary-card__hint {
  font-size: 0.85rem;
  color: #94a3b8;
}

.summary-card__sub-value {
  font-size: 0.65em;
  font-weight: 400;
  color: #64748b;
  margin-left: 0.25rem;
}

html.dark .summary-card {
  border-color: rgba(255, 255, 255, 0.12);
  background: radial-gradient(circle at top, rgba(148, 163, 184, 0.2), rgba(15, 23, 42, 0.35));
}

html.dark .logs-model-share {
  border-color: rgba(255, 255, 255, 0.12);
  background: radial-gradient(circle at top left, rgba(96, 165, 250, 0.16), rgba(15, 23, 42, 0.36));
}

html.dark .logs-model-share__title {
  color: rgba(248, 250, 252, 0.95);
}

html.dark .logs-model-share__table-head {
  color: rgba(186, 194, 210, 0.86);
}

html.dark .logs-model-share__table-row {
  border-top-color: rgba(148, 163, 184, 0.24);
  color: rgba(226, 232, 240, 0.92);
}

html.dark .logs-model-share__cost {
  color: #6ee7b7;
}

html.dark .logs-model-share__empty {
  color: #94a3b8;
}

html.dark .summary-card__label {
  color: rgba(248, 250, 252, 0.75);
}

html.dark .summary-card__value {
  color: rgba(248, 250, 252, 0.95);
}

html.dark .summary-card__hint {
  color: rgba(186, 194, 210, 0.8);
}

html.dark .summary-card__sub-value {
  color: #94a3b8;
}

@media (max-width: 768px) {
  .logs-summary {
    grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
  }

  .logs-model-share {
    padding: 0.9rem 1rem;
  }

  .logs-model-share__body {
    grid-template-columns: 1fr;
    gap: 0.9rem;
  }

  .logs-model-share__chart-wrap {
    height: 170px;
  }

  .logs-model-share__table-head,
  .logs-model-share__table-row {
    grid-template-columns: minmax(0, 1.9fr) minmax(0, 0.8fr) minmax(0, 0.95fr) minmax(0, 0.85fr);
    gap: 0.55rem;
    font-size: 0.82rem;
  }

  .logs-model-share__table-body {
    max-height: 230px;
  }
}

/* 可点击卡片 */
.summary-card--clickable {
  cursor: pointer;
  transition: transform 0.15s ease, box-shadow 0.15s ease;
}
.summary-card--clickable:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(249, 115, 22, 0.15);
}
.summary-card--clickable:active {
  transform: translateY(0);
}
html.dark .summary-card--clickable:hover {
  box-shadow: 0 4px 12px rgba(249, 115, 22, 0.25);
}

/* 弹窗内容 */
.cost-detail-modal {
  min-height: 120px;
}
.cost-detail-loading,
.cost-detail-empty {
  text-align: center;
  color: #64748b;
  padding: 2rem 0;
}
html.dark .cost-detail-loading,
html.dark .cost-detail-empty {
  color: #94a3b8;
}
.cost-detail-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}
.cost-detail-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0.75rem 1rem;
  background: rgba(148, 163, 184, 0.08);
  border-radius: 8px;
  transition: background 0.15s ease;
}
.cost-detail-item:hover {
  background: rgba(148, 163, 184, 0.12);
}
html.dark .cost-detail-item {
  background: rgba(148, 163, 184, 0.12);
}
html.dark .cost-detail-item:hover {
  background: rgba(148, 163, 184, 0.18);
}
.cost-detail-item__name {
  font-weight: 500;
  color: #1e293b;
}
html.dark .cost-detail-item__name {
  color: #f1f5f9;
}
.cost-detail-item__value {
  font-weight: 600;
  color: #f97316;
  font-variant-numeric: tabular-nums;
}

/* Token 弹窗 */
.token-detail-modal {
  min-height: 80px;
}
.token-detail-list {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}
.token-detail-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0.75rem 1rem;
  background: rgba(148, 163, 184, 0.08);
  border-radius: 8px;
  transition: background 0.15s ease;
}
.token-detail-item:hover {
  background: rgba(148, 163, 184, 0.12);
}
html.dark .token-detail-item {
  background: rgba(148, 163, 184, 0.12);
}
html.dark .token-detail-item:hover {
  background: rgba(148, 163, 184, 0.18);
}
.token-detail-item__name {
  font-weight: 500;
  color: #1e293b;
}
html.dark .token-detail-item__name {
  color: #f1f5f9;
}
.token-detail-item__value {
  font-weight: 600;
  color: #34d399;
  font-variant-numeric: tabular-nums;
}

.token-breakdown-row {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 0.35rem 0.45rem;
}

.cache-create-badges {
  display: inline-flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 0.35rem;
}

.cache-create-badge {
  display: inline-flex;
  align-items: center;
  border-radius: 999px;
  padding: 0.08rem 0.45rem;
  font-size: 0.67rem;
  font-weight: 600;
  line-height: 1.35;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

.cache-create-badge--5m {
  color: #92400e;
  background: rgba(251, 191, 36, 0.2);
  border: 1px solid rgba(245, 158, 11, 0.35);
}

.cache-create-badge--1h {
  color: #0e7490;
  background: rgba(125, 211, 252, 0.2);
  border: 1px solid rgba(14, 165, 233, 0.3);
}

html.dark .cache-create-badge--5m {
  color: #fcd34d;
  background: rgba(161, 98, 7, 0.32);
  border-color: rgba(251, 191, 36, 0.42);
}

html.dark .cache-create-badge--1h {
  color: #7dd3fc;
  background: rgba(12, 74, 110, 0.34);
  border-color: rgba(56, 189, 248, 0.42);
}

/* 表格列 */
.col-model {
  width: 230px;
}
.col-verify {
  width: 132px;
}
.col-cost {
  width: 80px;
}
.cost-cell {
  position: relative;
  color: #f97316;
  font-weight: 500;
  font-variant-numeric: tabular-nums;
}

.cost-cell__value {
  display: inline-flex;
  align-items: center;
  border-radius: 8px;
  padding: 2px 6px;
  margin: -2px -6px;
  cursor: help;
  transition: background 0.18s ease, color 0.18s ease;
}

.cost-cell__value:hover {
  background: rgba(249, 115, 22, 0.14);
  color: #ea580c;
}

.cost-cell__value:focus-visible {
  outline: 2px solid rgba(249, 115, 22, 0.55);
  outline-offset: 1px;
  background: rgba(249, 115, 22, 0.16);
}

html.dark .cost-cell__value:hover {
  background: rgba(251, 146, 60, 0.22);
  color: #fdba74;
}

html.dark .cost-cell__value:focus-visible {
  outline-color: rgba(251, 146, 60, 0.7);
  background: rgba(251, 146, 60, 0.24);
}

.cost-breakdown-tooltip {
  position: fixed;
  transform: translateX(-50%);
  width: min(560px, calc(100vw - 20px));
  max-height: min(72vh, 460px);
  overflow-y: auto;
  overflow-x: hidden;
  border-radius: 14px;
  padding: 0.9rem 1rem;
  border: 1px solid rgba(15, 23, 42, 0.14);
  background: rgba(255, 255, 255, 0.97);
  box-shadow: 0 18px 42px rgba(15, 23, 42, 0.24);
  backdrop-filter: blur(8px);
  z-index: 2600;
  pointer-events: auto;
  overscroll-behavior: contain;
}

.cost-breakdown-tooltip::after {
  content: '';
  position: absolute;
  left: 50%;
  transform: translateX(-50%);
  width: 0;
  height: 0;
  border-left: 8px solid transparent;
  border-right: 8px solid transparent;
}

.cost-breakdown-tooltip.is-above::after {
  top: 100%;
  border-top: 8px solid rgba(255, 255, 255, 0.97);
}

.cost-breakdown-tooltip.is-below::after {
  bottom: 100%;
  border-bottom: 8px solid rgba(255, 255, 255, 0.97);
}

.cost-breakdown-tooltip__model {
  margin: 0;
  font-size: 0.78rem;
  letter-spacing: 0.02em;
  color: #64748b;
}

.cost-breakdown-tooltip__prices {
  margin-top: 0.5rem;
  display: flex;
  flex-direction: column;
  gap: 0.45rem;
}

.cost-breakdown-tooltip__price-row {
  display: grid;
  grid-template-columns: minmax(88px, 0.9fr) minmax(0, 2fr);
  gap: 0.5rem;
  align-items: start;
}

.cost-breakdown-tooltip__price-label {
  font-size: 0.76rem;
  color: #475569;
  line-height: 1.35;
}

.cost-breakdown-tooltip__price-value {
  font-family: 'SFMono-Regular', Menlo, Consolas, monospace;
  font-size: 0.76rem;
  line-height: 1.35;
  color: #0f172a;
  word-break: break-word;
}

.cost-breakdown-tooltip__formula {
  margin: 0.7rem 0 0;
  padding-top: 0.62rem;
  border-top: 1px dashed rgba(148, 163, 184, 0.5);
  font-family: 'SFMono-Regular', Menlo, Consolas, monospace;
  font-size: 0.76rem;
  line-height: 1.45;
  color: #1e293b;
  word-break: break-word;
}

.cost-breakdown-tooltip__note {
  margin: 0.45rem 0 0;
  font-size: 0.72rem;
  line-height: 1.35;
  color: #64748b;
}

html.dark .cost-breakdown-tooltip {
  border-color: rgba(148, 163, 184, 0.36);
  background: rgba(15, 23, 42, 0.95);
  box-shadow: 0 18px 42px rgba(2, 6, 23, 0.55);
}

html.dark .cost-breakdown-tooltip.is-above::after {
  border-top-color: rgba(15, 23, 42, 0.95);
}

html.dark .cost-breakdown-tooltip.is-below::after {
  border-bottom-color: rgba(15, 23, 42, 0.95);
}

html.dark .cost-breakdown-tooltip__model {
  color: #94a3b8;
}

html.dark .cost-breakdown-tooltip__price-label {
  color: #cbd5e1;
}

html.dark .cost-breakdown-tooltip__price-value {
  color: #f8fafc;
}

html.dark .cost-breakdown-tooltip__formula {
  border-top-color: rgba(148, 163, 184, 0.45);
  color: #e2e8f0;
}

html.dark .cost-breakdown-tooltip__note {
  color: #aebcd1;
}

.log-info-tooltip {
  position: fixed;
  transform: translateX(-50%);
  width: min(360px, calc(100vw - 24px));
  max-width: 360px;
  border-radius: 12px;
  padding: 0.75rem 0.85rem;
  border: 1px solid rgba(15, 23, 42, 0.14);
  background: rgba(255, 255, 255, 0.97);
  box-shadow: 0 16px 34px rgba(15, 23, 42, 0.2);
  backdrop-filter: blur(8px);
  z-index: 2550;
  pointer-events: auto;
}

.log-info-tooltip--model {
  width: min(560px, calc(100vw - 20px));
  max-width: 560px;
  max-height: min(72vh, 460px);
  overflow-y: auto;
  overflow-x: hidden;
  padding: 0.82rem 0.95rem;
}

.log-info-tooltip::after {
  content: '';
  position: absolute;
  left: 50%;
  transform: translateX(-50%);
  width: 0;
  height: 0;
  border-left: 7px solid transparent;
  border-right: 7px solid transparent;
}

.log-info-tooltip.is-above::after {
  top: 100%;
  border-top: 7px solid rgba(255, 255, 255, 0.97);
}

.log-info-tooltip.is-below::after {
  bottom: 100%;
  border-bottom: 7px solid rgba(255, 255, 255, 0.97);
}

.log-info-tooltip__title {
  margin: 0;
  font-size: 0.74rem;
  line-height: 1.3;
  letter-spacing: 0.03em;
  text-transform: uppercase;
  color: #64748b;
}

.log-info-tooltip__rows {
  margin-top: 0.48rem;
  display: flex;
  flex-direction: column;
  gap: 0.36rem;
}

.log-info-tooltip--model .log-info-tooltip__rows {
  margin-top: 0.56rem;
  gap: 0.42rem;
}

.log-info-tooltip__row {
  display: grid;
  grid-template-columns: minmax(74px, max-content) minmax(0, 1fr);
  gap: 0.62rem;
  align-items: start;
}

.log-info-tooltip--model .log-info-tooltip__row {
  grid-template-columns: minmax(110px, max-content) minmax(0, 1fr);
  gap: 0.7rem;
}

.log-info-tooltip__label {
  font-size: 0.74rem;
  line-height: 1.35;
  color: #475569;
}

.log-info-tooltip__value {
  font-size: 0.74rem;
  line-height: 1.38;
  color: #0f172a;
  font-family: 'SFMono-Regular', Menlo, Consolas, monospace;
  word-break: break-word;
}

.log-info-tooltip__value.tone-muted {
  color: #64748b;
  font-style: italic;
}

.log-info-tooltip__value.tone-source-provider-api {
  color: #0369a1;
  font-style: normal;
}

.log-info-tooltip__value.tone-source-builtin {
  color: #0f766e;
  font-style: normal;
}

.log-info-tooltip__value.tone-source-none {
  color: #64748b;
  font-style: normal;
}

html.dark .log-info-tooltip {
  border-color: rgba(148, 163, 184, 0.36);
  background: rgba(15, 23, 42, 0.95);
  box-shadow: 0 16px 34px rgba(2, 6, 23, 0.5);
}

html.dark .log-info-tooltip.is-above::after {
  border-top-color: rgba(15, 23, 42, 0.95);
}

html.dark .log-info-tooltip.is-below::after {
  border-bottom-color: rgba(15, 23, 42, 0.95);
}

html.dark .log-info-tooltip__title {
  color: #94a3b8;
}

html.dark .log-info-tooltip__label {
  color: #cbd5e1;
}

html.dark .log-info-tooltip__value {
  color: #f8fafc;
}

html.dark .log-info-tooltip__value.tone-muted,
html.dark .log-info-tooltip__value.tone-source-none {
  color: #94a3b8;
}

html.dark .log-info-tooltip__value.tone-source-provider-api {
  color: #7dd3fc;
}

html.dark .log-info-tooltip__value.tone-source-builtin {
  color: #5eead4;
}

.model-cell {
  max-width: 230px;
  white-space: nowrap;
}

.model-name {
  display: inline-flex;
  align-items: center;
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.model-meta-trigger,
.verify-meta-trigger {
  cursor: help;
  border-radius: 8px;
  transition: background 0.18s ease, box-shadow 0.18s ease, transform 0.18s ease;
}

.model-meta-trigger {
  padding: 2px 6px;
  margin: -2px -6px;
}

.model-meta-trigger:hover {
  background: rgba(59, 130, 246, 0.12);
}

.model-meta-trigger:focus-visible {
  outline: 2px solid rgba(59, 130, 246, 0.52);
  outline-offset: 1px;
  background: rgba(59, 130, 246, 0.16);
}

.verify-meta-trigger:hover {
  transform: translateY(-1px);
  box-shadow: 0 3px 10px rgba(15, 23, 42, 0.12);
}

.verify-meta-trigger:focus-visible {
  outline: 2px solid rgba(99, 102, 241, 0.45);
  outline-offset: 1px;
}

.verify-cell {
  min-width: 120px;
  white-space: nowrap;
}

.verify-tag {
  display: inline-flex;
  align-items: center;
  border-radius: 999px;
  padding: 0.1rem 0.45rem;
  font-size: 0.72rem;
  font-weight: 600;
  line-height: 1.35;
  user-select: none;
}

.verify-tag.verify-match {
  color: #0f766e;
  background: rgba(15, 118, 110, 0.12);
}

.verify-tag.verify-mismatch {
  color: #b42318;
  background: rgba(180, 35, 24, 0.12);
}

.verify-tag.verify-unknown {
  color: #64748b;
  background: rgba(100, 116, 139, 0.12);
}

html.dark .model-meta-trigger:hover {
  background: rgba(59, 130, 246, 0.24);
}

html.dark .model-meta-trigger:focus-visible {
  outline-color: rgba(96, 165, 250, 0.7);
  background: rgba(59, 130, 246, 0.28);
}

html.dark .verify-meta-trigger:hover {
  box-shadow: 0 3px 10px rgba(15, 23, 42, 0.45);
}

html.dark .verify-meta-trigger:focus-visible {
  outline-color: rgba(129, 140, 248, 0.72);
}

html.dark .verify-tag.verify-match {
  color: #5eead4;
  background: rgba(45, 212, 191, 0.2);
}

html.dark .verify-tag.verify-mismatch {
  color: #fda4af;
  background: rgba(244, 63, 94, 0.2);
}

html.dark .verify-tag.verify-unknown {
  color: #cbd5e1;
  background: rgba(148, 163, 184, 0.22);
}
</style>

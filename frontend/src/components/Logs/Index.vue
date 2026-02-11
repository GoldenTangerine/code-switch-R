<template>
  <div class="logs-page">
    <div class="logs-header-bar">
      <div class="logs-header">
        <BaseButton variant="outline" type="button" @click="backToHome">
          {{ t('components.logs.back') }}
        </BaseButton>
        <div class="refresh-indicator">
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
                  <option v-for="provider in providerOptions" :key="provider" :value="provider">
                    {{ provider }}
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

          <section class="logs-storage">
            <div class="logs-storage-header">
              <div class="logs-storage-title">{{ t('components.logs.storage.title') }}</div>
              <div class="logs-storage-actions">
                <BaseButton variant="outline" size="sm" :disabled="storageLoading" @click="loadStorageStats">
                  {{ t('components.logs.storage.refresh') }}
                </BaseButton>
                <BaseButton
                  size="sm"
                  type="button"
                  :disabled="loading || !isFilterValid"
                  @click="applyFilters"
                >
                  {{ t('components.logs.query') }}
                </BaseButton>
              </div>
            </div>

            <div v-if="storageStats" class="mac-panel logs-storage-panel">
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
          </section>
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
                  <div class="model-name">{{ item.model || '—' }}</div>
                  <div
                    v-if="item.matched_pricing_model && item.matched_pricing_model !== item.model"
                    class="model-pricing-match"
                  >
                    {{ t('components.logs.table.matchedPricingModel', { model: item.matched_pricing_model }) }}
                  </div>
                  <div
                    class="model-pricing-source"
                    :class="`source-${priceSourceClass(item)}`"
                  >
                    {{ t('components.logs.table.priceSource', { source: formatPriceSource(item) }) }}
                  </div>
                </td>
                <td class="verify-cell">
                  <span :class="['verify-tag', `verify-${resolveModelVerifyStatus(item)}`]">
                    {{ formatModelVerifyStatus(item) }}
                  </span>
                  <div class="verify-detail">
                    {{ formatModelVerifyDetail(item) }}
                  </div>
                </td>
                <td :class="['code', httpCodeClass(item.http_code)]">{{ item.http_code }}</td>
                <td><span :class="['stream-tag', item.is_stream ? 'on' : 'off']">{{ formatStream(item.is_stream) }}</span></td>
                <td><span :class="['duration-tag', durationColor(item.duration_sec)]">{{ formatDuration(item.duration_sec) }}</span></td>
                <td class="cost-cell">
                  <span
                    class="cost-cell__value"
                    tabindex="0"
                    @mouseenter="showCostTooltip(item, $event)"
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
                  <div>
                    <span class="token-label">{{ t('components.logs.tokenLabels.cacheWrite') }}</span>
                    <span class="token-value">{{ formatTokenNumber(item.cache_create_tokens) }}</span>
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
        v-if="costTooltip.visible && costTooltip.detail"
        ref="costTooltipRef"
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
  fetchLogProviders,
  fetchLogStatsV2,
  fetchProviderStatsV2,
  fetchLogStorageStats,
  clearRequestLogs,
  clearLogStats,
  type RequestLog,
  type LogStats,
  type LogStatsSeries,
  type LogPlatform,
  type ProviderDailyStat,
  type LogStorageStats,
} from '../../services/logs'
import { listModelPricing, type ModelPricingRow } from '../../services/modelPricing'
import {
  Chart,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Tooltip,
  Legend,
} from 'chart.js'
import type { ChartOptions } from 'chart.js'
import { Line } from 'vue-chartjs'
import { showToast } from '../../utils/toast'
import { extractErrorMessage } from '../../utils/error'

Chart.register(CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Legend)

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
const loading = ref(false)
const storageStats = ref<LogStorageStats | null>(null)
const storageLoading = ref(false)
const storageClearing = ref(false)
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
const providerOptions = ref<string[]>([])
const PROVIDER_CONFIG_CACHE_TTL_MS = 60_000
const providerConfigCache = new Map<string, { loadedAt: number; names: string[] }>()
const statsSeries = computed<LogStatsSeries[]>(() => stats.value?.series ?? [])

type CostTooltipPlacement = 'above' | 'below'

type CostTooltipPriceLine = {
  key: string
  label: string
  value: string
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

const costTooltipRef = ref<HTMLElement | null>(null)
const costTooltipAnchorRef = ref<HTMLElement | null>(null)
const costTooltipRequestId = ref(0)
let costTooltipHideTimer: number | null = null
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

const chartData = computed(() => {
  const series = statsSeries.value
  return {
    labels: series.map((item) => formatSeriesLabel(item.day)),
    datasets: [
      {
        label: t('components.logs.tokenLabels.cost'),
        data: series.map((item) => Number(((item.total_cost ?? 0)).toFixed(4))),
        borderColor: '#f97316',
        backgroundColor: 'rgba(249, 115, 22, 0.2)',
        tension: 0.3,
        fill: false,
        yAxisID: 'yCost',
      },
      {
        label: t('components.logs.tokenLabels.input'),
        data: series.map((item) => item.input_tokens ?? 0),
        borderColor: '#34d399',
        backgroundColor: 'rgba(52, 211, 153, 0.25)',
        tension: 0.35,
        fill: true,
      },
      {
        label: t('components.logs.tokenLabels.output'),
        data: series.map((item) => item.output_tokens ?? 0),
        borderColor: '#60a5fa',
        backgroundColor: 'rgba(96, 165, 250, 0.2)',
        tension: 0.35,
        fill: true,
      },
      {
        label: t('components.logs.tokenLabels.reasoning'),
        data: series.map((item) => item.reasoning_tokens ?? 0),
        borderColor: '#f472b6',
        backgroundColor: 'rgba(244, 114, 182, 0.2)',
        tension: 0.35,
        fill: true,
      },
      {
        label: t('components.logs.tokenLabels.cacheWrite'),
        data: series.map((item) => item.cache_create_tokens ?? 0),
        borderColor: '#fbbf24',
        backgroundColor: 'rgba(251, 191, 36, 0.2)',
        tension: 0.35,
        fill: false,
      },
      {
        label: t('components.logs.tokenLabels.cacheRead'),
        data: series.map((item) => item.cache_read_tokens ?? 0),
        borderColor: '#38bdf8',
        backgroundColor: 'rgba(56, 189, 248, 0.15)',
        tension: 0.35,
        fill: false,
      },
    ],
  }
})

const chartOptions: ChartOptions<'line'> = {
  responsive: true,
  maintainAspectRatio: false,
  interaction: {
    mode: 'index',
    intersect: false,
  },
  plugins: {
    legend: {
      labels: {
        color: '#0f172a',
        font: {
          size: 12,
          weight: 500,
        },
      },
    },
  },
  scales: {
    x: {
      grid: { display: false },
      ticks: { color: '#94a3b8' },
    },
    y: {
      beginAtZero: true,
      ticks: { color: '#94a3b8' },
      grid: { color: 'rgba(148, 163, 184, 0.2)' },
    },
    yCost: {
      position: 'right',
      beginAtZero: true,
      grid: { drawOnChartArea: false },
      ticks: {
        color: '#475569',
        callback: (value: string | number) => {
          const numeric = typeof value === 'number' ? value : Number(value)
          if (Number.isNaN(numeric)) return '$0'
          if (numeric >= 1) return `$${numeric.toFixed(2)}`
          return `$${numeric.toFixed(4)}`
        },
      },
    },
  },
}

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

const loadProviderNamesFromConfig = async (platform: LogPlatform | ''): Promise<string[]> => {
  const cacheKey = platform
  const now = Date.now()
  const cached = providerConfigCache.get(cacheKey)
  if (cached && now - cached.loadedAt < PROVIDER_CONFIG_CACHE_TTL_MS) {
    return cached.names
  }

  const names = new Set<string>()

  const includeClaude = platform === '' || platform === 'claude'
  const includeCodex = platform === '' || platform === 'codex'
  const includeGemini = platform === '' || platform === 'gemini'

  if (includeClaude) {
    try {
      const providers = await LoadProviders('claude')
      for (const provider of providers ?? []) {
        const name = normalizeProviderName(provider?.name ?? '')
        if (name) names.add(name)
      }
    } catch (error) {
      console.error('failed to load claude providers from config', error)
    }
  }

  if (includeCodex) {
    try {
      const providers = await LoadProviders('codex')
      for (const provider of providers ?? []) {
        const name = normalizeProviderName(provider?.name ?? '')
        if (name) names.add(name)
      }
    } catch (error) {
      console.error('failed to load codex providers from config', error)
    }
  }

  if (includeGemini) {
    try {
      const providers = await GetGeminiProviders()
      for (const provider of providers ?? []) {
        const name = normalizeProviderName(provider?.name ?? '')
        if (name) names.add(name)
      }
    } catch (error) {
      console.error('failed to load gemini providers from config', error)
    }
  }

  const result = Array.from(names)
  providerConfigCache.set(cacheKey, { loadedAt: now, names: result })
  return result
}

const syncProviderOptionsFromLogs = (items: RequestLog[]) => {
  if (!items.length) return
  const merged = new Set(providerOptions.value.map(normalizeProviderName).filter(Boolean))
  for (const item of items) {
    const name = normalizeProviderName(item.provider ?? '')
    if (name) {
      merged.add(name)
    }
  }
  const next = Array.from(merged)
  next.sort((a, b) => a.localeCompare(b))
  providerOptions.value = next
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

const loadDashboard = async () => {
  await Promise.all([loadLogs(), loadStats(), loadProviderOptions()])
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

const formatModelVerifyDetail = (item: RequestLog) => {
  const requestedModel = String(item.requested_model ?? '').trim()
  const responseModel = String(item.response_model ?? '').trim()
  if (!requestedModel || !responseModel) {
    return t('components.logs.table.verifyDetailUnknown')
  }
  return t('components.logs.table.verifyDetail', {
    requested: requestedModel,
    response: responseModel,
  })
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
  if (modelPricingLoaded.value || modelPricingLoading.value) return
  modelPricingLoading.value = true
  try {
    modelPricingRows.value = (await listModelPricing()) ?? []
    modelPricingLoaded.value = true
  } catch (error) {
    console.error('failed to load model pricing rows', error)
  } finally {
    modelPricingLoading.value = false
  }
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
    .some(value => value !== undefined && value !== null)

const buildCostTooltipDetail = (item: RequestLog): CostTooltipDetail => {
  const source = resolvePriceSource(item)
  const fallbackModelName = String(item.matched_pricing_model ?? item.model ?? '').trim() || '—'
  const recordedCost = safeNumber(item.total_cost)

  if (source === 'provider_api') {
    return {
      pricingModel: fallbackModelName,
      hasPricing: true,
      priceLines: [],
      formula: t('components.logs.costTooltip.providerApiFormula'),
      note: t('components.logs.costTooltip.providerApiHint'),
      recordedCostHint: t('components.logs.costTooltip.recordedCostHint', {
        cost: formatUsdPrecise(recordedCost),
      }),
    }
  }

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

  const completionMultiplier = inputPerToken > 0 ? outputPerToken / inputPerToken : 0
  const cacheCreateMultiplier = inputPerToken > 0 ? cacheCreatePerToken / inputPerToken : 0
  const cacheReadMultiplier = inputPerToken > 0 ? cacheReadPerToken / inputPerToken : 0
  const groupMultiplier = resolveGroupMultiplier(item)
  const calculatedTotal = inputCost + cacheCreateCost + cacheReadCost + outputCost + reasoningCost

  const tokensUnit = '/ 1M tokens'
  const priceLines: CostTooltipPriceLine[] = [
    {
      key: 'prompt',
      label: t('components.logs.costTooltip.promptPrice'),
      value: `${formatUsdPerMillion(inputPerToken)} ${tokensUnit}`,
    },
  ]

  const completionValue =
    completionMultiplier > 0 && inputPerToken > 0
      ? `${formatUsdPerMillion(inputPerToken)} * ${formatMultiplierValue(completionMultiplier)} = ${formatUsdPerMillion(outputPerToken)} ${tokensUnit}`
      : `${formatUsdPerMillion(outputPerToken)} ${tokensUnit}`
  priceLines.push({
    key: 'completion',
    label: t('components.logs.costTooltip.completionPrice'),
    value: completionValue,
  })

  const cacheCreateValue =
    cacheCreateMultiplier > 0 && inputPerToken > 0
      ? `${formatUsdPerMillion(inputPerToken)} * ${formatMultiplierValue(cacheCreateMultiplier)} = ${formatUsdPerMillion(cacheCreatePerToken)} ${tokensUnit} (${t('components.logs.costTooltip.cacheCreateMultiplierLabel', { multiplier: formatMultiplierValue(cacheCreateMultiplier) })})`
      : `${formatUsdPerMillion(cacheCreatePerToken)} ${tokensUnit}`
  priceLines.push({
    key: 'cacheCreate',
    label: t('components.logs.costTooltip.cacheCreatePrice'),
    value: cacheCreateValue,
  })

  if (cacheReadTokens > 0) {
    const cacheReadValue =
      cacheReadMultiplier > 0 && inputPerToken > 0
        ? `${formatUsdPerMillion(inputPerToken)} * ${formatMultiplierValue(cacheReadMultiplier)} = ${formatUsdPerMillion(cacheReadPerToken)} ${tokensUnit} (${t('components.logs.costTooltip.cacheReadMultiplierLabel', { multiplier: formatMultiplierValue(cacheReadMultiplier) })})`
        : `${formatUsdPerMillion(cacheReadPerToken)} ${tokensUnit}`
    priceLines.push({
      key: 'cacheRead',
      label: t('components.logs.costTooltip.cacheReadPrice'),
      value: cacheReadValue,
    })
  }

  if (reasoningTokens > 0) {
    priceLines.push({
      key: 'reasoning',
      label: t('components.logs.costTooltip.reasoningPrice'),
      value: `${formatUsdPerMillion(reasoningPerToken)} ${tokensUnit}`,
    })
  }

  const formulaParts: string[] = []
  if (inputTokens > 0 && inputPerToken > 0) {
    formulaParts.push(
      `${t('components.logs.costTooltip.usagePrompt')} ${inputTokens.toLocaleString()} tokens / 1M tokens * ${formatUsdPerMillion(inputPerToken)}`
    )
  }
  if (cacheCreateTokens > 0 && cacheCreatePerToken > 0) {
    formulaParts.push(
      `${t('components.logs.costTooltip.usageCacheCreate')} ${cacheCreateTokens.toLocaleString()} tokens / 1M tokens * ${formatUsdPerMillion(cacheCreatePerToken)} (${t('components.logs.costTooltip.cacheCreateMultiplierLabel', { multiplier: formatMultiplierValue(cacheCreateMultiplier) })})`
    )
  }
  if (cacheReadTokens > 0 && cacheReadPerToken > 0) {
    formulaParts.push(
      `${t('components.logs.costTooltip.usageCacheRead')} ${cacheReadTokens.toLocaleString()} tokens / 1M tokens * ${formatUsdPerMillion(cacheReadPerToken)} (${t('components.logs.costTooltip.cacheReadMultiplierLabel', { multiplier: formatMultiplierValue(cacheReadMultiplier) })})`
    )
  }
  if (outputTokens > 0 && outputPerToken > 0) {
    formulaParts.push(
      `${t('components.logs.costTooltip.usageCompletion')} ${outputTokens.toLocaleString()} tokens / 1M tokens * ${formatUsdPerMillion(outputPerToken)} * ${t('components.logs.costTooltip.groupMultiplierLabel', { multiplier: formatMultiplierValue(groupMultiplier) })}`
    )
  }
  if (reasoningTokens > 0 && reasoningPerToken > 0) {
    formulaParts.push(
      `${t('components.logs.costTooltip.usageReasoning')} ${reasoningTokens.toLocaleString()} tokens / 1M tokens * ${formatUsdPerMillion(reasoningPerToken)} * ${t('components.logs.costTooltip.groupMultiplierLabel', { multiplier: formatMultiplierValue(groupMultiplier) })}`
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

const clearCostTooltipHideTimer = () => {
  if (costTooltipHideTimer != null) {
    window.clearTimeout(costTooltipHideTimer)
    costTooltipHideTimer = null
  }
}

const hideCostTooltipImmediately = () => {
  clearCostTooltipHideTimer()
  costTooltipRequestId.value += 1
  costTooltipAnchorRef.value = null
  costTooltip.visible = false
  costTooltip.detail = null
}

const scheduleHideCostTooltip = () => {
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

const showCostTooltip = async (item: RequestLog, event: MouseEvent | FocusEvent) => {
  const target = event.currentTarget as HTMLElement | null
  if (!target) return
  clearCostTooltipHideTimer()
  costTooltipAnchorRef.value = target
  const requestId = ++costTooltipRequestId.value
  if (resolvePriceSource(item) !== 'provider_api') {
    await loadModelPricingRows()
  }
  if (requestId !== costTooltipRequestId.value) return
  if (costTooltipAnchorRef.value !== target) return
  costTooltip.detail = buildCostTooltipDetail(item)
  costTooltip.visible = true
  updateCostTooltipPosition(target)
  await nextTick()
  if (requestId !== costTooltipRequestId.value) return
  if (costTooltipAnchorRef.value !== target) return
  updateCostTooltipPosition(target)
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

const startOfTodayLocal = () => {
  const now = new Date()
  now.setHours(0, 0, 0, 0)
  return now
}

const statsCards = computed(() => {
  const data = stats.value
  const scopeHint = summaryScopeHint.value
  const totalTokens =
    (data?.input_tokens ?? 0) + (data?.output_tokens ?? 0) + (data?.reasoning_tokens ?? 0)
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
    fetchLogProviders(filters.platform).catch((error) => {
      console.error('failed to load providers from request logs', error)
      return [] as string[]
    }),
    loadProviderNamesFromConfig(filters.platform).catch((error) => {
      console.error('failed to load providers from config', error)
      return [] as string[]
    }),
  ])

  const merged = new Set<string>()
  for (const name of [...(fromLogs ?? []), ...(fromConfig ?? [])]) {
    const normalized = normalizeProviderName(name ?? '')
    if (normalized) merged.add(normalized)
  }
  providerOptions.value = Array.from(merged)
  providerOptions.value.sort((a, b) => a.localeCompare(b))
}

watch(
  () => filters.platform,
  async () => {
    await loadProviderOptions()
    if (filters.provider && !providerOptions.value.includes(filters.provider)) {
      filters.provider = ''
    }
  },
)

watch(
  [page, () => logs.value.length],
  () => {
    if (costTooltip.visible) {
      hideCostTooltipImmediately()
    }
  },
)

onMounted(async () => {
  startThemeObserver()
  window.addEventListener('scroll', handleViewportChange, true)
  window.addEventListener('resize', handleViewportChange)
  await Promise.all([loadDashboard(), loadStorageStats(), loadModelPricingRows()])
  startCountdown()
})

onUnmounted(() => {
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

/* 金额列 */
.col-verify {
  width: 190px;
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

.model-cell {
  white-space: normal;
}

.model-name {
  word-break: break-word;
}

.model-pricing-match {
  margin-top: 0.2rem;
  font-size: 0.75rem;
  color: #64748b;
  line-height: 1.35;
  word-break: break-word;
}

html.dark .model-pricing-match {
  color: #94a3b8;
}

.model-pricing-source {
  margin-top: 0.2rem;
  font-size: 0.72rem;
  line-height: 1.35;
  word-break: break-word;
}

.model-pricing-source.source-provider-api {
  color: #0369a1;
}

.model-pricing-source.source-builtin {
  color: #0f766e;
}

.model-pricing-source.source-none {
  color: #94a3b8;
}

.verify-cell {
  min-width: 180px;
}

.verify-tag {
  display: inline-flex;
  align-items: center;
  border-radius: 999px;
  padding: 0.1rem 0.45rem;
  font-size: 0.72rem;
  font-weight: 600;
  line-height: 1.35;
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

.verify-detail {
  margin-top: 0.22rem;
  font-size: 0.7rem;
  line-height: 1.35;
  color: #64748b;
  word-break: break-word;
}

html.dark .model-pricing-source.source-provider-api {
  color: #7dd3fc;
}

html.dark .model-pricing-source.source-builtin {
  color: #5eead4;
}

html.dark .model-pricing-source.source-none {
  color: #94a3b8;
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

html.dark .verify-detail {
  color: #94a3b8;
}
</style>

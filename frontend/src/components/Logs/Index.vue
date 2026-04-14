<template>
  <div class="logs-page">
    <LogsHeaderBar
      :countdown="countdown"
      :loading="loading"
      :storage-loading="storageLoading"
      @back="backToHome"
      @open-storage="openStorageModal"
      @refresh="manualRefresh"
    />

    <div class="logs-content">
      <LogsFilterBar
        :filters="filters"
        :provider-options="providerOptions"
        :loading="loading"
        :is-filter-valid="isFilterValid"
        :year-picker-value="yearPickerValue"
        :month-picker-value="monthPickerValue"
        :day-picker-value="dayPickerValue"
        :range-picker-value="rangePickerValue"
        :is-dark-theme="isDarkTheme"
        @submit="applyFilters"
        @update:platform="updateFilterPlatform"
        @update:provider="updateFilterProvider"
        @update:date-type="updateFilterDateType"
        @update:year-picker-value="updateYearPickerValue"
        @update:month-picker-value="updateMonthPickerValue"
        @update:day-picker-value="updateDayPickerValue"
        @update:range-picker-value="updateRangePickerValue"
      />

      <LogsSummaryCards :stats-cards="statsCards" @card-click="handleCardClick" />

      <LogsChartsPanel
        :model-share-rows="modelShareRows"
        :model-share-chart-data="modelShareChartData"
        :model-share-chart-options="modelShareChartOptions"
        :chart-data="chartData"
        :chart-options="chartOptions"
        :format-number="formatNumber"
        :format-token-number="formatTokenNumber"
        :format-currency="formatCurrency"
      />

      <LogsTable
        :items="pagedLogs"
        :loading="loading"
        :log-info-tooltip-visible="logInfoTooltip.visible"
        :cost-tooltip-visible="costTooltip.visible"
        :formatters="logsTableFormatters"
        :handlers="logsTableHandlers"
      />

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

      <BasePagination
        v-if="logs.length > 0"
        class="logs-pagination-bar"
        :page="page"
        :total-pages="totalPages"
        :page-size="pageSize"
        :page-size-options="pageSizeOptions"
        :loading="loading"
        align="end"
        compact
        @update:page="setPage"
        @update:page-size="setPageSize"
      />

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
        <LogsStorageModal
          :open="storageModal.open"
          :storage-loading="storageLoading"
          :storage-clearing="storageClearing"
          :storage-stats="storageStats"
          :provider-storage-stats="providerStorageStats"
          :storage-heatmap-year="storageHeatmapYear"
          :storage-heatmap-years="storageHeatmapYears"
          :storage-heatmap-loading="storageHeatmapLoading"
          :storage-heatmap="storageHeatmap"
          :selected-storage-heatmap-day="selectedStorageHeatmapDay"
          :selected-storage-heatmap-date-label="selectedStorageHeatmapDateLabel"
          :storage-day-logs-showing-text="storageDayLogsShowingText"
          :storage-day-logs-loading="storageDayLogsLoading"
          :storage-day-logs="storageDayLogs"
          :paged-storage-day-logs="pagedStorageDayLogs"
          :storage-day-logs-page="storageDayLogsPage"
          :storage-day-logs-page-size="storageDayLogsPageSize"
          :storage-day-logs-page-size-options="storageDayLogsPageSizeOptions"
          :storage-day-logs-total-pages="storageDayLogsTotalPages"
          :storage-heatmap-has-data="storageHeatmapHasData"
          :storage-heatmap-tooltip="storageHeatmapTooltip"
          :bind-storage-heatmap-tooltip-ref="bindStorageHeatmapTooltipRef"
          :formatters="storageModalFormatters"
          :handlers="storageModalHandlers"
          @close="closeStorageModal"
        />

        <!-- 金额明细弹窗 -->
        <LogsCostDetailModal
          :open="costDetailModal.open"
          :loading="costDetailModal.loading"
          :data="costDetailModal.data"
          :format-currency="formatCurrency"
          @close="closeCostDetailModal"
        />

        <!-- Token 明细弹窗 -->
        <LogsTokenDetailModal
          :open="tokenDetailModal.open"
          :stats="stats"
          :format-token-number="formatTokenNumber"
          @close="closeTokenDetailModal"
        />

        <!-- 请求体 / 返回体弹窗 -->
        <LogsPayloadDetailModal
          :open="payloadDetailModal.open"
          :loading="payloadDetailModal.loading"
          :log-id="payloadDetailModal.logId"
          :detail="payloadDetailModal.detail"
          :request-payload-preview="requestPayloadPreview"
          :response-payload-preview="responsePayloadPreview"
          :copy-payload-detail="copyPayloadDetail"
          @close="closePayloadDetailModal"
        />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { Events } from '@wailsio/runtime'
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
import BaseButton from '../common/BaseButton.vue'
import BaseModal from '../common/BaseModal.vue'
import BasePagination from '../common/BasePagination.vue'
import LogsChartsPanel from './components/LogsChartsPanel.vue'
import LogsFilterBar from './components/LogsFilterBar.vue'
import LogsHeaderBar from './components/LogsHeaderBar.vue'
import LogsSummaryCards from './components/LogsSummaryCards.vue'
import LogsTable from './components/LogsTable.vue'
import LogsCostDetailModal from './modals/LogsCostDetailModal.vue'
import LogsPayloadDetailModal from './modals/LogsPayloadDetailModal.vue'
import LogsStorageModal from './modals/LogsStorageModal.vue'
import LogsTokenDetailModal from './modals/LogsTokenDetailModal.vue'
import { useLogsAutoRefresh } from './composables/useLogsAutoRefresh'
import { useLogsChartsPresentation } from './composables/useLogsChartsPresentation'
import { useLogsCostTooltip } from './composables/useLogsCostTooltip'
import { useLogsDetailModals } from './composables/useLogsDetailModals'
import { useLogsFilters } from './composables/useLogsFilters'
import { useLogsInfoTooltip } from './composables/useLogsInfoTooltip'
import { useLogsPageData } from './composables/useLogsPageData'
import { useLogsPayloadDetail } from './composables/useLogsPayloadDetail'
import { useLogsPricingDetails } from './composables/useLogsPricingDetails'
import { useLogsStorageModalController } from './composables/useLogsStorageModalController'
import { MODEL_PRICING_CHANGED_EVENT } from '../../services/modelPricing'
import { formatCurrency, formatNumber, formatTokenNumber } from './utils'

Chart.register(CategoryScale, LinearScale, ArcElement, PointElement, LineElement, Tooltip, Legend)

const { t, locale } = useI18n()
const router = useRouter()

const isDarkTheme = ref(document.documentElement.classList.contains('dark'))
let themeObserver: MutationObserver | null = null
let unsubscribeModelPricingChanged: (() => void) | null = null

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

const {
  filters,
  yearPickerValue,
  monthPickerValue,
  dayPickerValue,
  rangePickerValue,
  updateFilterPlatform,
  updateFilterProvider,
  updateFilterDateType,
  updateYearPickerValue,
  updateMonthPickerValue,
  updateDayPickerValue,
  updateRangePickerValue,
  computeDateRange,
  isFilterValid,
  summaryScopeHint,
} = useLogsFilters({ t })

const {
  logs,
  stats,
  modelStats,
  loading,
  page,
  pageSize,
  pageSizeOptions,
  providerOptions,
  pagedLogs,
  totalPages,
  loadDashboard,
  setPage,
  setPageSize,
  resetPage,
} = useLogsPageData({
  filters,
  computeDateRange,
})

const {
  statsCards,
  modelShareRows,
  modelShareChartData,
  modelShareChartOptions,
  chartData,
  chartOptions,
  logsTableFormatters,
} = useLogsChartsPresentation({
  t,
  isDarkTheme,
  stats,
  modelStats,
  statsSeries: computed(() => stats.value?.series ?? []),
  summaryScopeHint,
  computeDateRange,
  dateType: computed(() => filters.dateType),
})

const {
  costDetailModal,
  tokenDetailModal,
  closeCostDetailModal,
  closeTokenDetailModal,
  handleCardClick,
} = useLogsDetailModals({
  filters,
  computeDateRange,
})

const {
  payloadDetailModal,
  requestPayloadPreview,
  responsePayloadPreview,
  copyPayloadDetail,
  openPayloadDetailModal,
  closePayloadDetailModal,
} = useLogsPayloadDetail({ t })

const {
  storageStats,
  providerStorageStats,
  storageLoading,
  storageClearing,
  storageModal,
  openStorageModal,
  closeStorageModal,
  storageClearConfirm,
  storageClearConfirmMessage,
  storageClearConfirmActionLabel,
  closeStorageClearConfirm,
  confirmStorageClear,
  storageHeatmapYear,
  storageHeatmapYears,
  storageHeatmapLoading,
  storageHeatmap,
  selectedStorageHeatmapDay,
  selectedStorageHeatmapDateLabel,
  storageDayLogsShowingText,
  storageDayLogsLoading,
  storageDayLogs,
  pagedStorageDayLogs,
  storageDayLogsPage,
  storageDayLogsPageSize,
  storageDayLogsPageSizeOptions,
  storageDayLogsTotalPages,
  storageHeatmapHasData,
  storageHeatmapTooltip,
  bindStorageHeatmapTooltipRef,
  storageModalFormatters,
  storageModalHandlers,
  disposeStorageModalController,
} = useLogsStorageModalController({
  locale,
  t,
  loadDashboard,
  openPayloadDetailModal,
})

const {
  modelPricingLoaded,
  modelPricingStale,
  markModelPricingStale,
  loadModelPricingRows,
  buildCostTooltipDetail,
  buildModelInfoTooltipDetail,
  buildVerifyInfoTooltipDetail,
} = useLogsPricingDetails({ t })

const {
  countdown,
  resetTimer,
  startCountdown,
  stopCountdown,
  manualRefresh,
} = useLogsAutoRefresh(loadDashboard)

const applyFilters = async () => {
  if (!isFilterValid.value) {
    return
  }
  resetPage()
  await loadDashboard()
  resetTimer()
}

const backToHome = () => {
  router.push('/')
}

function hideLogInfoTooltipPeer() {
  hideLogInfoTooltipImmediately()
}

function hideCostTooltipPeer() {
  hideCostTooltipImmediately()
}

const {
  logInfoTooltipRef,
  logInfoTooltip,
  showModelInfoTooltip,
  showVerifyInfoTooltip,
  scheduleShowModelInfoTooltip,
  scheduleShowVerifyInfoTooltip,
  moveLogInfoTooltip,
  hideLogInfoTooltip,
  hideLogInfoTooltipImmediately,
  handleLogInfoTooltipMouseEnter,
  handleLogInfoTooltipMouseLeave,
} = useLogsInfoTooltip({
  buildModelInfoTooltipDetail,
  buildVerifyInfoTooltipDetail,
  ensureModelPricingLoaded: loadModelPricingRows,
  modelPricingLoaded,
  modelPricingStale,
  hidePeerTooltipImmediately: hideCostTooltipPeer,
})

const {
  costTooltipRef,
  costTooltip,
  showCostTooltip,
  scheduleShowCostTooltip,
  moveCostTooltip,
  hideCostTooltip,
  hideCostTooltipImmediately,
  handleCostTooltipMouseEnter,
  handleCostTooltipMouseLeave,
} = useLogsCostTooltip({
  buildCostTooltipDetail,
  ensureModelPricingLoaded: loadModelPricingRows,
  modelPricingLoaded,
  modelPricingStale,
  hidePeerTooltipImmediately: hideLogInfoTooltipPeer,
})

const handleModelPricingChanged = () => {
  markModelPricingStale()
}

const handleViewportChange = () => {
  if (logInfoTooltip.visible) {
    hideLogInfoTooltipImmediately()
  }
  if (costTooltip.visible) {
    hideCostTooltipImmediately()
  }
}

const logsTableHandlers = {
  scheduleShowModelInfoTooltip,
  moveLogInfoTooltip,
  hideLogInfoTooltip,
  showModelInfoTooltip,
  hideLogInfoTooltipImmediately,
  scheduleShowVerifyInfoTooltip,
  showVerifyInfoTooltip,
  openPayloadDetailModal,
  scheduleShowCostTooltip,
  moveCostTooltip,
  hideCostTooltip,
  showCostTooltip,
  hideCostTooltipImmediately,
}

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
  unsubscribeModelPricingChanged = Events.On(
    MODEL_PRICING_CHANGED_EVENT,
    handleModelPricingChanged as Events.Callback,
  )
  await Promise.all([loadDashboard(), loadModelPricingRows()])
  startCountdown()
})

onUnmounted(() => {
  hideLogInfoTooltipImmediately()
  hideCostTooltipImmediately()
  disposeStorageModalController()
  window.removeEventListener('scroll', handleViewportChange, true)
  window.removeEventListener('resize', handleViewportChange)
  stopCountdown()
  unsubscribeModelPricingChanged?.()
  unsubscribeModelPricingChanged = null
  themeObserver?.disconnect()
  themeObserver = null
})
</script>

<style scoped src="./Index.css"></style>

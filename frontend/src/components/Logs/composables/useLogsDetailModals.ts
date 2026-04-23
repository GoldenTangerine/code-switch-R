import { reactive, type Ref } from 'vue'
import { fetchProviderStatsV2, type ProviderDailyStat } from '../../../services/logs'
import { extractErrorMessage } from '../../../utils/error'
import type { LogsFiltersState } from '../types'
import type { LogsDateRange } from './useLogsFilters'

type UseLogsDetailModalsOptions = {
  appliedFilters: Ref<LogsFiltersState>
  appliedDateRange: Ref<LogsDateRange>
}

export function useLogsDetailModals(options: UseLogsDetailModalsOptions) {
  const { appliedFilters, appliedDateRange } = options

  const costDetailModal = reactive<{
    open: boolean
    loading: boolean
    error: string
    updatedAt: number
    data: ProviderDailyStat[]
  }>({
    open: false,
    loading: false,
    error: '',
    updatedAt: 0,
    data: [],
  })

  const tokenDetailModal = reactive<{
    open: boolean
  }>({
    open: false,
  })

  let activeCostDetailRequestId = 0

  const openCostDetailModal = async () => {
    const requestId = ++activeCostDetailRequestId
    costDetailModal.open = true
    costDetailModal.loading = true
    costDetailModal.error = ''
    costDetailModal.updatedAt = 0
    costDetailModal.data = []

    try {
      const range = appliedDateRange.value
      const filters = appliedFilters.value
      const stats = await fetchProviderStatsV2({
        platform: filters.platform,
        provider: filters.provider,
        startAt: range.startAt,
        endAt: range.endAt,
      })
      if (requestId !== activeCostDetailRequestId) return
      costDetailModal.data = (stats ?? [])
        .filter(item => item.cost_total > 0)
        .sort((a, b) => b.cost_total - a.cost_total)
      costDetailModal.updatedAt = Date.now()
    } catch (error) {
      if (requestId !== activeCostDetailRequestId) return
      costDetailModal.error = extractErrorMessage(error)
      console.error('failed to load provider daily stats', error)
    } finally {
      if (requestId !== activeCostDetailRequestId) return
      costDetailModal.loading = false
    }
  }

  const closeCostDetailModal = () => {
    activeCostDetailRequestId += 1
    costDetailModal.open = false
    costDetailModal.loading = false
  }

  const openTokenDetailModal = () => {
    tokenDetailModal.open = true
  }

  const closeTokenDetailModal = () => {
    tokenDetailModal.open = false
  }

  const handleCardClick = (key: string) => {
    if (key === 'cost') {
      void openCostDetailModal()
    } else if (key === 'tokens') {
      openTokenDetailModal()
    }
  }

  return {
    costDetailModal,
    tokenDetailModal,
    openCostDetailModal,
    closeCostDetailModal,
    openTokenDetailModal,
    closeTokenDetailModal,
    handleCardClick,
  }
}

import { reactive } from 'vue'
import { fetchProviderStatsV2, type ProviderDailyStat } from '../../../services/logs'
import type { LogsFiltersState } from '../types'

type LogsDateRange = {
  startAt: string
  endAt: string
}

type UseLogsDetailModalsOptions = {
  filters: LogsFiltersState
  computeDateRange: () => LogsDateRange | null
}

export function useLogsDetailModals(options: UseLogsDetailModalsOptions) {
  const { filters, computeDateRange } = options

  const costDetailModal = reactive<{
    open: boolean
    loading: boolean
    data: ProviderDailyStat[]
  }>({
    open: false,
    loading: false,
    data: [],
  })

  const tokenDetailModal = reactive<{
    open: boolean
  }>({
    open: false,
  })

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
      costDetailModal.data = (stats ?? [])
        .filter(item => item.cost_total > 0)
        .sort((a, b) => b.cost_total - a.cost_total)
    } catch (error) {
      console.error('failed to load provider daily stats', error)
    } finally {
      costDetailModal.loading = false
    }
  }

  const closeCostDetailModal = () => {
    costDetailModal.open = false
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

import type { LogPlatform } from '../../services/logs'

export type LogDateFilterType = 'all' | 'today' | 'year' | 'month' | 'day' | 'range'

export type LogsFiltersState = {
  platform: LogPlatform | ''
  provider: string
  model: string
  dateType: LogDateFilterType
  year: string
  month: string
  day: string
  rangeStart: string
  rangeEnd: string
}

export type LogProviderOption = {
  value: string
  label: string
  providerId?: string
  providerName: string
}

export type LogsDataTab = 'requests' | 'providers' | 'models'

export type LogsSummaryCardTone = 'blue' | 'purple' | 'amber' | 'green'

export type LogsSummaryCardValueSize = 'regular' | 'compact' | 'dense'

export type LogsSummaryBadgeTone = 'neutral' | 'alert' | 'success' | 'warning'

export type LogsSummaryMetricTone = 'neutral' | 'success' | 'warning' | 'danger'

export type LogsSummaryMicroPoint = {
  label: string
  value: number
  intensity: number
  active?: boolean
}

export type LogsSummaryProgress = {
  label: string
  value: number
  valueLabel: string
  tone: 'primary' | 'alert' | 'success'
}

export type LogsSummaryMetric = {
  label: string
  value: string
  tone?: LogsSummaryMetricTone
  icon?: 'up' | 'alert' | 'spark'
  animated?: boolean
}

export type LogsSummaryRatioSegment = {
  label: string
  value: number
  valueLabel?: string
  color: string
}

export type LogsSummaryBadge = {
  text: string
  tone: LogsSummaryBadgeTone
}

export type LogsSummaryCard = {
  key: string
  label: string
  subtitle: string
  statusLabel?: string
  value: string
  valueSuffix?: string
  hint: string
  subValue?: string
  tone: LogsSummaryCardTone
  valueSize?: LogsSummaryCardValueSize
  badge?: LogsSummaryBadge
  miniBars?: {
    label: string
    points: LogsSummaryMicroPoint[]
    footerLeft: string
    footerRight: string
  }
  progress?: LogsSummaryProgress
  ratio?: {
    label: string
    segments: LogsSummaryRatioSegment[]
  }
  ring?: {
    label: string
    value: number
    valueLabel: string
    pulse: boolean
  }
  trend?: {
    label: string
    points: LogsSummaryMicroPoint[]
  }
  metrics?: LogsSummaryMetric[]
}

export type ModelShareRow = {
  model: string
  requests: number
  tokens: number
  cost: number
  color: string
}


export type TooltipPlacement = 'above' | 'below'

export type LogInfoTooltipTone = 'muted' | 'source-provider-api' | 'source-builtin' | 'source-none'

export type LogInfoTooltipVariant = 'model' | 'verify' | 'stream'

export type LogInfoTooltipRow = {
  key: string
  label: string
  value: string
  tone?: LogInfoTooltipTone
}

export type LogInfoTooltipDetail = {
  title: string
  variant: LogInfoTooltipVariant
  rows: LogInfoTooltipRow[]
}

export type CostTooltipPriceLine = {
  key: string
  label: string
  value: string
}

export type CostTooltipDetail = {
  pricingModel: string
  hasPricing: boolean
  priceLines: CostTooltipPriceLine[]
  formula: string
  note: string
  recordedCostHint: string
}

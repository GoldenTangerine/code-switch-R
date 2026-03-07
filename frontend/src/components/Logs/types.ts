import type { LogPlatform } from '../../services/logs'

export type LogDateFilterType = 'all' | 'today' | 'year' | 'month' | 'day' | 'range'

export type LogsFiltersState = {
  platform: LogPlatform | ''
  provider: string
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

export type LogsSummaryCard = {
  key: string
  label: string
  value: string
  hint: string
  subValue?: string
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

export type LogInfoTooltipVariant = 'model' | 'verify'

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

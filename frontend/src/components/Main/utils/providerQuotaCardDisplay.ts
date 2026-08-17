import type { ProviderQuotaDisplayItem, TranslateFn } from '../types'

const isFiniteNumber = (value: unknown): value is number => (
  typeof value === 'number' && Number.isFinite(value)
)

export const isProviderQuotaBalanceItem = (
  item: Pick<ProviderQuotaDisplayItem, 'valueMode' | 'nextReset' | 'queriedAt' | 'invalidMessage' | 'total' | 'used'>,
) => (
  !isProviderQuotaErrorItem(item)
  && item.valueMode === 'currency'
  && item.nextReset === null
  && isFiniteNumber(item.queriedAt)
)

export const isProviderQuotaErrorItem = (
  item: Pick<ProviderQuotaDisplayItem, 'invalidMessage' | 'total' | 'used' | 'nextReset' | 'queriedAt'>,
) => (
  `${item.invalidMessage ?? ''}`.trim().length > 0
  && (!isFiniteNumber(item.total) || item.total <= 0)
  && (!isFiniteNumber(item.used) || item.used <= 0)
  && item.nextReset === null
  && isFiniteNumber(item.queriedAt)
)

export const getProviderQuotaVisibleNote = (
  item: Pick<ProviderQuotaDisplayItem, 'refreshErrorMessage' | 'invalidMessage' | 'extra'>,
) => (
  [
    `${item.refreshErrorMessage ?? ''}`.trim(),
    `${item.invalidMessage ?? ''}`.trim(),
    `${item.extra ?? ''}`.trim(),
  ]
    .filter(Boolean)
    .join(' · ')
)

export const isProviderQuotaRefreshErrored = (
  item: Pick<ProviderQuotaDisplayItem, 'refreshErrorMessage'>,
) => (
  `${item.refreshErrorMessage ?? ''}`.trim().length > 0
)

export const getProviderQuotaRemainingValue = (
  item: Pick<ProviderQuotaDisplayItem, 'total' | 'used'>,
) => {
  const total = isFiniteNumber(item.total) ? item.total : 0
  const used = isFiniteNumber(item.used) ? item.used : 0
  return Math.max(total - used, 0)
}

export const getProviderQuotaBalanceTone = (
  item: Pick<ProviderQuotaDisplayItem, 'total' | 'used' | 'unlimited' | 'invalidMessage'>,
) => {
  if (`${item.invalidMessage ?? ''}`.trim()) {
    return 'invalid'
  }

  if (item.unlimited === true) {
    return 'healthy'
  }

  const remaining = getProviderQuotaRemainingValue(item)
  const total = isFiniteNumber(item.total) && item.total > 0
    ? item.total
    : remaining

  if (remaining <= 0) {
    return 'danger'
  }

  if (total > 0 && remaining < total * 0.1) {
    return 'warning'
  }

  return 'healthy'
}

export const formatProviderQuotaRelativeUpdatedAt = (
  queriedAt: number | undefined,
  now: number,
  t: TranslateFn,
) => {
  if (!isFiniteNumber(queriedAt)) {
    return t('components.main.providers.quotaNeverUpdated')
  }

  const safeNow = isFiniteNumber(now) ? now : Date.now()
  const diffSeconds = Math.max(Math.floor((safeNow - queriedAt) / 1000), 0)

  if (diffSeconds < 60) {
    return t('components.main.providers.quotaUpdatedJustNow')
  }

  if (diffSeconds < 3600) {
    return t('components.main.providers.quotaUpdatedMinutesAgo', {
      count: Math.floor(diffSeconds / 60),
    })
  }

  if (diffSeconds < 86_400) {
    return t('components.main.providers.quotaUpdatedHoursAgo', {
      count: Math.floor(diffSeconds / 3600),
    })
  }

  return t('components.main.providers.quotaUpdatedDaysAgo', {
    count: Math.floor(diffSeconds / 86_400),
  })
}

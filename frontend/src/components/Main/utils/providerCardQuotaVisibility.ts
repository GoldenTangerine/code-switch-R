import type { ProviderQuotaDisplayItem, ProviderStatDisplay } from '../types'

export type ProviderCardQuotaSectionMode = 'hidden' | 'inline-with-performance' | 'standalone'

export function resolveProviderCardQuotaSectionMode(
  stats: Pick<ProviderStatDisplay, 'state'>,
  quotaDisplay: ProviderQuotaDisplayItem[],
): ProviderCardQuotaSectionMode {
  if (quotaDisplay.length === 0) {
    return 'hidden'
  }

  if (stats.state === 'ready') {
    return 'inline-with-performance'
  }

  return 'standalone'
}

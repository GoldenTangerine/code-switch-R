import type { ProviderTab } from '../types'

export function shouldShowProviderProxyToggle(tab: ProviderTab): boolean {
  return tab !== 'opencode'
}

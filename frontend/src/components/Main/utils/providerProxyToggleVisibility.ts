import type { ProviderTab } from '../types'

export function shouldShowProviderProxyToggle(tab: ProviderTab): boolean {
  // claude-desktop 的代理模式由供应商表单内的 claudeDesktopMode 控制，不展示顶部开关
  // openclaw / hermes / pi 为 additive 共存模式，供应商直接写入原生配置，无本地代理概念
  return tab !== 'opencode' && tab !== 'claude-desktop' && tab !== 'openclaw' && tab !== 'hermes' && tab !== 'pi'
}

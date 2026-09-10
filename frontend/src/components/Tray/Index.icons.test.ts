/**
 * @name: 托盘平台图标测试
 * @Descripttion: 验证首次打开托盘时平台图标独立于供应商图标完成加载和渲染。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-09-10 11:02:15
 * @LastEditTime: 2026-09-10 11:02:15
 * @FilePath: frontend/src/components/Tray/Index.icons.test.ts
 */

// @vitest-environment happy-dom

import { createApp, nextTick } from 'vue'
import { createI18n } from 'vue-i18n'
import { beforeEach, expect, it, vi } from 'vitest'
import { getProviderDisplayIconSvg, preloadProviderDisplayIcons } from '../../utils/providerIconAssets'
import lobeIconMap from '../../icons/lobeIconMap'
import type { SharedTraySnapshot } from './traySharedSnapshot'
import Tray from './Index.vue'

vi.mock('@wailsio/runtime', () => ({ Call: { ByName: vi.fn() } }))
vi.mock('../../services/appSettings', () => ({ fetchAppSettings: async () => ({}) }))
vi.mock('../../services/logs', () => ({
  fetchCostSince: async () => 0,
  fetchFiveHourQuotaStatus: async () => ({}),
  fetchLogStats: async () => ({ cost_total: 0 }),
}))
vi.mock('../../services/claudeSettings', () => ({ fetchProxyStatus: async () => ({}) }))
vi.mock('../../services/customCliService', () => ({ getCustomCliProxyStatus: async () => ({}) }))
vi.mock('../../utils/providerIconAssets', async () => {
  const actual = await vi.importActual<typeof import('../../utils/providerIconAssets')>('../../utils/providerIconAssets')
  return { ...actual, preloadProviderDisplayIcons: vi.fn(actual.preloadProviderDisplayIcons) }
})

const snapshot: SharedTraySnapshot = {
  version: 1,
  session: 'test',
  sequence: 1,
  heartbeatAt: 0,
  platforms: [
    {
      platform: 'claude', name: 'Claude Code', icon: 'claude', error: false,
      providers: [{
        providerId: 'test-provider', providerName: 'GLM', icon: 'zhipu',
        activeRequests: 0, status: 'default', loading: false, updatedAt: 0,
        quotas: [], stats: null,
      }],
    },
    { platform: 'gemini', name: 'Gemini', icon: 'gemini', error: false, providers: [] },
  ],
}

beforeEach(async () => {
  const actual = await vi.importActual<typeof import('../../utils/providerIconAssets')>('../../utils/providerIconAssets')
  vi.mocked(preloadProviderDisplayIcons).mockReset().mockImplementation(actual.preloadProviderDisplayIcons)
  for (const key of ['claude-color', 'gemini-color', 'zhipu-color']) lobeIconMap[key] = ''
})

async function mountTray() {
  const { Call } = await import('@wailsio/runtime')
  vi.mocked(Call.ByName).mockReset()
  vi.mocked(Call.ByName).mockResolvedValue(snapshot)
  vi.spyOn(document, 'hasFocus').mockReturnValue(true)
  vi.spyOn(document, 'hidden', 'get').mockReturnValue(false)
  const root = document.createElement('div')
  document.body.append(root)
  const app = createApp(Tray)
  app.use(createI18n({ legacy: false, locale: 'en', missingWarn: false, fallbackWarn: false }))
  app.mount(root)
  return { app, root }
}

async function flushUpdates() {
  for (let index = 0; index < 20; index += 1) await Promise.resolve()
  await nextTick()
}

function populateTestIcons() {
  for (const key of ['claude-color', 'gemini-color', 'zhipu-color']) {
    lobeIconMap[key] = `<svg viewBox="0 0 24 24" data-test-icon="${key}"></svg>`
  }
}

it('loads platform and provider SVGs on first open with unrelated or absent provider icons', async () => {
  expect(getProviderDisplayIconSvg('claude')).toBe('')
  expect(getProviderDisplayIconSvg('gemini')).toBe('')
  const { app, root } = await mountTray()

  try {
    const deadline = Date.now() + 2000
    while ((root.querySelectorAll('.tray-brand__icon-svg svg').length < 2
      || !root.querySelector('.tray-provider-source__icon-svg svg')) && Date.now() < deadline) {
      await new Promise((resolve) => setTimeout(resolve, 10))
      await nextTick()
    }

    const brandIcons = root.querySelectorAll('.tray-brand__icon-svg svg')
    expect(brandIcons).toHaveLength(2)
    expect(root.querySelector('.tray-brand__icon-fallback')).toBeNull()
    expect(brandIcons[0]?.outerHTML).toBe(getProviderDisplayIconSvg('claude'))
    expect(brandIcons[1]?.outerHTML).toBe(getProviderDisplayIconSvg('gemini'))
    expect(root.querySelector('.tray-provider-source__icon-svg svg')?.outerHTML).toBe(getProviderDisplayIconSvg('zhipu'))
    expect(root.querySelector('.tray-provider-source__icon-fallback')).toBeNull()
  } finally {
    app.unmount()
    root.remove()
    vi.restoreAllMocks()
  }
})

it('retries failed icons after a cooldown even when the snapshot stays unchanged', async () => {
  vi.useFakeTimers()
  const warning = vi.spyOn(console, 'warn').mockImplementation(() => {})
  vi.mocked(preloadProviderDisplayIcons)
    .mockRejectedValueOnce(new Error('asset unavailable'))
    .mockImplementationOnce(async () => populateTestIcons())
  const { app, root } = await mountTray()

  try {
    await flushUpdates()
    expect(warning).toHaveBeenCalledTimes(1)
    expect(preloadProviderDisplayIcons).toHaveBeenCalledTimes(1)
    expect(Array.from(root.querySelectorAll('.tray-brand__icon-fallback'), (node) => node.textContent)).toEqual(['C', 'G'])
    await vi.advanceTimersByTimeAsync(4500)
    expect(preloadProviderDisplayIcons).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(500)
    expect(preloadProviderDisplayIcons).toHaveBeenCalledTimes(2)
    expect(root.querySelectorAll('.tray-brand__icon-svg svg')).toHaveLength(2)
    expect(root.querySelectorAll('.tray-provider-source__icon-svg svg')).toHaveLength(1)
    await vi.advanceTimersByTimeAsync(10_000)
    expect(preloadProviderDisplayIcons).toHaveBeenCalledTimes(2)
  } finally {
    app.unmount()
    root.remove()
    vi.useRealTimers()
    vi.restoreAllMocks()
  }
})

it('deduplicates pending loads and resumes failed loads only while the tray is active', async () => {
  vi.useFakeTimers()
  vi.spyOn(console, 'warn').mockImplementation(() => {})
  let rejectLoad!: (error: Error) => void
  vi.mocked(preloadProviderDisplayIcons)
    .mockImplementationOnce(() => new Promise<void>((_, reject) => { rejectLoad = reject }))
    .mockRejectedValue(new Error('asset unavailable'))
  const { app, root } = await mountTray()

  try {
    await flushUpdates()
    await vi.advanceTimersByTimeAsync(10_000)
    expect(preloadProviderDisplayIcons).toHaveBeenCalledTimes(1)
    window.dispatchEvent(new Event('blur'))
    rejectLoad(new Error('asset unavailable'))
    await flushUpdates()
    await vi.advanceTimersByTimeAsync(10_000)
    expect(preloadProviderDisplayIcons).toHaveBeenCalledTimes(1)
    window.dispatchEvent(new Event('focus'))
    await flushUpdates()
    expect(preloadProviderDisplayIcons).toHaveBeenCalledTimes(2)
  } finally {
    app.unmount()
    root.remove()
    await vi.advanceTimersByTimeAsync(10_000)
    const callsAfterUnmount = vi.mocked(preloadProviderDisplayIcons).mock.calls.length
    vi.useRealTimers()
    vi.restoreAllMocks()
    expect(callsAfterUnmount).toBe(2)
  }
})

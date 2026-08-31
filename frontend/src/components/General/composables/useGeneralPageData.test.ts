/**
 * @name: General 页面数据编排测试
 * @Descripttion: 验证设置页初始化依赖、并行加载、错误隔离和卸载清理
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-31 20:17:57
 * @LastEditTime: 2026-08-31 20:17:57
 * @FilePath: frontend/src/components/General/composables/useGeneralPageData.test.ts
 */

import { afterEach, describe, expect, it, vi } from 'vitest'
import {
  createGeneralPageDataLifecycle,
  type GeneralPageDataLifecycleOptions,
} from './useGeneralPageData'

function createDeferred() {
  let resolve!: () => void
  let reject!: (error: unknown) => void
  const promise = new Promise<void>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise
    reject = rejectPromise
  })
  return { promise, resolve, reject }
}

function createOptions(overrides: Partial<GeneralPageDataLifecycleOptions> = {}) {
  const unsubscribeWebDAVSync = vi.fn()
  const options: GeneralPageDataLifecycleOptions = {
    preloadProviderIcons: vi.fn(),
    loadAppSettings: vi.fn(async () => {}),
    loadClaudeModelRoutingStatus: vi.fn(async () => {}),
    loadAppVersion: vi.fn(async () => {}),
    loadUpdateState: vi.fn(async () => {}),
    loadBlacklistSettings: vi.fn(async () => {}),
    loadImportStatus: vi.fn(async () => {}),
    loadWebDAV: vi.fn(async () => {}),
    subscribeWebDAVSync: vi.fn(() => unsubscribeWebDAVSync),
    flushPendingPersist: vi.fn(async () => {}),
    ...overrides,
  }
  return { options, unsubscribeWebDAVSync }
}

async function flushPromises() {
  await Promise.resolve()
  await Promise.resolve()
}

afterEach(() => {
  vi.useRealTimers()
})

describe('createGeneralPageDataLifecycle', () => {
  it('loads app settings first and starts all independent loaders together', async () => {
    const appSettings = createDeferred()
    const independentLoads = Array.from({ length: 6 }, createDeferred)
    const { options } = createOptions({
      loadAppSettings: vi.fn(() => appSettings.promise),
      loadClaudeModelRoutingStatus: vi.fn(() => independentLoads[0].promise),
      loadAppVersion: vi.fn(() => independentLoads[1].promise),
      loadUpdateState: vi.fn(() => independentLoads[2].promise),
      loadBlacklistSettings: vi.fn(() => independentLoads[3].promise),
      loadImportStatus: vi.fn(() => independentLoads[4].promise),
      loadWebDAV: vi.fn(() => independentLoads[5].promise),
    })
    const lifecycle = createGeneralPageDataLifecycle(options)

    const mountPromise = lifecycle.mount()
    expect(options.preloadProviderIcons).toHaveBeenCalledTimes(1)
    expect(options.loadAppSettings).toHaveBeenCalledTimes(1)
    expect(options.loadClaudeModelRoutingStatus).not.toHaveBeenCalled()

    appSettings.resolve()
    await flushPromises()
    expect(options.loadClaudeModelRoutingStatus).toHaveBeenCalledTimes(1)
    expect(options.loadAppVersion).toHaveBeenCalledTimes(1)
    expect(options.loadUpdateState).toHaveBeenCalledTimes(1)
    expect(options.loadBlacklistSettings).toHaveBeenCalledTimes(1)
    expect(options.loadImportStatus).toHaveBeenCalledTimes(1)
    expect(options.loadWebDAV).toHaveBeenCalledTimes(1)
    expect(options.subscribeWebDAVSync).not.toHaveBeenCalled()

    independentLoads.forEach((load) => load.resolve())
    await mountPromise
    expect(options.subscribeWebDAVSync).toHaveBeenCalledTimes(1)
  })

  it('isolates an independent loader failure and still completes initialization', async () => {
    const { options } = createOptions({
      loadAppVersion: vi.fn(async () => {
        throw new Error('version unavailable')
      }),
    })
    const lifecycle = createGeneralPageDataLifecycle(options)

    await expect(lifecycle.mount()).resolves.toBeUndefined()
    expect(options.loadClaudeModelRoutingStatus).toHaveBeenCalledTimes(1)
    expect(options.loadUpdateState).toHaveBeenCalledTimes(1)
    expect(options.loadBlacklistSettings).toHaveBeenCalledTimes(1)
    expect(options.loadImportStatus).toHaveBeenCalledTimes(1)
    expect(options.loadWebDAV).toHaveBeenCalledTimes(1)
    expect(options.subscribeWebDAVSync).toHaveBeenCalledTimes(1)
  })

  it('deduplicates repeated mounts', async () => {
    const appSettings = createDeferred()
    const { options } = createOptions({
      loadAppSettings: vi.fn(() => appSettings.promise),
    })
    const lifecycle = createGeneralPageDataLifecycle(options)

    const firstMount = lifecycle.mount()
    const secondMount = lifecycle.mount()
    expect(secondMount).toBe(firstMount)

    appSettings.resolve()
    await firstMount
    expect(options.preloadProviderIcons).toHaveBeenCalledTimes(1)
    expect(options.loadAppSettings).toHaveBeenCalledTimes(1)
    expect(options.subscribeWebDAVSync).toHaveBeenCalledTimes(1)
  })

  it('flushes pending persistence and unsubscribes once on unmount', async () => {
    const { options, unsubscribeWebDAVSync } = createOptions()
    const lifecycle = createGeneralPageDataLifecycle(options)
    await lifecycle.mount()

    lifecycle.unmount()
    lifecycle.unmount()

    expect(options.flushPendingPersist).toHaveBeenCalledTimes(1)
    expect(unsubscribeWebDAVSync).toHaveBeenCalledTimes(1)
  })

  it('does not subscribe after unmounting during initialization', async () => {
    const webDAV = createDeferred()
    const { options } = createOptions({
      loadWebDAV: vi.fn(() => webDAV.promise),
    })
    const lifecycle = createGeneralPageDataLifecycle(options)
    const mountPromise = lifecycle.mount()
    await flushPromises()

    lifecycle.unmount()
    webDAV.resolve()
    await mountPromise

    expect(options.flushPendingPersist).toHaveBeenCalledTimes(1)
    expect(options.subscribeWebDAVSync).not.toHaveBeenCalled()
  })

  it('reduces equal-delay initialization from seven serial waits to two waits', async () => {
    vi.useFakeTimers()
    const delay = () => new Promise<void>((resolve) => globalThis.setTimeout(resolve, 100))
    const serialLoaders = Array.from({ length: 7 }, () => delay)
    const serialStartedAt = Date.now()
    const serialLoad = (async () => {
      for (const load of serialLoaders) await load()
    })()
    await vi.runAllTimersAsync()
    await serialLoad
    const serialElapsed = Date.now() - serialStartedAt

    const { options } = createOptions({
      loadAppSettings: vi.fn(delay),
      loadClaudeModelRoutingStatus: vi.fn(delay),
      loadAppVersion: vi.fn(delay),
      loadUpdateState: vi.fn(delay),
      loadBlacklistSettings: vi.fn(delay),
      loadImportStatus: vi.fn(delay),
      loadWebDAV: vi.fn(delay),
    })
    const lifecycle = createGeneralPageDataLifecycle(options)
    const parallelStartedAt = Date.now()
    const parallelLoad = lifecycle.mount()
    await vi.runAllTimersAsync()
    await parallelLoad
    const parallelElapsed = Date.now() - parallelStartedAt

    expect(serialElapsed).toBe(700)
    expect(parallelElapsed).toBe(200)
  })
})

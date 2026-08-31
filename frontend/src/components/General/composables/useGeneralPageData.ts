/**
 * @name: General 页面数据编排
 * @Descripttion: 统一设置页初始化加载、事件订阅和卸载清理
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-31 20:20:22
 * @LastEditTime: 2026-08-31 20:20:22
 * @FilePath: frontend/src/components/General/composables/useGeneralPageData.ts
 */

import { onBeforeUnmount, onMounted } from 'vue'

export interface GeneralPageDataLifecycleOptions {
  preloadProviderIcons: () => void
  loadAppSettings: () => Promise<void>
  loadClaudeModelRoutingStatus: () => Promise<void>
  loadAppVersion: () => Promise<void>
  loadUpdateState: () => Promise<void>
  loadBlacklistSettings: () => Promise<void>
  loadImportStatus: () => Promise<void>
  loadWebDAV: () => Promise<void>
  subscribeWebDAVSync: () => () => void
  flushPendingPersist: () => Promise<void>
}

export interface GeneralPageDataLifecycle {
  mount: () => Promise<void>
  unmount: () => void
}

export function createGeneralPageDataLifecycle(
  options: GeneralPageDataLifecycleOptions,
): GeneralPageDataLifecycle {
  let mountPromise: Promise<void> | undefined
  let unsubscribeWebDAVSync: (() => void) | undefined
  let disposed = false

  async function initialize() {
    options.preloadProviderIcons()
    await options.loadAppSettings()
    await Promise.allSettled([
      options.loadClaudeModelRoutingStatus(),
      options.loadAppVersion(),
      options.loadUpdateState(),
      options.loadBlacklistSettings(),
      options.loadImportStatus(),
      options.loadWebDAV(),
    ])
    if (disposed) return
    unsubscribeWebDAVSync = options.subscribeWebDAVSync()
  }

  function mount() {
    if (disposed) return Promise.resolve()
    mountPromise ??= initialize()
    return mountPromise
  }

  function unmount() {
    if (disposed) return
    disposed = true
    void options.flushPendingPersist()
    unsubscribeWebDAVSync?.()
    unsubscribeWebDAVSync = undefined
  }

  return { mount, unmount }
}

export function useGeneralPageData(options: GeneralPageDataLifecycleOptions) {
  const lifecycle = createGeneralPageDataLifecycle(options)
  onMounted(lifecycle.mount)
  onBeforeUnmount(lifecycle.unmount)
  return lifecycle
}

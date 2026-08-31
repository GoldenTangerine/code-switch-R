/**
 * @name: 主页面轮询生命周期
 * @Descripttion: 根据主窗口可见状态统一暂停、恢复和清理主页面周期轮询
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-31 19:02:42
 * @LastEditTime: 2026-08-31 19:02:42
 * @FilePath: frontend/src/components/Main/composables/mainPollingLifecycle.ts
 */

export interface MainPollingLifecycleOptions {
  initialVisible: boolean
  startPolling: () => void
  stopPolling: () => void
  refresh: () => Promise<void>
}

export interface MainPollingLifecycle {
  start: () => void
  setVisible: (visible: boolean) => void
  isActive: () => boolean
  dispose: () => void
}

export interface MainPollingVisibilitySource {
  onWindowHide: (handler: () => void) => () => void
  onWindowShow: (handler: () => void) => () => void
  addVisibilityListener: (handler: () => void) => void
  removeVisibilityListener: (handler: () => void) => void
  isDocumentHidden: () => boolean
}

export function bindMainPollingVisibility(
  lifecycle: MainPollingLifecycle,
  source: MainPollingVisibilitySource,
): () => void {
  const handleVisibilityChange = () => {
    lifecycle.setVisible(!source.isDocumentHidden())
  }
  const unsubscribeHide = source.onWindowHide(() => lifecycle.setVisible(false))
  const unsubscribeShow = source.onWindowShow(() => lifecycle.setVisible(true))
  source.addVisibilityListener(handleVisibilityChange)
  let unbound = false

  return () => {
    if (unbound) return
    unbound = true
    unsubscribeHide()
    unsubscribeShow()
    source.removeVisibilityListener(handleVisibilityChange)
  }
}

export function createMainPollingLifecycle(options: MainPollingLifecycleOptions): MainPollingLifecycle {
  let ready = false
  let visible = options.initialVisible
  let active = false
  let pollingStarted = false
  let disposed = false
  let generation = 0

  function startPollingForGeneration(expectedGeneration: number) {
    if (
      disposed
      || !ready
      || !visible
      || !active
      || pollingStarted
      || generation !== expectedGeneration
    ) return

    options.startPolling()
    pollingStarted = true
  }

  function activate(refresh: boolean) {
    if (disposed || !ready || !visible || active) return

    active = true
    const expectedGeneration = ++generation
    if (!refresh) {
      startPollingForGeneration(expectedGeneration)
      return
    }

    void options.refresh().then(
      () => startPollingForGeneration(expectedGeneration),
      () => startPollingForGeneration(expectedGeneration),
    )
  }

  function deactivate() {
    if (!active && !pollingStarted) return

    active = false
    generation++
    if (pollingStarted) {
      options.stopPolling()
      pollingStarted = false
    }
  }

  function start() {
    if (disposed || ready) return

    ready = true
    activate(false)
  }

  function setVisible(nextVisible: boolean) {
    if (disposed || visible === nextVisible) return

    visible = nextVisible
    if (!ready) return
    if (visible) {
      activate(true)
      return
    }
    deactivate()
  }

  function dispose() {
    if (disposed) return

    disposed = true
    ready = false
    active = false
    generation++
    if (pollingStarted) {
      options.stopPolling()
      pollingStarted = false
    }
  }

  return {
    start,
    setVisible,
    isActive: () => active && pollingStarted,
    dispose,
  }
}

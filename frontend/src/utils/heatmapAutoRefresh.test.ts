import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createHeatmapAutoRefreshController } from './heatmapAutoRefresh'

const flushPromises = async () => {
  await Promise.resolve()
  await Promise.resolve()
}

describe('heatmapAutoRefresh', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('reloads repeatedly while the controller is active', async () => {
    const reload = vi.fn().mockResolvedValue(undefined)
    const controller = createHeatmapAutoRefreshController({
      intervalMs: 60_000,
      reload,
    })

    controller.start()
    await vi.advanceTimersByTimeAsync(60_000)
    await flushPromises()
    expect(reload).toHaveBeenCalledTimes(1)

    await vi.advanceTimersByTimeAsync(60_000)
    await flushPromises()
    expect(reload).toHaveBeenCalledTimes(2)
  })

  it('does not reschedule after stop is called during an in-flight reload', async () => {
    let resolveReload!: () => void
    const reload = vi.fn().mockImplementation(
      () =>
        new Promise<void>((resolve) => {
          resolveReload = resolve
        }),
    )
    const controller = createHeatmapAutoRefreshController({
      intervalMs: 60_000,
      reload,
    })

    controller.start()
    await vi.advanceTimersByTimeAsync(60_000)
    await flushPromises()
    expect(reload).toHaveBeenCalledTimes(1)

    controller.stop()
    resolveReload()
    await flushPromises()
    await vi.advanceTimersByTimeAsync(120_000)
    await flushPromises()

    expect(reload).toHaveBeenCalledTimes(1)
    expect(controller.isActive()).toBe(false)
  })
})

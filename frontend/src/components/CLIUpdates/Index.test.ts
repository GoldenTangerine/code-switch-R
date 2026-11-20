/**
 * @name: CLI 更新页面测试
 * @Descripttion: 验证页面每次进入时检查更新并显示最新结果。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-11-20 14:40:19
 * @LastEditTime: 2026-11-20 14:40:19
 * @FilePath: frontend/src/components/CLIUpdates/Index.test.ts
 */
// @vitest-environment happy-dom

import { createApp, defineComponent, h, KeepAlive, nextTick, ref } from 'vue'
import { createI18n } from 'vue-i18n'
import { describe, expect, it, vi } from 'vitest'
import zh from '../../locales/zh.json'
import { checkCLIVersions, type CLIVersionInfo } from '../../services/cliUpdates'
import CLIUpdatesPage from './Index.vue'

vi.mock('../../services/cliUpdates', () => ({ checkCLIVersions: vi.fn() }))

const flushUpdates = () => new Promise(resolve => setTimeout(resolve, 0))

describe('CLI update checks', () => {
  it('checks on initial entry and on return, and ignores a result from an earlier visit', async () => {
    const visible = ref(true)
    const root = document.createElement('div')
    const OtherPage = defineComponent({ render: () => h('div') })
    const app = createApp({
      render: () => h(KeepAlive, null, () => h(visible.value ? CLIUpdatesPage : OtherPage)),
    })
    app.use(createI18n({ legacy: false, locale: 'zh', messages: { zh } }))

    const cancel = vi.fn()
    let resolveFirstCheck!: (items: CLIVersionInfo[]) => void
    const firstCheck = new Promise<CLIVersionInfo[]>(resolve => { resolveFirstCheck = resolve })
    let resolveRefresh!: (items: CLIVersionInfo[]) => void
    const refresh = new Promise<CLIVersionInfo[]>(resolve => { resolveRefresh = resolve })
    const codex: CLIVersionInfo = {
      command: 'codex', name: 'Codex', installedVersion: '1.0.0', latestVersion: '1.0.1', status: 'updateAvailable',
    }
    const cancellable = (promise: Promise<CLIVersionInfo[]>) =>
      Object.assign(promise, { cancel }) as unknown as ReturnType<typeof checkCLIVersions>
    vi.mocked(checkCLIVersions)
      .mockReturnValueOnce(cancellable(firstCheck))
      .mockReturnValueOnce(cancellable(Promise.resolve([codex])))
      .mockReturnValueOnce(cancellable(refresh))
      .mockImplementationOnce(() => cancellable(Promise.reject(new Error('offline'))))

    app.mount(root)
    try {
      expect(checkCLIVersions).toHaveBeenCalledTimes(1)
      expect(root.textContent).toContain('正在检查')

      visible.value = false
      await nextTick()
      expect(cancel).toHaveBeenCalledTimes(1)
      visible.value = true
      await flushUpdates()
      expect(checkCLIVersions).toHaveBeenCalledTimes(2)
      expect(root.textContent).toContain('有更新')

      resolveFirstCheck([])
      await flushUpdates()
      expect(root.textContent).toContain('有更新')

      root.querySelector('button')?.click()
      await flushUpdates()
      expect(checkCLIVersions).toHaveBeenCalledTimes(3)
      expect(root.textContent).toContain('正在重新检查，当前显示上次结果')
      expect(root.querySelector('tbody')?.textContent).toContain('有更新')

      resolveRefresh([{ ...codex, installedVersion: '1.0.1', status: 'upToDate' }])
      await flushUpdates()
      expect(root.textContent).toContain('无需更新')

      visible.value = false
      await nextTick()
      visible.value = true
      await flushUpdates()
      expect(checkCLIVersions).toHaveBeenCalledTimes(4)
      expect(root.querySelector('[role="alert"]')?.textContent).toContain('检查失败，当前显示上次结果')
      expect(root.querySelector('tbody')?.textContent).toContain('无需更新')
    } finally {
      app.unmount()
    }
  })
})

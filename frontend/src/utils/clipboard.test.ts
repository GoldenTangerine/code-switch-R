/**
 * @name: 剪贴板工具测试
 * @Descripttion: 验证原生剪贴板与浏览器降级复制逻辑
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-07-13 15:16:12
 * @LastEditTime: 2026-07-13 15:16:12
 * @FilePath: frontend/src/utils/clipboard.test.ts
 */
import { afterEach, describe, expect, it, vi } from 'vitest'

const { setTextMock } = vi.hoisted(() => ({
  setTextMock: vi.fn(),
}))

vi.mock('@wailsio/runtime', () => ({
  Clipboard: {
    SetText: setTextMock,
  },
}))

import { writeTextToClipboard } from './clipboard'

describe('clipboard', () => {
  afterEach(() => {
    vi.clearAllMocks()
    vi.unstubAllGlobals()
  })

  it('优先使用 Wails 原生剪贴板', async () => {
    const webWriteText = vi.fn()
    setTextMock.mockResolvedValue(undefined)
    vi.stubGlobal('window', { _wails: { environment: { OS: 'darwin' } } })
    vi.stubGlobal('navigator', { clipboard: { writeText: webWriteText } })

    await writeTextToClipboard('provider error')

    expect(setTextMock).toHaveBeenCalledWith('provider error')
    expect(webWriteText).not.toHaveBeenCalled()
  })

  it('原生剪贴板失败后降级到浏览器 Clipboard API', async () => {
    const webWriteText = vi.fn().mockResolvedValue(undefined)
    setTextMock.mockRejectedValue(new Error('runtime unavailable'))
    vi.stubGlobal('window', { _wails: { environment: { OS: 'darwin' } } })
    vi.stubGlobal('navigator', { clipboard: { writeText: webWriteText } })

    await writeTextToClipboard('provider error')

    expect(webWriteText).toHaveBeenCalledWith('provider error')
  })

  it('浏览器预览环境不调用可能假成功的 Wails Runtime', async () => {
    const webWriteText = vi.fn().mockResolvedValue(undefined)
    setTextMock.mockResolvedValue(undefined)
    vi.stubGlobal('window', { _wails: { environment: { OS: 'darwin', Arch: 'browser' } } })
    vi.stubGlobal('navigator', { clipboard: { writeText: webWriteText } })

    await writeTextToClipboard('provider error')

    expect(setTextMock).not.toHaveBeenCalled()
    expect(webWriteText).toHaveBeenCalledWith('provider error')
  })

  it('浏览器 Clipboard API 被拒绝后降级到 textarea 复制', async () => {
    const textarea = {
      value: '',
      style: {},
      setAttribute: vi.fn(),
      focus: vi.fn(),
      select: vi.fn(),
    }
    const appendChild = vi.fn()
    const removeChild = vi.fn()
    const execCommand = vi.fn().mockReturnValue(true)
    setTextMock.mockRejectedValue(new Error('runtime unavailable'))
    vi.stubGlobal('navigator', {
      clipboard: {
        writeText: vi.fn().mockRejectedValue(new Error('not allowed')),
      },
    })
    vi.stubGlobal('document', {
      createElement: vi.fn().mockReturnValue(textarea),
      body: { appendChild, removeChild },
      execCommand,
    })

    await writeTextToClipboard('provider error')

    expect(textarea.value).toBe('provider error')
    expect(execCommand).toHaveBeenCalledWith('copy')
    expect(removeChild).toHaveBeenCalledWith(textarea)
  })

  it('所有剪贴板方式失败时保留底层错误与降级错误', async () => {
    const execCommand = vi.fn().mockReturnValue(false)
    const textarea = {
      value: '',
      style: {},
      setAttribute: vi.fn(),
      focus: vi.fn(),
      select: vi.fn(),
    }
    setTextMock.mockRejectedValue(new Error('runtime unavailable'))
    vi.stubGlobal('window', { _wails: { environment: { OS: 'darwin', Arch: 'arm64' } } })
    vi.stubGlobal('navigator', {
      clipboard: {
        writeText: vi.fn().mockRejectedValue(new Error('not allowed')),
      },
    })
    vi.stubGlobal('document', {
      createElement: vi.fn().mockReturnValue(textarea),
      body: { appendChild: vi.fn(), removeChild: vi.fn() },
      execCommand,
    })

    await expect(writeTextToClipboard('provider error')).rejects.toThrow(
      'not allowed; fallback: execCommand copy failed',
    )
  })
})

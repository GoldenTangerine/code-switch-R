import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { AutomationCard } from '../../../../data/cards'

vi.mock('../../../../../bindings/codeswitch/services/providerservice', () => ({
  DuplicateProvider: vi.fn(),
  LoadProviders: vi.fn(),
  SaveProviders: vi.fn(),
}))

vi.mock('../../../../../bindings/codeswitch/services/geminiservice', () => ({
  AddProvider: vi.fn(),
  DeleteProvider: vi.fn(),
  GetProviders: vi.fn(),
  ReorderProviders: vi.fn(),
  UpdateProvider: vi.fn(),
}))

vi.mock('../../../../../bindings/codeswitch/services/opencodeservice', () => ({
  AddProvider: vi.fn(),
  DeleteProvider: vi.fn(),
  DuplicateProvider: vi.fn(),
  GetPresets: vi.fn(),
  GetProviders: vi.fn(),
  ImportFromLive: vi.fn(),
  ReorderProviders: vi.fn(),
  SaveProviders: vi.fn(),
  UpdateProvider: vi.fn(),
}))

vi.mock('@wailsio/runtime', () => ({
  Call: {
    ByName: vi.fn(),
  },
}))

vi.mock('../../../../utils/toast', () => ({
  showToast: vi.fn(),
}))

// 服务层 mock：展开真实导出（保留纯 mapper 供映射断言），仅覆盖会触达后端的函数
vi.mock('../../../../services/openClaw', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../../../services/openClaw')>()
  return {
    ...actual,
    getOpenClawProviders: vi.fn(),
    addOpenClawProvider: vi.fn(),
    updateOpenClawProvider: vi.fn(),
    deleteOpenClawProvider: vi.fn(),
    duplicateOpenClawProvider: vi.fn(),
    getOpenClawStatus: vi.fn(),
    setCurrentOpenClawProvider: vi.fn(),
  }
})

vi.mock('../../../../services/hermes', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../../../services/hermes')>()
  return {
    ...actual,
    getHermesProviders: vi.fn(),
    addHermesProvider: vi.fn(),
    updateHermesProvider: vi.fn(),
    deleteHermesProvider: vi.fn(),
    duplicateHermesProvider: vi.fn(),
    getHermesStatus: vi.fn(),
    setCurrentHermesProvider: vi.fn(),
  }
})

vi.mock('../../../../services/pi', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../../../services/pi')>()
  return {
    ...actual,
    getPiProviders: vi.fn(),
    addPiProvider: vi.fn(),
    updatePiProvider: vi.fn(),
    deletePiProvider: vi.fn(),
    duplicatePiProvider: vi.fn(),
    getPiStatus: vi.fn(),
    setCurrentPiProvider: vi.fn(),
  }
})

vi.mock('../../../../services/opencode', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../../../services/opencode')>()
  return {
    ...actual,
    getOpenCodeProviders: vi.fn(),
    saveOpenCodeProviders: vi.fn(),
    duplicateOpenCodeProvider: vi.fn(),
    importOpenCodeProvidersFromLive: vi.fn(),
  }
})

vi.mock('../../../../services/grokSettings', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../../../services/grokSettings')>()
  return {
    ...actual,
    applyGrokSingleProvider: vi.fn(),
    getGrokStatus: vi.fn(),
  }
})

vi.mock('../../../../services/claudeDesktopSettings', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../../../services/claudeDesktopSettings')>()
  return {
    ...actual,
    applyClaudeDesktopSingleProvider: vi.fn(),
    getClaudeDesktopStatus: vi.fn(),
  }
})

import { Call } from '@wailsio/runtime'
import { LoadProviders } from '../../../../../bindings/codeswitch/services/providerservice'
import { showToast } from '../../../../utils/toast'
import {
  cardToOpenClawProvider,
  openClawToCard,
  addOpenClawProvider,
  deleteOpenClawProvider,
  getOpenClawProviders,
  updateOpenClawProvider,
  type OpenClawProvider,
} from '../../../../services/openClaw'
import {
  cardToHermesProvider,
  hermesToCard,
  type HermesProvider,
} from '../../../../services/hermes'
import {
  cardToPiProvider,
  piToCard,
  type PiProvider,
} from '../../../../services/pi'
import { useProviderCards } from '../useProviderCards'

const createCard = (
  id: number,
  overrides: Partial<AutomationCard> = {},
): AutomationCard => ({
  id,
  name: `Provider ${id}`,
  apiUrl: `https://example-${id}.com`,
  apiKey: '',
  officialSite: '',
  icon: '',
  tint: '',
  accent: '',
  enabled: true,
  level: 1,
  sortOrder: 1,
  ...overrides,
})

const createUseProviderCards = () => useProviderCards({
  t: (key: string) => key,
  getActiveTab: () => 'openclaw',
  isActiveProxyEnabled: () => false,
  getSelectedToolId: () => null,
})

describe('additive provider ⇄ card 映射（openclaw/hermes/pi）', () => {
  it('openClawToCard：baseUrl/model/apiKey 透传，字符串 ID 保真，数字 ID 直接复用', () => {
    const provider: OpenClawProvider = {
      id: 'openclaw-1690000000',
      name: 'OpenClaw Live',
      baseUrl: 'https://api.openclaw.example.com/v1',
      apiKey: 'sk-openclaw',
      model: 'claude-sonnet-4-5',
      enabled: true,
      level: 3,
      category: 'custom',
      cliConfig: { native: 'fragment' },
    }

    const card = openClawToCard(provider, 0)

    expect(card.providerRef).toBe('openclaw-1690000000')
    expect(card.apiUrl).toBe('https://api.openclaw.example.com/v1')
    expect(card.apiKey).toBe('sk-openclaw')
    expect(card.cliConfig?.model).toBe('claude-sonnet-4-5')
    expect(card.cliConfig?.native).toBe('fragment')
    expect(card.enabled).toBe(true)
    expect(card.level).toBe(3)
    expect(card.category).toBe('custom')

    const numericCard = openClawToCard({ ...provider, id: '42' }, 1)
    expect(numericCard.id).toBe(42)
    expect(numericCard.providerRef).toBe('42')
  })

  it('数字回退 ID 使用互不碰撞的负数区间', () => {
    const fallback = (id: string, index: number, mapper: (provider: any, index: number) => AutomationCard) => (
      mapper({ id, name: 'x', enabled: true }, index).id
    )

    expect(fallback('openclaw-x', 0, openClawToCard)).toBe(-401)
    expect(fallback('openclaw-x', 2, openClawToCard)).toBe(-403)
    expect(fallback('hermes-x', 0, hermesToCard)).toBe(-501)
    expect(fallback('hermes-x', 2, hermesToCard)).toBe(-503)
    expect(fallback('pi-x', 0, piToCard)).toBe(-601)
    expect(fallback('pi-x', 2, piToCard)).toBe(-603)

    // 同 index 下三平台回退 ID 互不相同，且与真实正数 ID 无碰撞
    expect(new Set([fallback('a', 0, openClawToCard), fallback('b', 0, hermesToCard), fallback('c', 0, piToCard)]).size).toBe(3)
  })

  it('hermesToCard / piToCard：baseUrl→apiUrl、model→cliConfig.model 透传', () => {
    const hermesCard = hermesToCard({
      id: 'hermes-1',
      name: 'Hermes Live',
      baseUrl: 'https://hermes.example.com',
      apiKey: 'sk-hermes',
      model: 'h-model',
      enabled: false,
      level: 2,
      category: 'custom',
    }, 0)
    expect(hermesCard.apiUrl).toBe('https://hermes.example.com')
    expect(hermesCard.apiKey).toBe('sk-hermes')
    expect(hermesCard.cliConfig?.model).toBe('h-model')
    expect(hermesCard.enabled).toBe(false)

    const piCard = piToCard({
      id: 'pi-1',
      name: 'Pi Live',
      baseUrl: 'https://pi.example.com',
      apiKey: 'sk-pi',
      model: 'p-model',
      enabled: true,
      level: 4,
      category: 'custom',
    }, 1)
    expect(piCard.apiUrl).toBe('https://pi.example.com')
    expect(piCard.apiKey).toBe('sk-pi')
    expect(piCard.cliConfig?.model).toBe('p-model')
  })

  it('cardToOpenClawProvider：apiUrl→baseUrl、cliConfig.model→model，original 片段兜底保留', () => {
    const original: OpenClawProvider = {
      id: 'openclaw-keep',
      name: 'OpenClaw Live',
      baseUrl: 'https://old.example.com',
      apiKey: 'old-key',
      enabled: true,
      cliConfig: { api: 'native-api', extra: 'kept' },
    }

    // 卡片携带 model：映射进结构化字段并从片段移除
    const withModel = cardToOpenClawProvider(createCard(1, {
      providerRef: 'openclaw-keep',
      name: 'OpenClaw Live',
      apiUrl: 'https://new.example.com/v1',
      apiKey: 'sk-new',
      cliConfig: { model: 'claude-sonnet-4-5' },
    }), original)
    expect(withModel).toMatchObject({
      id: 'openclaw-keep',
      name: 'OpenClaw Live',
      baseUrl: 'https://new.example.com/v1',
      apiKey: 'sk-new',
      model: 'claude-sonnet-4-5',
      enabled: true,
    })
    expect(withModel.cliConfig).toEqual({ api: 'native-api', extra: 'kept' })

    // 卡片段落为空时保留 original 的原生片段
    const fallbackFragment = cardToOpenClawProvider(createCard(2, {
      providerRef: 'openclaw-keep',
      name: 'OpenClaw Live',
      apiUrl: '',
      apiKey: '',
      cliConfig: {},
    }), original)
    expect(fallbackFragment.baseUrl).toBe('')
    expect(fallbackFragment.cliConfig).toEqual(original.cliConfig)

    // 无 original 的新增卡片：model 从 cliConfig 提取
    const added = cardToOpenClawProvider(createCard(3, {
      name: 'New Entry',
      apiUrl: 'https://add.example.com',
      apiKey: 'sk-add',
      cliConfig: { model: 'gpt-5' },
    }))
    expect(added.id).toBe('3')
    expect(added.model).toBe('gpt-5')
    expect(added.cliConfig).toEqual({})
  })

  it('cardToHermesProvider / cardToPiProvider：与 openClaw 对称的反向映射', () => {
    const hermesPayload = cardToHermesProvider(createCard(5, {
      providerRef: 'hermes-keep',
      name: 'Hermes Live',
      apiUrl: 'https://hermes.example.com',
      apiKey: 'sk-hermes',
      cliConfig: { model: 'h-model', note: 'kept' },
    }), {
      id: 'hermes-keep',
      name: 'Hermes Live',
      enabled: true,
      cliConfig: { legacy: 'kept' },
    } as HermesProvider)
    expect(hermesPayload).toMatchObject({
      id: 'hermes-keep',
      baseUrl: 'https://hermes.example.com',
      apiKey: 'sk-hermes',
      model: 'h-model',
    })
    expect(hermesPayload.cliConfig).toEqual({ note: 'kept' })

    const piPayload = cardToPiProvider(createCard(6, {
      providerRef: 'pi-keep',
      name: 'Pi Live',
      apiUrl: 'https://pi.example.com',
      apiKey: 'sk-pi',
      cliConfig: { model: 'p-model' },
    }))
    expect(piPayload).toMatchObject({
      id: 'pi-keep',
      baseUrl: 'https://pi.example.com',
      model: 'p-model',
    })
  })
})

describe('useProviderCards additive 持久化（openclaw/hermes/pi）', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(Call.ByName).mockResolvedValue(undefined)
    vi.mocked(LoadProviders).mockResolvedValue([])
    vi.mocked(getOpenClawProviders).mockResolvedValue([])
    vi.mocked(deleteOpenClawProvider).mockResolvedValue(undefined)
    vi.mocked(updateOpenClawProvider).mockResolvedValue(null)
    vi.mocked(addOpenClawProvider).mockResolvedValue(null)
  })

  it('加载走 openClawToCard：baseUrl 进 apiUrl、真实字符串 ID 进 providerRef', async () => {
    vi.mocked(getOpenClawProviders).mockResolvedValue([
      { id: 'openclaw-live-1', name: 'Live One', baseUrl: 'https://one.example.com', apiKey: 'sk-1', enabled: true },
      { id: 'openclaw-live-2', name: 'Live Two', baseUrl: 'https://two.example.com', apiKey: 'sk-2', enabled: false },
    ])

    const providerCards = createUseProviderCards()
    await providerCards.loadProvidersFromDisk(vi.fn().mockResolvedValue(undefined))

    expect(providerCards.cards.openclaw.map((card) => card.providerRef)).toEqual([
      'openclaw-live-1',
      'openclaw-live-2',
    ])
    expect(providerCards.cards.openclaw.map((card) => card.apiUrl)).toEqual([
      'https://one.example.com',
      'https://two.example.com',
    ])
  })

  it('persist diff：消失项 Delete / 存在项 Update / 新增项 Add，并用返回值精确回填 providerRef', async () => {
    const keep: OpenClawProvider = {
      id: 'oc-keep',
      name: 'Keep Me',
      baseUrl: 'https://keep.example.com',
      apiKey: 'sk-keep',
      enabled: true,
    }
    const removed: OpenClawProvider = {
      id: 'oc-remove',
      name: 'Remove Me',
      baseUrl: 'https://remove.example.com',
      apiKey: 'sk-remove',
      enabled: true,
    }

    vi.mocked(getOpenClawProviders)
      .mockResolvedValueOnce([keep, removed])
      .mockResolvedValueOnce([{ ...keep }, { id: 'oc-final', name: 'Fresh Card', enabled: true }])
    vi.mocked(updateOpenClawProvider).mockResolvedValue({ ...keep })
    vi.mocked(addOpenClawProvider).mockResolvedValue({ id: 'oc-final', name: 'Fresh Card', enabled: true })

    const providerCards = createUseProviderCards()
    await providerCards.loadProvidersFromDisk(vi.fn().mockResolvedValue(undefined))

    // UI 变更：删除 oc-remove 卡片，追加无 providerRef 的新卡
    providerCards.cards.openclaw.splice(1, 1)
    providerCards.appendCardToGroup('openclaw', createCard(901, {
      name: 'Fresh Card',
      apiUrl: 'https://fresh.example.com',
      apiKey: 'sk-fresh',
      enabled: true,
    }))

    await providerCards.persistProviders('openclaw')

    expect(vi.mocked(deleteOpenClawProvider)).toHaveBeenCalledTimes(1)
    expect(vi.mocked(deleteOpenClawProvider)).toHaveBeenCalledWith('oc-remove')

    expect(vi.mocked(updateOpenClawProvider)).toHaveBeenCalledTimes(1)
    expect(vi.mocked(updateOpenClawProvider)).toHaveBeenCalledWith(expect.objectContaining({
      id: 'oc-keep',
      baseUrl: 'https://keep.example.com',
      apiKey: 'sk-keep',
    }))

    expect(vi.mocked(addOpenClawProvider)).toHaveBeenCalledTimes(1)
    expect(vi.mocked(addOpenClawProvider)).toHaveBeenCalledWith(expect.objectContaining({
      name: 'Fresh Card',
      baseUrl: 'https://fresh.example.com',
      apiKey: 'sk-fresh',
    }))

    // P0 固化：新增卡片的 providerRef 用后端返回的最终 ID 精确回填，不依赖名称匹配
    const freshCard = providerCards.cards.openclaw.find((card) => card.name === 'Fresh Card')
    expect(freshCard?.providerRef).toBe('oc-final')
  })

  it('后端返回值缺失时名称兜底回填：同名条目无法消歧则跳过', async () => {
    vi.mocked(getOpenClawProviders)
      .mockResolvedValueOnce([])
      // 返回值缺失（后端未返回完整 provider）且落盘 ID 被重新分配为两个同名条目
      .mockResolvedValueOnce([
        { id: 'oc-b1', name: 'Same Name', enabled: true },
        { id: 'oc-b2', name: 'Same Name', enabled: true },
      ])
    vi.mocked(addOpenClawProvider).mockResolvedValue(null)

    const providerCards = createUseProviderCards()
    await providerCards.loadProvidersFromDisk(vi.fn().mockResolvedValue(undefined))

    providerCards.appendCardToGroup('openclaw', createCard(902, {
      name: 'Same Name',
      apiUrl: 'https://same.example.com',
      enabled: true,
    }))

    await providerCards.persistProviders('openclaw')

    // 同名不回填：保持卡片原状（providerRef 缺失），避免按顺序错配到错误条目
    expect(providerCards.cards.openclaw[0]?.providerRef).toBeUndefined()
  })

  it('保存失败时从后端重拉缓存与卡片，而非仅回滚 UI 快照', async () => {
    const persisted: OpenClawProvider = {
      id: 'oc-live',
      name: 'Live Card',
      baseUrl: 'https://live.example.com',
      enabled: true,
    }
    vi.mocked(getOpenClawProviders)
      .mockResolvedValueOnce([persisted])
      .mockResolvedValueOnce([{ ...persisted, name: 'Reloaded Live Card' }])
    vi.mocked(deleteOpenClawProvider).mockRejectedValueOnce(new Error('live write failed'))

    const providerCards = createUseProviderCards()
    await providerCards.loadProvidersFromDisk(vi.fn().mockResolvedValue(undefined))

    // UI 删除卡片后保存失败：卡片应以落盘状态为准恢复
    providerCards.cards.openclaw.splice(0, 1)
    await expect(providerCards.persistProviders('openclaw')).rejects.toThrow('live write failed')

    expect(providerCards.cards.openclaw).toHaveLength(1)
    expect(providerCards.cards.openclaw[0]?.name).toBe('Reloaded Live Card')
    expect(providerCards.cards.openclaw[0]?.providerRef).toBe('oc-live')
  })

  it('同 tab 并发保存被锁拦下：第二次直接跳过并提示', async () => {
    vi.mocked(getOpenClawProviders).mockResolvedValue([
      { id: 'oc-only', name: 'Only One', baseUrl: 'https://only.example.com', enabled: true },
    ])
    const releaseUpdateRef: { current?: () => void } = {}
    vi.mocked(updateOpenClawProvider).mockImplementationOnce(() => new Promise<OpenClawProvider | null>((resolve) => {
      releaseUpdateRef.current = () => resolve(null)
    }))

    const providerCards = createUseProviderCards()
    await providerCards.loadProvidersFromDisk(vi.fn().mockResolvedValue(undefined))

    const first = providerCards.persistProviders('openclaw')
    await providerCards.persistProviders('openclaw')

    expect(vi.mocked(updateOpenClawProvider)).toHaveBeenCalledTimes(1)
    expect(vi.mocked(showToast)).toHaveBeenCalledWith('components.main.form.saveInProgress', 'error')

    releaseUpdateRef.current?.()
    await first
    // 锁随首次保存结束释放
    await providerCards.persistProviders('openclaw')
    expect(vi.mocked(updateOpenClawProvider)).toHaveBeenCalledTimes(2)
  })

  it('hermes 与 pi 走同样的 diff 与回填链路', async () => {
    vi.mocked(Call.ByName).mockResolvedValue(undefined)
    const { getHermesProviders, updateHermesProvider, addHermesProvider, deleteHermesProvider } = await import('../../../../services/hermes')
    const { getPiProviders, updatePiProvider, addPiProvider, deletePiProvider } = await import('../../../../services/pi')

    const hermesKeep = { id: 'hm-keep', name: 'Hermes Keep', baseUrl: 'https://h.example.com', enabled: true }
    const hermesFresh = 'hm-fresh'
    vi.mocked(getHermesProviders)
      .mockResolvedValueOnce([hermesKeep, { id: 'hm-gone', name: 'Hermes Gone', enabled: true }])
      .mockResolvedValueOnce([hermesKeep])
    vi.mocked(updateHermesProvider).mockResolvedValue(null)
    vi.mocked(addHermesProvider).mockResolvedValue({ id: hermesFresh, name: 'Hermes Fresh', enabled: true })
    vi.mocked(deleteHermesProvider).mockResolvedValue(undefined)

    const providerCards = createUseProviderCards()
    await providerCards.loadProvidersFromDisk(vi.fn().mockResolvedValue(undefined))

    providerCards.cards.hermes.splice(1, 1)
    providerCards.appendCardToGroup('hermes', createCard(911, {
      name: 'Hermes Fresh',
      apiUrl: 'https://h-fresh.example.com',
      enabled: true,
    }))
    await providerCards.persistProviders('hermes')

    expect(vi.mocked(deleteHermesProvider)).toHaveBeenCalledWith('hm-gone')
    expect(vi.mocked(updateHermesProvider)).toHaveBeenCalledWith(expect.objectContaining({ id: 'hm-keep' }))
    expect(vi.mocked(addHermesProvider)).toHaveBeenCalledWith(expect.objectContaining({ baseUrl: 'https://h-fresh.example.com' }))
    expect(providerCards.cards.hermes.find((card) => card.name === 'Hermes Fresh')?.providerRef).toBe(hermesFresh)

    const piKeep = { id: 'pi-keep', name: 'Pi Keep', baseUrl: 'https://p.example.com', enabled: true }
    vi.mocked(getPiProviders)
      .mockResolvedValueOnce([piKeep, { id: 'pi-gone', name: 'Pi Gone', enabled: true }])
      .mockResolvedValueOnce([piKeep])
    vi.mocked(updatePiProvider).mockResolvedValue(null)
    vi.mocked(addPiProvider).mockResolvedValue(null)
    vi.mocked(deletePiProvider).mockResolvedValue(undefined)

    await providerCards.loadProvidersFromDisk(vi.fn().mockResolvedValue(undefined))

    providerCards.cards.pi.splice(1, 1)
    providerCards.appendCardToGroup('pi', createCard(921, {
      name: 'Pi Fresh',
      apiUrl: 'https://p-fresh.example.com',
      enabled: true,
    }))
    await providerCards.persistProviders('pi')

    expect(vi.mocked(deletePiProvider)).toHaveBeenCalledWith('pi-gone')
    expect(vi.mocked(updatePiProvider)).toHaveBeenCalledWith(expect.objectContaining({ id: 'pi-keep' }))
    expect(vi.mocked(addPiProvider)).toHaveBeenCalledWith(expect.objectContaining({ baseUrl: 'https://p-fresh.example.com' }))
  })
})

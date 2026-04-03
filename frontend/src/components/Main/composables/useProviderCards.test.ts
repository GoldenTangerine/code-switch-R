import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { AutomationCard } from '../../../data/cards'
import type { ProviderDragTarget, ProviderTab } from '../types'

vi.mock('../../../../bindings/codeswitch/services/providerservice', () => ({
  DuplicateProvider: vi.fn(),
  LoadProviders: vi.fn(),
  SaveProviders: vi.fn(),
}))

vi.mock('../../../../bindings/codeswitch/services/geminiservice', () => ({
  AddProvider: vi.fn(),
  DeleteProvider: vi.fn(),
  GetProviders: vi.fn(),
  ReorderProviders: vi.fn(),
  UpdateProvider: vi.fn(),
}))

vi.mock('@wailsio/runtime', () => ({
  Call: {
    ByName: vi.fn(),
  },
}))

import { SaveProviders } from '../../../../bindings/codeswitch/services/providerservice'
import { LoadProviders } from '../../../../bindings/codeswitch/services/providerservice'
import { AddProvider as AddGeminiProvider, GetProviders as GetGeminiProviders } from '../../../../bindings/codeswitch/services/geminiservice'
import { useProviderCards } from './useProviderCards'

const createCard = (
  id: number,
  overrides: Partial<AutomationCard> = {},
): AutomationCard => ({
  id,
  name: `Provider ${id}`,
  apiUrl: `https://example-${id}.com`,
  apiKey: '',
  officialSite: `https://example-${id}.com`,
  icon: 'openai',
  tint: 'rgba(10, 132, 255, 0.14)',
  accent: '#0a84ff',
  enabled: true,
  level: 1,
  sortOrder: 1,
  ...overrides,
})

const getIds = (cards: AutomationCard[]) => cards.map((card) => card.id)
const createDragEndPayload = (overrides: Partial<{
  dropEffect: DataTransfer['dropEffect'] | 'none'
  clientX: number | null
  clientY: number | null
  endedInsideList: boolean | null
}> = {}) => ({
  dropEffect: 'none' as DataTransfer['dropEffect'] | 'none',
  clientX: 320,
  clientY: 240,
  endedInsideList: null as boolean | null,
  ...overrides,
})

describe('useProviderCards drag sort', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(SaveProviders).mockResolvedValue(undefined)
    vi.mocked(LoadProviders).mockResolvedValue([])
    vi.mocked(AddGeminiProvider).mockResolvedValue(undefined)
    vi.mocked(GetGeminiProviders).mockResolvedValue([])
  })

  it('keeps reordered result after drop and persists once', async () => {
    let activeTab: ProviderTab = 'codex'
    const dragTarget: ProviderDragTarget = { id: 3, position: 'after' }
    const providerCards = useProviderCards({
      t: (key: string) => key,
      getActiveTab: () => activeTab,
      isActiveProxyEnabled: () => false,
      getSelectedToolId: () => null,
    })

    providerCards.cards.codex.splice(0, providerCards.cards.codex.length, createCard(1), createCard(2), createCard(3))

    providerCards.onDragStart(1)
    providerCards.onDragOverCard(dragTarget)

    expect(getIds(providerCards.cards.codex)).toEqual([2, 3, 1])

    await providerCards.onDrop(dragTarget)
    await providerCards.onDragEnd(createDragEndPayload())

    expect(getIds(providerCards.cards.codex)).toEqual([2, 3, 1])
    expect(providerCards.draggingId.value).toBeNull()
    expect(vi.mocked(SaveProviders)).toHaveBeenCalledTimes(1)
    expect(vi.mocked(SaveProviders).mock.calls[0]?.[0]).toBe(activeTab)
  })

  it('persists dragged order when existing cards already have distinct sortOrder values', async () => {
    const dragTarget: ProviderDragTarget = { id: 3, position: 'after' }
    const providerCards = useProviderCards({
      t: (key: string) => key,
      getActiveTab: () => 'codex',
      isActiveProxyEnabled: () => false,
      getSelectedToolId: () => null,
    })

    providerCards.cards.codex.splice(
      0,
      providerCards.cards.codex.length,
      createCard(1, { sortOrder: 1 }),
      createCard(2, { sortOrder: 2 }),
      createCard(3, { sortOrder: 3 }),
    )

    providerCards.onDragStart(1)
    providerCards.onDragOverCard(dragTarget)

    await providerCards.onDrop(dragTarget)
    await providerCards.onDragEnd(createDragEndPayload({ endedInsideList: true }))

    expect(getIds(providerCards.cards.codex)).toEqual([2, 3, 1])
    expect(providerCards.cards.codex.map((card) => card.sortOrder)).toEqual([1, 2, 3])
    expect(vi.mocked(SaveProviders)).toHaveBeenCalledTimes(1)
    expect((vi.mocked(SaveProviders).mock.calls[0]?.[1] as AutomationCard[]).map((card) => card.id)).toEqual([2, 3, 1])
  })

  it('keeps original order when drag ends without any effective reorder', async () => {
    const providerCards = useProviderCards({
      t: (key: string) => key,
      getActiveTab: () => 'codex',
      isActiveProxyEnabled: () => false,
      getSelectedToolId: () => null,
    })

    providerCards.cards.codex.splice(0, providerCards.cards.codex.length, createCard(1), createCard(2), createCard(3))

    providerCards.onDragStart(1)

    await providerCards.onDragEnd(createDragEndPayload({ endedInsideList: true }))

    expect(getIds(providerCards.cards.codex)).toEqual([1, 2, 3])
    expect(vi.mocked(SaveProviders)).not.toHaveBeenCalled()
  })

  it('persists reordered result on drag end when drop event is missing', async () => {
    const providerCards = useProviderCards({
      t: (key: string) => key,
      getActiveTab: () => 'codex',
      isActiveProxyEnabled: () => false,
      getSelectedToolId: () => null,
    })

    providerCards.cards.codex.splice(0, providerCards.cards.codex.length, createCard(1), createCard(2), createCard(3))

    providerCards.onDragStart(1)
    providerCards.onDragOverCard({ id: 3, position: 'after' })

    expect(getIds(providerCards.cards.codex)).toEqual([2, 3, 1])

    await providerCards.onDragEnd()

    expect(getIds(providerCards.cards.codex)).toEqual([2, 3, 1])
    expect(vi.mocked(SaveProviders)).toHaveBeenCalledTimes(1)
  })

  it('restores original order when dragging leaves the list before release', async () => {
    const providerCards = useProviderCards({
      t: (key: string) => key,
      getActiveTab: () => 'codex',
      isActiveProxyEnabled: () => false,
      getSelectedToolId: () => null,
    })

    providerCards.cards.codex.splice(0, providerCards.cards.codex.length, createCard(1), createCard(2), createCard(3))

    providerCards.onDragStart(1)
    providerCards.onDragOverCard({ id: 3, position: 'after' })
    providerCards.onDragLeaveList()

    await providerCards.onDragEnd(createDragEndPayload({ endedInsideList: false }))

    expect(getIds(providerCards.cards.codex)).toEqual([1, 2, 3])
    expect(vi.mocked(SaveProviders)).not.toHaveBeenCalled()
  })

  it('keeps reordered result when dragend fires before drop in desktop webviews', async () => {
    const dragTarget: ProviderDragTarget = { id: 3, position: 'after' }
    const providerCards = useProviderCards({
      t: (key: string) => key,
      getActiveTab: () => 'codex',
      isActiveProxyEnabled: () => false,
      getSelectedToolId: () => null,
    })

    providerCards.cards.codex.splice(0, providerCards.cards.codex.length, createCard(1), createCard(2), createCard(3))

    providerCards.onDragStart(1)
    providerCards.onDragOverCard(dragTarget)

    const dragEndPromise = providerCards.onDragEnd(createDragEndPayload({ endedInsideList: true }))
    await providerCards.onDrop(dragTarget)
    await dragEndPromise

    expect(getIds(providerCards.cards.codex)).toEqual([2, 3, 1])
    expect(vi.mocked(SaveProviders)).toHaveBeenCalledTimes(1)
  })

  it('does not persist reorder when dragend reports outside-list release even if dropEffect is move', async () => {
    const providerCards = useProviderCards({
      t: (key: string) => key,
      getActiveTab: () => 'codex',
      isActiveProxyEnabled: () => false,
      getSelectedToolId: () => null,
    })

    providerCards.cards.codex.splice(0, providerCards.cards.codex.length, createCard(1), createCard(2), createCard(3))

    providerCards.onDragStart(1)
    providerCards.onDragOverCard({ id: 3, position: 'after' })

    await providerCards.onDragEnd(createDragEndPayload({
      dropEffect: 'move',
      endedInsideList: false,
    }))

    expect(getIds(providerCards.cards.codex)).toEqual([1, 2, 3])
    expect(vi.mocked(SaveProviders)).not.toHaveBeenCalled()
  })

  it('reorders before or after target based on pointer half', () => {
    const providerCards = useProviderCards({
      t: (key: string) => key,
      getActiveTab: () => 'codex',
      isActiveProxyEnabled: () => false,
      getSelectedToolId: () => null,
    })

    providerCards.cards.codex.splice(0, providerCards.cards.codex.length, createCard(1), createCard(2), createCard(3))
    providerCards.onDragStart(1)
    providerCards.onDragOverCard({ id: 3, position: 'before' })

    expect(getIds(providerCards.cards.codex)).toEqual([2, 1, 3])

    providerCards.onDragOverCard({ id: 3, position: 'after' })

    expect(getIds(providerCards.cards.codex)).toEqual([2, 3, 1])
  })

  it('allows dragging disabled providers and persists the manual order', async () => {
    const providerCards = useProviderCards({
      t: (key: string) => key,
      getActiveTab: () => 'codex',
      isActiveProxyEnabled: () => false,
      getSelectedToolId: () => null,
    })

    providerCards.cards.codex.splice(
      0,
      providerCards.cards.codex.length,
      createCard(1, { enabled: true }),
      createCard(2, { enabled: false }),
      createCard(3, { enabled: false }),
    )

    providerCards.onDragStart(3)
    providerCards.onDragOverCard({ id: 2, position: 'before' })
    await providerCards.onDrop({ id: 2, position: 'before' })

    expect(getIds(providerCards.cards.codex)).toEqual([1, 3, 2])
    expect(vi.mocked(SaveProviders)).toHaveBeenCalledTimes(1)
  })

  it('does not allow dragging a disabled provider ahead of enabled providers', async () => {
    const providerCards = useProviderCards({
      t: (key: string) => key,
      getActiveTab: () => 'codex',
      isActiveProxyEnabled: () => false,
      getSelectedToolId: () => null,
    })

    providerCards.cards.codex.splice(
      0,
      providerCards.cards.codex.length,
      createCard(1, { enabled: true, sortOrder: 1 }),
      createCard(2, { enabled: true, sortOrder: 2 }),
      createCard(3, { enabled: false, sortOrder: 1 }),
    )

    providerCards.onDragStart(3)
    providerCards.onDragOverCard({ id: 1, position: 'before' })
    await providerCards.onDrop({ id: 1, position: 'before' })

    expect(getIds(providerCards.cards.codex)).toEqual([1, 2, 3])
    expect(vi.mocked(SaveProviders)).not.toHaveBeenCalled()
  })

  it('normalizes provider order after reload with enabled group first and group-local sort order', async () => {
    const providerCards = useProviderCards({
      t: (key: string) => key,
      getActiveTab: () => 'codex',
      isActiveProxyEnabled: () => false,
      getSelectedToolId: () => null,
    })

    vi.mocked(LoadProviders).mockImplementation(((kind: string) => {
      if (kind === 'codex') {
        return Promise.resolve([
          createCard(4, { enabled: false, sortOrder: 2 }),
          createCard(2, { enabled: true, sortOrder: 2 }),
          createCard(1, { enabled: true, sortOrder: 1 }),
          createCard(3, { enabled: false, sortOrder: 1 }),
        ]) as any
      }
      return Promise.resolve([]) as any
    }) as any)

    await providerCards.loadProvidersFromDisk(async () => {})

    expect(getIds(providerCards.cards.codex)).toEqual([1, 2, 3, 4])
    expect(providerCards.cards.codex.map((card) => card.enabled)).toEqual([true, true, false, false])
    expect(providerCards.cards.codex.map((card) => card.sortOrder)).toEqual([1, 2, 1, 2])
  })

  it('keeps persisted custom CLI provider order after reload', async () => {
    const providerCards = useProviderCards({
      t: (key: string) => key,
      getActiveTab: () => 'others',
      isActiveProxyEnabled: () => false,
      getSelectedToolId: () => 'tool-1',
    })

    vi.mocked(LoadProviders).mockImplementation(((kind: string) => {
      if (kind === 'custom:tool-1') {
        return Promise.resolve([
          createCard(2, { enabled: false, sortOrder: 1 }),
          createCard(1, { enabled: true, sortOrder: 1 }),
        ]) as any
      }
      return Promise.resolve([]) as any
    }) as any)

    await providerCards.loadCustomCliProviders('tool-1')

    expect(getIds(providerCards.cards.others)).toEqual([1, 2])
    expect(providerCards.cards.others.map((card) => card.enabled)).toEqual([true, false])
  })

  it('keeps persisted Gemini provider order after reload', async () => {
    const providerCards = useProviderCards({
      t: (key: string) => key,
      getActiveTab: () => 'gemini',
      isActiveProxyEnabled: () => false,
      getSelectedToolId: () => null,
    })

    vi.mocked(GetGeminiProviders).mockResolvedValue([
      {
        id: 'gemini-disabled',
        name: 'Gemini Disabled',
        baseUrl: 'https://disabled.example.com',
        apiKey: 'disabled-key',
        websiteUrl: 'https://disabled.example.com',
        enabled: false,
        level: 5,
      },
      {
        id: 'gemini-enabled',
        name: 'Gemini Enabled',
        baseUrl: 'https://enabled.example.com',
        apiKey: 'enabled-key',
        websiteUrl: 'https://enabled.example.com',
        enabled: true,
        level: 1,
      },
    ] as any)

    await providerCards.loadProvidersFromDisk(async () => {})

    expect(providerCards.cards.gemini.map((card) => card.providerRef)).toEqual([
      'gemini-enabled',
      'gemini-disabled',
    ])
    expect(providerCards.cards.gemini.map((card) => card.enabled)).toEqual([true, false])
  })

  it('persists full Gemini provider payload when creating a new provider', async () => {
    const providerCards = useProviderCards({
      t: (key: string) => key,
      getActiveTab: () => 'gemini',
      isActiveProxyEnabled: () => false,
      getSelectedToolId: () => null,
    })

    providerCards.appendCardToGroup('gemini', createCard(9, {
      providerRef: 'gemini-new',
      enabled: true,
      sortOrder: 1,
      level: 4,
      apiUrl: 'https://gemini.example.com',
      apiKey: 'gemini-key',
      officialSite: 'https://gemini.example.com',
      cliConfig: { GEMINI_MODEL: 'gemini-2.5-pro' },
      requestBodyOverrides: { temperature: 0.3 },
    }))

    vi.mocked(GetGeminiProviders).mockResolvedValueOnce([])
    vi.mocked(GetGeminiProviders).mockResolvedValueOnce([
      {
        id: 'gemini-new',
        name: 'Provider 9',
        baseUrl: 'https://gemini.example.com',
        apiKey: 'gemini-key',
        websiteUrl: 'https://gemini.example.com',
        enabled: true,
        sortOrder: 1,
        level: 4,
      },
    ] as any)
    vi.mocked(GetGeminiProviders).mockResolvedValueOnce([
      {
        id: 'gemini-new',
        name: 'Provider 9',
        baseUrl: 'https://gemini.example.com',
        apiKey: 'gemini-key',
        websiteUrl: 'https://gemini.example.com',
        enabled: true,
        sortOrder: 1,
        level: 4,
      },
    ] as any)

    await providerCards.persistProviders('gemini')

    expect(vi.mocked(AddGeminiProvider)).toHaveBeenCalledTimes(1)
    expect(vi.mocked(AddGeminiProvider).mock.calls[0]?.[0]).toMatchObject({
      id: 'gemini-new',
      baseUrl: 'https://gemini.example.com',
      apiKey: 'gemini-key',
      websiteUrl: 'https://gemini.example.com',
      enabled: true,
      sortOrder: 1,
      level: 4,
      model: 'gemini-2.5-pro',
      requestBodyOverrides: { temperature: 0.3 },
    })
  })
})

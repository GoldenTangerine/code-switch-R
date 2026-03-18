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
import { useProviderCards } from './useProviderCards'

const createCard = (id: number): AutomationCard => ({
  id,
  name: `Provider ${id}`,
  apiUrl: `https://example-${id}.com`,
  apiKey: '',
  officialSite: `https://example-${id}.com`,
  icon: 'openai',
  tint: 'rgba(10, 132, 255, 0.14)',
  accent: '#0a84ff',
  enabled: true,
})

const getIds = (cards: AutomationCard[]) => cards.map((card) => card.id)

describe('useProviderCards drag sort', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(SaveProviders).mockResolvedValue(undefined)
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
    providerCards.onDragEnd()

    expect(getIds(providerCards.cards.codex)).toEqual([2, 3, 1])
    expect(providerCards.draggingId.value).toBeNull()
    expect(vi.mocked(SaveProviders)).toHaveBeenCalledTimes(1)
    expect(vi.mocked(SaveProviders).mock.calls[0]?.[0]).toBe(activeTab)
  })

  it('restores original order when drag ends without drop', () => {
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

    providerCards.onDragEnd()

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
})

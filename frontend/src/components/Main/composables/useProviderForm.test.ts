import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { AutomationCard } from '../../../data/cards'
import type { ProviderTab, VendorForm } from '../types'

vi.mock('@wailsio/runtime', () => ({
  Call: {
    ByName: vi.fn(),
  },
}))

import { useProviderForm } from './useProviderForm'

const createCard = (
  id: number,
  overrides: Partial<AutomationCard> = {},
): AutomationCard => ({
  id,
  providerRef: `${id}`,
  name: `Provider ${id}`,
  apiUrl: `https://example-${id}.com`,
  apiKey: `key-${id}`,
  officialSite: `https://example-${id}.com`,
  icon: 'openai',
  tint: 'rgba(10, 132, 255, 0.14)',
  accent: '#0a84ff',
  enabled: true,
  level: 1,
  supportedModels: {},
  modelMapping: {},
  requestBodyOverrides: {},
  cliConfig: {},
  apiEndpoint: '',
  availabilityMonitorEnabled: false,
  connectivityAutoBlacklist: false,
  availabilityConfig: {
    testModel: '',
    testEndpoint: '/responses',
    timeout: 15000,
  },
  connectivityCheck: false,
  connectivityTestModel: '',
  connectivityTestEndpoint: '',
  connectivityAuthType: '',
  ...overrides,
})

const createCardRecord = (): Record<ProviderTab, AutomationCard[]> => ({
  claude: [],
  codex: [],
  gemini: [],
  others: [],
})

const createForm = (overrides: Partial<VendorForm> = {}): VendorForm => ({
  name: 'New Provider',
  apiUrl: 'https://new-provider.example.com',
  apiKey: 'new-key',
  officialSite: 'https://new-provider.example.com',
  icon: 'openai',
  enabled: true,
  level: 1,
  supportedModels: {},
  modelMapping: {},
  requestBodyOverrides: {},
  cliConfig: {},
  apiEndpoint: '',
  availabilityMonitorEnabled: false,
  connectivityAutoBlacklist: false,
  availabilityConfig: {
    testModel: '',
    testEndpoint: '/responses',
    timeout: 15000,
  },
  connectivityCheck: false,
  connectivityTestModel: '',
  connectivityTestEndpoint: '',
  connectivityAuthType: '',
  ...overrides,
})

const normalizeLevel = (level: number | string | undefined) => {
  const numeric = Number(level)
  if (!Number.isFinite(numeric) || numeric < 1) return 1
  if (numeric > 10) return 10
  return Math.floor(numeric)
}

describe('useProviderForm order preservation', () => {
  beforeEach(() => {
    vi.stubGlobal('window', {
      dispatchEvent: vi.fn(),
    })
    vi.stubGlobal('CustomEvent', class {
      type: string

      constructor(type: string) {
        this.type = type
      }
    })
  })

  afterEach(() => {
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })

  it('keeps manual order when editing a provider level', async () => {
    const cards = createCardRecord()
    const first = createCard(2, { level: 3 })
    const second = createCard(1, { level: 1 })
    cards.codex.push(first, second)

    const persistProviders = vi.fn().mockResolvedValue(undefined)
    const providerForm = useProviderForm({
      initialTab: 'codex',
      t: (key: string) => key,
      showToast: vi.fn(),
      getActiveTab: () => 'codex',
      cards,
      normalizeLevel,
      persistProviders,
      refreshDirectAppliedStatus: vi.fn().mockResolvedValue(undefined),
      removeProvider: vi.fn().mockResolvedValue(undefined),
      duplicateProvider: vi.fn().mockResolvedValue(false),
      reloadProviders: vi.fn().mockResolvedValue(undefined),
    })

    providerForm.configure(first)
    await providerForm.submitProviderModal(createForm({
      name: first.name,
      apiUrl: first.apiUrl,
      apiKey: first.apiKey,
      officialSite: first.officialSite,
      level: 1,
    }))

    expect(cards.codex.map((card) => card.id)).toEqual([2, 1])
    expect(cards.codex[0]?.level).toBe(1)
    expect(persistProviders).toHaveBeenCalledWith('codex')
  })

  it('appends new providers to the end instead of reordering the list', async () => {
    const cards = createCardRecord()
    cards.codex.push(
      createCard(2, { level: 3 }),
      createCard(1, { level: 1 }),
    )

    const persistProviders = vi.fn().mockResolvedValue(undefined)
    vi.spyOn(Date, 'now').mockReturnValue(999)

    const providerForm = useProviderForm({
      initialTab: 'codex',
      t: (key: string) => key,
      showToast: vi.fn(),
      getActiveTab: () => 'codex',
      cards,
      normalizeLevel,
      persistProviders,
      refreshDirectAppliedStatus: vi.fn().mockResolvedValue(undefined),
      removeProvider: vi.fn().mockResolvedValue(undefined),
      duplicateProvider: vi.fn().mockResolvedValue(false),
      reloadProviders: vi.fn().mockResolvedValue(undefined),
    })

    providerForm.openCreateModal()
    await providerForm.submitProviderModal(createForm({
      name: 'Appended Provider',
      level: 1,
    }))

    expect(cards.codex.map((card) => card.id)).toEqual([2, 1, 999])
    expect(cards.codex[2]?.name).toBe('Appended Provider')
    expect(cards.codex[2]?.level).toBe(1)
    expect(persistProviders).toHaveBeenCalledWith('codex')
  })
})

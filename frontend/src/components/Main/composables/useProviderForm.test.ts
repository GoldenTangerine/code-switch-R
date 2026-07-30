import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { toRaw } from 'vue'
import type { AutomationCard } from '../../../data/cards'
import type { ProviderTab, VendorForm } from '../types'
import { insertProviderToStatusGroup, moveProviderToStatusGroup } from '../utils/providerOrder'

vi.mock('@wailsio/runtime', () => ({
  Call: {
    ByName: vi.fn(),
  },
}))

import { Call } from '@wailsio/runtime'
import {
  normalizeModelMappingDisabled,
  normalizeModelMappingReasoningEfforts,
  normalizeModelMappingSupports1M,
} from '../adapters/providerFormMappers'
import { useProviderForm } from './useProviderForm'

describe('model mapping reasoning effort normalization', () => {
  it('keeps configured values only for existing mapping rules', () => {
    expect(normalizeModelMappingReasoningEfforts({
      'claude-*': ' high ',
      orphan: 'low',
      empty: ' ',
    }, {
      'claude-*': 'vendor-*',
      empty: 'vendor-empty',
    })).toEqual({
      'claude-*': 'high',
    })
  })
})

describe('model mapping disabled state normalization', () => {
  it('keeps only disabled entries that still have mapping rules', () => {
    expect(normalizeModelMappingDisabled({
      'claude-*': true,
      'gpt-*': false,
      orphan: true,
    }, {
      'claude-*': 'vendor-*',
      'gpt-*': 'openai-*',
    })).toEqual({
      'claude-*': true,
    })
  })
})

describe('model mapping 1M state normalization', () => {
  it('keeps only enabled entries that still have mapping rules', () => {
    expect(normalizeModelMappingSupports1M({
      'claude-*': true,
      'gpt-*': false,
      orphan: true,
    }, {
      'claude-*': 'vendor-*',
      'gpt-*': 'openai-*',
    })).toEqual({
      'claude-*': true,
    })
  })
})

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
  sortOrder: 1,
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
  opencode: [],
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
    vi.clearAllMocks()
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
    const first = createCard(2, { enabled: true, sortOrder: 2, level: 3 })
    const second = createCard(1, { enabled: true, sortOrder: 1, level: 1 })
    const third = createCard(3, { enabled: false, sortOrder: 1, level: 1 })
    cards.codex.push(first, second, third)

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
      moveCardToStatusGroup: (tabId, card, enabled) => moveProviderToStatusGroup(cards[tabId], card, enabled),
      appendCardToGroup: (tabId, card) => insertProviderToStatusGroup(cards[tabId], card),
    })

    providerForm.configure(first)
    await providerForm.submitProviderModal(createForm({
      name: first.name,
      apiUrl: first.apiUrl,
      apiKey: first.apiKey,
      officialSite: first.officialSite,
      level: 1,
    }))

    expect(cards.codex.map((card) => card.id)).toEqual([2, 1, 3])
    expect(cards.codex[0]?.level).toBe(1)
    expect(persistProviders).toHaveBeenCalledWith('codex')
  })

  it('appends new enabled providers to the end of enabled group', async () => {
    const cards = createCardRecord()
    cards.codex.push(
      createCard(2, { enabled: true, sortOrder: 1, level: 3 }),
      createCard(1, { enabled: true, sortOrder: 2, level: 1 }),
      createCard(3, { enabled: false, sortOrder: 1, level: 1 }),
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
      moveCardToStatusGroup: (tabId, card, enabled) => moveProviderToStatusGroup(cards[tabId], card, enabled),
      appendCardToGroup: (tabId, card) => insertProviderToStatusGroup(cards[tabId], card),
    })

    providerForm.openCreateModal()
    await providerForm.submitProviderModal(createForm({
      name: 'Appended Provider',
      enabled: true,
      level: 1,
    }))

    expect(cards.codex.map((card) => card.id)).toEqual([2, 1, 999, 3])
    expect(cards.codex[2]?.name).toBe('Appended Provider')
    expect(cards.codex[2]?.level).toBe(1)
    expect(persistProviders).toHaveBeenCalledWith('codex')
  })

  it('prepends new disabled providers to the top of disabled group', async () => {
    const cards = createCardRecord()
    cards.codex.push(
      createCard(1, { enabled: true, sortOrder: 1, enabledSortOrder: 1 }),
      createCard(2, { enabled: false, sortOrder: 1, disabledSortOrder: 1 }),
      createCard(3, { enabled: false, sortOrder: 2, disabledSortOrder: 2 }),
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
      moveCardToStatusGroup: (tabId, card, enabled) => moveProviderToStatusGroup(cards[tabId], card, enabled),
      appendCardToGroup: (tabId, card) => insertProviderToStatusGroup(cards[tabId], card),
    })

    providerForm.openCreateModal()
    await providerForm.submitProviderModal(createForm({
      name: 'Prepended Disabled Provider',
      enabled: false,
      level: 1,
    }))

    expect(cards.codex.map((card) => card.id)).toEqual([1, 999, 2, 3])
    expect(cards.codex[1]?.name).toBe('Prepended Disabled Provider')
    expect(cards.codex.slice(1).map((card) => card.disabledSortOrder ?? null)).toEqual([1, 2, 3])
    expect(persistProviders).toHaveBeenCalledWith('codex')
  })

  it('keeps create modal open and removes optimistic card when persist fails', async () => {
    const cards = createCardRecord()
    const persistProviders = vi.fn().mockRejectedValue(new Error('save failed'))
    vi.spyOn(Date, 'now').mockReturnValue(999)

    const providerForm = useProviderForm({
      initialTab: 'opencode',
      t: (key: string) => key,
      showToast: vi.fn(),
      getActiveTab: () => 'opencode',
      cards,
      normalizeLevel,
      persistProviders,
      refreshDirectAppliedStatus: vi.fn().mockResolvedValue(undefined),
      removeProvider: vi.fn().mockResolvedValue(undefined),
      duplicateProvider: vi.fn().mockResolvedValue(false),
      reloadProviders: vi.fn().mockResolvedValue(undefined),
      moveCardToStatusGroup: (tabId, card, enabled) => moveProviderToStatusGroup(cards[tabId], card, enabled),
      appendCardToGroup: (tabId, card) => insertProviderToStatusGroup(cards[tabId], card),
    })

    providerForm.openCreateModal()
    await expect(providerForm.submitProviderModal(createForm({
      name: 'OpenCode Provider',
      providerRef: 'raw-key',
      opencodeNpm: '@ai-sdk/openai-compatible',
      opencodeSettingsConfig: { npm: '@ai-sdk/openai-compatible', models: { 'gpt-4o': { name: 'GPT-4o' } } },
    }))).rejects.toThrow('save failed')

    expect(providerForm.providerModalState.open).toBe(true)
    expect(cards.opencode).toHaveLength(0)
    expect(window.dispatchEvent).not.toHaveBeenCalled()
  })

  it('keeps edit modal open and restores card when persist fails', async () => {
    const cards = createCardRecord()
    const original = createCard(1, {
      providerRef: 'Imported.Provider',
      name: 'Imported Provider',
      enabled: true,
      sortOrder: 1,
      enabledSortOrder: 1,
    })
    cards.opencode.push(original)
    const persistProviders = vi.fn().mockRejectedValue(new Error('save failed'))

    const providerForm = useProviderForm({
      initialTab: 'opencode',
      t: (key: string) => key,
      showToast: vi.fn(),
      getActiveTab: () => 'opencode',
      cards,
      normalizeLevel,
      persistProviders,
      refreshDirectAppliedStatus: vi.fn().mockResolvedValue(undefined),
      removeProvider: vi.fn().mockResolvedValue(undefined),
      duplicateProvider: vi.fn().mockResolvedValue(false),
      reloadProviders: vi.fn().mockResolvedValue(undefined),
      moveCardToStatusGroup: (tabId, card, enabled) => moveProviderToStatusGroup(cards[tabId], card, enabled),
      appendCardToGroup: (tabId, card) => insertProviderToStatusGroup(cards[tabId], card),
    })

    providerForm.configure(original)
    await expect(providerForm.submitProviderModal(createForm({
      name: 'Broken Update',
      providerRef: 'Imported.Provider',
      enabled: false,
      opencodeNpm: '@ai-sdk/anthropic',
      opencodeSettingsConfig: { npm: '@ai-sdk/anthropic', models: { 'claude-3-5-sonnet-latest': { name: 'Claude 3.5 Sonnet' } } },
    }))).rejects.toThrow('save failed')

    expect(providerForm.providerModalState.open).toBe(true)
    expect(cards.opencode).toHaveLength(1)
    expect(cards.opencode[0]).toMatchObject({
      providerRef: 'Imported.Provider',
      name: 'Imported Provider',
      enabled: true,
      sortOrder: 1,
      enabledSortOrder: 1,
    })
    expect(window.dispatchEvent).not.toHaveBeenCalled()
  })

  it('restores provider order when status edit persistence fails', async () => {
    const cards = createCardRecord()
    const first = createCard(1, { enabled: true, sortOrder: 1, enabledSortOrder: 1 })
    const editing = createCard(2, { enabled: true, sortOrder: 2, enabledSortOrder: 2 })
    const third = createCard(3, { enabled: true, sortOrder: 3, enabledSortOrder: 3 })
    const disabled = createCard(4, { enabled: false, sortOrder: 1, disabledSortOrder: 1 })
    cards.codex.push(first, editing, third, disabled)
    const persistProviders = vi.fn().mockRejectedValue(new Error('save failed'))

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
      moveCardToStatusGroup: (tabId, card, enabled) => moveProviderToStatusGroup(cards[tabId], card, enabled),
      appendCardToGroup: (tabId, card) => insertProviderToStatusGroup(cards[tabId], card),
    })

    providerForm.configure(editing)
    await expect(providerForm.submitProviderModal(createForm({
      name: editing.name,
      apiUrl: editing.apiUrl,
      apiKey: editing.apiKey,
      enabled: false,
    }))).rejects.toThrow('save failed')

    expect(cards.codex.map((card) => card.id)).toEqual([1, 2, 3, 4])
    expect(cards.codex.map((card) => card.enabled)).toEqual([true, true, true, false])
    expect(cards.codex.map((card) => card.sortOrder)).toEqual([1, 2, 3, 1])
    expect(cards.codex.map((card) => card.enabledSortOrder ?? null)).toEqual([1, 2, 3, null])
    expect(cards.codex.map((card) => card.disabledSortOrder ?? null)).toEqual([null, null, null, 1])
    expect(toRaw(providerForm.providerModalState.card!)).toBe(cards.codex[1])
  })

  it('preserves hidden OpenCode request overrides and syncs live flags from enabled on edit', async () => {
    const cards = createCardRecord()
    const original = createCard(1, {
      providerRef: 'Imported.Provider',
      name: 'Imported Provider',
      enabled: true,
      liveConfigManaged: false,
      isInConfig: false,
      requestBodyOverrides: { metadata: { source: 'imported' } },
    })
    cards.opencode.push(original)
    const persistProviders = vi.fn().mockResolvedValue(undefined)

    const providerForm = useProviderForm({
      initialTab: 'opencode',
      t: (key: string) => key,
      showToast: vi.fn(),
      getActiveTab: () => 'opencode',
      cards,
      normalizeLevel,
      persistProviders,
      refreshDirectAppliedStatus: vi.fn().mockResolvedValue(undefined),
      removeProvider: vi.fn().mockResolvedValue(undefined),
      duplicateProvider: vi.fn().mockResolvedValue(false),
      reloadProviders: vi.fn().mockResolvedValue(undefined),
      moveCardToStatusGroup: (tabId, card, enabled) => moveProviderToStatusGroup(cards[tabId], card, enabled),
      appendCardToGroup: (tabId, card) => insertProviderToStatusGroup(cards[tabId], card),
    })

    providerForm.configure(original)
    await providerForm.submitProviderModal(createForm({
      name: 'Edited Provider',
      providerRef: 'Imported.Provider',
      enabled: true,
      requestBodyOverrides: { metadata: { source: 'imported' } },
      liveConfigManaged: false,
      isInConfig: false,
      opencodeNpm: '@ai-sdk/openai-compatible',
      opencodeSettingsConfig: { npm: '@ai-sdk/openai-compatible', models: { 'gpt-4o': { name: 'GPT-4o' } } },
    }))

    expect(cards.opencode[0]).toMatchObject({
      providerRef: 'Imported.Provider',
      name: 'Edited Provider',
      liveConfigManaged: true,
      isInConfig: true,
      requestBodyOverrides: { metadata: { source: 'imported' } },
    })
    expect(persistProviders).toHaveBeenCalledWith('opencode')
  })

  it('syncs OpenCode live flags when toggling enabled state', async () => {
    const cards = createCardRecord()
    const provider = createCard(1, {
      providerRef: 'Managed.Provider',
      enabled: true,
      liveConfigManaged: true,
      isInConfig: true,
      sortOrder: 1,
      enabledSortOrder: 1,
    })
    cards.opencode.push(provider)
    const persistProviders = vi.fn().mockResolvedValue(undefined)
    const providerForm = useProviderForm({
      initialTab: 'opencode',
      t: (key: string) => key,
      showToast: vi.fn(),
      getActiveTab: () => 'opencode',
      cards,
      normalizeLevel,
      persistProviders,
      refreshDirectAppliedStatus: vi.fn().mockResolvedValue(undefined),
      removeProvider: vi.fn().mockResolvedValue(undefined),
      duplicateProvider: vi.fn().mockResolvedValue(false),
      reloadProviders: vi.fn().mockResolvedValue(undefined),
      moveCardToStatusGroup: (tabId, card, enabled) => moveProviderToStatusGroup(cards[tabId], card, enabled),
      appendCardToGroup: (tabId, card) => insertProviderToStatusGroup(cards[tabId], card),
    })

    await providerForm.handleProviderEnabledChange(provider, false)

    expect(provider).toMatchObject({
      enabled: false,
      liveConfigManaged: false,
      isInConfig: false,
    })
    expect(persistProviders).toHaveBeenCalledWith('opencode')
  })

  it('moves provider to the start of enabled group when toggled on', async () => {
    const cards = createCardRecord()
    const first = createCard(1, { enabled: true, sortOrder: 1 })
    const second = createCard(2, { enabled: true, sortOrder: 2 })
    const third = createCard(3, { enabled: false, sortOrder: 1 })
    cards.codex.push(first, second, third)

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
      moveCardToStatusGroup: (tabId, card, enabled) => moveProviderToStatusGroup(cards[tabId], card, enabled),
      appendCardToGroup: (tabId, card) => insertProviderToStatusGroup(cards[tabId], card),
    })

    await providerForm.handleProviderEnabledChange(third, true)

    expect(cards.codex.map((card) => card.id)).toEqual([3, 1, 2])
    expect(cards.codex.map((card) => card.enabled)).toEqual([true, true, true])
    expect(cards.codex.map((card) => card.sortOrder)).toEqual([1, 2, 3])
    expect(persistProviders).toHaveBeenCalledWith('codex')
  })

  it('moves provider to the start of disabled group when toggled off again', async () => {
    const cards = createCardRecord()
    const first = createCard(1, { enabled: true, sortOrder: 1, enabledSortOrder: 1 })
    const second = createCard(2, { enabled: false, sortOrder: 1, disabledSortOrder: 1 })
    const third = createCard(3, { enabled: false, sortOrder: 2, disabledSortOrder: 2 })
    const fourth = createCard(4, { enabled: false, sortOrder: 3, disabledSortOrder: 3 })
    cards.codex.push(first, second, third, fourth)

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
      moveCardToStatusGroup: (tabId, card, enabled) => moveProviderToStatusGroup(cards[tabId], card, enabled),
      appendCardToGroup: (tabId, card) => insertProviderToStatusGroup(cards[tabId], card),
    })

    await providerForm.handleProviderEnabledChange(third, true)
    expect(cards.codex.map((card) => card.id)).toEqual([3, 1, 2, 4])

    await providerForm.handleProviderEnabledChange(third, false)

    expect(cards.codex.map((card) => card.id)).toEqual([1, 3, 2, 4])
    expect(cards.codex.map((card) => card.disabledSortOrder ?? null)).toEqual([null, 1, 2, 3])
    expect(persistProviders).toHaveBeenCalledTimes(2)
  })

  it('saves claude providers with transformed apiFormat without attempting direct apply', async () => {
    const cards = createCardRecord()
    const showToast = vi.fn()
    const persistProviders = vi.fn().mockResolvedValue(undefined)
    const refreshDirectAppliedStatus = vi.fn().mockResolvedValue(undefined)
    const providerForm = useProviderForm({
      initialTab: 'claude',
      t: (key: string) => key,
      showToast,
      getActiveTab: () => 'claude',
      cards,
      normalizeLevel,
      persistProviders,
      refreshDirectAppliedStatus,
      removeProvider: vi.fn().mockResolvedValue(undefined),
      duplicateProvider: vi.fn().mockResolvedValue(false),
      reloadProviders: vi.fn().mockResolvedValue(undefined),
      moveCardToStatusGroup: (tabId, card, enabled) => moveProviderToStatusGroup(cards[tabId], card, enabled),
      appendCardToGroup: (tabId, card) => insertProviderToStatusGroup(cards[tabId], card),
    })

    vi.spyOn(Date, 'now').mockReturnValue(999)
    providerForm.openCreateModal()

    await providerForm.submitAndApplyProviderModal(createForm({
      name: 'Claude Routed Provider',
      apiFormat: 'openai_responses',
      apiKey: 'claude-key',
      modelMapping: { 'claude-*': 'vendor-*' },
      modelMappingSupports1M: { 'claude-*': true },
    }))

    expect(persistProviders).toHaveBeenCalledWith('claude')
    expect(vi.mocked(Call.ByName)).not.toHaveBeenCalled()
    expect(refreshDirectAppliedStatus).not.toHaveBeenCalled()
    expect(showToast).toHaveBeenCalledWith('components.main.directApply.requiresHostedRouting', 'warning')
    expect(cards.claude[0]?.apiFormat).toBe('openai_responses')
    expect(cards.claude[0]?.modelMappingSupports1M).toEqual({ 'claude-*': true })
  })

  it('saves claude providers with custom auth headers without attempting direct apply', async () => {
    const cards = createCardRecord()
    const showToast = vi.fn()
    const persistProviders = vi.fn().mockResolvedValue(undefined)
    const refreshDirectAppliedStatus = vi.fn().mockResolvedValue(undefined)
    const providerForm = useProviderForm({
      initialTab: 'claude',
      t: (key: string) => key,
      showToast,
      getActiveTab: () => 'claude',
      cards,
      normalizeLevel,
      persistProviders,
      refreshDirectAppliedStatus,
      removeProvider: vi.fn().mockResolvedValue(undefined),
      duplicateProvider: vi.fn().mockResolvedValue(false),
      reloadProviders: vi.fn().mockResolvedValue(undefined),
      moveCardToStatusGroup: (tabId, card, enabled) => moveProviderToStatusGroup(cards[tabId], card, enabled),
      appendCardToGroup: (tabId, card) => insertProviderToStatusGroup(cards[tabId], card),
    })

    vi.spyOn(Date, 'now').mockReturnValue(1000)
    providerForm.openCreateModal()

    await providerForm.submitAndApplyProviderModal(createForm({
      name: 'Claude Custom Auth',
      apiFormat: 'anthropic',
      apiKey: 'claude-key',
      connectivityAuthType: 'X-Custom-Auth',
    }))

    expect(persistProviders).toHaveBeenCalledWith('claude')
    expect(vi.mocked(Call.ByName)).not.toHaveBeenCalled()
    expect(refreshDirectAppliedStatus).not.toHaveBeenCalled()
    expect(showToast).toHaveBeenCalledWith('components.main.directApply.requiresHostedRouting', 'warning')
    expect(cards.claude[0]?.connectivityAuthType).toBe('X-Custom-Auth')
  })

  it('saves quota-auto-disabled providers without attempting direct apply', async () => {
    const cards = createCardRecord()
    const card = createCard(1001, {
      enabled: false,
      quotaAutoDisabled: true,
      providerQuotaQueryType: 'balance',
    })
    cards.codex.push(card)
    const showToast = vi.fn()
    const persistProviders = vi.fn().mockResolvedValue(undefined)
    const refreshDirectAppliedStatus = vi.fn().mockResolvedValue(undefined)
    const providerForm = useProviderForm({
      initialTab: 'codex',
      t: (key: string) => key,
      showToast,
      getActiveTab: () => 'codex',
      cards,
      normalizeLevel,
      persistProviders,
      refreshDirectAppliedStatus,
      removeProvider: vi.fn().mockResolvedValue(undefined),
      duplicateProvider: vi.fn().mockResolvedValue(false),
      reloadProviders: vi.fn().mockResolvedValue(undefined),
      moveCardToStatusGroup: (tabId, target, enabled) => moveProviderToStatusGroup(cards[tabId], target, enabled),
      appendCardToGroup: (tabId, target) => insertProviderToStatusGroup(cards[tabId], target),
    })

    providerForm.configure(card)
    await providerForm.submitAndApplyProviderModal(createForm({
      name: card.name,
      apiUrl: card.apiUrl,
      apiKey: card.apiKey,
      enabled: true,
      quotaAutoDisabled: true,
      providerQuotaQueryType: 'balance',
    }))

    expect(persistProviders).toHaveBeenCalledWith('codex')
    expect(card).toMatchObject({ enabled: false, quotaAutoDisabled: true })
    expect(vi.mocked(Call.ByName)).not.toHaveBeenCalled()
    expect(refreshDirectAppliedStatus).not.toHaveBeenCalled()
    expect(showToast).toHaveBeenCalledWith('components.main.providers.quotaAutoDisabledHint', 'warning')
  })

  it('persists Anthropic cache TTL only for native Claude providers', async () => {
    const cards = createCardRecord()
    const persistProviders = vi.fn().mockResolvedValue(undefined)
    const providerForm = useProviderForm({
      initialTab: 'claude',
      t: (key: string) => key,
      showToast: vi.fn(),
      getActiveTab: () => 'claude',
      cards,
      normalizeLevel,
      persistProviders,
      refreshDirectAppliedStatus: vi.fn().mockResolvedValue(undefined),
      removeProvider: vi.fn().mockResolvedValue(undefined),
      duplicateProvider: vi.fn().mockResolvedValue(false),
      reloadProviders: vi.fn().mockResolvedValue(undefined),
      moveCardToStatusGroup: (tabId, card, enabled) => moveProviderToStatusGroup(cards[tabId], card, enabled),
      appendCardToGroup: (tabId, card) => insertProviderToStatusGroup(cards[tabId], card),
    })

    vi.spyOn(Date, 'now').mockReturnValue(1001)
    providerForm.openCreateModal()
    await providerForm.submitProviderModal(createForm({
      apiFormat: 'anthropic',
      anthropicCacheTTL: '1h',
    }))

    expect(cards.claude[0]?.anthropicCacheTTL).toBe('1h')

    vi.spyOn(Date, 'now').mockReturnValue(1002)
    providerForm.openCreateModal()
    await providerForm.submitProviderModal(createForm({
      name: 'OpenAI-style Claude',
      apiFormat: 'openai_chat',
      anthropicCacheTTL: '1h',
    }))

    expect(cards.claude[1]?.apiFormat).toBe('openai_chat')
    expect(cards.claude[1]?.anthropicCacheTTL).toBe('')
  })

  it('persists only the selected model mapping disabled state immediately', async () => {
    const cards = createCardRecord()
    const card = createCard(1, {
      modelMapping: { 'claude-*': 'vendor-*' },
      modelMappingDisabled: {},
    })
    cards.claude.push(card)
    const persistProviders = vi.fn().mockResolvedValue(undefined)
    const providerForm = useProviderForm({
      initialTab: 'claude',
      t: (key: string) => key,
      showToast: vi.fn(),
      getActiveTab: () => 'claude',
      cards,
      normalizeLevel,
      persistProviders,
      refreshDirectAppliedStatus: vi.fn().mockResolvedValue(undefined),
      removeProvider: vi.fn().mockResolvedValue(undefined),
      duplicateProvider: vi.fn().mockResolvedValue(false),
      reloadProviders: vi.fn().mockResolvedValue(undefined),
      moveCardToStatusGroup: (tabId, target, enabled) => moveProviderToStatusGroup(cards[tabId], target, enabled),
      appendCardToGroup: (tabId, target) => insertProviderToStatusGroup(cards[tabId], target),
    })

    providerForm.openEditModal(card)
    await providerForm.persistModelMappingRuleEnabled('claude-*', false)

    expect(card.modelMappingDisabled).toEqual({ 'claude-*': true })
    expect(persistProviders).toHaveBeenCalledWith('claude')

    await providerForm.persistModelMappingRuleEnabled('claude-*', true)
    expect(card.modelMappingDisabled).toEqual({})
  })

  it('rolls back model mapping disabled state when immediate persistence fails', async () => {
    const cards = createCardRecord()
    const card = createCard(1, {
      modelMapping: { 'claude-*': 'vendor-*' },
      modelMappingDisabled: {},
    })
    cards.claude.push(card)
    const persistProviders = vi.fn().mockRejectedValue(new Error('save failed'))
    const providerForm = useProviderForm({
      initialTab: 'claude',
      t: (key: string) => key,
      showToast: vi.fn(),
      getActiveTab: () => 'claude',
      cards,
      normalizeLevel,
      persistProviders,
      refreshDirectAppliedStatus: vi.fn().mockResolvedValue(undefined),
      removeProvider: vi.fn().mockResolvedValue(undefined),
      duplicateProvider: vi.fn().mockResolvedValue(false),
      reloadProviders: vi.fn().mockResolvedValue(undefined),
      moveCardToStatusGroup: (tabId, target, enabled) => moveProviderToStatusGroup(cards[tabId], target, enabled),
      appendCardToGroup: (tabId, target) => insertProviderToStatusGroup(cards[tabId], target),
    })

    providerForm.openEditModal(card)
    await expect(providerForm.persistModelMappingRuleEnabled('claude-*', false)).rejects.toThrow('save failed')
    expect(card.modelMappingDisabled).toEqual({})
  })

  it('persists the current provider log badge preference immediately', async () => {
    const cards = createCardRecord()
    const card = createCard(1)
    cards.codex.push(card)
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
      moveCardToStatusGroup: (tabId, target, enabled) => moveProviderToStatusGroup(cards[tabId], target, enabled),
      appendCardToGroup: (tabId, target) => insertProviderToStatusGroup(cards[tabId], target),
    })

    providerForm.openProviderLogs(card)
    await providerForm.updateProviderLogBadgeEnabled(false)

    expect(card.hideLogBadge).toBe(true)
    expect(persistProviders).toHaveBeenCalledWith('codex')
    expect(providerForm.providerLogBadgeSaving.value).toBe(false)
  })

  it('rolls back the log badge preference when immediate persistence fails', async () => {
    const cards = createCardRecord()
    const card = createCard(1, { hideLogBadge: false })
    cards.gemini.push(card)
    const persistProviders = vi.fn().mockRejectedValue(new Error('save failed'))
    const providerForm = useProviderForm({
      initialTab: 'gemini',
      t: (key: string) => key,
      showToast: vi.fn(),
      getActiveTab: () => 'gemini',
      cards,
      normalizeLevel,
      persistProviders,
      refreshDirectAppliedStatus: vi.fn().mockResolvedValue(undefined),
      removeProvider: vi.fn().mockResolvedValue(undefined),
      duplicateProvider: vi.fn().mockResolvedValue(false),
      reloadProviders: vi.fn().mockResolvedValue(undefined),
      moveCardToStatusGroup: (tabId, target, enabled) => moveProviderToStatusGroup(cards[tabId], target, enabled),
      appendCardToGroup: (tabId, target) => insertProviderToStatusGroup(cards[tabId], target),
    })

    providerForm.openProviderLogs(card)
    await providerForm.updateProviderLogBadgeEnabled(false)

    expect(card.hideLogBadge).toBe(false)
    expect(providerForm.providerLogBadgeSaving.value).toBe(false)
  })
})

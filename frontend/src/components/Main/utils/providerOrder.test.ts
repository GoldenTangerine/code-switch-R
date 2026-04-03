import { describe, expect, it } from 'vitest'
import type { AutomationCard } from '../../../data/cards'
import { applyNormalizedProviderOrder, commitProviderOrder } from './providerOrder'

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
  sortOrder: id,
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

describe('providerOrder', () => {
  it('loads persisted provider order with enabled group first and group-local sort order', () => {
    const list = [
      createCard(4, { enabled: false, sortOrder: 2 }),
      createCard(2, { enabled: true, sortOrder: 2 }),
      createCard(1, { enabled: true, sortOrder: 1 }),
      createCard(3, { enabled: false, sortOrder: 1 }),
    ]

    applyNormalizedProviderOrder(list)

    expect(list.map((card) => card.id)).toEqual([1, 2, 3, 4])
    expect(list.map((card) => card.sortOrder)).toEqual([1, 2, 1, 2])
  })

  it('commits the current visual order instead of re-sorting by stale sortOrder', () => {
    const list = [
      createCard(2, { enabled: true, sortOrder: 2 }),
      createCard(3, { enabled: true, sortOrder: 3 }),
      createCard(1, { enabled: true, sortOrder: 1 }),
      createCard(5, { enabled: false, sortOrder: 2 }),
      createCard(4, { enabled: false, sortOrder: 1 }),
    ]

    commitProviderOrder(list)

    expect(list.map((card) => card.id)).toEqual([2, 3, 1, 5, 4])
    expect(list.map((card) => card.sortOrder)).toEqual([1, 2, 3, 1, 2])
  })
})

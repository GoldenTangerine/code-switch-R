import { describe, expect, it } from 'vitest'
import type { AutomationCard } from '../../../data/cards'
import {
  insertProviderToStatusGroup,
  applyNormalizedProviderOrder,
  commitProviderOrder,
  moveProviderToStatusGroup,
} from './providerOrder'

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
      createCard(4, { enabled: false, sortOrder: 2, disabledSortOrder: 2 }),
      createCard(2, { enabled: true, sortOrder: 2, enabledSortOrder: 2 }),
      createCard(1, { enabled: true, sortOrder: 1, enabledSortOrder: 1 }),
      createCard(3, { enabled: false, sortOrder: 1, disabledSortOrder: 1 }),
    ]

    applyNormalizedProviderOrder(list)

    expect(list.map((card) => card.id)).toEqual([1, 2, 3, 4])
    expect(list.map((card) => card.sortOrder)).toEqual([1, 2, 1, 2])
    expect(list.map((card) => card.enabledSortOrder ?? null)).toEqual([1, 2, null, null])
    expect(list.map((card) => card.disabledSortOrder ?? null)).toEqual([null, null, 1, 2])
  })

  it('commits the current visual order instead of re-sorting by stale sortOrder', () => {
    const list = [
      createCard(2, { enabled: true, sortOrder: 2, enabledSortOrder: 2 }),
      createCard(3, { enabled: true, sortOrder: 3, enabledSortOrder: 3, disabledSortOrder: 2 }),
      createCard(1, { enabled: true, sortOrder: 1, enabledSortOrder: 1 }),
      createCard(5, { enabled: false, sortOrder: 2, disabledSortOrder: 2 }),
      createCard(4, { enabled: false, sortOrder: 1, disabledSortOrder: 1 }),
    ]

    commitProviderOrder(list)

    expect(list.map((card) => card.id)).toEqual([2, 3, 1, 5, 4])
    expect(list.map((card) => card.sortOrder)).toEqual([1, 2, 3, 1, 2])
    expect(list[1]?.disabledSortOrder).toBe(2)
  })

  it('restores previous disabled position when toggled back off', () => {
    const list = [
      createCard(1, { enabled: true, sortOrder: 1, enabledSortOrder: 1 }),
      createCard(2, { enabled: false, sortOrder: 1, disabledSortOrder: 1 }),
      createCard(3, { enabled: false, sortOrder: 2, disabledSortOrder: 2 }),
      createCard(4, { enabled: false, sortOrder: 3, disabledSortOrder: 3 }),
    ]

    expect(moveProviderToStatusGroup(list, list[2]!, true)).toBe(true)
    expect(list.map((card) => card.id)).toEqual([1, 3, 2, 4])

    expect(moveProviderToStatusGroup(list, list[1]!, false)).toBe(true)
    expect(list.map((card) => card.id)).toEqual([1, 2, 3, 4])
    expect(list.map((card) => card.disabledSortOrder ?? null)).toEqual([null, 1, 2, 3])
  })

  it('prepends new disabled providers to the top of disabled group', () => {
    const list = [
      createCard(1, { enabled: true, sortOrder: 1, enabledSortOrder: 1 }),
      createCard(2, { enabled: false, sortOrder: 1, disabledSortOrder: 1 }),
      createCard(3, { enabled: false, sortOrder: 2, disabledSortOrder: 2 }),
    ]

    const newCard = createCard(4, {
      enabled: false,
      sortOrder: 99,
      disabledSortOrder: 99,
    })

    insertProviderToStatusGroup(list, newCard)

    expect(list.map((card) => card.id)).toEqual([1, 4, 2, 3])
    expect(list.map((card) => card.disabledSortOrder ?? null)).toEqual([null, 1, 2, 3])
    expect(list[1]?.sortOrder).toBe(1)
  })
})

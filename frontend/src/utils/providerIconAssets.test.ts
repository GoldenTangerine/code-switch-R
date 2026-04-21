import { describe, expect, it } from 'vitest'
import {
  buildProviderIconOptionKeys,
  collectProviderDisplayIconKeys,
  resolveProviderDisplayIconKey,
} from './providerIconAssets'

describe('providerIconAssets', () => {
  it.each([
    ['claude', 'claude-color'],
    ['azure', 'azure-color'],
    ['gemini', 'gemini-color'],
    ['fireworks', 'fireworks-color'],
    ['volcengine', 'doubao-color'],
    ['qwen', 'qwen-color'],
  ])('should prefer colored asset for %s -> %s', (iconKey, expected) => {
    expect(resolveProviderDisplayIconKey(iconKey)).toBe(expected)
  })

  it.each([
    ['openai', 'openai'],
    ['grok', 'grok'],
    ['groq', 'groq'],
    ['openrouter', 'openrouter'],
    ['ollama', 'ollama'],
    ['moonshot', 'moonshot'],
  ])('should keep monochrome asset when no color variant exists for %s', (iconKey, expected) => {
    expect(resolveProviderDisplayIconKey(iconKey)).toBe(expected)
  })

  it('should collect unique display icon keys', () => {
    expect(collectProviderDisplayIconKeys([
      'azure',
      'Azure',
      'volcengine',
      'openrouter',
      'azure',
      'openrouter',
    ])).toEqual([
      'azure-color',
      'doubao-color',
      'openrouter',
    ])
  })

  it('should place canonical provider icons before the generic icon list', () => {
    expect(buildProviderIconOptionKeys([
      'zhipu',
      'aicoding',
      'openrouter',
      'azure',
      'claude',
      'volcengine',
      'gemini-color',
    ]).slice(0, 4)).toEqual([
      'claude',
      'azure',
      'zhipu',
      'volcengine',
    ])
  })
})

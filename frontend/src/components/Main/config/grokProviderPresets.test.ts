import { describe, expect, it } from 'vitest'
import {
  buildGrokPresetConfigTOML,
  grokProviderPresets,
  syncGrokCredentialsIntoTOML,
  validateGrokConfigTOML,
} from './grokProviderPresets'

describe('grokProviderPresets TOML 工具', () => {
  const xai = grokProviderPresets.find((preset) => preset.profile === 'xai')!
  const toml = buildGrokPresetConfigTOML(xai)

  it('预设按后端字段顺序生成完整 TOML', () => {
    expect(toml).toContain('[models]\ndefault = "xai"')
    expect(toml).toContain('[model.xai]')
    expect(toml).toContain('base_url = "https://api.x.ai/v1"')
    expect(toml).toContain('api_backend = "responses"')
    expect(toml).toContain('context_window = 2000000')
  })

  it('官方预设生成空 TOML', () => {
    const official = grokProviderPresets.find((preset) => preset.isOfficial)!
    expect(buildGrokPresetConfigTOML(official)).toBe('')
  })

  it('同步凭据：API Key 注入、替换、转义、空值不动', () => {
    const withKey = syncGrokCredentialsIntoTOML(toml, 'sk-abc', '')
    expect(withKey).toContain('api_key = "sk-abc"')
    expect((withKey.match(/api_key/g) ?? []).length).toBe(1)

    const reKey = syncGrokCredentialsIntoTOML(withKey, 'sk-xyz"1', '')
    expect((reKey.match(/api_key/g) ?? []).length).toBe(1)
    expect(reKey).toContain('api_key = "sk-xyz\\"1"')

    expect(syncGrokCredentialsIntoTOML(withKey, '', '')).toBe(withKey)
  })

  it('同步 baseUrl：原位替换已有行，空值不动', () => {
    const synced = syncGrokCredentialsIntoTOML(toml, '', 'https://proxy.example.com/v1')
    expect(synced).toContain('base_url = "https://proxy.example.com/v1"')
    expect((synced.match(/base_url/g) ?? []).length).toBe(1)
    expect(synced).toContain('base_url = "https://proxy.example.com/v1"')
    expect(synced).not.toContain('https://api.x.ai/v1')

    expect(syncGrokCredentialsIntoTOML(toml, '', '')).toBe(toml)
  })

  it('同步 baseUrl：缺失 base_url 行时追加到选中表内', () => {
    const withoutBaseUrl = [
      '[models]',
      'default = "xai"',
      '',
      '[model.xai]',
      'model = "grok-4-fast"',
      'name = "xAI 官方直连"',
      '',
    ].join('\n')
    const synced = syncGrokCredentialsIntoTOML(withoutBaseUrl, '', 'https://api.x.ai/v1')
    expect(synced.split('[model.xai]')[1]).toContain('base_url = "https://api.x.ai/v1"')
    expect(synced.split('[model.xai]')[1]).toContain('model = "grok-4-fast"')
  })

  it('多表场景仅写入 [models].default 指向的表', () => {
    const multi = [
      '[models]',
      'default = "b"',
      '',
      '[model.a]',
      'model = "m1"',
      'base_url = "u1"',
      '',
      '[model.b]',
      'model = "m2"',
      'base_url = "u2"',
      '',
    ].join('\n')
    const synced = syncGrokCredentialsIntoTOML(multi, 'sk-multi', 'u2-new')
    expect(synced.split('[model.b]')[1]).toContain('api_key = "sk-multi"')
    expect(synced.split('[model.b]')[1]).toContain('base_url = "u2-new"')
    expect(synced.split('[model.b]')[0]).not.toContain('api_key')
    expect(synced.split('[model.b]')[0]).not.toContain('u2-new')
  })

  it('校验：空 TOML 合法、缺 model 表报错', () => {
    expect(validateGrokConfigTOML('')).toBe(true)
    expect(validateGrokConfigTOML(toml)).toBe(true)
    expect(validateGrokConfigTOML('[foo]\nbar = 1')).toBe(false)
  })
})

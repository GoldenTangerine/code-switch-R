/**
 * @name: 供应商工具兼容界面测试
 * @Descripttion: 验证规则输入、预设合并及配置保存回读。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-10-24 23:20:14
 * @LastEditTime: 2026-10-24 23:20:14
 * @FilePath: frontend/src/components/Main/components/ProviderToolCompatibilityEditor.test.ts
 */
// @vitest-environment happy-dom
import { createApp, h, nextTick, ref } from 'vue'
import { createI18n } from 'vue-i18n'
import { describe, expect, it, vi } from 'vitest'
import type { ProviderToolCompatibility } from '../../../data/cards'
import zh from '../../../locales/zh.json'
import en from '../../../locales/en.json'
import ProviderToolCompatibilityEditor from './ProviderToolCompatibilityEditor.vue'
import { createDefaultVendorForm, buildNormalizedVendorForm, buildPersistedProviderFieldsFromForm, createVendorFormFromCard } from '../adapters/providerFormMappers'
import { providerToCard, serializeProviders, type PersistedProvider } from '../adapters/providerCardMappers'

vi.mock('@wailsio/runtime', () => ({ Call: { ByName: vi.fn() }, Create: { Array: () => (value: unknown) => value, Map: () => (value: unknown) => value, Any: (value: unknown) => value } }))

describe('provider tool compatibility editor', () => {
  it.each(['zh', 'en'])('edits rules and merges the Advisor preset (%s)', async (locale) => {
    localStorage.setItem('theme', 'dark')
    const value = ref<ProviderToolCompatibility>({ disabledNames: ['WebSearch'] })
    const root = document.createElement('div')
    document.body.append(root)
    const app = createApp({ render: () => h(ProviderToolCompatibilityEditor, {
      modelValue: value.value,
      'onUpdate:modelValue': (next: ProviderToolCompatibility) => { value.value = next },
    }) })
    app.use(createI18n({ legacy: false, locale, messages: { zh, en } }))
    app.mount(root)
    try {
      expect(root.querySelector('.theme-dark')).not.toBeNull()
      const names = root.querySelectorAll('textarea')[0]!
      names.value = 'WebSearch, Read\nWebSearch\n'
      names.dispatchEvent(new Event('input', { bubbles: true }))
      await nextTick()
      expect(value.value.disabledNames).toEqual(['WebSearch', 'Read'])
      expect(names.value.endsWith('\n')).toBe(true)
      root.querySelector('button')!.click()
      await nextTick()
      expect(value.value).toEqual({ disabledNames: ['WebSearch', 'Read', 'advisor'], disabledTypes: ['advisor_20260301'], normalizeDynamicTools: true })
      root.querySelector('button')!.click()
      await nextTick()
      expect(value.value.disabledNames).toHaveLength(3)
      const toggle = root.querySelector('input[type="checkbox"]') as HTMLInputElement
      toggle.checked = false
      toggle.dispatchEvent(new Event('change', { bubbles: true }))
      await nextTick()
      expect(value.value.normalizeDynamicTools).toBe(false)
      names.value = ''
      names.dispatchEvent(new Event('input', { bubbles: true }))
      await nextTick()
      expect(value.value.disabledNames).toEqual([])
      expect(root.textContent).not.toContain('components.main.form.toolCompatibility')
    } finally { app.unmount(); root.remove() }
  })

  it('retains independently cloned rules through form, card, save and reload', () => {
    const form = createDefaultVendorForm('claude', 'kimi')
    expect(form.toolCompatibility).toEqual({})
    form.toolCompatibility = { disabledNames: ['advisor'], disabledTypes: ['advisor_20260301'], normalizeDynamicTools: true }
    const normalized = buildNormalizedVendorForm({ form, tabId: 'claude', defaultIconKey: 'kimi', resolveAuthType: () => 'bearer' })
    const fields = buildPersistedProviderFieldsFromForm(normalized, 'claude', () => 1)
    const provider = { ...fields, id: 1, name: 'kimi', apiUrl: 'https://example.com', tint: '', accent: '' } as PersistedProvider
    const card = providerToCard(provider, 'claude')
    const saved = serializeProviders([card], 'claude')
    const reloaded = providerToCard(JSON.parse(JSON.stringify(saved[0])), 'claude')
    const reopened = createVendorFormFromCard(reloaded, 'claude')
    expect(reopened.toolCompatibility).toEqual(form.toolCompatibility)
    reopened.toolCompatibility!.disabledNames!.push('Read')
    expect(form.toolCompatibility.disabledNames).toEqual(['advisor'])
    expect(card.toolCompatibility!.disabledNames).toEqual(['advisor'])
  })
})

// @vitest-environment happy-dom
/**
 * @name: 云端计费规则展示测试
 * @Descripttion: 验证轨道顺序、边界、字段倍率和空规则展示。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-09-07 11:17:00
 * @LastEditTime: 2026-09-07 11:17:00
 * @FilePath: frontend/src/components/Setting/CloudPricingRulesSummary.test.ts
 */
import { createApp, createSSRApp, nextTick } from 'vue'
import { createI18n } from 'vue-i18n'
import { renderToString } from 'vue/server-renderer'
import { describe, expect, it } from 'vitest'
import zh from '../../locales/zh.json'
import type { CloudPricingRules, CloudPriceTableConflictPricing } from '../../services/modelPricing'
import CloudPricingRulesSummary from './CloudPricingRulesSummary.vue'
import CloudPricingConflictModal from './CloudPricingConflictModal.vue'

async function render(rules?: CloudPricingRules) {
  const app = createSSRApp(CloudPricingRulesSummary, { rules })
  app.use(createI18n({ legacy: false, locale: 'zh', messages: { zh } }))
  return renderToString(app)
}

describe('cloud pricing rules summary', () => {
  it('does not expose translation keys for unsupported CPT charges', async () => {
    const html = await render({ charges: { prompt: 0.0000025, completion: 0.000015 }, tracks: [
      { label: 'Priority', factor: 1, triggers: [], charge_factors: { prompt: 2, completion: 2, web_search: 1, file_search_call: 1, image_input: 2, future_charge: 3 } },
    ] })
    expect(html).toContain('输入 ×2')
    expect(html).toContain('输出 ×2')
    expect(html).not.toContain('pricingRules.')
    expect(html).not.toContain('web_search')
    expect(html).not.toContain('future_charge')
  })
  it('renders ordered conditions and explicit zero multipliers', async () => {
    const html = await render({ charges: { prompt: 0.000001 }, tracks: [
      { label: 'priority', factor: 2, triggers: [
        { kind: 'input_tokens_above', threshold: 128000, inclusive: true },
        { kind: 'body_matches', field: 'service_tier', pattern: '^priority$' },
      ], charge_factors: { cache_read: 0 } },
      { label: 'standard', factor: 1, triggers: [] },
    ] })
    expect(html).toContain('计费规则（2）')
    expect(html).toContain('≥ 128,000')
    expect(html).toContain('服务档位')
    expect(html).toContain('缓存读取 ×0')
    expect(html.indexOf('priority')).toBeLessThan(html.indexOf('standard'))
  })

  it('omits an empty disclosure for legacy pricing', async () => {
    expect(await render()).not.toContain('<details')
    expect(await render({ charges: {}, tracks: [] })).not.toContain('<details')
  })

  it('shows both current and incoming rules for a track-only conflict', async () => {
    const prices: CloudPriceTableConflictPricing = {
      input_cost_per_token: 0.000001, output_cost_per_token: 0.000002,
      output_cost_per_reasoning_token: 0, cache_creation_input_token_cost: 0,
      cache_read_input_token_cost: 0, ephemeral_1h_cost_per_token: 0, group_multiplier: 1,
    }
    const app = createApp(CloudPricingConflictModal, {
      open: true, fetchedAt: '', syncing: false,
      rows: [{ model: 'example-model', current: {
        ...prices, cloud_pricing: { charges: {}, tracks: [{ label: 'old-rule', factor: 1, triggers: [] }] },
      }, incoming: {
        ...prices, cloud_pricing: { charges: {}, tracks: [{ label: 'new-rule', factor: 2, triggers: [] }] },
      } }],
    })
    app.use(createI18n({ legacy: false, locale: 'zh', messages: { zh } }))
    const container = document.createElement('div')
    document.body.append(container)
    try {
      app.mount(container)
      await nextTick()
      expect(document.body.textContent).toContain('old-rule')
      expect(document.body.textContent).toContain('new-rule')
    } finally {
      app.unmount()
      container.remove()
    }
  })
})

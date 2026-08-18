/**
 * @name: 会话切换样式回归测试
 * @Descripttion: 验证候选供应商卡片不受全局按钮布局覆盖。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-18 15:35:58
 * @LastEditTime: 2026-08-18 15:35:58
 * @FilePath: frontend/src/components/Main/modals/providerConcurrencySwitchStyles.test.ts
 */

import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const componentSource = readFileSync(new URL('./ProviderConcurrencyDetailsModal.vue', import.meta.url), 'utf8')
const globalStyleSource = readFileSync(new URL('../../../style.css', import.meta.url), 'utf8')

function globalButtonRule() {
  const start = globalStyleSource.indexOf('button:where(')
  const end = globalStyleSource.indexOf('}', start)
  return globalStyleSource.slice(start, end + 1)
}

describe('provider concurrency switch styles', () => {
  it('keeps custom switch controls out of the global button reset', () => {
    const rule = globalButtonRule()

    expect(rule).toContain(':not(.provider-concurrency-row__switch)')
    expect(rule).toContain(':not(.provider-concurrency-switch-option)')
  })

  it('keeps long candidate text constrained inside the card', () => {
    expect(componentSource).toContain('grid-template-columns: repeat(auto-fit, minmax(min(180px, 100%), 1fr));')
    expect(componentSource).toContain('overflow-wrap: anywhere;')
    expect(componentSource).toContain('white-space: normal;')
    expect(componentSource).not.toContain('opacity: 0.48;')
  })
})

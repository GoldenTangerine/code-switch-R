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
import { parse as parseSfc } from 'vue/compiler-sfc'
import { describe, expect, it } from 'vitest'

const componentSource = readFileSync(new URL('./ProviderConcurrencyDetailsModal.vue', import.meta.url), 'utf8')
const globalStyleSource = readFileSync(new URL('../../../style.css', import.meta.url), 'utf8')
const parsedComponent = parseSfc(componentSource, { filename: 'ProviderConcurrencyDetailsModal.vue' })
const componentStyleSource = parsedComponent.descriptor.styles.map((style) => style.content).join('\n')

type TemplateExpression = {
  content?: string
}

type TemplateProp = {
  name?: string
  value?: TemplateExpression
  arg?: TemplateExpression
  modifiers?: TemplateExpression[]
}

type TemplateNode = {
  tag?: string
  props?: TemplateProp[]
  children?: TemplateNode[]
}

const templateRoot = parsedComponent.descriptor.template?.ast as unknown as TemplateNode

function staticClasses(node: TemplateNode) {
  const classAttribute = node.props?.find((prop) => prop.name === 'class' && prop.value?.content)
  return classAttribute?.value?.content?.split(/\s+/).filter(Boolean) ?? []
}

function findElementByClass(node: TemplateNode, className: string): TemplateNode | undefined {
  if (staticClasses(node).includes(className)) return node
  for (const child of node.children ?? []) {
    const match = findElementByClass(child, className)
    if (match) return match
  }
  return undefined
}

function requireElementByClass(node: TemplateNode, className: string): TemplateNode {
  const element = findElementByClass(node, className)
  if (element) return element
  throw new Error(`Template element not found: ${className}`)
}

function directives(node: TemplateNode, name: string, argument?: string) {
  return (node.props ?? []).filter((prop) => (
    prop.name === name
    && (argument === undefined || prop.arg?.content === argument)
  ))
}

function hasAttribute(node: TemplateNode, name: string) {
  return (node.props ?? []).some((prop) => (
    prop.name === name
    || (prop.name === 'bind' && prop.arg?.content === name)
  ))
}

function ruleSource(styleSource: string, selector: string) {
  const start = styleSource.indexOf(selector)
  const open = styleSource.indexOf('{', start)
  const close = styleSource.indexOf('}', open)
  return start >= 0 && open >= 0 && close >= 0 ? styleSource.slice(start, close + 1) : ''
}

function globalButtonRule() {
  const start = globalStyleSource.indexOf('button:where(')
  const end = globalStyleSource.indexOf('}', start)
  return globalStyleSource.slice(start, end + 1)
}

describe('provider concurrency switch styles', () => {
  it('parses the component template without errors', () => {
    expect(parsedComponent.errors).toEqual([])
    expect(templateRoot).toBeDefined()
  })

  it('keeps custom switch controls out of the global button reset', () => {
    const rule = globalButtonRule()

    expect(rule).toContain(':not(.provider-concurrency-row__switch)')
    expect(rule).toContain(':not(.provider-concurrency-switch-option)')
  })

  it('keeps long candidate text constrained inside the card', () => {
    const gridRule = ruleSource(componentStyleSource, '.provider-concurrency-switch-panel__grid')
    const contentRule = ruleSource(
      componentStyleSource,
      '.provider-concurrency-switch-option > span,\n.provider-concurrency-switch-option > small',
    )

    expect(gridRule).toContain('grid-template-columns: repeat(auto-fit, minmax(min(180px, 100%), 1fr));')
    expect(contentRule).toContain('overflow-wrap: anywhere;')
    expect(contentRule).toContain('white-space: normal;')
    expect(componentStyleSource).not.toContain('opacity: 0.48;')
  })

  it('keeps session controls on the model row without crowding the endpoint', () => {
    const main = requireElementByClass(templateRoot, 'provider-concurrency-row__main')
    const content = requireElementByClass(main, 'provider-concurrency-row__content')
    const sessionActions = requireElementByClass(main, 'provider-concurrency-row__session-actions')
    const mainRule = ruleSource(componentStyleSource, '.provider-concurrency-row__main')
    const sessionActionsRule = ruleSource(componentStyleSource, '.provider-concurrency-row__session-actions')
    const contextRule = ruleSource(componentStyleSource, '.provider-concurrency-row__context')

    expect(content).toBeDefined()
    expect(sessionActions).toBeDefined()
    expect(mainRule).toContain('grid-template-columns: minmax(0, 1fr) auto;')
    expect(sessionActionsRule).toContain('white-space: nowrap;')
    expect(contextRule).toContain('text-overflow: ellipsis;')
    expect(contextRule).toContain('white-space: nowrap;')
  })

  it('uses a text-only switch button and keeps unavailable reasons in the modal', () => {
    const switchButton = requireElementByClass(templateRoot, 'provider-concurrency-row__switch')

    expect(switchButton.tag).toBe('button')
    expect(componentSource).not.toContain('<span aria-hidden="true">↔</span>')
    expect(componentSource).not.toContain('provider-concurrency-row__switch-hint')
    expect(componentSource).toContain('selectedSessionNumber === 0')
    expect(componentSource).toContain("t('components.main.concurrencyDetails.sessionUnavailable')")
  })

  it('keeps keyboard and pointer actions isolated from nested controls', () => {
    const row = requireElementByClass(templateRoot, 'provider-concurrency-row')
    const modelDetails = requireElementByClass(row, 'provider-concurrency-row__model-details')
    const switchButton = requireElementByClass(row, 'provider-concurrency-row__switch')

    expect(hasAttribute(row, 'role')).toBe(false)
    expect(hasAttribute(row, 'tabindex')).toBe(false)
    expect(directives(row, 'on', 'keydown')).toHaveLength(0)
    expect(directives(row, 'on', 'click')).toHaveLength(1)
    expect(directives(modelDetails, 'on', 'click')[0]?.modifiers?.map((modifier) => modifier.content)).toContain('stop')
    expect(directives(switchButton, 'on', 'click')[0]?.modifiers?.map((modifier) => modifier.content)).toContain('stop')
  })
})

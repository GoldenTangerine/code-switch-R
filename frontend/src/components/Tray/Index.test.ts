/**
 * @name: 托盘额度错误展示测试
 * @Descripttion: 验证托盘额度查询错误图标、无障碍状态和详情隐藏契约
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-20 18:31:50
 * @LastEditTime: 2026-08-20 18:31:50
 * @FilePath: frontend/src/components/Tray/Index.test.ts
 */

import { readFileSync } from 'node:fs'
import { parse as parseSfc } from 'vue/compiler-sfc'
import { describe, expect, it } from 'vitest'

const componentSource = readFileSync(new URL('./Index.vue', import.meta.url), 'utf8')
const providerCardStyleSource = readFileSync(new URL('../Main/styles/provider-card.css', import.meta.url), 'utf8')
const parsedComponent = parseSfc(componentSource, { filename: 'Index.vue' })
const componentStyleSource = parsedComponent.descriptor.styles.map((style) => style.content).join('\n')

type TemplateExpression = {
  content?: string
}

type TemplateProp = {
  name?: string
  value?: TemplateExpression
  exp?: TemplateExpression
}

type TemplateNode = {
  tag?: string
  props?: TemplateProp[]
  children?: TemplateNode[]
}

const templateRoot = parsedComponent.descriptor.template?.ast as unknown as TemplateNode

function staticClasses(node: TemplateNode): string[] {
  const classAttribute = node.props?.find((prop) => prop.name === 'class' && prop.value?.content)
  return classAttribute?.value?.content?.split(/\s+/).filter(Boolean) ?? []
}

function findElementsByClass(node: TemplateNode, className: string): TemplateNode[] {
  const matches = staticClasses(node).includes(className) ? [node] : []
  for (const child of node.children ?? []) {
    matches.push(...findElementsByClass(child, className))
  }
  return matches
}

function attributeValue(node: TemplateNode, name: string): string | undefined {
  return node.props?.find((prop) => prop.name === name)?.value?.content
}

function directiveValue(node: TemplateNode, name: string): string | undefined {
  return node.props?.find((prop) => prop.name === name)?.exp?.content
}

function ruleBody(styleSource: string, selector: string): string {
  const start = styleSource.indexOf(selector)
  const open = styleSource.indexOf('{', start)
  const close = styleSource.indexOf('}', open)
  return start >= 0 && open >= 0 && close >= 0
    ? styleSource.slice(open + 1, close).trim()
    : ''
}

describe('tray provider quota error display', () => {
  it('renders only a static icon and an accessible status for quota errors', () => {
    expect(parsedComponent.errors).toEqual([])

    const errorIcon = findElementsByClass(templateRoot, 'tray-quota-error-icon')[0]
    const errorStatus = findElementsByClass(templateRoot, 'sr-only').find((node) => (
      attributeValue(node, 'role') === 'status'
    ))
    const guardedMeta = findElementsByClass(templateRoot, 'tray-meta').find((node) => (
      directiveValue(node, 'if') === 'shouldShowTrayProviderQuotaMeta(quota)'
    ))

    expect(errorIcon?.tag).toBe('svg')
    expect(attributeValue(errorIcon, 'aria-hidden')).toBe('true')
    expect(attributeValue(errorIcon, 'role')).toBeUndefined()
    expect(attributeValue(errorStatus ?? {}, 'aria-live')).toBe('polite')
    expect(guardedMeta).toBeDefined()
    expect(componentSource).toContain('d="M12 7.5v5.25M12 16.25v.25"')
  })

  it('keeps tray error icon styles identical to the home provider icon', () => {
    expect(ruleBody(componentStyleSource, '.tray-quota-error-icon')).toBe(
      ruleBody(providerCardStyleSource, '.card-balance-quota__error-icon'),
    )
    expect(ruleBody(componentStyleSource, ':global(.dark) .tray-quota-error-icon')).toBe(
      ruleBody(providerCardStyleSource, '.automation-card.theme-dark .card-balance-quota__error-icon'),
    )
  })
})

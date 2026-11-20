/**
 * @name: 技能管理页面测试
 * @Descripttion: 验证离线打开已安装页、应用开关与发现页的安装目标。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-11-20 15:47:00
 * @LastEditTime: 2026-11-20 15:47:00
 * @FilePath: frontend/src/components/Skill/Index.test.ts
 */
// @vitest-environment happy-dom

import { createApp } from 'vue'
import { createI18n } from 'vue-i18n'
import { Call } from '@wailsio/runtime'
import { expect, it, vi } from 'vitest'
import en from '../../locales/en.json'
import SkillPage from './Index.vue'

vi.mock('@wailsio/runtime', () => ({
  Browser: { OpenURL: vi.fn() },
  Call: { ByName: vi.fn() }
}))
vi.mock('vue-router', () => ({ useRouter: () => ({ push: vi.fn() }) }))
vi.mock('../common/BaseModal.vue', () => ({
  default: { props: ['open'], template: '<div v-if="open"><slot /></div>' }
}))

const flushUpdates = () => new Promise(resolve => setTimeout(resolve, 0))

it('loads installed skills locally and sends the selected application targets', async () => {
  const service = 'codeswitch/services.SkillService.'
  const skill = {
    key: 'library:demo', name: 'Demo', description: 'A managed skill', directory: 'demo', readme_url: '',
    installed: true, master_status: '', app_statuses: {
      claude: 'synced', codex: 'disabled', gemini: 'disabled', opencode: 'inherited',
      grokbuild: 'blocked', openclaw: 'disabled', hermes: 'unavailable', pi: 'disabled'
    }
  }
  const recovered = { ...skill, key: 'library:orphan', directory: 'orphan', name: 'Orphan', master_status: 'unregistered' }
  vi.mocked(Call.ByName).mockImplementation((method, ...args) => {
    let result: unknown
    if (method === service + 'ListSkillLibrary') {
      result = { managed: [skill, recovered], local: [], warnings: ['Hermes: folder unavailable'] }
    }
    if (method === service + 'SetManagedSkillEnabled') {
      if (args[1] === 'codex') skill.app_statuses.codex = 'synced'
      if (args[1] === 'opencode') skill.app_statuses.opencode = 'blocked'
    }
    if (method === service + 'RecoverManagedSkill') recovered.master_status = ''
    if (method === service + 'DiscoverSkills') {
      result = [{
        key: 'test/repo:skills/sample', name: 'Sample', description: 'A repository skill', directory: 'sample',
        repo_owner: 'test', repo_name: 'repo', repo_branch: 'main', repo_path: 'skills/sample'
      }]
    }
    return Promise.resolve(result) as ReturnType<typeof Call.ByName>
  })

  const root = document.createElement('div')
  const app = createApp(SkillPage)
  app.use(createI18n({ legacy: false, locale: 'en', messages: { en } }))
  app.mount(root)
  try {
    await flushUpdates()
    expect(Call.ByName).toHaveBeenCalledTimes(1)
    expect(Call.ByName).toHaveBeenCalledWith(service + 'ListSkillLibrary')
    const codexSwitch = root.querySelector<HTMLButtonElement>('[aria-label="Demo · Toggle in Codex"]')
    expect(codexSwitch?.getAttribute('aria-checked')).toBe('false')
    expect(root.textContent).toContain('Loaded from another folder')
    expect(root.textContent).toContain('Hermes: folder unavailable')
    codexSwitch?.click()
    await flushUpdates()
    expect(Call.ByName).toHaveBeenCalledWith(service + 'SetManagedSkillEnabled', 'demo', 'codex', true)
    expect(codexSwitch?.getAttribute('aria-checked')).toBe('true')

    const openCodeSwitch = root.querySelector<HTMLButtonElement>('[aria-label="Demo · Toggle in OpenCode"]')
    expect(openCodeSwitch?.getAttribute('aria-checked')).toBe('true')
    openCodeSwitch?.click()
    await flushUpdates()
    expect(Call.ByName).toHaveBeenCalledWith(service + 'SetManagedSkillEnabled', 'demo', 'opencode', false)
    expect(openCodeSwitch?.getAttribute('aria-checked')).toBe('false')

    Array.from(root.querySelectorAll<HTMLButtonElement>('button'))
      .find(button => button.textContent?.trim() === 'Add to Library')?.click()
    await flushUpdates()
    expect(Call.ByName).toHaveBeenCalledWith(service + 'RecoverManagedSkill', 'orphan')

    Array.from(root.querySelectorAll<HTMLButtonElement>('nav button'))
      .find(button => button.textContent === 'Discover')?.click()
    await flushUpdates()
    expect(Call.ByName).toHaveBeenCalledWith(service + 'DiscoverSkills')
    root.querySelector<HTMLButtonElement>('article button')?.click()
    await flushUpdates()
    const gemini = root.querySelector<HTMLInputElement>('input[type="checkbox"][value="gemini"]')!
    expect(gemini).not.toBeNull()
    gemini.checked = true
    gemini.dispatchEvent(new Event('change'))
    await flushUpdates()
    Array.from(root.querySelectorAll('button')).find(button => button.textContent?.trim() === 'Confirm Install')?.click()
    await flushUpdates()
    expect(Call.ByName).toHaveBeenCalledWith(service + 'InstallSkill', {
      directory: 'sample', repo_owner: 'test', repo_name: 'repo', repo_branch: 'main', repo_path: 'skills/sample',
      location: 'user', apps: ['claude', 'gemini']
    })
  } finally {
    app.unmount()
  }
})

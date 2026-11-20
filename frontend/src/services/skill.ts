/**
 * @name: 技能服务接口
 * @Descripttion: 封装技能库、各应用技能和仓库管理的 Wails 调用。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-11-20 15:31:00
 * @LastEditTime: 2026-11-20 15:31:00
 * @FilePath: frontend/src/services/skill.ts
 */
import { Call } from '@wailsio/runtime'

export const skillPlatforms = ['claude', 'codex', 'gemini', 'opencode', 'grokbuild', 'openclaw', 'hermes', 'pi'] as const
export type SkillPlatform = typeof skillPlatforms[number]

export type SkillSummary = {
  key: string
  name: string
  description: string
  directory: string
  readme_url: string
  installed: boolean
  read_only?: boolean

  // 新增字段
  enabled: boolean
  license_file?: string
  platform: SkillPlatform | ''
  install_location?: 'user' | 'project' | 'opencode-legacy-project' | 'pi-legacy-project' | 'openclaw-legacy-project' | 'library'

  // 仓库字段
  repo_owner?: string
  repo_name?: string
  repo_branch?: string
  repo_path?: string
}

export type ManagedSkill = SkillSummary & {
  apps: Record<SkillPlatform, boolean>
  conflicts: SkillPlatform[]
  app_statuses: Record<SkillPlatform, SkillAppStatus>
  master_status: '' | 'missing' | 'unregistered'
}

export type SkillAppStatus = 'synced' | 'disabled' | 'inherited' | 'blocked' | 'broken' | 'conflict' | 'unavailable'

export type SkillLibrary = {
  managed: ManagedSkill[]
  local: SkillSummary[]
  warnings: string[]
}

export type SkillRepoConfig = {
  owner: string
  name: string
  branch: string
  enabled: boolean
}

export type InstallSkillPayload = {
  directory: string
  repo_owner?: string
  repo_name?: string
  repo_branch?: string
  repo_path?: string
  platform?: SkillPlatform
  location?: 'user' | 'project'
  apps?: SkillPlatform[]
}

export const fetchSkillLibrary = async (): Promise<SkillLibrary> => {
  return await Call.ByName('codeswitch/services.SkillService.ListSkillLibrary') as SkillLibrary
}

export const discoverSkills = async (): Promise<SkillSummary[]> => {
  const response = await Call.ByName('codeswitch/services.SkillService.DiscoverSkills')
  return (response as SkillSummary[]) ?? []
}

export const importLocalSkill = async (directory: string, app: SkillPlatform): Promise<void> => {
  await Call.ByName('codeswitch/services.SkillService.ImportLocalSkill', directory, app)
}

export const setManagedSkillEnabled = async (
  directory: string,
  app: SkillPlatform,
  enabled: boolean
): Promise<void> => {
  await Call.ByName('codeswitch/services.SkillService.SetManagedSkillEnabled', directory, app, enabled)
}

export const uninstallManagedSkill = async (directory: string): Promise<void> => {
  await Call.ByName('codeswitch/services.SkillService.UninstallManagedSkill', directory)
}

export const recoverManagedSkill = async (directory: string): Promise<void> => {
  await Call.ByName('codeswitch/services.SkillService.RecoverManagedSkill', directory)
}

// 获取所有技能列表（原有方法，向后兼容）
export const fetchSkills = async (): Promise<SkillSummary[]> => {
  const response = await Call.ByName('codeswitch/services.SkillService.ListSkills')
  return (response as SkillSummary[]) ?? []
}

// 获取指定平台的技能列表（新方法）
export const fetchSkillsForPlatform = async (platform: SkillPlatform): Promise<SkillSummary[]> => {
  const response = await Call.ByName('codeswitch/services.SkillService.ListSkillsForPlatform', platform)
  return (response as SkillSummary[]) ?? []
}

// 安装技能（支持 platform 和 location）
export const installSkill = async (payload: InstallSkillPayload): Promise<void> => {
  await Call.ByName('codeswitch/services.SkillService.InstallSkill', payload)
}

// 卸载技能（原有方法，向后兼容）
export const uninstallSkill = async (directory: string): Promise<void> => {
  await Call.ByName('codeswitch/services.SkillService.UninstallSkill', directory)
}

// 卸载技能（支持 platform 和 location）
export const uninstallSkillEx = async (
  directory: string,
  platform: string,
  location: string
): Promise<void> => {
  await Call.ByName('codeswitch/services.SkillService.UninstallSkillEx', directory, platform, location)
}

// 切换技能启用状态
export const toggleSkill = async (
  directory: string,
  platform: string,
  location: string,
  enabled: boolean
): Promise<void> => {
  await Call.ByName('codeswitch/services.SkillService.ToggleSkill', directory, platform, location, enabled)
}

// 获取技能内容
export const getSkillContent = async (
  directory: string,
  platform: string,
  location: string
): Promise<string> => {
  const response = await Call.ByName(
    'codeswitch/services.SkillService.GetSkillContent',
    directory,
    platform,
    location
  )
  return response as string
}

// 保存技能内容
export const saveSkillContent = async (
  directory: string,
  platform: string,
  location: string,
  content: string
): Promise<void> => {
  await Call.ByName(
    'codeswitch/services.SkillService.SaveSkillContent',
    directory,
    platform,
    location,
    content
  )
}

// 打开技能文件夹
export const openSkillFolder = async (platform: string, location: string): Promise<void> => {
  await Call.ByName('codeswitch/services.SkillService.OpenSkillFolder', platform, location)
}

// 仓库管理相关方法
export const fetchSkillRepos = async (): Promise<SkillRepoConfig[]> => {
  const response = await Call.ByName('codeswitch/services.SkillService.ListRepos')
  return (response as SkillRepoConfig[]) ?? []
}

export const addSkillRepo = async (repo: Partial<SkillRepoConfig>): Promise<SkillRepoConfig[]> => {
  const payload = {
    owner: repo.owner ?? '',
    name: repo.name ?? '',
    branch: repo.branch ?? 'main',
    enabled: repo.enabled ?? true
  }
  const response = await Call.ByName('codeswitch/services.SkillService.AddRepo', payload)
  return (response as SkillRepoConfig[]) ?? []
}

export const removeSkillRepo = async (owner: string, name: string): Promise<SkillRepoConfig[]> => {
  const response = await Call.ByName('codeswitch/services.SkillService.RemoveRepo', owner, name)
  return (response as SkillRepoConfig[]) ?? []
}

import { Call } from '@wailsio/runtime'

export type SkillSummary = {
  key: string
  name: string
  description: string
  directory: string
  readme_url: string
  installed: boolean
  repo_owner?: string
  repo_name?: string
  repo_branch?: string
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
}

export const fetchSkills = async (): Promise<SkillSummary[]> => {
  const response = await Call.ByName('codeswitch/services.SkillService.ListSkills')
  return (response as SkillSummary[]) ?? []
}

export const installSkill = async (payload: InstallSkillPayload): Promise<void> => {
  await Call.ByName('codeswitch/services.SkillService.InstallSkill', payload)
}

export const uninstallSkill = async (directory: string): Promise<void> => {
  await Call.ByName('codeswitch/services.SkillService.UninstallSkill', directory)
}

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

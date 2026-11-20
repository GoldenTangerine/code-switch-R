<!--
@name: 技能管理页面
@Descripttion: 展示已安装技能的应用开关、仓库发现和本地技能。
@version: 1.0.0
@Author: sm
@Date: 2026-11-20 15:31:00
@LastEditTime: 2026-11-20 15:31:00
@FilePath: frontend/src/components/Skill/Index.vue
-->
<template>
  <div class="main-shell">
    <div class="global-actions">
      <p class="global-eyebrow">{{ t('components.skill.hero.eyebrow') }}</p>
      <button class="ghost-icon" :title="t('components.skill.actions.back')"
        :data-tooltip="t('components.skill.actions.back')" @click="goHome">
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <path d="M15 18l-6-6 6-6" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"
            stroke-linejoin="round" />
        </svg>
      </button>
      <button class="ghost-icon" :title="t('components.skill.actions.refresh')"
        :data-tooltip="t('components.skill.actions.refresh')" :disabled="refreshing || mutationBusy" @click="refresh">
        <svg viewBox="0 0 24 24" aria-hidden="true" :class="{ spin: refreshing }">
          <path d="M20.5 8a8.5 8.5 0 10-2.38 7.41" fill="none" stroke="currentColor" stroke-width="1.5"
            stroke-linecap="round" stroke-linejoin="round" />
          <path d="M20.5 4v4h-4" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"
            stroke-linejoin="round" />
        </svg>
      </button>
      <button class="ghost-icon" :title="t('components.skill.actions.openLibrary')"
        :data-tooltip="t('components.skill.actions.openLibrary')" @click="handleOpenFolder()">
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <path d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z" fill="none"
            stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" />
        </svg>
      </button>
      <button class="ghost-icon" :title="t('components.skill.repos.open')"
        :data-tooltip="t('components.skill.repos.open')" @click="openRepoModal">
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <path d="M5 5h14v6H5zM7 13h10v6H7z" fill="none" stroke="currentColor" stroke-width="1.5"
            stroke-linecap="round" stroke-linejoin="round" />
          <path d="M12 7.5v1M12 15.5v1" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" />
        </svg>
      </button>
    </div>

    <div class="contrib-page skill-page">
      <header class="skill-hero">
        <div class="skill-hero-text">
          <h1>{{ t('components.skill.hero.title') }}</h1>
          <p class="skill-lead">{{ t('components.skill.hero.lead') }}</p>
        </div>
      </header>


      <nav class="skill-platform-tabs" :aria-label="t('components.skill.hero.title')">
        <button
          v-for="view in views"
          :key="view"
          type="button"
          :class="['skill-platform-tab', { active: activeView === view }]"
          :aria-current="activeView === view ? 'page' : undefined"
          @click="switchView(view)"
        >{{ t(`components.skill.views.${view}`) }}</button>
      </nav>

      <div class="skill-toolbar">
        <input
          v-model="search"
          type="search"
          :aria-label="t('components.skill.list.search')"
          :placeholder="t('components.skill.list.search')"
        />
      </div>
      <p v-if="skillsError" class="skill-error" role="alert">{{ skillsError }}</p>

      <section v-if="activeView === 'installed'" class="skill-list-section">
        <div v-if="libraryLoading && !libraryLoaded" class="skill-empty">{{ t('components.skill.list.loading') }}</div>
        <template v-else-if="libraryLoaded">
          <details v-if="library.warnings.length" class="skill-warnings">
            <summary>{{ t('components.skill.list.partial') }}</summary>
            <ul>
              <li v-for="warning in library.warnings" :key="warning">{{ warning }}</li>
            </ul>
          </details>
          <div
            v-if="managedSkills.length"
            class="skill-matrix"
            tabindex="0"
            role="region"
            :aria-label="t('components.skill.views.installed')"
          >
            <table>
              <thead>
                <tr>
                  <th scope="col">{{ t('components.skill.list.column') }}</th>
                  <th v-for="app in platforms" :key="app" scope="col">{{ appLabel(app) }}</th>
                  <th scope="col"><span class="sr-only">{{ t('components.skill.actions.label') }}</span></th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="skill in managedSkills" :key="skill.key">
                  <th scope="row">
                    <button type="button" class="skill-name" @click="viewManagedSkill(skill)">{{ skill.name }}</button>
                    <p class="skill-source">
                      {{ skill.repo_owner && skill.repo_name ? `${skill.repo_owner}/${skill.repo_name}` : skill.directory }}
                    </p>
                    <p v-if="skill.description" class="skill-card-desc">{{ skill.description }}</p>
                    <div v-if="skill.master_status" class="skill-recovery">
                      <span>{{ t(`components.skill.masterStatus.${skill.master_status}`) }}</span>
                      <button type="button" class="skill-link" :disabled="libraryBusy" @click="handleRecover(skill)">
                        {{ t(skill.master_status === 'unregistered' ? 'components.skill.actions.register' : 'components.skill.actions.recover') }}
                      </button>
                    </div>
                  </th>
                  <td v-for="app in platforms" :key="app">
                    <span v-if="skill.app_statuses[app] === 'conflict'" class="skill-conflict" :title="t('components.skill.actions.conflictDetail')">
                      {{ t('components.skill.actions.conflict') }}
                    </span>
                    <button
                      v-else-if="skill.app_statuses[app] !== 'unavailable' && skill.app_statuses[app] !== 'broken'"
                      type="button"
                      role="switch"
                      class="app-switch"
                      :aria-checked="isAppActive(skill, app)"
                      :aria-label="t('components.skill.actions.appSwitch', { name: skill.name, app: appLabel(app) })"
                      :disabled="libraryBusy || !!skill.master_status || hasReadOnlyInheritance(skill, app)"
                      @click="toggleApp(skill, app)"
                    ><span></span></button>
                    <span v-if="visibleAppStatus(skill, app)" class="skill-inherited">
                      {{ t(`components.skill.appStatus.${skill.app_statuses[app]}`) }}
                    </span>
                  </td>
                  <td>
                    <button
                      type="button"
                      class="ghost-icon sm danger"
                      :aria-label="t('components.skill.actions.uninstall')"
                      :title="t('components.skill.actions.uninstall')"
                      :disabled="libraryBusy || skill.master_status === 'unregistered'"
                      @click="uninstallTarget = { skill, managed: true }"
                    >
                      <svg viewBox="0 0 24 24" aria-hidden="true">
                        <path d="M5 7h14m-13 0 1 13h10l1-13M9 7V4h6v3" fill="none"
                          stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" />
                      </svg>
                    </button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>

          <!-- Project Skills Group and User Skills Group -->
          <section v-for="group in localGroups" :key="group.location" class="skill-group">
            <h2 class="skill-group-title">{{ t(`components.skill.groups.${group.location}`) }}</h2>
            <div class="skill-list installed-skills">
              <SkillCard
                v-for="skill in group.skills"
                :key="skill.key"
                :skill="skill"
                :expanded="expandedSkills.has(skill.key)"
                :busy="libraryBusy"
                @toggle="handleLocalToggle"
                @expand="toggleExpand"
                @uninstall="uninstallTarget = { skill: $event, managed: false }"
                @view="openExternal"
                @folder="handleLocalFolder"
                @import="handleImport"
              />
            </div>
          </section>

          <div v-if="!managedSkills.length && !localGroups.length" class="skill-empty">
            {{ t(search.trim() ? 'components.skill.list.noResults' : 'components.skill.list.noInstalled') }}
            <button v-if="!search.trim()" class="btn-secondary" type="button" @click="switchView('discover')">
              {{ t('components.skill.views.discover') }}
            </button>
          </div>
        </template>
      </section>

      <section v-else class="skill-list-section">
        <div v-if="discoverLoading" class="skill-empty">{{ t('components.skill.list.loading') }}</div>
        <div v-else-if="availableSkills.length" class="skill-list">
          <article v-for="skill in availableSkills" :key="skill.key" class="skill-card available-card">
            <div class="skill-card-head">
              <div>
                <p class="skill-card-eyebrow">{{ skill.repo_owner }}/{{ skill.repo_name }}</p>
                <h3>{{ skill.name }}</h3>
              </div>
              <button type="button" class="btn-secondary" :disabled="libraryBusy || !libraryLoaded" @click="openInstallModal(skill)">
                {{ t(isInLibrary(skill) ? 'components.skill.install.projectAction' : 'components.skill.actions.install') }}
              </button>
            </div>
            <p class="skill-card-desc">{{ skill.description || t('components.skill.list.noDescription') }}</p>
            <button type="button" class="skill-link" @click="openExternal(skill.readme_url)">
              {{ t('components.skill.actions.view') }}
            </button>
          </article>
        </div>
        <div v-else-if="!discoverError" class="skill-empty">
          {{ t(search.trim() ? 'components.skill.list.noResults' : 'components.skill.list.empty') }}
        </div>
        <p v-if="discoverError" class="skill-error" role="alert">{{ discoverError }}</p>
      </section>
    </div>

    <!-- Install Location Modal -->
    <BaseModal :open="installTarget !== null" :title="t('components.skill.install.title')" @close="closeInstallModal">
      <div class="install-modal-content">
        <p class="install-modal-desc">{{ t('components.skill.install.desc', { name: installTarget?.name }) }}</p>
        <div v-if="installTarget && !isInLibrary(installTarget)" class="install-location-options">
          <label><input v-model="installLocation" type="radio" value="user" :disabled="mutationBusy" /> {{ t('components.skill.install.userLevel') }}</label>
          <label><input v-model="installLocation" type="radio" value="project" :disabled="mutationBusy" /> {{ t('components.skill.install.projectLevel') }}</label>
        </div>
        <fieldset v-if="installLocation === 'user'" class="install-location-options" :disabled="mutationBusy">
          <legend>{{ t('components.skill.install.apps') }}</legend>
          <label v-for="app in platforms" :key="app">
            <input v-model="installApps" type="checkbox" :value="app" /> {{ appLabel(app) }}
          </label>
        </fieldset>
        <div v-else class="install-project">
          <label>
            {{ t('components.skill.install.app') }}
            <select v-model="projectApp" :disabled="mutationBusy">
              <option v-for="app in platforms" :key="app" :value="app">{{ appLabel(app) }}</option>
            </select>
          </label>
          <p class="install-option-warning">{{ t('components.skill.install.gitWarning') }}</p>
        </div>
        <p v-if="installError" class="skill-error" role="alert">{{ installError }}</p>
        <div class="install-modal-actions">
          <button class="btn-secondary" type="button" :disabled="mutationBusy" @click="closeInstallModal">{{ t('common.cancel') }}</button>
          <button class="btn-primary" type="button" :disabled="mutationBusy" @click="confirmInstall">
            {{ t(mutationBusy ? 'components.skill.install.installing' : 'components.skill.install.confirm') }}
          </button>
        </div>
      </div>
    </BaseModal>

    <BaseModal :open="uninstallTarget !== null" :title="t('components.skill.actions.uninstall')" variant="confirm" @close="closeUninstallModal">
      <p>{{ t(uninstallTarget?.managed ? 'components.skill.actions.uninstallManagedPrompt' : 'components.skill.actions.uninstallLocalPrompt', { name: uninstallTarget?.skill.name }) }}</p>
      <p v-if="uninstallError" class="skill-error" role="alert">{{ uninstallError }}</p>
      <div class="install-modal-actions">
        <button class="btn-secondary" type="button" :disabled="mutationBusy" @click="closeUninstallModal">{{ t('common.cancel') }}</button>
        <button class="btn-primary" type="button" :disabled="mutationBusy" @click="confirmUninstall">{{ t('components.skill.actions.uninstall') }}</button>
      </div>
    </BaseModal>

    <BaseModal :open="detailTarget !== null" :title="detailTarget?.name ?? ''" panel-width="min(720px, 92vw)" @close="detailTarget = null">
      <button v-if="detailTarget?.readme_url" type="button" class="skill-link" @click="openExternal(detailTarget.readme_url)">
        {{ t('components.skill.actions.view') }}
      </button>
      <pre class="skill-detail">{{ detailContent }}</pre>
    </BaseModal>

    <!-- Repository Modal -->
    <BaseModal :open="repoModalOpen" :title="t('components.skill.repos.title')" @close="closeRepoModal">
      <div class="skill-repo-section repo-modal-content">
        <p class="skill-repo-subtitle">{{ t('components.skill.repos.subtitle') }}</p>
        <form class="skill-repo-form" @submit.prevent="submitRepo">
          <div class="repo-input-field">
            <input v-model="repoForm.url" type="text" :placeholder="t('components.skill.repos.urlPlaceholder')"
              :disabled="repoBusy" autocomplete="off" autocorrect="off" autocapitalize="off" spellcheck="false" />
          </div>
          <div class="repo-form-actions">
            <input v-model="repoForm.branch" type="text" :placeholder="t('components.skill.repos.branchPlaceholder')"
              :disabled="repoBusy" autocomplete="off" autocorrect="off" autocapitalize="off" spellcheck="false" />
            <button class="ghost-icon" type="submit" :disabled="repoBusy" :title="t('components.skill.repos.addLabel')"
              :data-tooltip="t('components.skill.repos.addLabel')">
              <svg viewBox="0 0 24 24" aria-hidden="true" :class="{ spin: repoBusy }">
                <path d="M12 5v14M5 12h14" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"
                  stroke-linejoin="round" fill="none" />
              </svg>
            </button>
          </div>
        </form>
        <p v-if="repoError" class="skill-error">{{ repoError }}</p>
        <div class="skill-repo-list" :class="{ loading: repoLoading }">
          <p v-if="repoLoading" class="skill-empty">{{ t('components.skill.repos.loading') }}</p>
          <p v-else-if="!repoList.length" class="skill-empty">{{ t('components.skill.repos.empty') }}</p>
          <div v-else>
            <article v-for="repo in repoList" :key="repoKey(repo)" class="skill-repo-item">
              <div class="skill-repo-meta">
                <p class="repo-name">{{ repo.owner }}/{{ repo.name }}</p>
                <span class="repo-branch">{{ t('components.skill.repos.branchLabel', { branch: repo.branch }) }}</span>
              </div>
              <div class="skill-repo-actions">
                <button class="ghost-icon sm" type="button" :title="t('components.skill.repos.viewLabel')"
                  :data-tooltip="t('components.skill.repos.viewLabel')" @click="openRepoGithub(repo)">
                  <svg viewBox="0 0 24 24" aria-hidden="true">
                    <path d="M12 5h7v7M19 5l-9 9" fill="none" stroke="currentColor" stroke-width="1.6"
                      stroke-linecap="round" stroke-linejoin="round" />
                    <path d="M11 6H7a2 2 0 00-2 2v9a2 2 0 002 2h9a2 2 0 002-2v-4" fill="none" stroke="currentColor"
                      stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" />
                  </svg>
                </button>
                <button class="ghost-icon sm danger" type="button" :title="t('components.skill.repos.removeLabel')"
                  :data-tooltip="t('components.skill.repos.removeLabel')" :disabled="repoBusy"
                  @click="removeRepo(repo)">
                  <svg viewBox="0 0 24 24" aria-hidden="true">
                    <path d="M5 7h14M10 11v6M14 11v6M9 7V5h6v2" fill="none" stroke="currentColor" stroke-width="1.6"
                      stroke-linecap="round" stroke-linejoin="round" />
                    <path d="M6.5 7l-.5 12a2 2 0 002 2h8a2 2 0 002-2L17.5 7" fill="none" stroke="currentColor"
                      stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" />
                  </svg>
                </button>
              </div>
            </article>
          </div>
        </div>
      </div>
    </BaseModal>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { Browser } from '@wailsio/runtime'
import {
  skillPlatforms as platforms,
  fetchSkillLibrary,
  discoverSkills,
  importLocalSkill,
  setManagedSkillEnabled,
  uninstallManagedSkill,
  recoverManagedSkill,
  installSkill,
  uninstallSkillEx,
  toggleSkill,
  getSkillContent,
  openSkillFolder,
  fetchSkillRepos,
  addSkillRepo,
  removeSkillRepo,
  type ManagedSkill,
  type SkillLibrary,
  type SkillPlatform,
  type SkillRepoConfig,
  type SkillSummary
} from '../../services/skill'
import BaseModal from '../common/BaseModal.vue'
import SkillCard from './SkillCard.vue'

const router = useRouter()
const { t } = useI18n()
const appLabel = (app: SkillPlatform) => t(`components.skill.platform.${app}`)

const views = ['installed', 'discover'] as const
const activeView = ref<typeof views[number]>('installed')
const search = ref('')
const library = ref<SkillLibrary>({ managed: [], local: [], warnings: [] })
const discovered = ref<SkillSummary[]>([])
const libraryLoaded = ref(false)
const libraryLoading = ref(false)
const discoverLoaded = ref(false)
const discoverLoading = ref(false)
const skillsError = ref('')
const discoverError = ref('')
const mutationBusy = ref(false)
const libraryBusy = computed(() => mutationBusy.value || libraryLoading.value)

const repoList = ref<SkillRepoConfig[]>([])
const repoLoading = ref(false)
const repoError = ref('')
const repoBusy = ref(false)
const repoForm = reactive({ url: '', branch: 'main' })
const repoModalOpen = ref(false)

// Install modal state
const installTarget = ref<SkillSummary | null>(null)
const installLocation = ref<'user' | 'project'>('user')
const installApps = ref<SkillPlatform[]>(['claude'])
const projectApp = ref<SkillPlatform>('claude')
const installError = ref('')
const uninstallTarget = ref<{ skill: SkillSummary; managed: boolean } | null>(null)
const uninstallError = ref('')
const detailTarget = ref<ManagedSkill | null>(null)
const detailContent = ref('')
let detailRequest = 0

// Expanded skills
const expandedSkills = ref(new Set<string>())

const refreshing = computed(() => libraryLoading.value || (activeView.value === 'discover' && discoverLoading.value))
const libraryDirectories = computed(() => new Set(library.value.managed.map(skill => skill.directory.toLowerCase())))
const isInLibrary = (skill: SkillSummary) => libraryDirectories.value.has(skill.directory.toLowerCase())
const searchText = computed(() => search.value.trim().toLowerCase())
const matchesSearch = (skill: SkillSummary) =>
  [skill.name, skill.directory, skill.description, `${skill.repo_owner ?? ''}/${skill.repo_name ?? ''}`]
    .some(value => value.toLowerCase().includes(searchText.value))
const managedSkills = computed(() => library.value.managed.filter(matchesSearch))
const isAppActive = (skill: ManagedSkill, app: SkillPlatform) =>
  ['synced', 'inherited'].includes(skill.app_statuses[app])
const hasReadOnlyInheritance = (skill: ManagedSkill, app: SkillPlatform) =>
  skill.app_statuses[app] === 'inherited' && app !== 'opencode' && app !== 'grokbuild'
const visibleAppStatus = (skill: ManagedSkill, app: SkillPlatform) =>
  ['inherited', 'blocked', 'broken', 'unavailable'].includes(skill.app_statuses[app])
const availableSkills = computed(() => discovered.value.filter(matchesSearch))
const localGroups = computed(() =>
  (['user', 'project'] as const).map(location => ({
    location,
    skills: library.value.local.filter(skill =>
      (location === 'user' ? skill.install_location === 'user' : skill.install_location !== 'user') && matchesSearch(skill))
  })).filter(group => group.skills.length)
)

const errorDetail = (message: string, error: unknown) => {
  const detail = typeof error === 'string' ? error : error instanceof Error ? error.message : ''
  return detail ? `${message} ${detail}` : message
}

const loadLibrary = async () => {
  libraryLoading.value = true
  try {
    library.value = await fetchSkillLibrary()
    libraryLoaded.value = true
  } catch (error) {
    skillsError.value = errorDetail(t('components.skill.list.libraryError'), error)
  } finally {
    libraryLoading.value = false
  }
}

let discoverRequest = 0
const loadDiscovery = async () => {
  const request = ++discoverRequest
  discoverLoading.value = true
  discoverError.value = ''
  try {
    const skills = await discoverSkills()
    if (request !== discoverRequest) return
    discovered.value = skills
    discoverLoaded.value = true
  } catch (error) {
    if (request !== discoverRequest) return
    discoverError.value = errorDetail(t('components.skill.list.error'), error)
  } finally {
    if (request === discoverRequest) discoverLoading.value = false
  }
}

const switchView = (view: typeof views[number]) => {
  activeView.value = view
  search.value = ''
  if (view === 'discover' && !discoverLoaded.value && !discoverLoading.value) void loadDiscovery()
}

const refresh = () => {
  skillsError.value = ''
  void loadLibrary()
  if (activeView.value === 'discover') void loadDiscovery()
}

const mutateLibrary = async (action: () => Promise<void>, message: string) => {
  if (libraryBusy.value) return
  mutationBusy.value = true
  skillsError.value = ''
  try {
    await action()
    await loadLibrary()
  } catch (error) {
    skillsError.value = errorDetail(message, error)
    await loadLibrary()
  } finally {
    mutationBusy.value = false
  }
}

const toggleApp = (skill: ManagedSkill, app: SkillPlatform) =>
  mutateLibrary(
    () => setManagedSkillEnabled(skill.directory, app, !isAppActive(skill, app)),
    t('components.skill.actions.toggleError')
  )

const handleRecover = (skill: ManagedSkill) =>
  mutateLibrary(() => recoverManagedSkill(skill.directory), t('components.skill.actions.recoverError'))

const handleLocalToggle = (skill: SkillSummary, enabled: boolean) =>
  mutateLibrary(
    () => toggleSkill(skill.directory, skill.platform, skill.install_location ?? 'user', enabled),
    t('components.skill.actions.toggleError')
  )

const handleImport = (skill: SkillSummary) => {
  if (!skill.platform) return
  const app = skill.platform
  return mutateLibrary(() => importLocalSkill(skill.directory, app), t('components.skill.actions.importError'))
}

// Toggle content expansion
const toggleExpand = (skill: SkillSummary) => {
  if (expandedSkills.value.has(skill.key)) expandedSkills.value.delete(skill.key)
  else expandedSkills.value.add(skill.key)
}

const viewManagedSkill = async (skill: ManagedSkill) => {
  const request = ++detailRequest
  detailTarget.value = skill
  detailContent.value = t('components.skill.actions.loading')
  try {
    const content = await getSkillContent(skill.directory, '', 'library')
    if (request === detailRequest) detailContent.value = content
  } catch {
    if (request === detailRequest) detailContent.value = t('components.skill.actions.loadFailed')
  }
}

const handleOpenFolder = async (platform = '', location = 'library') => {
  try {
    await openSkillFolder(platform, location)
  } catch (error) {
    skillsError.value = errorDetail(t('components.skill.actions.folderError'), error)
  }
}
const handleLocalFolder = (skill: SkillSummary) => handleOpenFolder(skill.platform, skill.install_location)

// Install modal
const openInstallModal = (skill: SkillSummary) => {
  installTarget.value = skill
  installLocation.value = isInLibrary(skill) ? 'project' : 'user'
  installError.value = ''
}

const closeInstallModal = () => {
  if (!mutationBusy.value) installTarget.value = null
}

const confirmInstall = async () => {
  const skill = installTarget.value
  if (!skill || mutationBusy.value) return
  mutationBusy.value = true
  installError.value = ''
  try {
    await installSkill({
      directory: skill.directory,
      repo_owner: skill.repo_owner,
      repo_name: skill.repo_name,
      repo_branch: skill.repo_branch,
      repo_path: skill.repo_path,
      location: installLocation.value,
      ...(installLocation.value === 'user' ? { apps: installApps.value } : { platform: projectApp.value })
    })
    installTarget.value = null
    await loadLibrary()
    switchView('installed')
  } catch (error) {
    installError.value = errorDetail(t('components.skill.actions.installError', { name: skill.name }), error)
  } finally {
    mutationBusy.value = false
  }
}

// Uninstall
const closeUninstallModal = () => {
  if (mutationBusy.value) return
  uninstallError.value = ''
  uninstallTarget.value = null
}

const confirmUninstall = async () => {
  const target = uninstallTarget.value
  if (!target || mutationBusy.value) return
  mutationBusy.value = true
  uninstallError.value = ''
  try {
    const { skill } = target
    if (target.managed) await uninstallManagedSkill(skill.directory)
    else await uninstallSkillEx(skill.directory, skill.platform, skill.install_location ?? 'user')
    uninstallTarget.value = null
    await loadLibrary()
  } catch (error) {
    uninstallError.value = errorDetail(t('components.skill.actions.uninstallError', { name: target.skill.name }), error)
  } finally {
    mutationBusy.value = false
  }
}

// Navigation
const goHome = () => router.push('/')
const openExternal = (target: string) => {
  if (!target) return
  Browser.OpenURL(target).catch(error => console.error('failed to open link', error))
}

const loadRepos = async () => {
  repoLoading.value = true
  repoError.value = ''
  try {
    repoList.value = await fetchSkillRepos()
  } catch (error) {
    console.error('failed to load skill repos', error)
    repoError.value = t('components.skill.repos.loadError')
  } finally {
    repoLoading.value = false
  }
}

// Repository modal
const openRepoModal = () => {
  repoModalOpen.value = true
  if (!repoList.value.length && !repoLoading.value) {
    void loadRepos()
  }
}

const closeRepoModal = () => {
  repoModalOpen.value = false
}

const repoKey = (repo: SkillRepoConfig) => `${repo.owner}/${repo.name}`

const parseRepoInput = (value: string) => {
  let input = value.trim()
  if (!input) return null
  input = input.replace(/^https?:\/\/(www\.)?github\.com\//i, '')
  input = input.replace(/\.git$/i, '')
  const parts = input.split('/')
  if (parts.length < 2) return null
  const owner = parts[0]
  const name = parts[1]
  if (!owner || !name) return null
  return { owner, name }
}

const submitRepo = async () => {
  const parsed = parseRepoInput(repoForm.url)
  if (!parsed) {
    repoError.value = t('components.skill.repos.formError')
    return
  }
  repoBusy.value = true
  repoError.value = ''
  try {
    repoList.value = await addSkillRepo({
      owner: parsed.owner,
      name: parsed.name,
      branch: repoForm.branch || 'main',
      enabled: true
    })
    repoForm.url = ''
    repoForm.branch = 'main'
    discoverLoaded.value = false
    if (activeView.value === 'discover') void loadDiscovery()
  } catch (error) {
    console.error('failed to add skill repo', error)
    repoError.value = t('components.skill.repos.addError')
  } finally {
    repoBusy.value = false
  }
}

const removeRepo = async (repo: SkillRepoConfig) => {
  repoBusy.value = true
  repoError.value = ''
  try {
    repoList.value = await removeSkillRepo(repo.owner, repo.name)
    discoverLoaded.value = false
    if (activeView.value === 'discover') void loadDiscovery()
  } catch (error) {
    console.error('failed to remove skill repo', error)
    repoError.value = t('components.skill.repos.removeError')
  } finally {
    repoBusy.value = false
  }
}

const openRepoGithub = (repo: SkillRepoConfig) => {
  if (!repo?.owner || !repo?.name) return
  openExternal(`https://github.com/${repo.owner}/${repo.name}`)
}

onMounted(loadLibrary)
</script>

<style scoped>
.skill-page {
  min-width: 0;
  gap: 16px;
  color: var(--mac-text);
}

/* Platform Tabs */
.skill-platform-tabs {
  display: flex;
  gap: 8px;
  border-bottom: 1px solid var(--mac-border);
  padding-bottom: 12px;
}

.skill-platform-tab, .btn-primary, .btn-secondary {
  padding: 8px 14px;
  border: 1px solid var(--mac-border);
  border-radius: 8px;
  background: transparent;
  color: var(--mac-text);
  font-size: 0.9rem;
  font-weight: 500;
  cursor: pointer;
}

.skill-platform-tab:hover, .btn-secondary:hover {
  background: var(--mac-surface);
}

.skill-platform-tab.active, .btn-primary {
  background: var(--mac-accent);
  border-color: var(--mac-accent);
  color: white;
}

.skill-toolbar input, .install-project select {
  width: min(100%, 320px);
  padding: 8px 12px;
  border: 1px solid var(--mac-border);
  border-radius: 8px;
  background: var(--mac-surface);
  color: var(--mac-text);
}

.skill-matrix {
  max-width: 100%;
  overflow-x: auto;
  border: 1px solid var(--mac-border);
  border-radius: 14px;
  background: var(--mac-surface);
}

.skill-matrix table {
  width: 100%;
  border-collapse: collapse;
}

.skill-matrix th, .skill-matrix td {
  padding: 11px 9px;
  text-align: center;
  border-bottom: 1px solid var(--mac-border);
}

.skill-matrix tbody tr:last-child > * {
  border-bottom: none;
}

.skill-matrix thead {
  background: var(--mac-surface-strong);
  color: var(--mac-text-secondary);
  font-size: 0.8rem;
}

.skill-matrix th:first-child {
  position: sticky;
  left: 0;
  width: 100%;
  min-width: 190px;
  text-align: left;
  padding-inline: 14px;
  background: var(--mac-surface-strong);
}

.skill-name, .skill-link {
  text-align: left;
  color: var(--mac-accent);
  font-weight: 600;
  cursor: pointer;
  overflow-wrap: anywhere;
}

.skill-name:hover, .skill-link:hover {
  text-decoration: underline;
}

.skill-source, .skill-conflict {
  color: var(--mac-text-secondary);
  font-size: 0.75rem;
}

.skill-inherited {
  display: block;
  color: var(--mac-text-secondary);
  font-size: 0.7rem;
}

.skill-warnings {
  margin-bottom: 12px;
  color: var(--mac-text-secondary);
  font-size: 0.85rem;
  overflow-wrap: anywhere;
}

.skill-warnings summary {
  cursor: pointer;
}

.skill-warnings ul {
  padding-inline-start: 20px;
}

.skill-recovery {
  display: flex;
  flex-wrap: wrap;
  gap: 4px 10px;
  font-size: 0.8rem;
}

.skill-matrix tbody th .skill-card-desc {
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  font-weight: 400;
}

.app-switch {
  display: inline-flex;
  align-items: center;
  width: 34px;
  height: 20px;
  padding: 2px;
  border-radius: 999px;
  background: var(--mac-surface-strong);
  border: 1px solid var(--mac-text-secondary);
  cursor: pointer;
}

.app-switch span {
  width: 14px;
  height: 14px;
  border-radius: 50%;
  background: var(--mac-text-secondary);
}

.app-switch[aria-checked="true"] {
  justify-content: flex-end;
  background: var(--mac-accent);
  border-color: var(--mac-accent);
}

.app-switch[aria-checked="true"] span {
  background: white;
}

button:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

/* Skill Groups */
.skill-group {
  margin-top: 24px;
}

.skill-group-title {
  font-size: 1rem;
  margin-bottom: 10px;
}

/* Skill List */
.skill-list {
  display: grid;
  gap: 12px;
  grid-template-columns: repeat(auto-fit, minmax(min(100%, 300px), 1fr));
}

.installed-skills {
  grid-template-columns: 1fr;
}

/* Available Skill Card */
.skill-card.available-card {
  display: grid;
  gap: 8px;
  align-content: start;
  padding: 14px 16px;
  border-radius: 14px;
  border: 1px solid var(--mac-border);
  background: var(--mac-surface-strong); /* fallback for old WebKit */
  background: color-mix(in srgb, var(--mac-surface) 90%, transparent);
}

.skill-card-head {
  display: flex;
  align-items: start;
  justify-content: space-between;
  gap: 8px;
}

.skill-card-head button {
  flex-shrink: 0;
}

.skill-card-eyebrow, .skill-card-desc {
  color: var(--mac-text-secondary);
  font-size: 0.85rem;
  overflow-wrap: anywhere;
}

.skill-card h3 {
  margin: 0;
  overflow-wrap: anywhere;
}

/* Install Modal */
.install-modal-content, .install-project {
  display: grid;
  gap: 12px;
}

.install-location-options {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 16px;
}

.install-location-options legend {
  margin-bottom: 6px;
}

.install-option-warning {
  color: var(--mac-text-secondary);
  font-size: 0.85rem;
}

.install-modal-actions {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  padding-top: 14px;
}

.skill-detail {
  padding: 12px 0;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

/* Repository Section (reused from original) */
.skill-repo-section {
  border: 1px solid var(--mac-border);
  border-radius: 20px;
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 16px;
  background: var(--mac-surface-strong); /* fallback for old WebKit */
  background: color-mix(in srgb, var(--mac-surface) 90%, transparent);
}

.skill-repo-subtitle {
  margin: 0 0 12px;
  color: var(--mac-text-secondary);
  font-size: 0.95rem;
}

.skill-repo-form {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
  width: 100%;
}

.repo-input-field {
  flex: 1;
  min-width: 220px;
}

.skill-repo-form input {
  border: 1px solid var(--mac-border);
  border-radius: 10px;
  padding: 8px 12px;
  background: var(--mac-surface);
  color: var(--mac-text);
  font-size: 0.9rem;
}

.repo-input-field input {
  width: 100%;
}

.repo-form-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  align-items: center;
}

.repo-form-actions input {
  width: 160px;
}

.skill-repo-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.skill-repo-list.loading {
  opacity: 0.7;
}

.skill-repo-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 18px;
  border: 1px solid var(--mac-border);
  border-radius: 12px;
  background: var(--mac-surface-strong); /* fallback for old WebKit */
  background: color-mix(in srgb, var(--mac-surface) 80%, transparent);
  gap: 16px;
  margin: 0 0 8px;
}

.skill-repo-item:last-child {
  margin-bottom: 0;
}

.skill-repo-meta {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.skill-repo-meta .repo-name {
  margin: 0;
  font-weight: 600;
}

.skill-repo-meta .repo-branch {
  font-size: 0.85rem;
  color: var(--mac-text-secondary);
}

.skill-repo-actions {
  display: flex;
  gap: 8px;
  align-items: center;
}

.repo-modal-content {
  min-width: min(600px, 80vw);
}

/* Common */
.skill-hero {
  margin: 12px 0 12px;
}

.skill-lead {
  color: var(--mac-text-secondary);
  font-size: 0.95rem;
  line-height: 1.5;
}

.skill-hero h1 {
  font-size: clamp(26px, 3vw, 34px);
  margin-bottom: 8px;
}

.skill-list-section {
  min-width: 0;
}

.skill-empty {
  margin-top: 32px;
  color: var(--mac-text-secondary);
  text-align: center;
}

.skill-repo-list .skill-empty {
  margin-top: 0;
}

.skill-error {
  color: #f87171;
  margin-top: 16px;
}

.ghost-icon svg.spin {
  animation: skill-spin 1s linear infinite;
}

.ghost-icon:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.ghost-icon.danger {
  color: #ef4444;
}

.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border-width: 0;
}

html.dark .skill-card.available-card {
  background: var(--mac-surface); /* fallback for old WebKit */
  background: color-mix(in srgb, var(--mac-surface) 70%, transparent);
}

@media (max-width: 600px) {
  .skill-page {
    padding-inline: 18px;
  }

  .skill-matrix th:first-child {
    min-width: 130px;
  }
}

@keyframes skill-spin {
  from {
    transform: rotate(0deg);
  }
  to {
    transform: rotate(360deg);
  }
}
</style>

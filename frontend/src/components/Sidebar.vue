<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { fetchCurrentVersion } from '../services/version'

const router = useRouter()
const route = useRoute()
const { t } = useI18n()

interface NavItem {
  path: string
  icon: string
  labelKey: string
  isNew?: boolean
}

interface NavSection {
  labelKey: string
  items: NavItem[]
}

const homeItem: NavItem = { path: '/', icon: 'grid', labelKey: 'sidebar.home' }
const promptsItem: NavItem = { path: '/prompts', icon: 'message', labelKey: 'sidebar.prompts', isNew: true }
const mcpItem: NavItem = { path: '/mcp', icon: 'cube', labelKey: 'sidebar.mcp' }
const skillItem: NavItem = { path: '/skill', icon: 'spark', labelKey: 'sidebar.skill' }
const openClawItem: NavItem = { path: '/openclaw-config', icon: 'sliders', labelKey: 'sidebar.openclawConfig', isNew: true }
const hermesMemoryItem: NavItem = { path: '/hermes-memory', icon: 'book', labelKey: 'sidebar.hermesMemory', isNew: true }
const authItem: NavItem = { path: '/auth', icon: 'lock', labelKey: 'sidebar.auth' }
const availabilityItem: NavItem = { path: '/availability', icon: 'activity', labelKey: 'sidebar.availability', isNew: true }
const speedtestItem: NavItem = { path: '/speedtest', icon: 'zap', labelKey: 'sidebar.speedtest', isNew: true }
const envItem: NavItem = { path: '/env', icon: 'compass', labelKey: 'sidebar.env', isNew: true }
const logsItem: NavItem = { path: '/logs', icon: 'history', labelKey: 'sidebar.logs' }
const consoleItem: NavItem = { path: '/console', icon: 'terminal', labelKey: 'sidebar.console' }
const settingsItem: NavItem = { path: '/settings', icon: 'settings', labelKey: 'sidebar.settings' }

const navSections: NavSection[] = [
  {
    labelKey: 'sidebar.sections.workspace',
    items: [homeItem, promptsItem, mcpItem, skillItem, openClawItem, hermesMemoryItem, authItem],
  },
  {
    labelKey: 'sidebar.sections.monitoring',
    items: [availabilityItem, speedtestItem, envItem, logsItem],
  },
  {
    labelKey: 'sidebar.sections.system',
    items: [consoleItem, settingsItem],
  },
]

const SIDEBAR_COLLAPSED_KEY = 'sidebar-collapsed'
const VISITED_PAGES_KEY = 'visited-pages'

const appVersion = ref('...')
const isCollapsed = ref(false)
const isOnline = ref(true)
const visitedPages = ref<Set<string>>(new Set())

const currentPath = computed(() => route.path)
const appVersionDisplay = computed(() => (appVersion.value || '—').replace(/^v/i, '') || '—')
const onlineLabel = computed(() => t(isOnline.value ? 'sidebar.meta.online' : 'sidebar.meta.offline'))

const updateOnlineStatus = () => {
  if (typeof navigator === 'undefined') {
    isOnline.value = true
    return
  }
  isOnline.value = navigator.onLine
}

onMounted(async () => {
  const saved = localStorage.getItem(SIDEBAR_COLLAPSED_KEY)
  if (saved !== null) {
    isCollapsed.value = saved === 'true'
  }

  const visitedJson = localStorage.getItem(VISITED_PAGES_KEY)
  if (visitedJson) {
    try {
      visitedPages.value = new Set(JSON.parse(visitedJson))
    } catch {
      visitedPages.value = new Set()
    }
  }

  markAsVisited(route.path)
  updateOnlineStatus()
  window.addEventListener('online', updateOnlineStatus)
  window.addEventListener('offline', updateOnlineStatus)

  try {
    appVersion.value = await fetchCurrentVersion()
  } catch {
    appVersion.value = 'v?.?.?'
  }
})

onBeforeUnmount(() => {
  window.removeEventListener('online', updateOnlineStatus)
  window.removeEventListener('offline', updateOnlineStatus)
})

watch(
  () => route.path,
  (newPath) => {
    markAsVisited(newPath)
  },
)

function markAsVisited(path: string) {
  if (!visitedPages.value.has(path)) {
    visitedPages.value.add(path)
    localStorage.setItem(VISITED_PAGES_KEY, JSON.stringify([...visitedPages.value]))
  }
}

function shouldShowNew(item: NavItem): boolean {
  return item.isNew === true && !visitedPages.value.has(item.path)
}

function toggleCollapse() {
  isCollapsed.value = !isCollapsed.value
  localStorage.setItem(SIDEBAR_COLLAPSED_KEY, String(isCollapsed.value))
}

function navigate(path: string) {
  router.push(path)
}
</script>

<template>
  <nav class="mac-sidebar" :class="{ collapsed: isCollapsed }">
    <div class="sidebar-shell">
      <header class="sidebar-header">
        <div class="brand-lockup">
          <div class="brand-mark" aria-hidden="true">
            <svg viewBox="0 0 24 24" class="brand-mark__icon" fill="currentColor">
              <path d="M12 4.2 13.7 8.8 18.3 10.5 13.7 12.2 12 16.8 10.3 12.2 5.7 10.5 10.3 8.8 12 4.2Z" />
              <path d="M18.2 4.5 18.9 6.4 20.8 7.1 18.9 7.8 18.2 9.7 17.5 7.8 15.6 7.1 17.5 6.4 18.2 4.5Z" />
            </svg>
          </div>
          <div v-if="!isCollapsed" class="brand-copy">
            <span class="brand-name">Code Switch R</span>
          </div>
        </div>

        <button
          class="collapse-btn"
          type="button"
          :title="isCollapsed ? 'Expand sidebar' : 'Collapse sidebar'"
          :aria-label="isCollapsed ? 'Expand sidebar' : 'Collapse sidebar'"
          @click="toggleCollapse"
        >
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round">
            <polyline v-if="isCollapsed" points="9 18 15 12 9 6" />
            <polyline v-else points="15 18 9 12 15 6" />
          </svg>
        </button>
      </header>

      <div class="nav-groups">
        <section v-for="section in navSections" :key="section.labelKey" class="nav-section">
          <p v-if="!isCollapsed" class="section-label">
            {{ t(section.labelKey) }}
          </p>

          <div class="section-items">
            <button
              v-for="item in section.items"
              :key="item.path"
              class="nav-item"
              :class="{ active: currentPath === item.path }"
              :title="isCollapsed ? t(item.labelKey) : ''"
              :aria-current="currentPath === item.path ? 'page' : undefined"
              @click="navigate(item.path)"
            >
              <span class="nav-item__icon-shell" aria-hidden="true">
                <svg
                  v-if="item.icon === 'grid'"
                  class="nav-icon"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                >
                  <rect x="4" y="4" width="6" height="6" rx="1.5" />
                  <rect x="14" y="4" width="6" height="6" rx="1.5" />
                  <rect x="4" y="14" width="6" height="6" rx="1.5" />
                  <rect x="14" y="14" width="6" height="6" rx="1.5" />
                </svg>

                <svg
                  v-else-if="item.icon === 'message'"
                  class="nav-icon"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                >
                  <path d="M6 6.5h12a2 2 0 0 1 2 2v7a2 2 0 0 1-2 2H11l-4.5 3v-3H6a2 2 0 0 1-2-2v-7a2 2 0 0 1 2-2Z" />
                </svg>

                <svg
                  v-else-if="item.icon === 'cube'"
                  class="nav-icon"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                >
                  <path d="M12 3.8 19 7.8 12 11.8 5 7.8 12 3.8Z" />
                  <path d="M19 7.8v8.4L12 20.2V11.8" />
                  <path d="M5 7.8v8.4L12 20.2" />
                </svg>

                <svg
                  v-else-if="item.icon === 'spark'"
                  class="nav-icon"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                >
                  <path d="M12 3.5 13.7 8.3 18.5 10 13.7 11.7 12 16.5 10.3 11.7 5.5 10 10.3 8.3 12 3.5Z" />
                  <path d="M18.5 15.5 19.3 17.7 21.5 18.5 19.3 19.3 18.5 21.5 17.7 19.3 15.5 18.5 17.7 17.7 18.5 15.5Z" />
                </svg>

                <svg
                  v-else-if="item.icon === 'sliders'"
                  class="nav-icon"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                >
                  <path d="M4 7h9M17 7h3" />
                  <circle cx="15" cy="7" r="2" />
                  <path d="M4 17h3M11 17h9" />
                  <circle cx="9" cy="17" r="2" />
                </svg>

                <svg
                  v-else-if="item.icon === 'book'"
                  class="nav-icon"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                >
                  <path d="M4 5.5A1.5 1.5 0 0 1 5.5 4H10a2 2 0 0 1 2 2v13a2 2 0 0 0-2-2H5.5A1.5 1.5 0 0 1 4 15.5Z" />
                  <path d="M20 5.5A1.5 1.5 0 0 0 18.5 4H14a2 2 0 0 0-2 2v13a2 2 0 0 1 2-2h4.5a1.5 1.5 0 0 0 1.5-1.5Z" />
                </svg>

                <svg
                  v-else-if="item.icon === 'lock'"
                  class="nav-icon"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                >
                  <rect x="5" y="10" width="14" height="10" rx="2" />
                  <path d="M8.5 10V7.5a3.5 3.5 0 0 1 7 0V10" />
                  <path d="M12 14v2" />
                </svg>

                <svg
                  v-else-if="item.icon === 'activity'"
                  class="nav-icon"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                >
                  <polyline points="2 12 6.5 12 9.2 5 14.4 19 17 12 22 12" />
                </svg>

                <svg
                  v-else-if="item.icon === 'zap'"
                  class="nav-icon"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                >
                  <polygon points="13 2.8 4.5 13.2 11.7 13.2 10.9 21.2 19.5 10.8 12.3 10.8 13 2.8" />
                </svg>

                <svg
                  v-else-if="item.icon === 'compass'"
                  class="nav-icon"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                >
                  <circle cx="12" cy="12" r="8" />
                  <path d="M14.8 9.2 13 13l-3.8 1.8L11 11l3.8-1.8Z" />
                </svg>

                <svg
                  v-else-if="item.icon === 'history'"
                  class="nav-icon"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                >
                  <path d="M4.5 12A7.5 7.5 0 1 0 7 6.4" />
                  <polyline points="4 4 4 9 9 9" />
                  <path d="M12 8v4l2.5 1.5" />
                </svg>

                <svg
                  v-else-if="item.icon === 'terminal'"
                  class="nav-icon"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                >
                  <polyline points="4 7 9 12 4 17" />
                  <line x1="12" y1="18" x2="20" y2="18" />
                </svg>

                <svg
                  v-else-if="item.icon === 'settings'"
                  class="nav-icon"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                >
                  <circle cx="12" cy="12" r="3.2" />
                  <path
                    d="M19.4 15a1.6 1.6 0 0 0 .3 1.8l.1.1a2 2 0 0 1 0 2.8 2 2 0 0 1-2.8 0l-.1-.1a1.6 1.6 0 0 0-1.8-.3 1.6 1.6 0 0 0-1 1.5V21a2 2 0 0 1-4 0v-.1a1.6 1.6 0 0 0-1-1.5 1.6 1.6 0 0 0-1.8.3l-.1.1a2 2 0 0 1-2.8 0 2 2 0 0 1 0-2.8l.1-.1a1.6 1.6 0 0 0 .3-1.8 1.6 1.6 0 0 0-1.5-1H3a2 2 0 0 1 0-4h.1a1.6 1.6 0 0 0 1.5-1 1.6 1.6 0 0 0-.3-1.8l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.6 1.6 0 0 0 1.8.3H9a1.6 1.6 0 0 0 1-1.5V3a2 2 0 0 1 4 0v.1a1.6 1.6 0 0 0 1 1.5 1.6 1.6 0 0 0 1.8-.3l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.6 1.6 0 0 0-.3 1.8V9a1.6 1.6 0 0 0 1.5 1h.1a2 2 0 0 1 0 4h-.1a1.6 1.6 0 0 0-1.5 1Z"
                  />
                </svg>
              </span>

              <span v-if="!isCollapsed" class="nav-label">{{ t(item.labelKey) }}</span>
              <span v-if="shouldShowNew(item) && !isCollapsed" class="nav-item__badge">NEW</span>
            </button>
          </div>
        </section>
      </div>

      <footer v-if="!isCollapsed" class="sidebar-footer">
        <div class="status-card">
          <div class="status-card__block">
            <span class="status-card__label">{{ t('sidebar.meta.version') }}</span>
            <span class="status-card__value">{{ appVersionDisplay }}</span>
          </div>

          <div class="status-pill" :class="{ offline: !isOnline }">
            <span class="status-pill__dot"></span>
            <span class="status-pill__text">{{ onlineLabel }}</span>
          </div>
        </div>
      </footer>
    </div>
  </nav>
</template>

<style scoped>
.mac-sidebar {
  position: relative;
  z-index: 10;

  --sidebar-shell-bg: linear-gradient(
    180deg,
    color-mix(in srgb, var(--mac-surface) 82%, #05070c 18%) 0%,
    color-mix(in srgb, var(--mac-surface-strong) 78%, #03060c 22%) 100%
  );
  --sidebar-shell-border: color-mix(in srgb, var(--mac-border) 92%, rgba(255, 255, 255, 0.02));
  --sidebar-shell-shadow: 0 26px 58px rgba(2, 6, 23, 0.2);
  --sidebar-shell-highlight: rgba(255, 255, 255, 0.08);
  --sidebar-shell-glow-primary: rgba(59, 130, 246, 0.1);
  --sidebar-shell-glow-secondary: rgba(34, 211, 238, 0.06);
  --sidebar-text: var(--mac-text);
  --sidebar-muted: color-mix(in srgb, var(--mac-text-secondary) 90%, transparent);
  --sidebar-section: color-mix(in srgb, var(--mac-text-secondary) 76%, transparent);
  --nav-hover-bg: rgba(255, 255, 255, 0.02);
  --nav-hover-border: rgba(255, 255, 255, 0);
  --nav-active-bg: rgba(59, 130, 246, 0.06);
  --nav-active-border: rgba(59, 130, 246, 0.1);
  --nav-active-shadow: inset 0 0 12px rgba(59, 130, 246, 0.05);
  --nav-active-text: #60a5fa;
  --nav-indicator: #3b82f6;
  --nav-indicator-glow: rgba(59, 130, 246, 0.8);
  --icon-active-bg: transparent;
  --icon-active-ring: transparent;
  --icon-active-glow: rgba(59, 130, 246, 0.18);
  --badge-bg: color-mix(in srgb, var(--mac-accent) 16%, transparent);
  --badge-text: color-mix(in srgb, #d6ebff 38%, var(--mac-accent) 62%);
  --status-card-bg: color-mix(in srgb, var(--mac-surface) 52%, transparent);
  --status-card-border: color-mix(in srgb, var(--mac-border) 96%, transparent);
  --status-card-shadow: 0 18px 36px rgba(15, 23, 42, 0.12);
  --status-online: #10b981;
  --status-online-glow: rgba(16, 185, 129, 0.4);
  --status-offline: #f59e0b;
  --status-offline-glow: rgba(245, 158, 11, 0.42);
  --scrollbar-thumb: rgba(100, 116, 139, 0.28);
  width: 228px;
  min-width: 228px;
  height: 100%;
  padding: 0;
  display: flex;
  flex-direction: column;
  box-sizing: border-box;
  flex-shrink: 0;
  color: var(--sidebar-text);
  transition: width 0.24s ease, min-width 0.24s ease, padding 0.24s ease;
}

:global(html.dark) .mac-sidebar {
  --sidebar-shell-bg: linear-gradient(180deg, rgba(12, 15, 22, 0.88) 0%, rgba(5, 7, 12, 0.96) 100%);
  --sidebar-shell-border: rgba(255, 255, 255, 0.06);
  --sidebar-shell-shadow: 0 28px 60px rgba(2, 6, 23, 0.52);
  --sidebar-shell-highlight: rgba(255, 255, 255, 0.05);
  --sidebar-shell-glow-primary: rgba(96, 165, 250, 0.1);
  --sidebar-shell-glow-secondary: rgba(56, 189, 248, 0.05);
  --sidebar-text: #f8fafc;
  --sidebar-muted: rgba(148, 163, 184, 0.88);
  --sidebar-section: rgba(120, 131, 155, 0.76);
  --nav-hover-bg: rgba(255, 255, 255, 0.02);
  --nav-hover-border: rgba(255, 255, 255, 0);
  --nav-active-bg: rgba(59, 130, 246, 0.06);
  --nav-active-border: rgba(59, 130, 246, 0.1);
  --nav-active-shadow: inset 0 0 12px rgba(59, 130, 246, 0.05);
  --nav-active-text: #60a5fa;
  --nav-indicator: #3b82f6;
  --nav-indicator-glow: rgba(59, 130, 246, 0.8);
  --icon-active-bg: transparent;
  --icon-active-ring: transparent;
  --icon-active-glow: rgba(59, 130, 246, 0.24);
  --badge-bg: rgba(125, 211, 252, 0.08);
  --badge-text: #9bd4ff;
  --status-card-bg: rgba(12, 15, 24, 0.66);
  --status-card-border: rgba(255, 255, 255, 0.06);
  --status-card-shadow: 0 16px 38px rgba(0, 0, 0, 0.38);
  --scrollbar-thumb: rgba(148, 163, 184, 0.18);
}

.mac-sidebar.collapsed {
  width: 72px;
  min-width: 72px;
}

.sidebar-shell {
  position: relative;
  display: flex;
  flex: 1;
  min-height: 0;
  flex-direction: column;
  border-radius: 24px;
  background: var(--sidebar-shell-bg);
  border: 1px solid var(--sidebar-shell-border);
  box-shadow: inset 0 1px 0 var(--sidebar-shell-highlight), var(--sidebar-shell-shadow);
  overflow: hidden;
  backdrop-filter: blur(28px) saturate(150%);
  -webkit-backdrop-filter: blur(28px) saturate(150%);
}

.sidebar-shell::before,
.sidebar-shell::after {
  content: '';
  position: absolute;
  inset: 0;
  pointer-events: none;
}

.sidebar-shell::before {
  background:
    radial-gradient(circle at 18% 8%, var(--sidebar-shell-glow-primary) 0%, transparent 34%),
    radial-gradient(circle at 84% 100%, var(--sidebar-shell-glow-secondary) 0%, transparent 26%);
}

.sidebar-shell::after {
  inset: 1px;
  border-radius: 23px;
  border: 1px solid rgba(255, 255, 255, 0.02);
}

.sidebar-header {
  position: relative;
  z-index: 1;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 40px 12px 14px 10px;
  -webkit-app-region: drag;
}

.brand-lockup {
  display: flex;
  flex: 1;
  min-width: 0;
  align-items: center;
  gap: 8px;
}

.brand-mark {
  position: relative;
  width: 28px;
  height: 28px;
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(180deg, #5d8dff 0%, #406cff 100%);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.2),
    0 10px 24px rgba(64, 108, 255, 0.3);
  color: #eef6ff;
  flex-shrink: 0;
}

.brand-mark::after {
  content: '';
  position: absolute;
  inset: 0;
  border-radius: inherit;
  background: rgba(59, 130, 246, 0.42);
  filter: blur(12px);
  opacity: 0.2;
  transform: scale(1.18);
  z-index: -1;
  animation: sidebar-brand-pulse 3.2s ease-in-out infinite;
}

.brand-mark__icon {
  width: 15px;
  height: 15px;
  fill: currentColor;
}

.brand-copy {
  display: flex;
  min-width: 0;
  flex-direction: column;
  flex: 1;
}

.brand-name {
  font-size: 0.92rem;
  font-weight: 700;
  letter-spacing: -0.02em;
  line-height: 1.1;
  color: var(--sidebar-text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.collapse-btn {
  width: 24px;
  height: 24px;
  border: 1px solid transparent;
  border-radius: 8px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: transparent;
  color: var(--sidebar-muted);
  cursor: pointer;
  transition:
    color 0.22s ease,
    background 0.22s ease,
    border-color 0.22s ease,
    transform 0.22s ease;
  flex-shrink: 0;
  -webkit-app-region: no-drag;
}

.collapse-btn:hover {
  color: var(--sidebar-text);
  background: var(--nav-hover-bg);
  border-color: var(--nav-hover-border);
  transform: translateX(-1px);
}

.collapse-btn svg {
  width: 13px;
  height: 13px;
  stroke-width: 1.7;
}

.nav-groups {
  position: relative;
  z-index: 1;
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  overflow-x: hidden;
  padding: 4px 10px 10px 0;
  margin-right: 0;
  scrollbar-width: none;
  -ms-overflow-style: none;
}

.nav-groups:hover {
  scrollbar-width: thin;
}

.nav-groups::-webkit-scrollbar {
  width: 0;
}

.nav-groups:hover::-webkit-scrollbar {
  width: 6px;
}

.nav-groups::-webkit-scrollbar-thumb {
  border-radius: 999px;
  background: var(--scrollbar-thumb);
}

.nav-groups::-webkit-scrollbar-track {
  background: transparent;
}

.nav-section + .nav-section {
  margin-top: 16px;
}

.section-label {
  margin: 0 0 8px;
  padding: 0 0 0 8px;
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0.16em;
  color: var(--sidebar-section);
}

.section-items {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding-right: 8px;
}

.nav-item {
  position: relative;
  width: calc(100% - 2px);
  box-sizing: border-box;
  min-height: 42px;
  padding: 7px 10px 7px 6px;
  display: flex;
  align-items: center;
  gap: 8px;
  border: 1px solid transparent;
  border-radius: 14px;
  background: transparent;
  color: var(--sidebar-muted);
  text-align: left;
  cursor: pointer;
  overflow: hidden;
  margin-left: 0;
  transition:
    transform 0.22s ease,
    color 0.22s ease,
    background 0.22s ease,
    border-color 0.22s ease,
    box-shadow 0.22s ease;
}

.nav-item::before {
  content: '';
  position: absolute;
  left: 0;
  top: 9px;
  bottom: 9px;
  width: 2px;
  border-radius: 999px;
  background: var(--nav-indicator);
  box-shadow: 0 0 14px var(--nav-indicator-glow);
  opacity: 0;
  transform: scaleY(0.35);
  transition: opacity 0.22s ease, transform 0.22s ease;
}

.nav-item:hover {
  color: var(--sidebar-text);
  background: var(--nav-hover-bg);
  border-color: var(--nav-hover-border);
  transform: translateX(1px);
}

.nav-item.active {
  z-index: 2;
  color: var(--nav-active-text);
  background: var(--nav-active-bg);
  border-color: var(--nav-active-border);
  box-shadow: var(--nav-active-shadow);
}

.nav-item.active::after {
  content: '';
  position: absolute;
  inset: 0;
  border-radius: inherit;
  border: 1px solid var(--nav-active-border);
  pointer-events: none;
}

.nav-item.active::before {
  opacity: 1;
  transform: scaleY(1);
}

.nav-item__icon-shell {
  width: 26px;
  height: 26px;
  border-radius: 9px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  color: currentColor;
  transition:
    transform 0.22s ease,
    color 0.22s ease,
    background 0.22s ease,
    box-shadow 0.22s ease,
    filter 0.22s ease;
}

.nav-item:hover .nav-item__icon-shell {
  transform: scale(1.1);
}

.nav-item.active .nav-item__icon-shell {
  color: currentColor;
  background: var(--icon-active-bg);
  box-shadow: none;
  filter: drop-shadow(0 0 8px var(--icon-active-glow));
  transform: scale(1.1);
}

.nav-icon {
  width: 15px;
  height: 15px;
  stroke-width: 1.8;
}

.nav-label {
  flex: 1;
  min-width: 0;
  font-size: 13px;
  font-weight: 600;
  letter-spacing: 0.015em;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.nav-item__badge {
  padding: 3px 7px;
  border-radius: 999px;
  background: var(--badge-bg);
  color: var(--badge-text);
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.14em;
  line-height: 1;
}

.nav-item.active .nav-item__badge {
  background: rgba(125, 211, 252, 0.12);
  color: #dbeafe;
}

.sidebar-footer {
  position: relative;
  z-index: 1;
  padding: 12px 10px 12px;
  border-top: 1px solid color-mix(in srgb, var(--mac-border) 88%, transparent);
}

.status-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 10px 11px;
  border-radius: 16px;
  border: 1px solid var(--status-card-border);
  background: var(--status-card-bg);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.05), var(--status-card-shadow);
  backdrop-filter: blur(16px);
  -webkit-backdrop-filter: blur(16px);
}

.status-card__block {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.status-card__label {
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0.16em;
  line-height: 1;
  color: var(--sidebar-section);
  text-transform: uppercase;
}

.status-card__value {
  font-size: 0.94rem;
  font-weight: 700;
  letter-spacing: 0.04em;
  color: var(--sidebar-text);
}

.status-pill {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.08em;
  color: var(--sidebar-text);
  white-space: nowrap;
}

.status-pill__text {
  text-transform: uppercase;
}

.status-pill__dot {
  width: 7px;
  height: 7px;
  border-radius: 999px;
  background: var(--status-online);
  box-shadow: 0 0 0 4px color-mix(in srgb, var(--status-online-glow) 30%, transparent), 0 0 18px var(--status-online-glow);
  flex-shrink: 0;
}

.status-pill.offline .status-pill__dot {
  background: var(--status-offline);
  box-shadow: 0 0 0 4px color-mix(in srgb, var(--status-offline-glow) 28%, transparent), 0 0 18px var(--status-offline-glow);
}

@keyframes sidebar-brand-pulse {
  0%,
  100% {
    opacity: 0.14;
    transform: scale(1.12);
  }

  50% {
    opacity: 0.24;
    transform: scale(1.26);
  }
}

.mac-sidebar.collapsed .sidebar-header {
  flex-direction: column;
  justify-content: center;
  padding: 40px 8px 12px;
}

.mac-sidebar.collapsed .brand-lockup {
  justify-content: center;
}

.mac-sidebar.collapsed .nav-groups {
  padding-inline: 4px;
}

.mac-sidebar.collapsed .nav-section + .nav-section {
  margin-top: 10px;
}

.mac-sidebar.collapsed .nav-item {
  width: 40px;
  min-height: 40px;
  justify-content: center;
  padding: 0;
  margin-inline: auto;
}

.mac-sidebar.collapsed .nav-item::before {
  left: 2px;
}

@media (max-width: 960px) {
  .mac-sidebar,
  .mac-sidebar.collapsed {
    width: 100%;
    min-width: 100%;
    height: auto;
  }

  .sidebar-shell {
    border-radius: 22px;
  }

  .sidebar-header {
    padding-top: 18px;
  }

  .mac-sidebar.collapsed .sidebar-header {
    flex-direction: row;
    justify-content: space-between;
  }
}
</style>

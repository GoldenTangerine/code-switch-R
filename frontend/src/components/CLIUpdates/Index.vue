<!--
@name: CLI 更新页面
@Descripttion: 每次打开页面时检查 CLI 的本机版本与最新版本。
@version: 1.0.0
@Author: sm
@Date: 2026-11-20 14:40:19
@LastEditTime: 2026-11-20 14:40:19
@FilePath: frontend/src/components/CLIUpdates/Index.vue
-->
<script setup lang="ts">
import { onActivated, onDeactivated, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { checkCLIVersions, type CLIVersionInfo } from '../../services/cliUpdates'

const { t } = useI18n()
const versions = ref<CLIVersionInfo[]>([])
const loading = ref(true)
const failed = ref(false)
type VersionCheck = ReturnType<typeof checkCLIVersions>
let activeCheck: VersionCheck | undefined

async function refresh() {
  if (activeCheck) return
  loading.value = true
  failed.value = false
  let request: VersionCheck | undefined
  try {
    request = checkCLIVersions()
    activeCheck = request
    const results = await request
    if (activeCheck === request) versions.value = results
  } catch {
    if (activeCheck === request) failed.value = true
  } finally {
    if (activeCheck === request) {
      activeCheck = undefined
      loading.value = false
    }
  }
}

onActivated(refresh)
onDeactivated(() => {
  const request = activeCheck
  activeCheck = undefined
  void request?.cancel()
})
</script>

<template>
  <section class="cli-updates" aria-labelledby="cli-updates-title">
    <header class="updates-header">
      <h1 id="cli-updates-title">{{ t('sidebar.cliUpdates') }}</h1>
      <button type="button" class="secondary-btn" :disabled="loading" @click="refresh">
        {{ t('cliUpdates.refresh') }}
      </button>
    </header>

    <p v-if="loading" class="updates-message" role="status">
      {{ t(versions.length ? 'cliUpdates.refreshing' : 'cliUpdates.checking') }}
    </p>
    <p v-else-if="failed" class="updates-message" role="alert">
      {{ t(versions.length ? 'cliUpdates.refreshFailed' : 'cliUpdates.failed') }}
    </p>
    <div v-if="versions.length" class="updates-results" aria-live="polite">
      <table aria-labelledby="cli-updates-title">
        <thead>
          <tr>
            <th scope="col">{{ t('cliUpdates.app') }}</th>
            <th scope="col">{{ t('cliUpdates.installed') }}</th>
            <th scope="col">{{ t('cliUpdates.latest') }}</th>
            <th scope="col">{{ t('cliUpdates.statusLabel') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="item in versions" :key="item.command">
            <th scope="row">
              {{ item.name }}
              <code class="tool-command">{{ item.command }}</code>
            </th>
            <td><code>{{ item.installedVersion || '—' }}</code></td>
            <td><code>{{ item.latestVersion || '—' }}</code></td>
            <td>
              <span class="update-status" :class="{ 'has-update': item.status === 'updateAvailable' }">
                {{ t(`cliUpdates.status.${item.status}`) }}
              </span>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>
</template>

<style scoped>
.cli-updates {
  position: relative;
  max-width: 1040px;
  margin: 0 auto;
  padding: clamp(16px, 3vw, 32px);
}

.updates-header {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 20px;
}

.updates-header h1 {
  margin: 0;
  font-size: 1.5rem;
  font-weight: 650;
}

.updates-results,
.updates-message {
  border: 1px solid var(--mac-border);
  border-radius: 16px;
  background: var(--mac-surface);
}

.updates-message {
  margin-bottom: 12px;
  padding: 18px;
  color: var(--mac-text-secondary);
}

table {
  width: 100%;
  border-collapse: collapse;
  table-layout: fixed;
  font-size: 0.875rem;
}

th,
td {
  padding: 10px clamp(6px, 2vw, 16px);
  text-align: start;
  overflow-wrap: anywhere;
}

thead th {
  font-size: 0.8125rem;
  font-weight: 600;
  color: var(--mac-text-secondary);
}

thead th:first-child {
  width: 30%;
}

tbody tr {
  border-top: 1px solid var(--mac-divider);
}

tbody th {
  font-weight: 600;
}

code {
  font-variant-numeric: tabular-nums;
}

.tool-command {
  display: block;
  font-size: 0.75rem;
  font-weight: 400;
  color: var(--mac-text-secondary);
}

.update-status {
  color: var(--mac-text-secondary);
}

.update-status.has-update {
  display: inline-block;
  padding: 3px 8px;
  border-radius: 8px;
  background: color-mix(in srgb, var(--mac-accent) 12%, var(--mac-surface));
  color: var(--mac-text);
  font-weight: 650;
}
</style>

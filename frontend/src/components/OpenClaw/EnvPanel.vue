<!--
 * @name: OpenClaw 环境变量面板
 * @Descripttion: 编辑 OpenClaw settings 的 vars 与 shellEnv 两组键值对环境变量，保存时整体写回原生配置
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-17 00:00:00
 * @LastEditTime: 2026-08-17 00:00:00
 * @FilePath: frontend/src/components/OpenClaw/EnvPanel.vue
-->
<template>
  <div class="claw-panel">
    <header class="claw-panel__header">
      <div>
        <h2 class="claw-panel__title">{{ t('components.openclawConfig.env.title') }}</h2>
        <p class="claw-panel__hint">{{ t('components.openclawConfig.env.hint') }}</p>
      </div>
      <BaseButton :disabled="loading || saving" type="button" variant="outline" @click="loadConfig">
        {{ t('components.openclawConfig.common.reload') }}
      </BaseButton>
    </header>

    <div v-if="errorMessage" class="alert-error">{{ errorMessage }}</div>
    <div v-if="loading" class="claw-empty">{{ t('components.openclawConfig.common.loading') }}</div>

    <template v-else>
      <section class="claw-panel__group">
        <h3 class="claw-panel__group-title">{{ t('components.openclawConfig.env.varsTitle') }}</h3>
        <p class="claw-panel__group-hint">{{ t('components.openclawConfig.env.varsHint') }}</p>
        <div class="env-table">
          <div v-for="entry in varsEntries" :key="entry.id" class="env-row">
            <BaseInput
              v-model="entry.key"
              type="text"
              :placeholder="t('components.openclawConfig.env.keyPlaceholder')"
              :disabled="saving"
            />
            <BaseInput
              v-model="entry.value"
              type="text"
              :placeholder="t('components.openclawConfig.env.valuePlaceholder')"
              :disabled="saving"
            />
            <button
              class="ghost-icon"
              type="button"
              :aria-label="t('components.openclawConfig.env.remove')"
              :disabled="saving"
              @click="removeEntry(varsEntries, entry.id)"
            >
              ✕
            </button>
          </div>
        </div>
        <BaseButton variant="outline" type="button" class="env-add" :disabled="saving" @click="addEntry(varsEntries)">
          {{ t('components.openclawConfig.env.add') }}
        </BaseButton>
      </section>

      <section class="claw-panel__group">
        <h3 class="claw-panel__group-title">{{ t('components.openclawConfig.env.shellEnvTitle') }}</h3>
        <p class="claw-panel__group-hint">{{ t('components.openclawConfig.env.shellEnvHint') }}</p>
        <div class="env-table">
          <div v-for="entry in shellEnvEntries" :key="entry.id" class="env-row">
            <BaseInput
              v-model="entry.key"
              type="text"
              :placeholder="t('components.openclawConfig.env.keyPlaceholder')"
              :disabled="saving"
            />
            <BaseInput
              v-model="entry.value"
              type="text"
              :placeholder="t('components.openclawConfig.env.valuePlaceholder')"
              :disabled="saving"
            />
            <button
              class="ghost-icon"
              type="button"
              :aria-label="t('components.openclawConfig.env.remove')"
              :disabled="saving"
              @click="removeEntry(shellEnvEntries, entry.id)"
            >
              ✕
            </button>
          </div>
        </div>
        <BaseButton variant="outline" type="button" class="env-add" :disabled="saving" @click="addEntry(shellEnvEntries)">
          {{ t('components.openclawConfig.env.add') }}
        </BaseButton>
      </section>

      <footer class="claw-panel__actions">
        <BaseButton type="button" :disabled="saving" @click="saveConfig">
          {{ saving ? t('components.openclawConfig.common.saving') : t('components.openclawConfig.common.save') }}
        </BaseButton>
      </footer>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseButton from '../common/BaseButton.vue'
import BaseInput from '../common/BaseInput.vue'
import { getOpenClawEnvConfig, setOpenClawEnvConfig } from '../../services/openClaw'
import { openClawPanelDirty } from './panelDirty'
import { showToast } from '../../utils/toast'

type EnvEntry = {
  id: number
  key: string
  value: string
}

const { t } = useI18n()

const loading = ref(false)
const saving = ref(false)
const errorMessage = ref('')
const varsEntries = ref<EnvEntry[]>([createEnvEntry()])
const shellEnvEntries = ref<EnvEntry[]>([createEnvEntry()])

// 未保存跟踪：以加载完成时的内容快照为基线，输入/增删行后偏离即视为 dirty
const entriesSnapshot = (): string => JSON.stringify({ vars: varsEntries.value, shellEnv: shellEnvEntries.value })
const baselineSnapshot = ref<string | null>(null)
const isDirty = computed(() => (
  baselineSnapshot.value !== null && entriesSnapshot() !== baselineSnapshot.value
))

watch(isDirty, (dirty) => {
  openClawPanelDirty.env = dirty
})

let envEntryId = 0

function createEnvEntry(key = '', value = ''): EnvEntry {
  return {
    id: ++envEntryId,
    key,
    value,
  }
}

const buildEntries = (source: Record<string, string>): EnvEntry[] => {
  const entries = Object.entries(source ?? {})
  if (!entries.length) return [createEnvEntry()]
  return entries.map(([key, value]) => createEnvEntry(key, value))
}

const addEntry = (list: EnvEntry[]) => {
  list.push(createEnvEntry())
}

const removeEntry = (list: EnvEntry[], id: number) => {
  const index = list.findIndex((entry) => entry.id === id)
  if (index !== -1) {
    list.splice(index, 1)
  }
}

// 行转对象：空 key 行忽略；重复 key 报错，避免静默覆盖
const entriesToRecord = (list: EnvEntry[]): Record<string, string> | null => {
  const record: Record<string, string> = {}
  for (const entry of list) {
    const key = entry.key.trim()
    if (!key) continue
    if (Object.prototype.hasOwnProperty.call(record, key)) {
      errorMessage.value = t('components.openclawConfig.env.duplicateKey', { key })
      return null
    }
    record[key] = entry.value
  }
  return record
}

const loadConfig = async () => {
  loading.value = true
  errorMessage.value = ''
  try {
    const config = await getOpenClawEnvConfig()
    varsEntries.value = buildEntries(config.vars)
    shellEnvEntries.value = buildEntries(config.shellEnv)
  } catch (error) {
    console.error('failed to load openclaw env config', error)
    errorMessage.value = t('components.openclawConfig.common.loadError')
  } finally {
    baselineSnapshot.value = entriesSnapshot()
    loading.value = false
  }
}

const saveConfig = async () => {
  if (saving.value) return
  errorMessage.value = ''

  const vars = entriesToRecord(varsEntries.value)
  if (vars === null) return
  const shellEnv = entriesToRecord(shellEnvEntries.value)
  if (shellEnv === null) return

  saving.value = true
  try {
    await setOpenClawEnvConfig({ vars, shellEnv })
    showToast(t('components.openclawConfig.common.saved'), 'success')
    await loadConfig()
  } catch (error) {
    console.error('failed to save openclaw env config', error)
    showToast(t('components.openclawConfig.common.saveError'), 'error')
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  void loadConfig()
})
</script>

<style scoped>
.claw-panel {
  display: flex;
  flex-direction: column;
  gap: 20px;
  padding: 20px;
  border-radius: 22px;
  border: 1px solid var(--mac-border);
  background: color-mix(in srgb, var(--mac-surface) 82%, transparent);
}

.claw-panel__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.claw-panel__title {
  margin: 0;
  font-size: 17px;
  font-weight: 700;
  color: var(--mac-text);
}

.claw-panel__hint {
  margin: 6px 0 0;
  font-size: 13px;
  color: var(--mac-text-secondary);
  line-height: 1.5;
}

.claw-panel__group {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding-top: 16px;
  border-top: 1px solid var(--mac-divider);
}

.claw-panel__group-title {
  margin: 0;
  font-size: 14px;
  font-weight: 700;
  color: var(--mac-text);
}

.claw-panel__group-hint {
  margin: 0;
  font-size: 12px;
  color: var(--mac-text-secondary);
  line-height: 1.5;
}

.env-table {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.env-row {
  display: grid;
  grid-template-columns: 1fr 1fr auto;
  gap: 8px;
  align-items: center;
}

.env-add {
  align-self: flex-start;
}

.claw-panel__actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  padding-top: 16px;
  border-top: 1px solid var(--mac-divider);
}

.claw-empty {
  text-align: center;
  padding: 24px;
  border: 1px dashed var(--mac-border);
  border-radius: 16px;
  color: var(--mac-text-secondary);
}

.alert-error {
  margin: 0;
  padding: 10px 14px;
  border-radius: 12px;
  border: 1px solid rgba(244, 67, 54, 0.35);
  background: rgba(244, 67, 54, 0.12);
  color: #ef4444;
  font-size: 13px;
}
</style>

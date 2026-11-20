<!--
@name: 本地技能卡片
@Descripttion: 展示本地用户级和项目级技能及其操作。
@version: 1.0.0
@Author: sm
@Date: 2026-11-20 15:31:00
@LastEditTime: 2026-11-20 15:31:00
@FilePath: frontend/src/components/Skill/SkillCard.vue
-->
<template>
  <article class="skill-card-installed">
    <div class="skill-card-header">
      <div v-if="skill.platform === 'claude' && !skill.read_only" class="skill-card-toggle">
        <button
          type="button"
          :class="['toggle-switch', { enabled: skill.enabled }]"
          :disabled="busy"
          role="switch"
          :aria-checked="skill.enabled"
          :aria-label="`${skill.name} · ${t('components.skill.actions.autoInvoke')}`"
          :title="t('components.skill.actions.autoInvoke')"
          @click="$emit('toggle', skill, !skill.enabled)"
        >
          <span class="toggle-slider"></span>
        </button>
      </div>

      <div class="skill-card-info">
        <div class="skill-card-meta">
          <h3 class="skill-card-title">{{ skill.name }}</h3>
          <!-- 目录名与技能名一致时不再重复展示 -->
          <span v-if="showDirectory" class="skill-card-dir" :title="skill.directory">{{ skill.directory }}</span>
        </div>
        <p class="skill-card-desc">
          {{ skill.description || t('components.skill.list.noDescription') }}
        </p>
        <div class="skill-card-badges">
          <span v-if="skill.license_file" class="skill-badge license">
            {{ t('components.skill.license.complete', { file: skill.license_file }) }}
          </span>
          <span v-if="skill.platform" class="skill-badge project">
            {{ t(`components.skill.platform.${skill.platform}`) }}
          </span>
          <span v-if="skill.read_only" class="skill-badge">{{ t('components.skill.actions.readOnly') }}</span>
        </div>
      </div>

      <div class="skill-card-actions">
        <button
          v-if="skill.install_location === 'user' && !skill.read_only"
          type="button"
          class="skill-action-btn"
          :disabled="busy"
          @click="$emit('import', skill)"
        >{{ t('components.skill.actions.import') }}</button>
        <button
          type="button"
          class="ghost-icon sm"
          :title="t('components.skill.actions.openFolder')"
          :aria-label="t('components.skill.actions.openFolder')"
          @click="$emit('folder', skill)"
        >
          <svg viewBox="0 0 24 24" aria-hidden="true">
            <path d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z" fill="none"
              stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" />
          </svg>
        </button>
        <button
          type="button"
          class="ghost-icon sm"
          :title="expanded ? t('components.skill.actions.hideContent') : t('components.skill.actions.viewContent')"
          :data-tooltip="expanded ? t('components.skill.actions.hideContent') : t('components.skill.actions.viewContent')"
          @click="$emit('expand', skill)"
        >
          <svg viewBox="0 0 24 24" aria-hidden="true" :class="{ rotated: expanded }">
            <path d="M6 9l6 6 6-6" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"
              stroke-linejoin="round" />
          </svg>
        </button>
        <button
          v-if="skill.readme_url"
          type="button"
          class="ghost-icon sm"
          :title="t('components.skill.actions.view')"
          :data-tooltip="t('components.skill.actions.view')"
          @click="$emit('view', skill.readme_url)"
        >
          <svg viewBox="0 0 24 24" aria-hidden="true">
            <path d="M12 5h7v7M19 5l-9 9" fill="none" stroke="currentColor" stroke-width="1.6"
              stroke-linecap="round" stroke-linejoin="round" />
            <path d="M11 6H7a2 2 0 00-2 2v9a2 2 0 002 2h9a2 2 0 002-2v-4" fill="none" stroke="currentColor"
              stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" />
          </svg>
        </button>
        <button
          v-if="!skill.read_only"
          type="button"
          class="ghost-icon sm danger"
          :title="t('components.skill.actions.uninstall')"
          :data-tooltip="t('components.skill.actions.uninstall')"
          :disabled="busy"
          @click="$emit('uninstall', skill)"
        >
          <svg viewBox="0 0 24 24" aria-hidden="true">
            <path d="M5 7h14M10 11v6M14 11v6M9 7V5h6v2" fill="none" stroke="currentColor" stroke-width="1.6"
              stroke-linecap="round" stroke-linejoin="round" />
            <path d="M6.5 7l-.5 12a2 2 0 002 2h8a2 2 0 002-2L17.5 7" fill="none" stroke="currentColor"
              stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" />
          </svg>
        </button>
      </div>
    </div>

    <transition name="expand">
      <div v-if="expanded" class="skill-card-content">
        <div v-if="loadingContent" class="skill-content-loading">
          {{ t('components.skill.actions.loading') }}
        </div>
        <pre v-else class="skill-content-pre">{{ content }}</pre>
      </div>
    </transition>
  </article>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { getSkillContent, type SkillSummary } from '../../services/skill'

const props = defineProps<{
  skill: SkillSummary
  expanded: boolean
  busy: boolean
}>()

defineEmits<{
  toggle: [skill: SkillSummary, enabled: boolean]
  expand: [skill: SkillSummary]
  uninstall: [skill: SkillSummary]
  view: [url: string]
  import: [skill: SkillSummary]
  folder: [skill: SkillSummary]
}>()

const { t } = useI18n()

const showDirectory = computed(() => props.skill.directory.toLowerCase() !== props.skill.name.toLowerCase())

const content = ref('')
const loadingContent = ref(false)

// Load content when expanded
watch(() => props.expanded, async (isExpanded) => {
  if (isExpanded && !content.value) {
    loadingContent.value = true
    try {
      content.value = await getSkillContent(
        props.skill.directory,
        props.skill.platform || 'claude',
        props.skill.install_location || 'user'
      )
    } catch (error) {
      console.error('failed to load skill content', error)
      content.value = t('components.skill.actions.loadFailed')
    } finally {
      loadingContent.value = false
    }
  }
})
</script>

<style scoped>
.skill-card-installed {
  background: var(--mac-surface-strong); /* fallback for old WebKit */
  background: color-mix(in srgb, var(--mac-surface) 90%, transparent);
  border: 1px solid var(--mac-border);
  border-radius: 16px;
  overflow: hidden;
  transition: border-color 0.18s ease;
}

.skill-card-installed:hover {
  border-color: color-mix(in srgb, var(--mac-accent) 30%, var(--mac-border));
}

.skill-card-header {
  display: flex;
  align-items: flex-start;
  gap: 16px;
  padding: 16px 18px 16px 20px;
}

.skill-card-toggle {
  flex-shrink: 0;
  padding-top: 1px;
}

.toggle-switch {
  position: relative;
  appearance: none;
  line-height: 0;
  width: 40px;
  height: 22px;
  padding: 0;
  border-radius: 999px;
  border: none;
  background: color-mix(in srgb, var(--mac-text) 16%, transparent);
  cursor: pointer;
  transition: background 0.2s ease;
}

.toggle-switch.enabled {
  background: var(--mac-accent);
}

.toggle-switch:focus-visible {
  outline: 2px solid color-mix(in srgb, var(--mac-accent) 60%, transparent);
  outline-offset: 2px;
}

.toggle-switch:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.toggle-slider {
  position: absolute;
  top: 2px;
  left: 2px;
  width: 18px;
  height: 18px;
  background: white;
  border-radius: 50%;
  transition: transform 0.2s ease;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.2);
}

.toggle-switch.enabled .toggle-slider {
  transform: translateX(18px);
}

.skill-card-info {
  flex: 1;
  min-width: 0;
}

.skill-card-meta {
  display: flex;
  align-items: baseline;
  flex-wrap: wrap;
  gap: 4px 10px;
  margin-bottom: 6px;
  min-height: 24px;
}

.skill-card-title {
  font-size: 15px;
  font-weight: 650;
  line-height: 24px;
  margin: 0;
  color: var(--mac-text);
  overflow-wrap: anywhere;
}

.skill-card-dir {
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 11.5px;
  color: var(--mac-text-secondary);
}

.skill-card-desc {
  max-width: 88ch;
  font-size: 13px;
  color: var(--mac-text-secondary);
  line-height: 1.6;
  margin: 0 0 10px;
}

.skill-card-badges {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.skill-card-badges:empty {
  display: none;
}

.skill-badge {
  font-size: 11.5px;
  line-height: 1.5;
  padding: 1px 8px;
  border-radius: 6px;
  border: 1px solid var(--mac-border);
  background: color-mix(in srgb, var(--mac-text) 5%, transparent);
  color: var(--mac-text-secondary);
}

.skill-badge.license {
  border-color: rgba(245, 158, 11, 0.28);
  background: rgba(245, 158, 11, 0.12);
  color: #d97706;
}

.skill-badge.project {
  border-color: rgba(59, 130, 246, 0.28);
  background: rgba(59, 130, 246, 0.12);
  color: #2563eb;
}

html.dark .skill-badge.license {
  color: #fbbf24;
}

html.dark .skill-badge.project {
  color: #60a5fa;
}

.skill-card-actions {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}

/* 主操作按钮：显式背景 + appearance:none，修复 WebKit 原生白底导致文字不可见 */
.skill-action-btn {
  appearance: none;
  display: inline-flex;
  align-items: center;
  width: auto;
  height: 30px;
  line-height: 1;
  padding: 0 12px;
  border: 1px solid color-mix(in srgb, var(--mac-accent) 40%, transparent);
  border-radius: 8px;
  background: color-mix(in srgb, var(--mac-accent) 12%, transparent);
  color: var(--mac-accent);
  font-size: 12.5px;
  font-weight: 600;
  white-space: nowrap;
  cursor: pointer;
  transition: background 0.18s ease, color 0.18s ease;
}

.skill-action-btn:hover:not(:disabled) {
  background: var(--mac-accent);
  color: #fff;
}

.skill-action-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.skill-card-actions .ghost-icon {
  width: 30px;
  height: 30px;
  border-radius: 8px;
}

.skill-card-actions .ghost-icon svg {
  width: 16px;
  height: 16px;
  transition: transform 0.2s ease;
}

.skill-card-actions .ghost-icon svg.rotated {
  transform: rotate(180deg);
}

.skill-card-actions .ghost-icon.danger {
  color: #ef4444;
}

.skill-card-actions .ghost-icon.danger:hover:not(:disabled) {
  background: rgba(239, 68, 68, 0.14);
}

.skill-card-actions .ghost-icon:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

/* Content Panel */
.skill-card-content {
  border-top: 1px solid var(--mac-border);
  background: var(--mac-surface-strong); /* fallback for old WebKit */
  background: color-mix(in srgb, var(--mac-surface) 50%, transparent);
  max-height: 400px;
  overflow: auto;
}

.skill-content-loading {
  padding: 16px 20px;
  color: var(--mac-text-secondary);
  font-size: 0.9rem;
}

.skill-content-pre {
  margin: 0;
  padding: 16px 20px;
  font-family: 'SF Mono', Monaco, 'Cascadia Code', monospace;
  font-size: 0.8rem;
  line-height: 1.5;
  white-space: pre-wrap;
  word-break: break-word;
  color: var(--mac-text);
}

/* Expand Transition */
.expand-enter-active,
.expand-leave-active {
  transition: all 0.2s ease;
  max-height: 400px;
}

.expand-enter-from,
.expand-leave-to {
  max-height: 0;
  opacity: 0;
}

@media (max-width: 640px) {
  .skill-card-header {
    flex-wrap: wrap;
  }

  .skill-card-actions {
    width: 100%;
    justify-content: flex-end;
  }
}

/* Dark Mode */
html.dark .skill-card-installed {
  background: var(--mac-surface); /* fallback for old WebKit */
  background: color-mix(in srgb, var(--mac-surface) 70%, transparent);
}

html.dark .skill-card-content {
  background: var(--mac-surface); /* fallback for old WebKit */
  background: color-mix(in srgb, var(--mac-surface) 30%, transparent);
}

</style>

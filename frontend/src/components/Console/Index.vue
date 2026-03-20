<script setup lang="ts">
import { ref, onMounted, onUnmounted, nextTick, computed, type Ref } from 'vue'
import { useRouter } from 'vue-router'
import { Call } from '@wailsio/runtime'
import BaseButton from '../common/BaseButton.vue'
import BaseModal from '../common/BaseModal.vue'
import { showToast } from '../../utils/toast'
import { extractErrorMessage } from '../../utils/error'
import { parseProviderErrorFromConsoleMessage, type ProviderErrorDetail } from '../../utils/providerError'
import { writeTextToClipboard } from '../../utils/clipboard'

interface ConsoleLog {
  timestamp: string
  level: string
  message: string
}

type LogLevel = 'INFO' | 'WARN' | 'ERROR' | 'DEBUG'
type LevelFilter = 'ALL' | LogLevel

interface ConsoleDisplayLog extends ConsoleLog {
  resolvedLevel: LogLevel
  diagnosticTags: string[]
  providerError: ProviderErrorDetail | null
}

const levelFilterOptions: Array<{ key: LevelFilter; label: string }> = [
  { key: 'ALL', label: '全部' },
  { key: 'ERROR', label: '错误' },
  { key: 'WARN', label: '警告' },
  { key: 'INFO', label: '信息' },
  { key: 'DEBUG', label: '调试' },
]

const router = useRouter()
const logs = ref<ConsoleLog[]>([])
const autoScroll = ref(true)
const loading = ref(false)
const logsContainer = ref<HTMLElement>()
const clearConfirmOpen = ref(false)
const clearing = ref(false)
const activeLevelFilter = ref<LevelFilter>('ALL')
const keywordQuery = ref('')
const hitErrorLinkMode = ref(false)
const copiedLogIdentity = ref('')
const copiedProviderErrorIdentity = ref('')
let refreshInterval: number | null = null

const goBack = () => {
  router.push('/')
}

const loadLogs = async () => {
  try {
    const result = await Call.ByName('codeswitch/services.ConsoleService.GetLogs')
    logs.value = result as ConsoleLog[]

    if (autoScroll.value) {
      await nextTick()
      scrollToBottom()
    }
  } catch (error) {
    console.error('加载控制台日志失败:', error)
  }
}

const openClearConfirm = () => {
  if (clearing.value) return
  clearConfirmOpen.value = true
}

const closeClearConfirm = () => {
  if (clearing.value) return
  clearConfirmOpen.value = false
}

const confirmClearLogs = async () => {
  if (clearing.value) return
  clearing.value = true
  try {
    await Call.ByName('codeswitch/services.ConsoleService.ClearLogs')
    logs.value = []
    showToast('清空成功', 'success')
    closeClearConfirm()
  } catch (error) {
    console.error('清空日志失败:', error)
    showToast(`清空失败：${extractErrorMessage(error)}`, 'error')
  } finally {
    clearing.value = false
  }
}

const scrollToBottom = () => {
  if (logsContainer.value) {
    logsContainer.value.scrollTop = logsContainer.value.scrollHeight
  }
}

const formatTimestamp = (timestamp: string) => {
  const date = new Date(timestamp)
  return date.toLocaleTimeString('zh-CN', { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

const normalizeLogLevel = (level: string): LogLevel => {
  const normalized = level.toUpperCase().trim()
  switch (normalized) {
    case 'ERROR':
    case 'ERR':
      return 'ERROR'
    case 'WARN':
    case 'WARNING':
      return 'WARN'
    case 'DEBUG':
      return 'DEBUG'
    default:
      return 'INFO'
  }
}

const inferLevelFromMessage = (message: string): LogLevel => {
  const normalizedMessage = message.toUpperCase()

  if (
    normalizedMessage.includes('[ERROR]') ||
    normalizedMessage.includes(' ERROR:') ||
    normalizedMessage.includes(' LEVEL=ERROR')
  ) {
    return 'ERROR'
  }

  if (
    normalizedMessage.includes('[WARN]') ||
    normalizedMessage.includes('[WARNING]') ||
    normalizedMessage.includes(' WARN:') ||
    normalizedMessage.includes(' LEVEL=WARN') ||
    normalizedMessage.includes(' LEVEL=WARNING')
  ) {
    return 'WARN'
  }

  if (
    normalizedMessage.includes('[DEBUG]') ||
    normalizedMessage.includes(' DEBUG:') ||
    normalizedMessage.includes(' LEVEL=DEBUG')
  ) {
    return 'DEBUG'
  }

  if (
    normalizedMessage.includes('[INFO]') ||
    normalizedMessage.includes(' INFO:') ||
    normalizedMessage.includes(' LEVEL=INFO')
  ) {
    return 'INFO'
  }

  return 'INFO'
}

const getResolvedLevel = (log: ConsoleLog, providerError: ProviderErrorDetail | null): LogLevel => {
  const baseLevel = normalizeLogLevel(log.level)
  const inferredLevel = inferLevelFromMessage(log.message)

  if (providerError != null) {
    return 'ERROR'
  }

  if (baseLevel === 'ERROR') {
    return 'ERROR'
  }

  if (inferredLevel !== 'INFO') {
    return inferredLevel
  }

  return baseLevel
}

const pushDiagnosticTag = (tags: string[], seen: Set<string>, value: string) => {
  const normalized = value.trim()
  if (!normalized || seen.has(normalized)) {
    return
  }
  seen.add(normalized)
  tags.push(normalized)
}

const extractDiagnosticTags = (message: string, providerError: ProviderErrorDetail | null) => {
  const diagnosticTags: string[] = []
  const seenTags = new Set<string>()
  const statusMatch = message.match(/\bstatus(?:=|:|\s)(\d{3})\b/i)
  const providerMatch = message.match(/\bprovider(?:=|:|\s)([^\s|,]+)/i)
  const urlMatch = message.match(/https?:\/\/[^\s|)]+/i)

  const statusCode = providerError?.statusCode ?? (statusMatch ? Number.parseInt(statusMatch[1], 10) : undefined)
  if (statusCode != null) {
    const statusType = statusCode >= 500 ? '服务端' : statusCode >= 400 ? '客户端' : '状态'
    pushDiagnosticTag(diagnosticTags, seenTags, `HTTP ${statusCode} (${statusType})`)
  }

  if (providerError?.semanticTag) {
    pushDiagnosticTag(diagnosticTags, seenTags, providerError.semanticTag)
  }

  if (providerError?.errorStatus) {
    pushDiagnosticTag(diagnosticTags, seenTags, `Status ${providerError.errorStatus}`)
  }

  if (providerError?.errorType) {
    pushDiagnosticTag(diagnosticTags, seenTags, `Type ${providerError.errorType}`)
  }

  if (providerError?.errorCode) {
    pushDiagnosticTag(diagnosticTags, seenTags, `Code ${providerError.errorCode}`)
  }

  if (providerError?.errorParam) {
    pushDiagnosticTag(diagnosticTags, seenTags, `Param ${providerError.errorParam}`)
  }

  if (providerMatch) {
    pushDiagnosticTag(diagnosticTags, seenTags, `Provider ${providerMatch[1]}`)
  }

  if (urlMatch) {
    const normalizedURL = urlMatch[0]
    pushDiagnosticTag(
      diagnosticTags,
      seenTags,
      normalizedURL.length > 72 ? `${normalizedURL.slice(0, 69)}...` : normalizedURL,
    )
  }

  if (message.includes('request failed after')) {
    pushDiagnosticTag(diagnosticTags, seenTags, '重试后仍失败')
  }

  if (message.includes('客户端中断')) {
    pushDiagnosticTag(diagnosticTags, seenTags, '客户端中断')
  }

  return diagnosticTags.slice(0, 6)
}

const parsedLogs = computed<ConsoleDisplayLog[]>(() => {
  return logs.value.map((log) => {
    const providerError = parseProviderErrorFromConsoleMessage(log.message)
    return {
      ...log,
      providerError,
      resolvedLevel: getResolvedLevel(log, providerError),
      diagnosticTags: extractDiagnosticTags(log.message, providerError),
    }
  })
})

const levelCounts = computed<Record<LevelFilter, number>>(() => {
  const counts: Record<LevelFilter, number> = {
    ALL: parsedLogs.value.length,
    INFO: 0,
    WARN: 0,
    ERROR: 0,
    DEBUG: 0,
  }

  for (const log of parsedLogs.value) {
    counts[log.resolvedLevel] += 1
  }

  return counts
})

const levelFilteredLogs = computed(() => {
  if (activeLevelFilter.value === 'ALL') {
    return parsedLogs.value
  }
  return parsedLogs.value.filter((log) => log.resolvedLevel === activeLevelFilter.value)
})

const normalizedKeyword = computed(() => keywordQuery.value.trim().toLowerCase())
const errorOnlyLogs = computed(() => parsedLogs.value.filter((log) => log.resolvedLevel === 'ERROR'))

const linkageSourceLogs = computed(() => {
  if (hitErrorLinkMode.value) {
    return errorOnlyLogs.value
  }
  return levelFilteredLogs.value
})

const matchesKeyword = (log: ConsoleDisplayLog, keyword: string) => {
  if (!keyword) {
    return true
  }
  const searchableText = [
    log.message,
    log.resolvedLevel,
    log.providerError?.summary ?? '',
    log.providerError?.providerMessage ?? '',
    log.providerError?.errorCode ?? '',
    log.providerError?.errorType ?? '',
    log.providerError?.errorStatus ?? '',
    log.providerError?.errorParam ?? '',
    log.providerError?.rawPayload ?? '',
    ...log.diagnosticTags,
    formatTimestamp(log.timestamp),
  ].join('\n').toLowerCase()
  return searchableText.includes(keyword)
}

const visibleLogs = computed(() => {
  const keyword = normalizedKeyword.value
  if (hitErrorLinkMode.value) {
    if (!keyword) {
      return []
    }
    return linkageSourceLogs.value.filter((log) => matchesKeyword(log, keyword))
  }
  if (!keyword) {
    return linkageSourceLogs.value
  }
  return linkageSourceLogs.value.filter((log) => matchesKeyword(log, keyword))
})

const searchHitText = computed(() => {
  if (hitErrorLinkMode.value && !normalizedKeyword.value) {
    return '输入关键词后仅看 ERROR 命中'
  }
  if (normalizedKeyword.value) {
    const suffix = hitErrorLinkMode.value ? '（ERROR）' : ''
    return `命中 ${visibleLogs.value.length} 条${suffix}`
  }
  return `共 ${linkageSourceLogs.value.length} 条`
})

const setLevelFilter = async (filter: LevelFilter) => {
  activeLevelFilter.value = filter
  if (autoScroll.value) {
    await nextTick()
    scrollToBottom()
  }
}

const clearKeywordSearch = async () => {
  if (!keywordQuery.value.trim()) {
    return
  }
  keywordQuery.value = ''
  if (autoScroll.value) {
    await nextTick()
    scrollToBottom()
  }
}

const getFilterCount = (filter: LevelFilter) => levelCounts.value[filter]

const toggleHitErrorLinkMode = async () => {
  hitErrorLinkMode.value = !hitErrorLinkMode.value
  if (autoScroll.value) {
    await nextTick()
    scrollToBottom()
  }
}

const getLogIdentity = (log: ConsoleDisplayLog) => `${log.timestamp}|${log.resolvedLevel}|${log.message}`

const isCopiedLog = (log: ConsoleDisplayLog) => copiedLogIdentity.value === getLogIdentity(log)
const getProviderErrorIdentity = (log: ConsoleDisplayLog) => `${getLogIdentity(log)}|provider-error`
const isCopiedProviderError = (log: ConsoleDisplayLog) => copiedProviderErrorIdentity.value === getProviderErrorIdentity(log)

const buildCopyPayload = (log: ConsoleDisplayLog) => {
  const formattedLine = `[${formatTimestamp(log.timestamp)}] [${log.resolvedLevel}] ${log.message}`
  if (log.diagnosticTags.length === 0) {
    return formattedLine
  }
  return `${formattedLine}\n[Tags] ${log.diagnosticTags.join(' | ')}`
}

const markCopiedState = (target: Ref<string>, identity: string) => {
  target.value = identity
  window.setTimeout(() => {
    if (target.value === identity) {
      target.value = ''
    }
  }, 1500)
}

const copyErrorLogLine = async (log: ConsoleDisplayLog) => {
  const payload = buildCopyPayload(log)
  try {
    await writeTextToClipboard(payload)
    const identity = getLogIdentity(log)
    markCopiedState(copiedLogIdentity, identity)
    showToast('错误日志已复制', 'success')
  } catch (error) {
    console.error('复制错误日志失败:', error)
    showToast(`复制失败：${extractErrorMessage(error)}`, 'error')
  }
}

const copyProviderError = async (log: ConsoleDisplayLog) => {
  const payload = log.providerError?.copyText?.trim() ?? ''
  if (!payload) {
    showToast('该条日志没有可复制的供应商错误信息', 'warning')
    return
  }

  try {
    await writeTextToClipboard(payload)
    markCopiedState(copiedProviderErrorIdentity, getProviderErrorIdentity(log))
    showToast('供应商错误已复制', 'success')
  } catch (error) {
    console.error('复制供应商错误失败:', error)
    showToast(`复制失败：${extractErrorMessage(error)}`, 'error')
  }
}

const getLevelClass = (level: LogLevel) => {
  switch (level) {
    case 'ERROR':
      return 'log-error'
    case 'WARN':
      return 'log-warn'
    case 'DEBUG':
      return 'log-debug'
    default:
      return 'log-info'
  }
}

const getLevelIcon = (level: LogLevel) => {
  switch (level) {
    case 'ERROR':
      return '⛔'
    case 'WARN':
      return '⚠'
    case 'DEBUG':
      return '🐞'
    default:
      return 'ℹ'
  }
}

onMounted(async () => {
  loading.value = true
  await loadLogs()
  loading.value = false

  // 每秒刷新一次日志
  refreshInterval = window.setInterval(loadLogs, 1000)
})

onUnmounted(() => {
  if (refreshInterval) {
    clearInterval(refreshInterval)
  }
})
</script>

<template>
  <div class="main-shell console-shell">
    <div class="global-actions">
      <p class="global-eyebrow">控制台</p>
      <div class="actions-group">
        <button class="secondary-btn" :disabled="clearing" @click="openClearConfirm">清空日志</button>
        <label class="auto-scroll-toggle">
          <input type="checkbox" v-model="autoScroll" />
          <span>自动滚动</span>
        </label>
        <div class="log-filters">
          <button
            v-for="option in levelFilterOptions"
            :key="option.key"
            class="level-filter-btn"
            :class="[
              `filter-${option.key.toLowerCase()}`,
              { active: activeLevelFilter === option.key, linked: hitErrorLinkMode && option.key !== 'ERROR' },
            ]"
            @click="setLevelFilter(option.key)"
          >
            {{ option.label }} {{ getFilterCount(option.key) }}
          </button>
        </div>
        <div class="search-group">
          <input
            v-model="keywordQuery"
            class="keyword-input"
            type="text"
            placeholder="搜索关键词（message / status / provider / error code / error type）"
            @keydown.esc.prevent="clearKeywordSearch"
          />
          <button
            class="search-control-btn"
            :disabled="!keywordQuery.trim()"
            @click="clearKeywordSearch"
          >
            清空
          </button>
          <button
            class="search-control-btn link-mode-btn"
            :class="{ active: hitErrorLinkMode }"
            @click="toggleHitErrorLinkMode"
          >
            命中 + ERROR
          </button>
          <span class="search-hit">{{ searchHitText }}</span>
        </div>
        <button class="ghost-icon" aria-label="返回" @click="goBack">
          <svg viewBox="0 0 24 24" aria-hidden="true">
            <path
              d="M15 18l-6-6 6-6"
              fill="none"
              stroke="currentColor"
              stroke-width="1.5"
              stroke-linecap="round"
              stroke-linejoin="round"
            />
          </svg>
        </button>
      </div>
    </div>

    <div class="console-container">
      <div v-if="loading" class="loading-state">
        <div class="spinner"></div>
        <p>加载中...</p>
      </div>

      <div v-else class="console-content" ref="logsContainer">
        <div v-if="visibleLogs.length === 0" class="empty-state">
          <p>
            {{
              logs.length === 0
                ? '暂无日志'
                : hitErrorLinkMode && !keywordQuery.trim()
                  ? '联动模式已开启：请输入关键词后仅显示 ERROR 命中日志'
                  : '当前筛选或搜索条件下暂无日志'
            }}
          </p>
        </div>

        <div
          v-for="(log, index) in visibleLogs"
          :key="index"
          class="log-entry"
          :class="[getLevelClass(log.resolvedLevel), { 'log-entry--provider-error': log.providerError }]"
        >
          <span class="log-timestamp">{{ formatTimestamp(log.timestamp) }}</span>
          <span class="log-level">
            <span class="log-level-icon">{{ getLevelIcon(log.resolvedLevel) }}</span>
            <span>{{ log.resolvedLevel }}</span>
          </span>
          <div class="log-main">
            <span class="log-message">{{ log.message }}</span>
            <section v-if="log.providerError" class="provider-error-panel">
              <div class="provider-error-panel__header">
                <span class="provider-error-panel__label">供应商错误详情</span>
                <span v-if="log.providerError.statusCode" class="provider-error-panel__status">
                  HTTP {{ log.providerError.statusCode }}
                </span>
              </div>
              <p class="provider-error-panel__summary">
                {{ log.providerError.summary }}
              </p>
            </section>
            <div v-if="log.diagnosticTags.length > 0" class="log-tags">
              <span
                v-for="(tag, tagIndex) in log.diagnosticTags"
                :key="`${index}-${tagIndex}`"
                class="log-tag"
              >
                {{ tag }}
              </span>
            </div>
          </div>
          <div v-if="log.resolvedLevel === 'ERROR' || log.providerError" class="log-actions">
            <button
              v-if="log.providerError?.copyText"
              class="copy-log-btn copy-provider-btn"
              :class="{ copied: isCopiedProviderError(log) }"
              @click="copyProviderError(log)"
            >
              {{ isCopiedProviderError(log) ? '已复制错误' : '复制供应商错误' }}
            </button>
            <button
              v-if="log.resolvedLevel === 'ERROR'"
              class="copy-log-btn"
              :class="{ copied: isCopiedLog(log) }"
              @click="copyErrorLogLine(log)"
            >
              {{ isCopiedLog(log) ? '已复制' : '复制错误行' }}
            </button>
          </div>
        </div>
      </div>
    </div>

    <BaseModal :open="clearConfirmOpen" title="确认清空" variant="confirm" @close="closeClearConfirm">
      <div class="confirm-body">
        <p>确认清空所有控制台日志？</p>
      </div>
      <footer class="form-actions confirm-actions">
        <BaseButton variant="outline" type="button" :disabled="clearing" @click="closeClearConfirm">
          取消
        </BaseButton>
        <BaseButton variant="danger" type="button" :disabled="clearing" @click="confirmClearLogs">
          {{ clearing ? '清理中...' : '确认清空' }}
        </BaseButton>
      </footer>
    </BaseModal>
  </div>
</template>

<style scoped>
.console-shell {
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow: hidden;
}

.actions-group {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 12px;
}

.auto-scroll-toggle {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 0.9rem;
  color: var(--mac-text-secondary);
  cursor: pointer;
  user-select: none;
}

.auto-scroll-toggle input[type="checkbox"] {
  cursor: pointer;
}

.log-filters {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}

.level-filter-btn {
  border: 1px solid rgba(148, 163, 184, 0.35);
  background: rgba(15, 23, 42, 0.45);
  color: #cbd5e1;
  border-radius: 8px;
  padding: 4px 10px;
  font-size: 0.74rem;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.15s ease;
}

.level-filter-btn:hover {
  border-color: rgba(148, 163, 184, 0.65);
  color: #e2e8f0;
}

.level-filter-btn.active {
  background: rgba(59, 130, 246, 0.2);
  border-color: rgba(59, 130, 246, 0.7);
  color: #dbeafe;
}

.level-filter-btn.linked {
  opacity: 0.6;
}

.level-filter-btn.filter-error.active {
  background: rgba(239, 68, 68, 0.28);
  border-color: rgba(248, 113, 113, 0.75);
  color: #fee2e2;
}

.level-filter-btn.filter-warn.active {
  background: rgba(245, 158, 11, 0.28);
  border-color: rgba(251, 191, 36, 0.75);
  color: #fef3c7;
}

.level-filter-btn.filter-debug.active {
  background: rgba(96, 165, 250, 0.24);
  border-color: rgba(96, 165, 250, 0.7);
  color: #dbeafe;
}

.search-group {
  display: flex;
  align-items: center;
  flex-wrap: nowrap;
  gap: 8px;
  flex: 1 1 360px;
  min-width: 260px;
  max-width: 620px;
}

.keyword-input {
  flex: 1 1 auto;
  min-width: 0;
  height: 30px;
  border-radius: 8px;
  border: 1px solid rgba(148, 163, 184, 0.38);
  background: rgba(15, 23, 42, 0.5);
  color: #e2e8f0;
  padding: 0 10px;
  font-size: 0.8rem;
}

.keyword-input:focus {
  outline: none;
  border-color: rgba(59, 130, 246, 0.7);
  box-shadow: 0 0 0 2px rgba(59, 130, 246, 0.22);
}

.search-control-btn {
  border: 1px solid rgba(148, 163, 184, 0.38);
  background: rgba(15, 23, 42, 0.45);
  color: #cbd5e1;
  border-radius: 8px;
  padding: 4px 10px;
  font-size: 0.74rem;
  font-weight: 600;
  cursor: pointer;
  white-space: nowrap;
  transition: all 0.15s ease;
}

.search-control-btn:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.search-control-btn:not(:disabled):hover {
  border-color: rgba(148, 163, 184, 0.65);
  color: #e2e8f0;
}

.link-mode-btn {
  border-color: rgba(248, 113, 113, 0.45);
  background: rgba(127, 29, 29, 0.28);
  color: #fecaca;
}

.link-mode-btn.active {
  border-color: rgba(248, 113, 113, 0.8);
  background: rgba(220, 38, 38, 0.32);
  color: #fee2e2;
}

.search-hit {
  flex-shrink: 0;
  color: #94a3b8;
  font-size: 0.72rem;
  font-weight: 600;
  white-space: nowrap;
}

.console-container {
  flex: 1;
  overflow: hidden;
  background: var(--mac-surface);
  border: 1px solid var(--mac-border);
  border-radius: 12px;
  display: flex;
  flex-direction: column;
}

.console-content {
  flex: 1;
  overflow-y: auto;
  padding: 16px;
  font-family: 'SF Mono', 'Monaco', 'Inconsolata', 'Fira Code', 'Consolas', monospace;
  font-size: 0.85rem;
  line-height: 1.6;
  background: #1e1e1e;
  color: #d4d4d4;
}

html.dark .console-content {
  background: #0d1117;
  color: #e6edf3;
}

.log-entry {
  display: grid;
  grid-template-columns: auto auto minmax(0, 1fr) auto;
  align-items: flex-start;
  column-gap: 12px;
  row-gap: 6px;
  padding: 6px 8px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.05);
  border-left: 3px solid transparent;
  border-radius: 8px;
  transition: background-color 0.15s ease, border-color 0.15s ease;
}

.log-entry--provider-error {
  box-shadow: inset 0 0 0 1px rgba(248, 113, 113, 0.18);
}

.log-entry + .log-entry {
  margin-top: 6px;
}

.log-timestamp {
  color: #858585;
  font-weight: 500;
  white-space: nowrap;
}

.log-level {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 4px;
  min-width: 64px;
  padding: 0 8px;
  border-radius: 6px;
  font-size: 0.76rem;
  letter-spacing: 0.04em;
  font-weight: 600;
}

.log-level-icon {
  font-size: 0.78rem;
}

.log-info .log-level {
  color: #4ec9b0;
  background: rgba(78, 201, 176, 0.15);
}

.log-debug .log-level {
  color: #93c5fd;
  background: rgba(96, 165, 250, 0.15);
}

.log-warn .log-level {
  color: #fef3c7;
  background: rgba(245, 158, 11, 0.35);
}

.log-error .log-level {
  color: #fee2e2;
  background: rgba(239, 68, 68, 0.45);
  font-weight: 700;
}

.log-warn {
  border-left-color: #f59e0b;
  background: linear-gradient(90deg, rgba(245, 158, 11, 0.14), rgba(245, 158, 11, 0.04) 58%, transparent);
}

.log-error {
  border-left-color: #ef4444;
  background: linear-gradient(90deg, rgba(239, 68, 68, 0.2), rgba(239, 68, 68, 0.08) 58%, transparent);
  box-shadow: inset 0 0 0 1px rgba(239, 68, 68, 0.28);
}

.log-warn .log-message {
  color: #fde68a;
}

.log-error .log-message {
  color: #fecaca;
  font-weight: 600;
}

.log-main {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.log-message {
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  word-break: break-word;
}

.provider-error-panel {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin-top: 2px;
  padding: 10px 12px;
  border-radius: 10px;
  border: 1px solid rgba(248, 113, 113, 0.28);
  background: linear-gradient(135deg, rgba(127, 29, 29, 0.28), rgba(69, 10, 10, 0.14));
}

.provider-error-panel__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.provider-error-panel__label {
  font-size: 0.72rem;
  font-weight: 700;
  letter-spacing: 0.05em;
  color: #fca5a5;
}

.provider-error-panel__status {
  display: inline-flex;
  align-items: center;
  padding: 2px 8px;
  border-radius: 999px;
  border: 1px solid rgba(248, 113, 113, 0.4);
  background: rgba(127, 29, 29, 0.35);
  color: #fecaca;
  font-size: 0.68rem;
  font-weight: 700;
}

.provider-error-panel__summary {
  margin: 0;
  color: #ffe4e6;
  font-size: 0.78rem;
  line-height: 1.55;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  word-break: break-word;
}

.log-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.log-tag {
  display: inline-flex;
  align-items: center;
  padding: 1px 8px;
  border-radius: 999px;
  border: 1px solid rgba(148, 163, 184, 0.35);
  background: rgba(15, 23, 42, 0.55);
  color: #cbd5e1;
  font-size: 0.7rem;
  line-height: 1.4;
}

.log-warn .log-tag {
  border-color: rgba(245, 158, 11, 0.5);
  color: #fde68a;
}

.log-error .log-tag {
  border-color: rgba(248, 113, 113, 0.6);
  background: rgba(127, 29, 29, 0.35);
  color: #fecaca;
}

.copy-log-btn {
  align-self: start;
  justify-self: end;
  border: 1px solid rgba(248, 113, 113, 0.55);
  background: rgba(127, 29, 29, 0.35);
  color: #fecaca;
  border-radius: 8px;
  padding: 3px 10px;
  font-size: 0.72rem;
  font-weight: 600;
  line-height: 1.4;
  white-space: nowrap;
  cursor: pointer;
  transition: all 0.15s ease;
}

.log-actions {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 6px;
}

.copy-provider-btn {
  border-color: rgba(251, 191, 36, 0.45);
  background: rgba(120, 53, 15, 0.35);
  color: #fde68a;
}

.copy-log-btn:hover {
  border-color: rgba(252, 165, 165, 0.75);
  background: rgba(220, 38, 38, 0.34);
}

.copy-provider-btn:hover {
  border-color: rgba(252, 211, 77, 0.72);
  background: rgba(180, 83, 9, 0.34);
}

.copy-log-btn.copied {
  border-color: rgba(16, 185, 129, 0.7);
  background: rgba(6, 95, 70, 0.4);
  color: #d1fae5;
}

@media (max-width: 1180px) {
  .actions-group {
    justify-content: flex-start;
  }

  .search-group {
    flex: 1 1 100%;
    max-width: none;
    flex-wrap: wrap;
  }

  .keyword-input {
    flex-basis: 260px;
  }

  .search-hit {
    width: 100%;
    white-space: normal;
  }
}

@media (max-width: 960px) {
  .log-entry {
    grid-template-columns: auto auto minmax(0, 1fr);
  }

  .log-actions {
    grid-column: 3;
    justify-self: start;
    align-items: flex-start;
    flex-direction: row;
    flex-wrap: wrap;
  }
}

@media (max-width: 680px) {
  .log-entry {
    grid-template-columns: minmax(0, 1fr);
  }

  .log-timestamp,
  .log-level,
  .log-main,
  .log-actions {
    grid-column: 1;
  }

  .log-level {
    justify-self: start;
  }

  .log-actions {
    justify-self: start;
  }

  .search-group {
    min-width: 0;
  }
}

.loading-state,
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  color: var(--mac-text-secondary);
}

.spinner {
  width: 32px;
  height: 32px;
  border: 3px solid rgba(0, 0, 0, 0.1);
  border-top-color: var(--mac-accent);
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
  margin-bottom: 12px;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}
</style>

<template>
  <nav
    class="base-pagination"
    :class="{
      'base-pagination--tools-only': !showMainControls,
      'base-pagination--align-end': normalizedAlign === 'end',
      'base-pagination--compact': compact,
    }"
    :aria-label="t('common.pagination.navigation')"
  >
    <div v-if="showMainControls" class="base-pagination__main">
      <button
        type="button"
        class="base-pagination__nav"
        :aria-label="t('common.pagination.previous')"
        :disabled="isPrevDisabled"
        @click="goToPage(normalizedPage - 1)"
      >
        <svg viewBox="0 0 16 16" aria-hidden="true">
          <path
            d="M9.5 3.5L5 8l4.5 4.5"
            fill="none"
            stroke="currentColor"
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="1.6"
          />
        </svg>
      </button>

      <div class="base-pagination__pages">
        <template v-for="item in paginationItems" :key="item.key">
          <button
            v-if="item.type === 'page'"
            type="button"
            class="base-pagination__page"
            :class="{ 'is-active': item.value === normalizedPage }"
            :aria-label="t('common.pagination.page', { page: item.value })"
            :aria-current="item.value === normalizedPage ? 'page' : undefined"
            :disabled="loading"
            @click="goToPage(item.value)"
          >
            {{ item.value }}
          </button>
          <span v-else class="base-pagination__ellipsis" aria-hidden="true">...</span>
        </template>
      </div>

      <button
        type="button"
        class="base-pagination__nav"
        :aria-label="t('common.pagination.next')"
        :disabled="isNextDisabled"
        @click="goToPage(normalizedPage + 1)"
      >
        <svg viewBox="0 0 16 16" aria-hidden="true">
          <path
            d="M6.5 3.5L11 8l-4.5 4.5"
            fill="none"
            stroke="currentColor"
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="1.6"
          />
        </svg>
      </button>
    </div>

    <div v-if="showTools" class="base-pagination__tools">
      <label v-if="showPageSizeControl" class="base-pagination__page-size">
        <select
          class="mac-select base-pagination__select"
          :value="normalizedPageSize"
          :disabled="loading"
          @change="handlePageSizeChange"
        >
          <option
            v-for="option in normalizedPageSizeOptions"
            :key="`base-pagination-size-${option}`"
            :value="option"
          >
            {{ t('common.pagination.pageSize', { size: option }) }}
          </option>
        </select>
      </label>

      <div v-if="showJumpControl" class="base-pagination__jump">
        <span class="base-pagination__jump-label">{{ t('common.pagination.jumpPrefix') }}</span>
        <BaseInput
          v-model="jumpPageInput"
          class="mac-input base-pagination__jump-input"
          type="text"
          inputmode="numeric"
          pattern="[0-9]*"
          :placeholder="t('common.pagination.jumpPlaceholder')"
          :disabled="loading"
          @keydown.enter.prevent="submitJumpPage"
          @blur="submitJumpPage"
        />
        <span class="base-pagination__jump-suffix">{{ t('common.pagination.jumpSuffix') }}</span>
      </div>
    </div>
  </nav>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseInput from './BaseInput.vue'

type PaginationItem =
  | { type: 'page'; key: string; value: number }
  | { type: 'ellipsis'; key: string }

type PaginationAlign = 'between' | 'end'

const clamp = (value: number, min: number, max: number) => Math.min(Math.max(value, min), max)

const sanitizePositiveInteger = (value: number, fallback: number) => {
  if (!Number.isFinite(value) || value <= 0) return fallback
  return Math.max(1, Math.floor(value))
}

const props = withDefaults(
  defineProps<{
    page: number
    totalPages: number
    loading?: boolean
    pageSize?: number
    pageSizeOptions?: number[]
    showPageSize?: boolean
    showJump?: boolean
    maxVisiblePages?: number
    align?: PaginationAlign
    compact?: boolean
  }>(),
  {
    loading: false,
    pageSize: 0,
    pageSizeOptions: () => [10, 20, 50],
    showPageSize: true,
    showJump: true,
    maxVisiblePages: 5,
    align: 'between',
    compact: false,
  },
)

const emit = defineEmits<{
  (event: 'update:page', value: number): void
  (event: 'update:pageSize', value: number): void
}>()

const { t } = useI18n()

const normalizedTotalPages = computed(() => sanitizePositiveInteger(props.totalPages, 1))
const normalizedPage = computed(() => clamp(sanitizePositiveInteger(props.page, 1), 1, normalizedTotalPages.value))
const normalizedPageSize = computed(() => sanitizePositiveInteger(props.pageSize, 0))
const normalizedAlign = computed<PaginationAlign>(() => (props.align === 'end' ? 'end' : 'between'))
const normalizedPageSizeOptions = computed(() => {
  const unique = new Set<number>()
  const options: number[] = []
  for (const option of props.pageSizeOptions) {
    const normalized = sanitizePositiveInteger(option, 0)
    if (normalized <= 0 || unique.has(normalized)) continue
    unique.add(normalized)
    options.push(normalized)
  }
  if (normalizedPageSize.value > 0 && !unique.has(normalizedPageSize.value)) {
    options.push(normalizedPageSize.value)
  }
  return options.sort((left, right) => left - right)
})

const showPageSizeControl = computed(() =>
  props.showPageSize && normalizedPageSize.value > 0 && normalizedPageSizeOptions.value.length > 0,
)
const showMainControls = computed(() => normalizedTotalPages.value > 1)
const showJumpControl = computed(() => props.showJump && normalizedTotalPages.value > 1)
const showTools = computed(() => showPageSizeControl.value || showJumpControl.value)
const isPrevDisabled = computed(() => props.loading || normalizedPage.value <= 1)
const isNextDisabled = computed(() => props.loading || normalizedPage.value >= normalizedTotalPages.value)

const buildPaginationItems = (currentPage: number, totalPages: number, maxVisiblePages: number): PaginationItem[] => {
  const pageValues = new Set<number>([1, totalPages, currentPage])
  const sideCount = Math.max(1, Math.floor(maxVisiblePages / 2))

  for (let offset = 1; offset <= sideCount; offset += 1) {
    pageValues.add(clamp(currentPage - offset, 1, totalPages))
    pageValues.add(clamp(currentPage + offset, 1, totalPages))
  }

  if (currentPage <= sideCount + 2) {
    for (let page = 1; page <= Math.min(totalPages, maxVisiblePages); page += 1) {
      pageValues.add(page)
    }
  }

  if (currentPage >= totalPages - sideCount - 1) {
    for (let page = Math.max(1, totalPages - maxVisiblePages + 1); page <= totalPages; page += 1) {
      pageValues.add(page)
    }
  }

  const sortedPages = [...pageValues].sort((left, right) => left - right)
  const items: PaginationItem[] = []

  sortedPages.forEach((page, index) => {
    if (index > 0) {
      const previous = sortedPages[index - 1]
      if (page - previous > 1) {
        items.push({ type: 'ellipsis', key: `ellipsis-${previous}-${page}` })
      }
    }
    items.push({ type: 'page', key: `page-${page}`, value: page })
  })

  return items
}

const paginationItems = computed(() =>
  buildPaginationItems(
    normalizedPage.value,
    normalizedTotalPages.value,
    sanitizePositiveInteger(props.maxVisiblePages, 5),
  ),
)

const jumpPageInput = ref(`${normalizedPage.value}`)

watch(
  normalizedPage,
  (value) => {
    jumpPageInput.value = `${value}`
  },
  { immediate: true },
)

const goToPage = (nextPage: number) => {
  const normalized = clamp(sanitizePositiveInteger(nextPage, normalizedPage.value), 1, normalizedTotalPages.value)
  jumpPageInput.value = `${normalized}`
  if (normalized === normalizedPage.value || props.loading) return
  emit('update:page', normalized)
}

const submitJumpPage = () => {
  if (props.loading) {
    jumpPageInput.value = `${normalizedPage.value}`
    return
  }
  const rawValue = `${jumpPageInput.value ?? ''}`.trim()
  if (rawValue === '') {
    jumpPageInput.value = `${normalizedPage.value}`
    return
  }
  const nextPage = Number.parseInt(rawValue, 10)
  if (!Number.isFinite(nextPage)) {
    jumpPageInput.value = `${normalizedPage.value}`
    return
  }
  goToPage(nextPage)
}

const handlePageSizeChange = (event: Event) => {
  if (props.loading) return
  const target = event.target as HTMLSelectElement
  const nextSize = sanitizePositiveInteger(Number(target.value), normalizedPageSize.value)
  if (nextSize <= 0 || nextSize === normalizedPageSize.value) return
  emit('update:pageSize', nextSize)
}
</script>

<style scoped>
.base-pagination {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px 16px;
  flex-wrap: wrap;
}

.base-pagination--tools-only {
  justify-content: flex-end;
}

.base-pagination--align-end {
  justify-content: flex-end;
}

.base-pagination__main {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.base-pagination__pages {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.base-pagination__nav,
.base-pagination__page {
  appearance: none;
  border: 1px solid var(--mac-border);
  background: color-mix(in srgb, var(--mac-surface) 86%, transparent);
  color: var(--mac-text-secondary);
  min-width: 34px;
  height: 34px;
  padding: 0 10px;
  border-radius: 10px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 0.9rem;
  font-weight: 600;
  line-height: 1;
  cursor: pointer;
  transition:
    border-color 0.2s ease,
    background 0.2s ease,
    color 0.2s ease,
    box-shadow 0.2s ease,
    transform 0.2s ease;
  font-variant-numeric: tabular-nums;
}

.base-pagination__nav svg {
  width: 14px;
  height: 14px;
}

.base-pagination__nav:hover:not(:disabled),
.base-pagination__page:hover:not(:disabled),
.base-pagination__nav:focus-visible,
.base-pagination__page:focus-visible {
  outline: none;
  border-color: color-mix(in srgb, var(--mac-accent) 45%, var(--mac-border));
  color: var(--mac-text);
  background: color-mix(in srgb, var(--mac-accent) 9%, var(--mac-surface));
  transform: translateY(-1px);
}

.base-pagination__page.is-active {
  border-color: color-mix(in srgb, var(--mac-accent) 55%, var(--mac-border));
  color: var(--mac-accent);
  background: color-mix(in srgb, var(--mac-accent) 14%, var(--mac-surface));
  box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--mac-accent) 18%, transparent);
}

.base-pagination__nav:disabled,
.base-pagination__page:disabled {
  cursor: not-allowed;
  opacity: 0.55;
  transform: none;
}

.base-pagination__ellipsis {
  min-width: 16px;
  text-align: center;
  color: var(--mac-text-secondary);
  font-size: 0.88rem;
  font-weight: 700;
  letter-spacing: 0.08em;
}

.base-pagination__tools {
  display: inline-flex;
  align-items: center;
  gap: 10px 12px;
  flex-wrap: wrap;
  justify-content: flex-end;
}

.base-pagination__page-size,
.base-pagination__jump {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.base-pagination__select {
  min-width: 108px;
  min-height: 34px;
  padding: 0.45rem 1.9rem 0.45rem 0.8rem;
  border-radius: 10px;
  font-size: 0.86rem;
  font-weight: 600;
}

.base-pagination__jump-label,
.base-pagination__jump-suffix {
  color: var(--mac-text-secondary);
  font-size: 0.84rem;
  font-weight: 600;
  white-space: nowrap;
}

.base-pagination__jump-input {
  width: 72px;
  min-width: 72px;
  min-height: 34px;
  padding: 0.45rem 0.7rem;
  text-align: center;
  font-size: 0.86rem;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

.base-pagination--compact {
  gap: 8px 12px;
}

.base-pagination--compact .base-pagination__main {
  gap: 6px;
}

.base-pagination--compact .base-pagination__pages {
  gap: 4px;
}

.base-pagination--compact .base-pagination__nav,
.base-pagination--compact .base-pagination__page {
  min-width: 26px;
  height: 26px;
  padding: 0 7px;
  border-radius: 8px;
  font-size: 0.8rem;
}

.base-pagination--compact .base-pagination__nav svg {
  width: 12px;
  height: 12px;
}

.base-pagination--compact .base-pagination__ellipsis {
  min-width: 12px;
  font-size: 0.8rem;
  letter-spacing: 0.05em;
}

.base-pagination--compact .base-pagination__tools {
  gap: 8px 10px;
}

.base-pagination--compact .base-pagination__page-size,
.base-pagination--compact .base-pagination__jump {
  gap: 6px;
}

.base-pagination--compact .base-pagination__select {
  min-width: 92px;
  min-height: 28px;
  padding: 0.3rem 1.65rem 0.3rem 0.68rem;
  border-radius: 8px;
  font-size: 0.8rem;
}

.base-pagination--compact .base-pagination__jump-label,
.base-pagination--compact .base-pagination__jump-suffix {
  font-size: 0.78rem;
}

.base-pagination--compact .base-pagination__jump-input {
  width: 60px;
  min-width: 60px;
  min-height: 28px;
  padding: 0.3rem 0.6rem;
  font-size: 0.8rem;
}

html.dark .base-pagination__nav,
html.dark .base-pagination__page {
  border-color: rgba(148, 163, 184, 0.22);
  background: color-mix(in srgb, var(--mac-surface-strong) 88%, transparent);
  color: rgba(226, 232, 240, 0.88);
}

html.dark .base-pagination__nav:hover:not(:disabled),
html.dark .base-pagination__page:hover:not(:disabled),
html.dark .base-pagination__nav:focus-visible,
html.dark .base-pagination__page:focus-visible {
  background: color-mix(in srgb, var(--mac-accent) 16%, rgba(15, 23, 42, 0.82));
  border-color: color-mix(in srgb, var(--mac-accent) 50%, rgba(148, 163, 184, 0.22));
  color: #f8fafc;
}

html.dark .base-pagination__page.is-active {
  color: #bfdbfe;
  background: color-mix(in srgb, var(--mac-accent) 22%, rgba(15, 23, 42, 0.82));
}

@media (max-width: 768px) {
  .base-pagination {
    align-items: stretch;
  }

  .base-pagination__tools {
    justify-content: space-between;
  }
}

@media (max-width: 640px) {
  .base-pagination__main,
  .base-pagination__tools {
    width: 100%;
  }

  .base-pagination__pages {
    flex: 1 1 auto;
  }

  .base-pagination__tools {
    justify-content: space-between;
  }

  .base-pagination__jump {
    margin-left: auto;
  }

  .base-pagination--align-end .base-pagination__main,
  .base-pagination--align-end .base-pagination__tools {
    justify-content: flex-end;
  }

  .base-pagination--align-end .base-pagination__pages {
    flex: 0 1 auto;
  }

  .base-pagination--compact .base-pagination__nav,
  .base-pagination--compact .base-pagination__page {
    min-width: 30px;
    height: 30px;
  }

  .base-pagination--compact .base-pagination__pages {
    gap: 6px;
  }
}
</style>

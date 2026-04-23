<template>
  <Teleport to="body">
    <Transition name="modal-fade">
      <div v-if="open" class="modal-backdrop" :style="backdropStyle" role="presentation">
        <!-- 遮罩层：仅负责视觉，不接收点击（避免 WebView 命中测试/层合成导致误触关闭） -->
        <div class="modal-overlay-noevent" aria-hidden="true"></div>

        <!-- 点击空白处关闭：只有点到 wrapper 自身时才触发 -->
        <div class="modal-wrapper" @click="onWrapperClick">
          <Transition name="modal-slide" appear @after-enter="handleAfterEnter">
            <div
              ref="panelRef"
              :class="['modal', variantClass, panelClass]"
              :style="panelStyle"
              role="dialog"
              aria-modal="true"
              :aria-labelledby="titleId"
              tabindex="-1"
            >
              <header class="modal-header">
                <h2 :id="titleId" class="modal-title">{{ title }}</h2>
                <button
                  ref="closeButtonRef"
                  class="ghost-icon"
                  type="button"
                  aria-label="Close"
                  :disabled="closeDisabled"
                  @click="emitClose"
                >
                  ✕
                </button>
              </header>
              <div :class="['modal-body', { 'modal-scrollable': bodyScrollable }]" :style="bodyStyle">
                <slot />
              </div>
            </div>
          </Transition>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch, type CSSProperties } from 'vue'
import { lockScroll, unlockScroll } from '../../utils/scrollLock'
import { getModalStackIndex, isTopMostModal, pushModalToTop, removeModalFromStack } from '../../utils/modalStack'

type Variant = 'default' | 'confirm'
type InlineModalPanelClass = string | string[] | Record<string, boolean>
const BASE_MODAL_Z_INDEX = 2000
const MODAL_STACK_STEP = 20
const MODAL_FOCUS_FALLBACK_MS = 380

const props = withDefaults(
  defineProps<{
    open: boolean
    title: string
    variant?: Variant
    closeOnBackdrop?: boolean
    closeDisabled?: boolean
    panelWidth?: string
    bodyScrollable?: boolean
    initialFocusSelector?: string
    panelClass?: InlineModalPanelClass
  }>(),
  {
    variant: 'default',
    closeOnBackdrop: true,
    closeDisabled: false,
    panelWidth: '',
    bodyScrollable: true,
    initialFocusSelector: '',
    panelClass: '',
  },
)

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'after-open'): void
}>()

const variantClass = computed(() => (props.variant === 'confirm' ? 'confirm-modal' : ''))
const titleId = `modal-title-${Math.random().toString(36).slice(2, 9)}`
const modalId = `inline-modal-${Math.random().toString(36).slice(2, 10)}`
const backdropStyle = computed<CSSProperties>(() => ({
  zIndex: BASE_MODAL_Z_INDEX + getModalStackIndex(modalId) * MODAL_STACK_STEP,
}))
const panelStyle = computed<CSSProperties>(() => ({
  maxWidth: 'calc(100vw - 48px)',
  ...(props.panelWidth ? { width: props.panelWidth } : {}),
}))
const bodyStyle = computed<CSSProperties | undefined>(() => {
  if (props.bodyScrollable) return undefined
  return {
    display: 'flex',
    flexDirection: 'column',
    minHeight: '0',
    flex: '1 1 auto',
  }
})

const panelRef = ref<HTMLElement | null>(null)
const closeButtonRef = ref<HTMLButtonElement | null>(null)
let lastActiveElement: Element | null = null
let initialFocusTimer: number | null = null
let pendingTransitionCleanup: { panel: HTMLElement; handler: (e: TransitionEvent) => void } | null = null

const emitClose = () => {
  if (!isTopMostModal(modalId)) return
  if (props.closeDisabled) return
  emit('close')
}

const handleAfterEnter = () => {
  if (!props.open) return
  emit('after-open')
}

// 统一阻断冒泡；只有点到 wrapper 空白处才关闭（等价于 @click.self）
const onWrapperClick = (event: MouseEvent) => {
  if (!isTopMostModal(modalId)) return
  event.stopPropagation()
  if (props.closeDisabled) return
  if (!props.closeOnBackdrop) return
  if (event.target === event.currentTarget) {
    emitClose()
  }
}

const getFocusableElements = (): HTMLElement[] => {
  if (!panelRef.value) return []
  const selector = [
    'a[href]',
    'button:not([disabled])',
    'input:not([disabled]):not([type="hidden"])',
    'select:not([disabled])',
    'textarea:not([disabled])',
    '[tabindex]:not([tabindex="-1"])',
  ].join(',')
  return Array.from(panelRef.value.querySelectorAll<HTMLElement>(selector)).filter((el) => {
    const style = getComputedStyle(el)
    return style.display !== 'none' && style.visibility !== 'hidden'
  })
}

const focusInitialTarget = () => {
  if (!panelRef.value) return false

  const selector = props.initialFocusSelector.trim()
  if (!selector) return false

  try {
    const target = panelRef.value.querySelector<HTMLElement>(selector)
    if (!target) return false
    if ('disabled' in target && (target as HTMLButtonElement | HTMLInputElement | HTMLTextAreaElement).disabled) {
      return false
    }
    target.focus()
    return document.activeElement === target
  } catch {
    return false
  }
}

const focusFallbackTarget = () => {
  if (!props.closeDisabled) {
    closeButtonRef.value?.focus()
    return
  }
  panelRef.value?.focus()
}

const clearInitialFocusTimer = () => {
  if (initialFocusTimer !== null) {
    window.clearTimeout(initialFocusTimer)
    initialFocusTimer = null
  }
  if (pendingTransitionCleanup) {
    pendingTransitionCleanup.panel.removeEventListener('transitionend', pendingTransitionCleanup.handler)
    pendingTransitionCleanup = null
  }
}

const scheduleInitialFocus = () => {
  clearInitialFocusTimer()

  nextTick(() => {
    const selector = props.initialFocusSelector.trim()
    if (!selector) {
      focusFallbackTarget()
      return
    }

    const panel = panelRef.value
    if (!panel) {
      focusFallbackTarget()
      return
    }

    let didFocus = false
    const doFocus = () => {
      if (didFocus) return
      didFocus = true
      clearInitialFocusTimer()
      if (focusInitialTarget()) return
      focusFallbackTarget()
    }

    // 监听面板进场动画结束后再聚焦，避免 WebKit/WebView 在 transform 动画期间
    // focus textarea 导致面板不绘制的渲染 bug。
    // 只监听 transform 属性——它是触发 WebKit 渲染异常的根因，
    // 且 transition: all 会对 opacity/transform 各触发一次 transitionend
    const onEnd = (e: TransitionEvent) => {
      if (e.target !== panel || e.propertyName !== 'transform') return
      panel.removeEventListener('transitionend', onEnd)
      pendingTransitionCleanup = null
      doFocus()
    }
    panel.addEventListener('transitionend', onEnd)
    pendingTransitionCleanup = { panel, handler: onEnd }

    // Fallback：如果 transitionend 未触发（WebKit 极端情况），兜底执行
    initialFocusTimer = window.setTimeout(() => {
      initialFocusTimer = null
      panel.removeEventListener('transitionend', onEnd)
      pendingTransitionCleanup = null
      doFocus()
    }, MODAL_FOCUS_FALLBACK_MS)
  })
}

const onKeyDown = (e: KeyboardEvent) => {
  if (!props.open || !isTopMostModal(modalId)) return

  if (e.key === 'Escape') {
    e.preventDefault()
    e.stopImmediatePropagation()
    if (props.closeDisabled) return
    emitClose()
    return
  }

  if (e.key !== 'Tab') return

  const focusables = getFocusableElements()
  if (focusables.length === 0) {
    e.preventDefault()
    panelRef.value?.focus()
    return
  }

  const active = document.activeElement as HTMLElement | null
  const first = focusables[0]
  const last = focusables[focusables.length - 1]
  const inside = active && panelRef.value?.contains(active)

  if (e.shiftKey) {
    if (!inside || active === first) {
      e.preventDefault()
      last.focus()
    }
  } else {
    if (!inside || active === last) {
      e.preventDefault()
      first.focus()
    }
  }
}

watch(
  () => props.open,
  (open) => {
    if (open) {
      pushModalToTop(modalId)
      lastActiveElement = document.activeElement
      window.addEventListener('keydown', onKeyDown, true)
      lockScroll()
      scheduleInitialFocus()
    } else {
      clearInitialFocusTimer()
      window.removeEventListener('keydown', onKeyDown, true)
      removeModalFromStack(modalId)
      unlockScroll()
      if (lastActiveElement instanceof HTMLElement) {
        try {
          lastActiveElement.focus()
        } catch {
          /* ignore */
        }
      }
      lastActiveElement = null
    }
  },
  { immediate: true },
)

onBeforeUnmount(() => {
  clearInitialFocusTimer()
  window.removeEventListener('keydown', onKeyDown, true)
  removeModalFromStack(modalId)
  unlockScroll()
})
</script>

<style scoped>
.modal-fade-enter-active,
.modal-fade-leave-active {
  transition: opacity 0.2s ease;
}
.modal-fade-enter-from,
.modal-fade-leave-to {
  opacity: 0;
}

.modal-slide-enter-active,
.modal-slide-leave-active {
  transition: all 0.2s ease;
}
.modal-slide-enter-from,
.modal-slide-leave-to {
  opacity: 0;
  transform: translateY(16px);
}
</style>

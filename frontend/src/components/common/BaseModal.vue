<template>
  <TransitionRoot as="template" :show="open">
    <Dialog as="div" class="modal-backdrop" :style="backdropStyle" :open="open" @close="emitClose">
      <div class="modal-overlay" aria-hidden="true"></div>
      <div class="modal-wrapper">
        <TransitionChild
          as="template"
          enter="ease-out duration-200"
          enter-from="opacity-0 translate-y-4"
          enter-to="opacity-100 translate-y-0"
          leave="ease-in duration-150"
          leave-from="opacity-100 translate-y-0"
          leave-to="opacity-0 translate-y-4"
        >
          <DialogPanel :class="['modal', variantClass]" :style="panelStyle">
            <header class="modal-header">
              <DialogTitle class="modal-title">{{ title }}</DialogTitle>
              <button class="ghost-icon" type="button" aria-label="Close" @click="emitClose">✕</button>
            </header>
            <div
              :class="['modal-body', { 'modal-scrollable': bodyScrollable }]"
              :style="bodyStyle"
            >
              <slot />
            </div>
          </DialogPanel>
        </TransitionChild>
      </div>
    </Dialog>
  </TransitionRoot>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, watch, type CSSProperties } from 'vue'
import { Dialog, DialogPanel, DialogTitle, TransitionChild, TransitionRoot } from '@headlessui/vue'
import { getModalStackIndex, isTopMostModal, pushModalToTop, removeModalFromStack } from '../../utils/modalStack'

type Variant = 'default' | 'confirm'
const BASE_MODAL_Z_INDEX = 2000
const MODAL_STACK_STEP = 20

const props = withDefaults(
  defineProps<{
    open: boolean
    title: string
    variant?: Variant
    bodyScrollable?: boolean
    panelWidth?: string
  }>(),
  { variant: 'default', bodyScrollable: true, panelWidth: '' },
)

const emit = defineEmits<{ (e: 'close'): void }>()
const modalId = `base-modal-${Math.random().toString(36).slice(2, 10)}`

const modalStackIndex = computed(() => getModalStackIndex(modalId))

const emitClose = () => {
  if (!isTopMostModal(modalId)) return
  emit('close')
}

const variantClass = computed(() => (props.variant === 'confirm' ? 'confirm-modal' : ''))
const backdropStyle = computed<CSSProperties>(() => ({
  zIndex: BASE_MODAL_Z_INDEX + modalStackIndex.value * MODAL_STACK_STEP,
}))
const panelStyle = computed<CSSProperties | undefined>(() => {
  if (!props.panelWidth) return undefined
  return { width: props.panelWidth }
})
const bodyStyle = computed<CSSProperties | undefined>(() => {
  if (props.bodyScrollable) return undefined
  return {
    display: 'flex',
    flexDirection: 'column' as const,
    minHeight: '0',
    flex: '1 1 auto',
  }
})

watch(
  () => props.open,
  (open) => {
    if (open) {
      pushModalToTop(modalId)
      return
    }
    removeModalFromStack(modalId)
  },
  { immediate: true },
)

onBeforeUnmount(() => {
  removeModalFromStack(modalId)
})
</script>

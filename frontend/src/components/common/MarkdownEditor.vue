<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch, computed } from 'vue'
import { EditorView, basicSetup } from 'codemirror'
import { markdown } from '@codemirror/lang-markdown'
import { oneDark } from '@codemirror/theme-one-dark'
import { EditorState } from '@codemirror/state'

const props = defineProps<{
  modelValue: string
}>()

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()

const editorRef = ref<HTMLElement>()
let view: EditorView | null = null

// 检测当前是否为深色模式
const isDark = computed(() => {
  return document.documentElement.classList.contains('dark')
})

onMounted(() => {
  if (!editorRef.value) return

  const extensions = [
    basicSetup,
    markdown(),
    EditorView.updateListener.of((update) => {
      if (update.docChanged) {
        emit('update:modelValue', update.state.doc.toString())
      }
    }),
    EditorView.theme({
      '&': {
        minHeight: '300px',
        maxHeight: '500px',
        fontSize: '14px'
      },
      '.cm-scroller': {
        fontFamily: "'SFMono-Regular', Menlo, Consolas, monospace",
        overflow: 'auto'
      },
      '.cm-content': {
        padding: '12px 0'
      },
      '.cm-line': {
        padding: '0 12px'
      }
    })
  ]

  if (isDark.value) {
    extensions.push(oneDark)
  }

  view = new EditorView({
    state: EditorState.create({
      doc: props.modelValue,
      extensions,
    }),
    parent: editorRef.value,
  })
})

onUnmounted(() => {
  view?.destroy()
})

// 外部值变化时同步
watch(() => props.modelValue, (newVal) => {
  if (view && view.state.doc.toString() !== newVal) {
    view.dispatch({
      changes: { from: 0, to: view.state.doc.length, insert: newVal }
    })
  }
})
</script>

<template>
  <div ref="editorRef" class="markdown-editor" :class="{ dark: isDark }"></div>
</template>

<style>
.markdown-editor {
  border: 1px solid var(--mac-border);
  border-radius: 12px;
  overflow: hidden;
  background: var(--mac-bg);
}

.markdown-editor:focus-within {
  border-color: var(--mac-accent);
  box-shadow: 0 0 0 3px rgba(10, 132, 255, 0.15);
}

.markdown-editor .cm-editor {
  min-height: 300px;
  max-height: 500px;
}

.markdown-editor .cm-focused {
  outline: none;
}

/* Light theme overrides */
.markdown-editor:not(.dark) .cm-editor {
  background: var(--mac-bg);
}

.markdown-editor:not(.dark) .cm-gutters {
  background: var(--mac-surface);
  border-right: 1px solid var(--mac-border);
}

/* Dark theme is handled by oneDark extension */
</style>

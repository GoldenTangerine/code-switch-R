<template>
  <div
    class="json-code-editor"
    :class="{
      'is-invalid': invalid,
      'is-readonly': readonly,
    }"
  >
    <div
      ref="editorHostRef"
      class="json-code-editor__surface"
      :style="surfaceStyle"
      :aria-invalid="invalid ? 'true' : 'false'"
    />

    <div class="json-code-editor__footer">
      <button
        type="button"
        class="json-code-editor__format-btn"
        :disabled="readonly"
        @click="formatDocument"
      >
        {{ t('components.cliConfig.jsonEditor.format') }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { EditorView, basicSetup } from 'codemirror'
import { Compartment, EditorState } from '@codemirror/state'
import type { Extension } from '@codemirror/state'
import { json } from '@codemirror/lang-json'
import { lintGutter, linter } from '@codemirror/lint'
import type { Diagnostic } from '@codemirror/lint'
import { oneDark } from '@codemirror/theme-one-dark'
import { placeholder as editorPlaceholder } from '@codemirror/view'

const props = withDefaults(defineProps<{
  modelValue?: string
  rows?: number
  readonly?: boolean
  invalid?: boolean
  placeholder?: string
  showValidation?: boolean
  mode?: 'json' | 'plain'
  surfaceHeight?: string
}>(), {
  modelValue: '',
  rows: 14,
  readonly: false,
  invalid: false,
  placeholder: '',
  showValidation: true,
  mode: 'json',
  surfaceHeight: '',
})

const emit = defineEmits<{
  (e: 'update:modelValue', value: string): void
  (e: 'format'): void
}>()

const { t } = useI18n()

const editorHostRef = ref<HTMLDivElement | null>(null)
const isDarkMode = ref(false)
const WHEEL_LINE_HEIGHT_PX = 16
const SCROLL_EPSILON = 1

let view: EditorView | null = null
let themeObserver: MutationObserver | null = null
let syncingFromProps = false
let editorWheelCleanup: (() => void) | null = null

const surfaceStyle = computed(() => {
  const surfaceHeight = props.surfaceHeight.trim()
  if (!surfaceHeight) return undefined

  return {
    height: surfaceHeight,
    minHeight: surfaceHeight,
  }
})

const layoutCompartment = new Compartment()
const readOnlyCompartment = new Compartment()
const placeholderCompartment = new Compartment()
const validationCompartment = new Compartment()
const themeCompartment = new Compartment()
const languageCompartment = new Compartment()

const stripJsonErrorMessage = (message: string) => message
  .replace(/^JSON\.parse:\s*/i, '')
  .replace(/\s+of the JSON data$/i, '')
  .trim()

const formatJsonErrorMessage = (error: unknown): string => {
  if (!(error instanceof SyntaxError)) {
    return t('components.cliConfig.jsonEditor.errors.invalidJsonGeneric')
  }

  const rawMessage = error.message
  const positionMatch = rawMessage.match(/at position (\d+)/i)
  if (positionMatch) {
    const detail = stripJsonErrorMessage(rawMessage.replace(/\s+in JSON at position \d+/i, ''))
    return t('components.cliConfig.jsonEditor.errors.invalidJsonAtPosition', {
      message: detail || t('components.cliConfig.jsonEditor.errors.invalidJsonGeneric'),
      position: positionMatch[1],
    })
  }

  const lineColumnMatch = rawMessage.match(/line (\d+) column (\d+)/i)
  if (lineColumnMatch) {
    return t('components.cliConfig.jsonEditor.errors.invalidJsonAtLineColumn', {
      line: lineColumnMatch[1],
      column: lineColumnMatch[2],
    })
  }

  return t('components.cliConfig.jsonEditor.errors.invalidJson', {
    message: stripJsonErrorMessage(rawMessage) || t('components.cliConfig.jsonEditor.errors.invalidJsonGeneric'),
  })
}

const resolveDiagnosticRange = (doc: string, error: unknown) => {
  if (error instanceof SyntaxError) {
    const positionMatch = error.message.match(/at position (\d+)/i)
    if (positionMatch) {
      const position = Number.parseInt(positionMatch[1], 10)
      const from = Number.isFinite(position) ? Math.min(Math.max(position, 0), Math.max(doc.length - 1, 0)) : 0
      return {
        from,
        to: Math.min(from + 1, Math.max(doc.length, 1)),
      }
    }
  }

  return {
    from: 0,
    to: Math.max(doc.length, 1),
  }
}

const jsonLinter = linter((editorView) => {
  const diagnostics: Diagnostic[] = []
  if (!props.showValidation) return diagnostics

  const doc = editorView.state.doc.toString()
  const trimmed = doc.trim()
  if (!trimmed) return diagnostics

  try {
    const parsed = JSON.parse(trimmed)
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
      diagnostics.push({
        from: 0,
        to: Math.max(doc.length, 1),
        severity: 'error',
        message: t('components.cliConfig.jsonEditor.errors.mustBeObject'),
      })
    }
  } catch (error) {
    const range = resolveDiagnosticRange(doc, error)
    diagnostics.push({
      from: range.from,
      to: range.to,
      severity: 'error',
      message: formatJsonErrorMessage(error),
    })
  }

  return diagnostics
})

const baseTheme = EditorView.theme({
  '.cm-scroller': {
    overflow: 'auto',
    overscrollBehavior: 'contain',
    fontFamily: "ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace",
  },
  '.cm-content': {
    padding: '12px 0',
    fontSize: '13px',
    lineHeight: '1.65',
    caretColor: 'var(--mac-text)',
  },
  '.cm-line': {
    padding: '0 14px',
  },
  '.cm-gutters': {
    minWidth: '52px',
    borderRight: '1px solid var(--mac-border)',
    background: 'color-mix(in srgb, var(--mac-surface) 82%, transparent)',
    color: 'var(--mac-text-secondary)',
  },
  '.cm-gutterElement': {
    padding: '0 12px 0 10px',
  },
  '.cm-activeLine': {
    background: 'color-mix(in srgb, var(--mac-accent) 8%, transparent)',
  },
  '.cm-activeLineGutter': {
    background: 'color-mix(in srgb, var(--mac-accent) 10%, transparent)',
  },
  '.cm-selectionBackground, .cm-content ::selection': {
    background: 'color-mix(in srgb, var(--mac-accent) 20%, transparent)',
  },
  '.cm-tooltip-lint': {
    border: '1px solid var(--mac-border)',
    borderRadius: '10px',
    background: 'var(--mac-surface)',
    color: 'var(--mac-text)',
    boxShadow: '0 14px 40px rgba(15, 23, 42, 0.18)',
    maxWidth: 'min(420px, 82vw)',
  },
  '.cm-diagnostic': {
    padding: '8px 10px',
  },
  '.cm-lintRange-error': {
    backgroundImage: 'linear-gradient(to right, color-mix(in srgb, #ff453a 75%, transparent) 50%, transparent 50%)',
    backgroundPosition: 'left bottom',
    backgroundRepeat: 'repeat-x',
    backgroundSize: '6px 2px',
  },
})

const createLayoutExtension = () => {
  const minHeightPx = Math.max(1, props.rows) * 20
  return EditorView.theme({
    '&': {
      minHeight: `${minHeightPx}px`,
      background: 'transparent',
    },
  })
}

const createReadOnlyExtension = () => [
  EditorState.readOnly.of(props.readonly),
  EditorView.editable.of(!props.readonly),
]

const createPlaceholderExtension = () => editorPlaceholder(props.placeholder)

const createValidationExtension = () => (
  props.showValidation && props.mode === 'json' ? [jsonLinter, lintGutter()] : []
)

const createLanguageExtension = () => (
  props.mode === 'json' ? [json()] : []
)

const createThemeExtension = () => {
  if (!isDarkMode.value) {
    return []
  }

  return [
    oneDark,
    EditorView.theme({
      '&': {
        background: 'transparent',
      },
      '.cm-gutters': {
        background: 'color-mix(in srgb, var(--mac-surface-strong) 82%, transparent)',
        borderRight: '1px solid var(--mac-border)',
      },
      '.cm-tooltip-lint': {
        background: 'var(--mac-surface)',
        color: 'var(--mac-text)',
      },
    }),
  ]
}

const reconfigureCompartment = (compartment: Compartment, extension: Extension) => {
  if (!view) return
  view.dispatch({
    effects: compartment.reconfigure(extension),
  })
}

const normalizeWheelDeltaY = (event: WheelEvent, referenceElement: HTMLElement) => {
  if (event.deltaMode === WheelEvent.DOM_DELTA_LINE) {
    return event.deltaY * WHEEL_LINE_HEIGHT_PX
  }
  if (event.deltaMode === WheelEvent.DOM_DELTA_PAGE) {
    return event.deltaY * referenceElement.clientHeight
  }
  return event.deltaY
}

const canScrollWithin = (element: HTMLElement, deltaY: number) => {
  if (element.scrollHeight <= element.clientHeight + SCROLL_EPSILON) {
    return false
  }

  if (deltaY < 0) {
    return element.scrollTop > SCROLL_EPSILON
  }

  if (deltaY > 0) {
    return element.scrollTop + element.clientHeight < element.scrollHeight - SCROLL_EPSILON
  }

  return false
}

const findScrollableAncestor = (startElement: HTMLElement | null, exclude: HTMLElement | null) => {
  let current = startElement

  while (current) {
    if (current !== exclude) {
      const style = window.getComputedStyle(current)
      const overflowY = style.overflowY
      const isScrollable = (overflowY === 'auto' || overflowY === 'scroll' || overflowY === 'overlay')
        && current.scrollHeight > current.clientHeight + SCROLL_EPSILON

      if (isScrollable) {
        return current
      }
    }

    current = current.parentElement
  }

  return null
}

const handleEditorWheel = (event: WheelEvent) => {
  if (!view || event.defaultPrevented || event.ctrlKey) return

  const editorScroller = view.scrollDOM as HTMLElement
  if (!editorScroller) return

  const deltaY = normalizeWheelDeltaY(event, editorScroller)
  if (!deltaY) return
  if (canScrollWithin(editorScroller, deltaY)) return

  const parentScroller = findScrollableAncestor(editorHostRef.value?.parentElement ?? null, editorScroller)
  if (!parentScroller) return

  parentScroller.scrollBy({
    top: deltaY,
    behavior: 'auto',
  })
  event.preventDefault()
}

const createEditor = (doc = props.modelValue ?? '') => {
  if (!editorHostRef.value) return

  view = new EditorView({
    state: EditorState.create({
      doc,
      extensions: [
        basicSetup,
        baseTheme,
        languageCompartment.of(createLanguageExtension()),
        layoutCompartment.of(createLayoutExtension()),
        readOnlyCompartment.of(createReadOnlyExtension()),
        placeholderCompartment.of(createPlaceholderExtension()),
        validationCompartment.of(createValidationExtension()),
        themeCompartment.of(createThemeExtension()),
        EditorView.updateListener.of((update) => {
          if (update.docChanged && !syncingFromProps) {
            emit('update:modelValue', update.state.doc.toString())
          }
        }),
      ],
    }),
    parent: editorHostRef.value,
  })

  const editorScroller = view.scrollDOM as HTMLElement
  editorScroller.addEventListener('wheel', handleEditorWheel, { passive: false })
  editorWheelCleanup = () => {
    editorScroller.removeEventListener('wheel', handleEditorWheel)
  }
}

const destroyEditor = () => {
  editorWheelCleanup?.()
  editorWheelCleanup = null
  view?.destroy()
  view = null
}

const focus = () => {
  view?.focus()
}

const replaceDocument = (nextValue: string) => {
  if (!view || view.state.doc.toString() === nextValue) return

  syncingFromProps = true
  try {
    view.dispatch({
      changes: {
        from: 0,
        to: view.state.doc.length,
        insert: nextValue,
      },
    })
  } finally {
    syncingFromProps = false
  }
}

const formatDocument = () => {
  if (props.readonly || !view) return
  emit('format')
}

defineExpose({ focus })

watch(() => props.modelValue, (value) => {
  replaceDocument(value ?? '')
})

watch(() => props.rows, () => {
  reconfigureCompartment(layoutCompartment, createLayoutExtension())
})

watch(() => props.readonly, () => {
  reconfigureCompartment(readOnlyCompartment, createReadOnlyExtension())
})

watch(() => props.placeholder, () => {
  reconfigureCompartment(placeholderCompartment, createPlaceholderExtension())
})

watch(() => props.showValidation, () => {
  reconfigureCompartment(validationCompartment, createValidationExtension())
})

watch(() => props.mode, () => {
  reconfigureCompartment(languageCompartment, createLanguageExtension())
  reconfigureCompartment(validationCompartment, createValidationExtension())
})

watch(isDarkMode, () => {
  reconfigureCompartment(themeCompartment, createThemeExtension())
})

onMounted(() => {
  isDarkMode.value = document.documentElement.classList.contains('dark')
  themeObserver = new MutationObserver(() => {
    isDarkMode.value = document.documentElement.classList.contains('dark')
  })
  themeObserver.observe(document.documentElement, {
    attributes: true,
    attributeFilter: ['class'],
  })

  createEditor()
})

onUnmounted(() => {
  themeObserver?.disconnect()
  themeObserver = null
  destroyEditor()
})
</script>

<style scoped>
.json-code-editor {
  display: flex;
  flex-direction: column;
  border: 1px solid var(--mac-border);
  border-radius: 16px;
  background: color-mix(in srgb, var(--mac-surface-strong) 94%, transparent);
  overflow: hidden;
  transition: border-color 0.2s ease, box-shadow 0.2s ease;
}

.json-code-editor:focus-within {
  border-color: color-mix(in srgb, var(--mac-accent) 55%, var(--mac-border));
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--mac-accent) 14%, transparent);
}

.json-code-editor.is-invalid {
  border-color: color-mix(in srgb, #ff453a 62%, var(--mac-border));
}

.json-code-editor.is-invalid:focus-within {
  box-shadow: 0 0 0 3px color-mix(in srgb, #ff453a 14%, transparent);
}

.json-code-editor__surface {
  min-height: 280px;
}

.json-code-editor__footer {
  display: flex;
  justify-content: flex-end;
  padding: 10px 12px 12px;
  border-top: 1px solid var(--mac-border);
  background: color-mix(in srgb, var(--mac-surface) 72%, transparent);
}

.json-code-editor__format-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 32px;
  padding: 0 12px;
  border: 1px solid var(--mac-border);
  border-radius: 10px;
  background: var(--mac-surface);
  color: var(--mac-text-secondary);
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
}

.json-code-editor__format-btn:hover:not(:disabled) {
  border-color: color-mix(in srgb, var(--mac-accent) 45%, var(--mac-border));
  color: var(--mac-accent);
}

.json-code-editor__format-btn:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.json-code-editor :deep(.cm-editor) {
  height: 100%;
  background: transparent;
}

.json-code-editor :deep(.cm-focused) {
  outline: none;
}

.json-code-editor.is-readonly :deep(.cm-content) {
  cursor: default;
}

.json-code-editor.is-readonly :deep(.cm-cursor) {
  display: none;
}
</style>

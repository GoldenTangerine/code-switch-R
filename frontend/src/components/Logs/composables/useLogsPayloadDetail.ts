import { computed, reactive, ref } from 'vue'
import { fetchRequestLogPayload, type RequestLog, type RequestLogPayloadDetail } from '../../../services/logs'
import { buildPayloadPreview } from '../../../utils/payloadPreview'
import { showToast } from '../../../utils/toast'
import { extractErrorMessage } from '../../../utils/error'

type TranslateFn = (key: string, params?: Record<string, unknown>) => string

type UseLogsPayloadDetailOptions = {
  t: TranslateFn
}

type PayloadDetailKind = 'request' | 'response'
type PayloadCopyMode = 'raw' | 'formatted'

const copyTextFallback = (text: string) => {
  if (typeof document === 'undefined') return false
  const textarea = document.createElement('textarea')
  textarea.value = text
  textarea.setAttribute('readonly', 'true')
  textarea.style.position = 'fixed'
  textarea.style.opacity = '0'
  textarea.style.pointerEvents = 'none'
  document.body.appendChild(textarea)
  textarea.select()
  textarea.setSelectionRange(0, textarea.value.length)
  const copied = document.execCommand('copy')
  document.body.removeChild(textarea)
  return copied
}

export function useLogsPayloadDetail(options: UseLogsPayloadDetailOptions) {
  const { t } = options

  const payloadDetailModal = reactive<{
    open: boolean
    loading: boolean
    logId: number
    detail: RequestLogPayloadDetail | null
  }>({
    open: false,
    loading: false,
    logId: 0,
    detail: null,
  })
  const payloadDetailRequestSeq = ref(0)

  const requestPayloadPreview = computed(() => buildPayloadPreview(payloadDetailModal.detail?.request_body))
  const responsePayloadPreview = computed(() => buildPayloadPreview(payloadDetailModal.detail?.response_body))

  const copyPayloadDetail = async (kind: PayloadDetailKind, mode: PayloadCopyMode) => {
    const preview = kind === 'request' ? requestPayloadPreview.value : responsePayloadPreview.value
    const textToCopy = mode === 'formatted' ? preview.renderedText : preview.rawText
    if (!textToCopy) {
      showToast(t('components.logs.payloadDetail.copyEmpty'), 'warning')
      return
    }

    const targetLabel = kind === 'request'
      ? t('components.logs.payloadDetail.requestBody')
      : t('components.logs.payloadDetail.responseBody')

    try {
      if (typeof navigator !== 'undefined' && navigator.clipboard?.writeText) {
        await navigator.clipboard.writeText(textToCopy)
      } else if (!copyTextFallback(textToCopy)) {
        throw new Error(t('components.logs.payloadDetail.copyUnavailable'))
      }
      showToast(
        t(
          mode === 'formatted'
            ? 'components.logs.payloadDetail.copySuccessFormatted'
            : 'components.logs.payloadDetail.copySuccessRaw',
          { target: targetLabel },
        ),
        'success',
      )
    } catch (error) {
      showToast(
        t('components.logs.payloadDetail.copyFailed', { error: extractErrorMessage(error) }),
        'error',
      )
    }
  }

  const openPayloadDetailModal = async (item: RequestLog) => {
    const logId = Number(item.id ?? 0)
    if (!Number.isFinite(logId) || logId <= 0) return
    payloadDetailModal.open = true
    payloadDetailModal.loading = true
    payloadDetailModal.logId = logId
    payloadDetailModal.detail = null

    const requestSeq = ++payloadDetailRequestSeq.value
    try {
      const detail = await fetchRequestLogPayload(logId)
      if (requestSeq !== payloadDetailRequestSeq.value) return
      payloadDetailModal.detail = detail
    } catch (error) {
      if (requestSeq !== payloadDetailRequestSeq.value) return
      showToast(
        t('components.logs.payloadDetail.loadFailed', {
          error: extractErrorMessage(error),
        }),
        'error',
      )
    } finally {
      if (requestSeq === payloadDetailRequestSeq.value) {
        payloadDetailModal.loading = false
      }
    }
  }

  const closePayloadDetailModal = () => {
    payloadDetailRequestSeq.value += 1
    payloadDetailModal.open = false
    payloadDetailModal.loading = false
  }

  return {
    payloadDetailModal,
    requestPayloadPreview,
    responsePayloadPreview,
    copyPayloadDetail,
    openPayloadDetailModal,
    closePayloadDetailModal,
  }
}

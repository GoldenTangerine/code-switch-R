import { computed, reactive, ref } from 'vue'
import { fetchRequestLogPayload, type RequestLog, type RequestLogPayloadDetail } from '../../../services/logs'
import { writeTextToClipboard } from '../../../utils/clipboard'
import { buildPayloadPreview } from '../../../utils/payloadPreview'
import { showToast } from '../../../utils/toast'
import { extractErrorMessage } from '../../../utils/error'

type TranslateFn = (key: string, params?: Record<string, unknown>) => string

type UseLogsPayloadDetailOptions = {
  t: TranslateFn
}

type PayloadDetailKind = 'request' | 'response'
type PayloadCopyMode = 'raw' | 'formatted'

export function useLogsPayloadDetail(options: UseLogsPayloadDetailOptions) {
  const { t } = options

  const payloadDetailModal = reactive<{
    open: boolean
    loading: boolean
    logId: number
    log: RequestLog | null
    detail: RequestLogPayloadDetail | null
  }>({
    open: false,
    loading: false,
    logId: 0,
    log: null,
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
      await writeTextToClipboard(textToCopy)
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
    payloadDetailModal.log = item
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
    payloadDetailModal.log = null
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

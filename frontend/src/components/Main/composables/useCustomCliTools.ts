import { computed, reactive, ref } from 'vue'
import {
  createCustomCliTool,
  deleteCustomCliTool,
  getCustomCliProxyStatus,
  listCustomCliTools,
  updateCustomCliTool,
  type CustomCliTool,
} from '../../../services/customCliService'
import { showToast } from '../../../utils/toast'
import type { CustomCliToolDraft, TranslateFn } from '../types'

type UseCustomCliToolsOptions = {
  t: TranslateFn
  setOthersProxyState: (enabled: boolean) => void
  loadCustomCliProviders: (toolId: string) => Promise<void>
  clearOthersCards: () => void
}

export function useCustomCliTools(options: UseCustomCliToolsOptions) {
  const { t, setOthersProxyState, loadCustomCliProviders, clearOthersCards } = options

  const customCliTools = ref<CustomCliTool[]>([])
  const selectedToolId = ref<string | null>(null)
  const customCliProxyStates = reactive<Record<string, boolean>>({})

  const selectedCustomCliTool = computed(() => {
    if (!selectedToolId.value) return null
    return customCliTools.value.find((tool) => tool.id === selectedToolId.value) || null
  })

  const loadCustomCliTools = async () => {
    try {
      const tools = await listCustomCliTools()
      customCliTools.value = tools

      const hasSelectedTool = selectedToolId.value
        ? tools.some((tool) => tool.id === selectedToolId.value)
        : false
      selectedToolId.value = hasSelectedTool ? selectedToolId.value : (tools[0]?.id ?? null)

      const activeToolIds = new Set(tools.map((tool) => tool.id))
      Object.keys(customCliProxyStates).forEach((toolId) => {
        if (!activeToolIds.has(toolId)) {
          delete customCliProxyStates[toolId]
        }
      })

      const statusEntries = await Promise.all(tools.map(async (tool) => {
        try {
          const status = await getCustomCliProxyStatus(tool.id)
          return [tool.id, Boolean(status?.enabled)] as const
        } catch {
          return [tool.id, false] as const
        }
      }))

      for (const [toolId, enabled] of statusEntries) {
        customCliProxyStates[toolId] = enabled
      }

      if (selectedToolId.value) {
        setOthersProxyState(customCliProxyStates[selectedToolId.value] ?? false)
        await loadCustomCliProviders(selectedToolId.value)
      } else {
        setOthersProxyState(false)
        clearOthersCards()
      }
    } catch (error) {
      console.error('Failed to load custom CLI tools', error)
      customCliTools.value = []
      setOthersProxyState(false)
      clearOthersCards()
    }
  }

  const onToolSelect = async () => {
    if (!selectedToolId.value) {
      setOthersProxyState(false)
      clearOthersCards()
      return
    }

    setOthersProxyState(customCliProxyStates[selectedToolId.value] ?? false)
    await loadCustomCliProviders(selectedToolId.value)
  }

  const saveCliTool = async (draft: CustomCliToolDraft, editingId: string | null = null) => {
    const name = draft.name.trim()
    if (!name) {
      showToast(t('components.main.customCli.nameRequired'), 'error')
      return false
    }

    const validConfigFiles = draft.configFiles
      .map((configFile) => ({
        ...configFile,
        label: configFile.label.trim(),
        path: configFile.path.trim(),
      }))
      .filter((configFile) => configFile.path)

    if (validConfigFiles.length === 0) {
      showToast(t('components.main.customCli.configRequired'), 'error')
      return false
    }

    const hasPrimary = validConfigFiles.some((configFile) => configFile.isPrimary)
    if (!hasPrimary) {
      validConfigFiles[0].isPrimary = true
    }

    const autoTargetFileId = validConfigFiles.length === 1 ? validConfigFiles[0].id : ''
    const proxyInjectionsToSave = draft.proxyInjection
      .map((proxyInjection) => {
        const baseUrlField = proxyInjection.baseUrlField.trim()
        const authTokenField = (proxyInjection.authTokenField ?? '').trim()
        const targetFileId =
          proxyInjection.targetFileId.trim() || ((baseUrlField || authTokenField) ? autoTargetFileId : '')

        return {
          targetFileId,
          baseUrlField,
          authTokenField,
        }
      })
      .filter((proxyInjection) => (
        proxyInjection.targetFileId || proxyInjection.baseUrlField || proxyInjection.authTokenField
      ))

    const hasIncompleteProxyInjection = proxyInjectionsToSave.some(
      (proxyInjection) => !proxyInjection.targetFileId || !proxyInjection.baseUrlField,
    )
    if (hasIncompleteProxyInjection) {
      showToast(t('components.main.customCli.proxyInjectionIncomplete'), 'error')
      return false
    }

    const allFileIds = new Set(draft.configFiles.map((configFile) => configFile.id))
    const validFileIds = new Set(validConfigFiles.map((configFile) => configFile.id))

    const hasInvalidProxyTarget = proxyInjectionsToSave.some((proxyInjection) => !allFileIds.has(proxyInjection.targetFileId))
    if (hasInvalidProxyTarget) {
      showToast(t('components.main.customCli.invalidProxyTarget'), 'error')
      return false
    }

    const hasProxyTargetPathMissing = proxyInjectionsToSave.some((proxyInjection) => !validFileIds.has(proxyInjection.targetFileId))
    if (hasProxyTargetPathMissing) {
      showToast(t('components.main.customCli.proxyTargetPathRequired'), 'error')
      return false
    }

    try {
      if (editingId) {
        await updateCustomCliTool(editingId, {
          id: editingId,
          name,
          configFiles: validConfigFiles,
          proxyInjection: proxyInjectionsToSave,
        })
        showToast(t('components.main.customCli.updateSuccess'), 'success')
      } else {
        const newTool = await createCustomCliTool({
          name,
          configFiles: validConfigFiles,
          proxyInjection: proxyInjectionsToSave,
        })
        selectedToolId.value = newTool.id
        showToast(t('components.main.customCli.createSuccess'), 'success')
      }

      await loadCustomCliTools()
      return true
    } catch (error) {
      console.error('Failed to save CLI tool', error)
      const message = error instanceof Error ? error.message : String(error ?? '')
      if (message.includes('ERR_CUSTOM_CLI_PROXY_INJECTION_INCOMPLETE')) {
        showToast(t('components.main.customCli.proxyInjectionIncomplete'), 'error')
        return false
      }
      if (message.includes('ERR_CUSTOM_CLI_INVALID_PROXY_TARGET')) {
        showToast(t('components.main.customCli.invalidProxyTarget'), 'error')
        return false
      }
      showToast(t('components.main.customCli.saveFailed'), 'error')
      return false
    }
  }

  const deleteCliToolById = async (toolId: string) => {
    try {
      await deleteCustomCliTool(toolId)
      showToast(t('components.main.customCli.deleteSuccess'), 'success')

      if (selectedToolId.value === toolId) {
        selectedToolId.value = null
        setOthersProxyState(false)
      }

      await loadCustomCliTools()
      return true
    } catch (error) {
      console.error('Failed to delete CLI tool', error)
      showToast(t('components.main.customCli.deleteFailed'), 'error')
      return false
    }
  }

  return {
    customCliTools,
    selectedToolId,
    customCliProxyStates,
    selectedCustomCliTool,
    loadCustomCliTools,
    onToolSelect,
    saveCliTool,
    deleteCliToolById,
  }
}

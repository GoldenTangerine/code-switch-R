import {
  AddProvider,
  DeleteProvider,
  DuplicateProvider,
  GetPresets,
  GetProviders,
  GetLiveProviderIds,
  ImportFromLive,
  ReorderProviders,
  SaveProviders,
  UpdateProvider,
} from '../../bindings/codeswitch/services/opencodeservice'
import type {
  OpenCodeProvider as OpenCodeProviderModel,
  OpenCodeProviderPreset as OpenCodeProviderPresetModel,
} from '../../bindings/codeswitch/services/models'

export type OpenCodeProvider = OpenCodeProviderModel & { icon?: string }
export type OpenCodeProviderPreset = OpenCodeProviderPresetModel

export const getOpenCodeProviders = (): Promise<OpenCodeProvider[]> => (
  GetProviders()
)

export const getOpenCodeLiveProviderIds = (): Promise<string[]> => (
  GetLiveProviderIds()
)

export const addOpenCodeProvider = (provider: OpenCodeProvider): Promise<void> => (
  AddProvider(provider)
)

export const updateOpenCodeProvider = (provider: OpenCodeProvider): Promise<void> => (
  UpdateProvider(provider)
)

export const deleteOpenCodeProvider = (id: string): Promise<void> => (
  DeleteProvider(id)
)

export const reorderOpenCodeProviders = (ids: string[]): Promise<void> => (
  ReorderProviders(ids)
)

export const saveOpenCodeProviders = (providers: OpenCodeProvider[]): Promise<void> => (
  SaveProviders(providers)
)

export const duplicateOpenCodeProvider = (id: string): Promise<OpenCodeProvider | null> => (
  DuplicateProvider(id)
)

export const importOpenCodeProvidersFromLive = (): Promise<number> => (
  ImportFromLive()
)

export const getOpenCodePresets = (): Promise<OpenCodeProviderPreset[]> => (
  GetPresets()
)

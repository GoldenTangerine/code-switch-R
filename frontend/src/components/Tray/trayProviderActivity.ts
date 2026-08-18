/**
 * @name: 托盘供应商调用状态
 * @Descripttion: 归并实时并发状态与最近调用供应商，供托盘展示使用
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-18 15:08:00
 * @LastEditTime: 2026-08-18 15:08:00
 * @FilePath: frontend/src/components/Tray/trayProviderActivity.ts
 */

export type TrayProviderActivityStatus = {
  providerId: string
  providerName: string
  activeRequests: number
}

export type TrayDefaultProvider = {
  providerId: string
  providerName: string
}

export type TrayProviderActivity = TrayProviderActivityStatus & {
  status: 'active' | 'default'
}

export type TrayProviderRuntimeState = {
  statuses: TrayProviderActivityStatus[]
  defaultProvider: TrayDefaultProvider | null
  error: boolean
}

export type TrayProviderActivitySnapshot = {
  statesByPlatform: Partial<Record<string, TrayProviderRuntimeState>> | null
  error?: unknown
}

type TrayProviderActivitySnapshotLoader = {
  loadStates: () => Promise<unknown>
}

function asRecord(value: unknown): Record<string, unknown> | null {
  return value !== null && typeof value === 'object'
    ? value as Record<string, unknown>
    : null
}

export function normalizeTrayProviderRef(value: unknown): string {
  return String(value ?? '').trim()
}

export function buildTrayProviderActivityRefreshKey(
  platforms: readonly string[],
  generation: number,
): string {
  return JSON.stringify([generation, ...platforms])
}

export function normalizeTrayProviderActivityStatus(value: unknown): TrayProviderActivityStatus {
  const status = asRecord(value) ?? {}
  return {
    providerId: normalizeTrayProviderRef(status.providerId ?? status.ProviderID),
    providerName: String(status.providerName ?? status.ProviderName ?? '').trim(),
    activeRequests: Number(status.activeRequests ?? status.ActiveRequests ?? 0),
  }
}

export function normalizeTrayDefaultProvider(value: unknown): TrayDefaultProvider | null {
  const provider = asRecord(value)
  if (!provider) return null
  const providerId = normalizeTrayProviderRef(provider.providerId ?? provider.ProviderID)
  const providerName = String(provider.providerName ?? provider.ProviderName ?? '').trim()
  if (!providerId && !providerName) return null
  return { providerId, providerName }
}

export function normalizeTrayProviderRuntimeState(value: unknown): TrayProviderRuntimeState {
  const state = asRecord(value) ?? {}
  const statuses = state.statuses ?? state.Statuses
  return {
    statuses: Array.isArray(statuses)
      ? statuses.map(normalizeTrayProviderActivityStatus)
      : [],
    defaultProvider: normalizeTrayDefaultProvider(state.defaultProvider ?? state.DefaultProvider),
    error: Boolean(state.error ?? state.Error),
  }
}

export async function loadTrayProviderActivitySnapshot(
  platforms: readonly string[],
  loaders: TrayProviderActivitySnapshotLoader,
): Promise<TrayProviderActivitySnapshot> {
  try {
    const rawStates = asRecord(await loaders.loadStates())
    if (!rawStates) {
      return {
        statesByPlatform: null,
        error: new Error('invalid tray provider runtime snapshot'),
      }
    }
    return {
      statesByPlatform: Object.fromEntries(platforms.map((platform) => [
        platform,
        Object.prototype.hasOwnProperty.call(rawStates, platform)
          ? normalizeTrayProviderRuntimeState(rawStates[platform])
          : { statuses: [], defaultProvider: null, error: true },
      ])),
    }
  } catch (error) {
    return {
      statesByPlatform: null,
      error,
    }
  }
}

export function resolveTrayProviderActivity(
  statuses: readonly TrayProviderActivityStatus[],
  defaultProvider: TrayDefaultProvider | null | undefined,
): TrayProviderActivity[] {
  const active = statuses
    .map((status) => ({
      providerId: normalizeTrayProviderRef(status.providerId),
      providerName: String(status.providerName ?? '').trim(),
      activeRequests: Number.isFinite(Number(status.activeRequests))
        ? Math.max(Math.floor(Number(status.activeRequests)), 0)
        : 0,
    }))
    .filter((status) => status.providerId && status.activeRequests > 0)
    .map((status): TrayProviderActivity => ({ ...status, status: 'active' }))

  if (active.length > 0) return active

  const providerId = normalizeTrayProviderRef(defaultProvider?.providerId)
  const providerName = String(defaultProvider?.providerName ?? '').trim()
  if (!providerId && !providerName) return []

  return [{
    providerId,
    providerName,
    activeRequests: 0,
    status: 'default',
  }]
}

export function hasTrayProviderActivityChanged(
  previous: readonly TrayProviderActivity[],
  next: readonly TrayProviderActivity[],
): boolean {
  if (previous.length !== next.length) return true
  return previous.some((item, index) => {
    const other = next[index]
    return !other
      || item.providerId !== other.providerId
      || item.status !== other.status
  })
}

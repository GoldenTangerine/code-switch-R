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

export type TrayRecentProvider = {
  provider_id?: string
  provider_name?: string
  updated_at?: number
}

export type TrayProviderActivity = TrayProviderActivityStatus & {
  isActive: boolean
}

export type TrayProviderActivitySnapshot = {
  statusesByPlatform: Partial<Record<string, TrayProviderActivityStatus[]>> | null
  recentByPlatform: Partial<Record<string, TrayRecentProvider | null>>
  hasRecentSnapshot: boolean
  statusesError?: unknown
  recentError?: unknown
}

type TrayProviderActivitySnapshotLoader = {
  loadStatuses: () => Promise<unknown>
  loadRecent: () => Promise<unknown>
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

export function normalizeTrayRecentProvider(value: unknown): TrayRecentProvider | null {
  const recent = asRecord(value)
  if (!recent) return null
  return {
    provider_id: normalizeTrayProviderRef(recent.provider_id ?? recent.providerId ?? recent.ProviderID),
    provider_name: String(recent.provider_name ?? recent.providerName ?? recent.ProviderName ?? '').trim(),
    updated_at: Number(recent.updated_at ?? recent.updatedAt ?? recent.UpdatedAt ?? 0),
  }
}

export async function loadTrayProviderActivitySnapshot(
  platforms: readonly string[],
  loaders: TrayProviderActivitySnapshotLoader,
): Promise<TrayProviderActivitySnapshot> {
  const [statusesResult, recentResult] = await Promise.allSettled([
    loaders.loadStatuses(),
    loaders.loadRecent(),
  ])

  const rawStatuses = statusesResult.status === 'fulfilled'
    ? asRecord(statusesResult.value)
    : null
  const statusesByPlatform: Partial<Record<string, TrayProviderActivityStatus[]>> | null = rawStatuses
    ? Object.fromEntries(platforms.flatMap((platform) => {
      if (!Object.prototype.hasOwnProperty.call(rawStatuses, platform)) return []
      const statuses = rawStatuses[platform]
      return [[platform, Array.isArray(statuses)
        ? statuses.map(normalizeTrayProviderActivityStatus)
        : []]]
    }))
    : null

  const rawRecent = recentResult.status === 'fulfilled'
    ? asRecord(recentResult.value)
    : null
  const recentByPlatform: Partial<Record<string, TrayRecentProvider | null>> = rawRecent
    ? Object.fromEntries(platforms.map((platform) => [
      platform,
      normalizeTrayRecentProvider(rawRecent[platform]),
    ]))
    : {}

  return {
    statusesByPlatform,
    recentByPlatform,
    hasRecentSnapshot: Boolean(rawRecent),
    statusesError: statusesResult.status === 'rejected'
      ? statusesResult.reason
      : rawStatuses ? undefined : new Error('invalid provider activity status snapshot'),
    recentError: recentResult.status === 'rejected'
      ? recentResult.reason
      : rawRecent ? undefined : new Error('invalid recent provider snapshot'),
  }
}

export function resolveTrayProviderActivity(
  statuses: readonly TrayProviderActivityStatus[],
  recentProvider: TrayRecentProvider | null | undefined,
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
    .map((status) => ({ ...status, isActive: true }))

  if (active.length > 0) return active

  const providerId = normalizeTrayProviderRef(recentProvider?.provider_id)
  const providerName = String(recentProvider?.provider_name ?? '').trim()
  if (!providerId && !providerName) return []

  return [{
    providerId,
    providerName,
    activeRequests: 0,
    isActive: false,
  }]
}

export function resolveTrayProviderActivityWithFallback(
  statuses: readonly TrayProviderActivityStatus[],
  recentProvider: TrayRecentProvider | null | undefined,
  previous: readonly TrayProviderActivity[],
  hasRecentSnapshot: boolean,
): TrayProviderActivity[] {
  const active = resolveTrayProviderActivity(statuses, null)
  if (active.length > 0) return active
  if (hasRecentSnapshot) return resolveTrayProviderActivity(statuses, recentProvider)

  const fallback = previous[0]
  if (!fallback) return []
  return [{
    providerId: fallback.providerId,
    providerName: fallback.providerName,
    activeRequests: 0,
    isActive: false,
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
      || item.isActive !== other.isActive
  })
}

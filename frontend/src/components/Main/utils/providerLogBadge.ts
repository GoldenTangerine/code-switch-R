/**
 * @name: 供应商日志红点
 * @Descripttion: 统一判断供应商日志红点及未读快捷交互是否启用
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-07-23 12:43:49
 * @LastEditTime: 2026-07-23 12:43:49
 * @FilePath: frontend/src/components/Main/utils/providerLogBadge.ts
 */

export function shouldShowProviderLogBadge(hideLogBadge: boolean | undefined, hasUnreadErrorLogs: boolean): boolean {
  return hideLogBadge !== true && hasUnreadErrorLogs
}

/**
 * @name: CLI 更新查询接口
 * @Descripttion: 通过 Wails 查询本机 CLI 版本与更新状态。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-11-20 14:40:19
 * @LastEditTime: 2026-11-20 14:40:19
 * @FilePath: frontend/src/services/cliUpdates.ts
 */
import { Call, type CancellablePromise } from '@wailsio/runtime'

export interface CLIVersionInfo {
  command: string
  name: string
  installedVersion: string
  latestVersion: string
  status: 'updateAvailable' | 'upToDate' | 'notDetected' | 'localUnavailable' | 'latestUnavailable' | 'uncomparable'
}

export function checkCLIVersions(): CancellablePromise<CLIVersionInfo[]> {
  return Call.ByName('codeswitch/services.CLIUpdateService.CheckVersions')
}

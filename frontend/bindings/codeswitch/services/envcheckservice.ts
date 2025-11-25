// EnvConflict 环境变量冲突
export interface EnvConflict {
  varName: string    // 变量名
  varValue: string   // 变量值
  sourceType: 'system' | 'file'  // 来源类型
  sourcePath: string // 来源路径
}

import { Call } from '@wailsio/runtime'

// CheckEnvConflicts 检查指定平台的环境变量冲突
export function CheckEnvConflicts(app: string): Promise<EnvConflict[]> {
  return Call.ByName('codeswitch/services.EnvCheckService.CheckEnvConflicts', app)
}

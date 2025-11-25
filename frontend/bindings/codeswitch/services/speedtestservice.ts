// EndpointLatency 端点延迟测试结果
export interface EndpointLatency {
  url: string              // 端点 URL
  latency: number | null   // 延迟（毫秒），null 表示失败
  status?: number          // HTTP 状态码
  error?: string           // 错误信息
}

import { Call } from '@wailsio/runtime'

// TestEndpoints 测试一组端点的响应延迟
export function TestEndpoints(urls: string[], timeoutSecs?: number): Promise<EndpointLatency[]> {
  return Call.ByName('codeswitch/services.SpeedTestService.TestEndpoints', urls, timeoutSecs)
}

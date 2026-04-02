export type EChartsInstanceLike = {
  setOption: (option: Record<string, unknown>, notMerge?: boolean) => void
  resize: () => void
  dispose: () => void
}

export type EChartsStaticLike = {
  init: (element: HTMLElement) => EChartsInstanceLike
  graphic: {
    LinearGradient: new (
      x0: number,
      y0: number,
      x1: number,
      y1: number,
      colorStops: Array<{ offset: number; color: string }>,
      globalCoord?: boolean,
    ) => unknown
  }
}

declare global {
  interface Window {
    echarts?: EChartsStaticLike
  }
}

const ECHARTS_SCRIPT_ID = 'vendor-echarts-script'
let echartsPromise: Promise<EChartsStaticLike> | null = null

const resolveEChartsSrc = () => {
  const base = import.meta.env.BASE_URL || '/'
  return `${base.replace(/\/?$/, '/')}vendor-echarts.min.js`
}

export const ensureEChartsLoaded = async (): Promise<EChartsStaticLike> => {
  if (window.echarts) return window.echarts
  if (echartsPromise) return echartsPromise

  echartsPromise = new Promise<EChartsStaticLike>((resolve, reject) => {
    const existing = document.getElementById(ECHARTS_SCRIPT_ID) as HTMLScriptElement | null
    if (existing) {
      existing.addEventListener('load', () => {
        if (window.echarts) {
          resolve(window.echarts)
          return
        }
        echartsPromise = null
        reject(new Error('ECharts script loaded but global was not found'))
      }, { once: true })
      existing.addEventListener('error', () => {
        echartsPromise = null
        reject(new Error('Failed to load ECharts script'))
      }, { once: true })
      return
    }

    const script = document.createElement('script')
    script.id = ECHARTS_SCRIPT_ID
    script.src = resolveEChartsSrc()
    script.async = true
    script.onload = () => {
      if (window.echarts) {
        resolve(window.echarts)
        return
      }
      echartsPromise = null
      reject(new Error('ECharts script loaded but global was not found'))
    }
    script.onerror = () => {
      echartsPromise = null
      reject(new Error('Failed to load ECharts script'))
    }
    document.head.appendChild(script)
  })

  return echartsPromise
}

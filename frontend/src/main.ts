import { createApp } from 'vue'
import App from './App.vue'
import '@vuepic/vue-datepicker/dist/main.css'
import './style.css'
import { i18n, setupI18n } from './utils/i18n'
import { initTheme } from './utils/ThemeManager'
import router from './router/index'

type BrowserWindowWithWailsBridge = Window & {
  chrome?: {
    webview?: {
      postMessage?: (...args: any[]) => void
    }
  }
  webkit?: {
    messageHandlers?: {
      external?: {
        postMessage?: (...args: any[]) => void
      }
    }
  }
  _wails?: {
    environment?: {
      OS?: string
      Arch?: string
    }
  }
}

const hasDesktopRuntimeBridge = () => {
  if (typeof window === 'undefined') {
    return false
  }
  const browserWindow = window as BrowserWindowWithWailsBridge
  return Boolean(
    browserWindow.chrome?.webview?.postMessage ||
    browserWindow.webkit?.messageHandlers?.external?.postMessage,
  )
}

const resolvePreviewOS = () => {
  const browserNavigator = navigator as Navigator & {
    userAgentData?: {
      platform?: string
    }
  }
  const normalizedPlatform = `${browserNavigator.userAgentData?.platform ?? navigator.platform ?? navigator.userAgent ?? ''}`.toLowerCase()
  if (normalizedPlatform.includes('win')) return 'windows'
  if (normalizedPlatform.includes('mac') || normalizedPlatform.includes('darwin')) return 'darwin'
  if (normalizedPlatform.includes('linux') || normalizedPlatform.includes('x11')) return 'linux'
  return 'browser'
}

const ensureBrowserPreviewWailsEnvironment = () => {
  if (typeof window === 'undefined' || hasDesktopRuntimeBridge()) {
    return
  }

  const browserWindow = window as BrowserWindowWithWailsBridge
  browserWindow._wails = browserWindow._wails ?? {}
  browserWindow._wails.environment = {
    OS: browserWindow._wails.environment?.OS || resolvePreviewOS(),
    Arch: browserWindow._wails.environment?.Arch || 'browser',
  }
}

initTheme()
ensureBrowserPreviewWailsEnvironment()
const isMac = navigator.userAgent.includes('Mac')
if (isMac) {
  document.documentElement.classList.add('mac')
}

async function bootstrap(){
    await setupI18n('zh')//默认语言或从后端读取
    createApp(App).use(router).use(i18n).mount('#app')
}

// 启动兜底：初始化链任何未捕获错误不再表现为无声全白，渲染可见错误面板便于定位
bootstrap().catch((error) => {
    console.error('[Bootstrap] 应用启动失败:', error)
    const container = document.getElementById('app')
    if (container) {
        container.innerHTML = [
            '<div style="font-family:system-ui;padding:32px;color:#e5484d;background:#1b1e24;min-height:100vh;">',
            '<h2 style="margin:0 0 12px;">应用启动失败</h2>',
            `<pre style="white-space:pre-wrap;word-break:break-all;font-size:13px;line-height:1.6;">${String(error?.stack ?? error ?? '未知错误').replace(/[<>]/g, '')}</pre>`,
            '</div>',
        ].join('')
    }
})

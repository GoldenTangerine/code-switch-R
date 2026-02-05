import { createApp } from 'vue'
import App from './App.vue'
import '@vuepic/vue-datepicker/dist/main.css'
import './style.css'
import { i18n, setupI18n } from './utils/i18n'
import { initTheme } from './utils/ThemeManager'
import router from './router/index'

initTheme()
const isMac = navigator.userAgent.includes('Mac')
if (isMac) {
  document.documentElement.classList.add('mac')
}

async function bootstrap(){
    await setupI18n('zh')//默认语言或从后端读取
    createApp(App).use(router).use(i18n).mount('#app')
}
bootstrap()

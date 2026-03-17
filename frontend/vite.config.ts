import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [vue()],
  build: {
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (
            id.includes('plugin-vue:export-helper') ||
            id.includes('_plugin-vue_export-helper')
          ) {
            return 'vendor-core'
          }

          if (id.includes('node_modules')) {
            if (id.includes('@lobehub/icons-static-svg')) {
              return undefined
            }
            if (id.includes('chart.js') || id.includes('vue-chartjs')) {
              return 'charts'
            }
            if (id.includes('codemirror') || id.includes('@codemirror')) {
              return 'editors'
            }
            if (
              id.includes('@vuepic/vue-datepicker') ||
              id.includes('date-fns')
            ) {
              return 'datepicker'
            }
            if (id.includes('@headlessui/vue')) {
              return 'headlessui'
            }
            if (
              id.includes('/vue/') ||
              id.includes('vue-router') ||
              id.includes('vue-i18n') ||
              id.includes('@wailsio/runtime')
            ) {
              return 'vendor-core'
            }
            return 'vendor'
          }

          if (
            id.includes('/src/icons/lobeIconMap.ts') ||
            id.includes('/src/icons/fallbackLobeIcons.ts')
          ) {
            return 'icon-manifest'
          }

          if (
            id.includes('/src/utils/trayBudgetDisplay.ts') ||
            id.includes('/src/utils/trayCountdown.ts') ||
            id.includes('/src/utils/budgetUsage.ts')
          ) {
            return 'tray-shared'
          }
          if (id.includes('/src/services/')) {
            return 'app-services'
          }
          if (id.includes('/src/utils/')) {
            return 'app-utils'
          }
          if (id.includes('/src/data/')) {
            return 'app-data'
          }

          if (id.includes('/src/components/Logs/')) {
            return 'logs-page'
          }
          if (id.includes('/src/components/Console/')) {
            return 'console-page'
          }
          if (id.includes('/src/components/Availability/')) {
            return 'availability-page'
          }
          if (id.includes('/src/components/Main/')) {
            return 'main-page'
          }
          if (id.includes('/src/components/General/')) {
            return 'settings-page'
          }
          if (id.includes('/src/components/Tray/')) {
            return 'tray-page'
          }
          if (id.includes('/src/components/Mcp/')) {
            return 'mcp-page'
          }
          if (id.includes('/src/components/Skill/')) {
            return 'skill-page'
          }
          if (id.includes('/src/components/Prompts/')) {
            return 'prompts-page'
          }
          if (id.includes('/src/components/EnvCheck/')) {
            return 'env-page'
          }
          if (id.includes('/src/components/SpeedTest/')) {
            return 'speedtest-page'
          }

          return undefined
        },
      },
    },
  },
})

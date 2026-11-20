/**
 * @name: 应用路由
 * @Descripttion: 注册桌面应用和托盘页面的 Hash 路由。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-11-20 14:40:19
 * @LastEditTime: 2026-11-20 14:40:19
 * @FilePath: frontend/src/router/index.ts
 */
import { createRouter, createWebHashHistory } from 'vue-router'

const MainPage = () => import('../components/Main/Index.vue')
const LogsPage = () => import('../components/Logs/Index.vue')
const GeneralPage = () => import('../components/General/Index.vue')
const McpPage = () => import('../components/Mcp/index.vue')
const SkillPage = () => import('../components/Skill/Index.vue')
const AuthPage = () => import('../components/Auth/Index.vue')
const PromptsPage = () => import('../components/Prompts/Index.vue')
const OpenClawConfigPage = () => import('../components/OpenClaw/Index.vue')
const HermesMemoryPage = () => import('../components/Hermes/MemoryPanel.vue')
const SpeedTestPage = () => import('../components/SpeedTest/Index.vue')
const EnvCheckPage = () => import('../components/EnvCheck/Index.vue')
const ConsolePage = () => import('../components/Console/Index.vue')
const CLIUpdatesPage = () => import('../components/CLIUpdates/Index.vue')
const AvailabilityPage = () => import('../components/Availability/Index.vue')
const TrayPage = () => import('../components/Tray/Index.vue')

const routes = [
  { path: '/', component: MainPage },
  { path: '/prompts', component: PromptsPage },
  { path: '/mcp', component: McpPage },
  { path: '/skill', component: SkillPage },
  { path: '/openclaw-config', component: OpenClawConfigPage },
  { path: '/hermes-memory', component: HermesMemoryPage },
  { path: '/auth', component: AuthPage },
  { path: '/availability', component: AvailabilityPage },
  { path: '/speedtest', component: SpeedTestPage },
  { path: '/env', component: EnvCheckPage },
  { path: '/logs', component: LogsPage },
  { path: '/console', component: ConsolePage },
  { path: '/cli-updates', component: CLIUpdatesPage },
  { path: '/settings', component: GeneralPage },
  { path: '/tray', component: TrayPage },
]

export default createRouter({
  history: createWebHashHistory(), // Use createWebHashHistory for hash-based routing
  routes,
})

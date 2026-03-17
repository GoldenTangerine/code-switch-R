import { createRouter, createWebHashHistory } from 'vue-router'

const MainPage = () => import('../components/Main/Index.vue')
const LogsPage = () => import('../components/Logs/Index.vue')
const GeneralPage = () => import('../components/General/Index.vue')
const McpPage = () => import('../components/Mcp/index.vue')
const SkillPage = () => import('../components/Skill/Index.vue')
const PromptsPage = () => import('../components/Prompts/Index.vue')
const SpeedTestPage = () => import('../components/SpeedTest/Index.vue')
const EnvCheckPage = () => import('../components/EnvCheck/Index.vue')
const ConsolePage = () => import('../components/Console/Index.vue')
const AvailabilityPage = () => import('../components/Availability/Index.vue')
const TrayPage = () => import('../components/Tray/Index.vue')

const routes = [
  { path: '/', component: MainPage },
  { path: '/prompts', component: PromptsPage },
  { path: '/mcp', component: McpPage },
  { path: '/skill', component: SkillPage },
  { path: '/availability', component: AvailabilityPage },
  { path: '/speedtest', component: SpeedTestPage },
  { path: '/env', component: EnvCheckPage },
  { path: '/logs', component: LogsPage },
  { path: '/console', component: ConsolePage },
  { path: '/settings', component: GeneralPage },
  { path: '/tray', component: TrayPage },
]

export default createRouter({
  history: createWebHashHistory(), // Use createWebHashHistory for hash-based routing
  routes,
})

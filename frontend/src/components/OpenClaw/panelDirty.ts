/**
 * @name: OpenClaw 子面板未保存状态
 * @Descripttion: 集中记录 env/tools/agents 三个子面板的未保存标记，供 Index.vue 在切子 tab 或离开页面前统一拦截
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-17 00:00:00
 * @LastEditTime: 2026-08-17 00:00:00
 * @FilePath: frontend/src/components/OpenClaw/panelDirty.ts
 */
import { reactive } from 'vue'

export type OpenClawPanelId = 'env' | 'tools' | 'agents'

// 模块级共享状态：面板自身是 v-if 挂载/卸载的，dirty 标记必须存活于组件之外
export const openClawPanelDirty = reactive<Record<OpenClawPanelId, boolean>>({
  env: false,
  tools: false,
  agents: false,
})

export const hasOpenClawDirtyPanel = (): boolean => (
  openClawPanelDirty.env || openClawPanelDirty.tools || openClawPanelDirty.agents
)

// 用户确认放弃修改后整体清零，避免面板卸载期间残留标记导致重复弹窗
export const resetOpenClawPanelDirty = () => {
  openClawPanelDirty.env = false
  openClawPanelDirty.tools = false
  openClawPanelDirty.agents = false
}

import { ref } from 'vue'

// 中心面板：在应用主界面中间区域（而非独立弹窗）展示预览 / 复习内容。
// 由待办任务（侧边栏 / 任务表格）触发，App.vue 监听后在主区域叠加渲染。
export type CenterPanelType = 'preview' | 'review'

export interface CenterPanelState {
  type: CenterPanelType
  planId: number
  planName: string
}

export const centerPanel = ref<CenterPanelState | null>(null)

export function openCenterPanel(type: CenterPanelType, planId: number, planName: string) {
  centerPanel.value = { type, planId, planName }
}

export function closeCenterPanel() {
  centerPanel.value = null
}

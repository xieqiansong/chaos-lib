// 主题状态单例：dark（终端暗色，默认）/ paper（浅黄护眼）
import { ref } from 'vue'

export type ThemeName = 'dark' | 'paper'

const STORAGE_KEY = 'chaos-ui:theme'

const theme = ref<ThemeName>('dark')

function applyTheme(name: ThemeName) {
  theme.value = name
  const root = document.documentElement
  root.classList.remove('dark', 'paper')
  root.classList.add(name)
}

/** 启动时调用：读取 localStorage，非法/缺失回退 dark，并应用到 <html> */
export function initTheme() {
  const saved = localStorage.getItem(STORAGE_KEY)
  applyTheme(saved === 'paper' ? 'paper' : 'dark')
}

/** 切换主题并持久化 */
export function toggleTheme() {
  const next: ThemeName = theme.value === 'dark' ? 'paper' : 'dark'
  applyTheme(next)
  localStorage.setItem(STORAGE_KEY, next)
  return next
}

export { theme }

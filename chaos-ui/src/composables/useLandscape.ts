import { onMounted, onUnmounted, ref, watch } from 'vue'

/**
 * 在支持 Screen Orientation API 的设备（主要是 Android Chrome / WebView）上，
 * 通过「进入全屏 + 锁定横屏」把视口真正旋转 90°，从而让侧边栏 + 主区布局、
 * Monaco 编辑器、ECharts 等重组件都在真实横屏尺寸下全功能正常工作。
 *
 * 这是系统级旋转（而非 CSS transform 障眼法），因此交互坐标不会错位。
 * 限制：iOS Safari 不支持 orientation.lock，能力探测会返回 false，按钮不显示。
 */
export function useLandscape() {
  const locked = ref(false)
  const supported = ref(false)

  // 横屏锁定态同步到 <html>：手机横屏视口较窄，全局 CSS 依赖该类
  // 抬高根字号下限（见 style.css 的 html.fs-landscape），避免整页缩成微缩模型
  watch(locked, on => {
    document.documentElement.classList.toggle('fs-landscape', on)
  })

  function isIOS(): boolean {
    const ua = navigator.userAgent
    return (
      /iPad|iPhone|iPod/.test(ua) ||
      // iPadOS 13+ 伪装成 Mac，但带触摸点
      (navigator.platform === 'MacIntel' && navigator.maxTouchPoints > 1)
    )
  }

  function detect(): boolean {
    return (
      typeof screen !== 'undefined' &&
      !!screen.orientation &&
      typeof (screen.orientation as unknown as { lock?: unknown }).lock === 'function' &&
      !isIOS()
    )
  }

  async function enter() {
    if (!detect()) return
    try {
      const el = document.getElementById('app')
      if (el && !document.fullscreenElement) {
        await el.requestFullscreen()
      }
      await screen.orientation.lock('landscape')
      locked.value = true
    } catch (e) {
      console.warn('[chaos-ui] 横屏锁定失败（当前浏览器/上下文不支持）：', e)
      // 若已进入全屏但 lock 失败，退回退出全屏，避免卡在尴尬状态
      if (document.fullscreenElement) {
        await document.exitFullscreen().catch(() => {})
      }
    }
  }

  async function exit() {
    try {
      screen.orientation.unlock()
    } catch {
      /* 部分浏览器在未 lock 时 unlock 会抛错，忽略 */
    }
    if (document.fullscreenElement) {
      await document.exitFullscreen().catch(() => {})
    }
    locked.value = false
  }

  function toggle() {
    if (locked.value) exit()
    else enter()
  }

  // 用户按 ESC 或系统退出全屏时，同步状态并解除方向锁定
  function onFullscreenChange() {
    if (!document.fullscreenElement) {
      locked.value = false
      try {
        screen.orientation.unlock()
      } catch {
        /* noop */
      }
    }
  }

  onMounted(() => {
    supported.value = detect()
    document.addEventListener('fullscreenchange', onFullscreenChange)
  })

  onUnmounted(() => {
    document.removeEventListener('fullscreenchange', onFullscreenChange)
    document.documentElement.classList.remove('fs-landscape')
  })

  return { locked, supported, enter, exit, toggle }
}

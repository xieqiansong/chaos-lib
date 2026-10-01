import {
  ElMessage,
  ElMessageBox,
  ElNotification,
  type MessageOptions,
  type MessageBoxData,
  type NotificationOptions,
} from 'element-plus'

/**
 * 全局消息提示统一封装。
 *
 * 目标：避免各业务文件散落 `import { ElMessage, ElMessageBox } from 'element-plus'`，
 * 统一从这里导出语义化方法（showSuccess / showError / ...）与 Promise 风格的确认框。
 * 后续若要接入 i18n、去重、统一时长，只需改本文件。
 */

// ---------- 轻提示 Message ----------
export function showSuccess(message: string, options?: MessageOptions) {
  return ElMessage.success({ message, ...options })
}

export function showError(message: string, options?: MessageOptions) {
  return ElMessage.error({ message, ...options })
}

export function showWarning(message: string, options?: MessageOptions) {
  return ElMessage.warning({ message, ...options })
}

export function showInfo(message: string, options?: MessageOptions) {
  return ElMessage.info({ message, ...options })
}

// ---------- 确认框（Promise 风格） ----------
export interface ConfirmOptions {
  /** 弹窗标题，默认「提示」 */
  title?: string
  /** 图标类型，默认 warning */
  type?: 'success' | 'warning' | 'info' | 'error'
  confirmButtonText?: string
  cancelButtonText?: string
}

/**
 * 二次确认框，返回 Promise。
 * - 点击「确定」：resolve
 * - 点击「取消」/ 关闭：reject
 *
 * 调用处统一用 `try { await confirm(...) } catch { return }` 即可。
 */
export function confirm(message: string, options?: ConfirmOptions): Promise<MessageBoxData> {
  return ElMessageBox.confirm(message, options?.title ?? '提示', {
    type: options?.type ?? 'warning',
    confirmButtonText: options?.confirmButtonText ?? '确定',
    cancelButtonText: options?.cancelButtonText ?? '取消',
  })
}

/** 危险操作快捷确认（红色警告图标，确定按钮文案「确定」） */
export function confirmDanger(message: string, title = '提示'): Promise<MessageBoxData> {
  return confirm(message, { title, type: 'warning', confirmButtonText: '确定' })
}

// ---------- 通知 Notification ----------
export function notifySuccess(message: string, title = '成功', options?: NotificationOptions) {
  return ElNotification.success({ title, message, ...options })
}

export function notifyError(message: string, title = '错误', options?: NotificationOptions) {
  return ElNotification.error({ title, message, ...options })
}

export function notifyWarning(message: string, title = '警告', options?: NotificationOptions) {
  return ElNotification.warning({ title, message, ...options })
}

export function notifyInfo(message: string, title = '提示', options?: NotificationOptions) {
  return ElNotification.info({ title, message, ...options })
}

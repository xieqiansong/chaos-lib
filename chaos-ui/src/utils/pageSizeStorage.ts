// 每页条数（pageSize）的浏览器本地持久化：仅 localStorage，不依赖后端。
// 用途：让用户在各分页页面自行选择每页数量后，下次进入仍记住各自的选择。
// 失败时（隐私模式 / localStorage 被禁用）静默降级为「不持久化」，不影响翻页功能。

const PREFIX = 'chaos:pagesize:'

export function getPageSize(key: string, fallback: number): number {
  try {
    const v = localStorage.getItem(PREFIX + key)
    const n = v ? parseInt(v, 10) : NaN
    return Number.isFinite(n) && n > 0 ? n : fallback
  } catch {
    return fallback
  }
}

export function setPageSize(key: string, size: number): void {
  try {
    localStorage.setItem(PREFIX + key, String(size))
  } catch {
    /* 隐私模式/禁用：静默降级，不影响翻页 */
  }
}

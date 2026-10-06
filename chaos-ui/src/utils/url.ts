// URL 工具：安全提取 hostname、构造站点 favicon 地址（多个页面/组件共用）。
// 非法/空 URL 返回安全空值，避免抛错。

/** 安全提取 URL 的 hostname；无法解析时返回空串。 */
export function hostnameOf(url: string): string {
  try {
    return new URL(url).hostname
  } catch {
    return ''
  }
}

/** 安全提取 URL 的 host:port（小写、带端口），与后端 /api/favicon 缓存 key 对齐；无法解析时返回空串。 */
export function hostPortOf(url: string): string {
  try {
    const u = new URL(url)
    return u.port ? `${u.hostname}:${u.port}` : u.hostname
  } catch {
    return ''
  }
}

/** 构造站点 favicon 地址（由自身后端代理 + 缓存，见 /api/v1/proxy/favicon/:host，host 需含端口）；URL 非法时返回空串。 */
export function favicon(url: string): string {
  const host = hostPortOf(url)
  return host ? `/api/v1/proxy/favicon/${host}` : ''
}
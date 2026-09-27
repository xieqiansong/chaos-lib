// 二进制值（前端经 base64 传输）的相关工具：可读性判定 / 解码 / 摘要 / MIME / 下载。
// 数据缓存等以 BLOB/BYTEA 存原始字节、JSON 传 base64 的模块共用，避免判定逻辑重复。
// 「可读文本」用严格 UTF-8 解码 + 可读性校验判定：非法序列或含空字节/控制字符即视为二进制。

/** 尝试把 base64 严格解码为可读 UTF-8 文本；不可读（二进制）或为空返回 null。 */
export function decodeUtf8(value: string): string | null {
  if (!value) return ''
  try {
    const bin = atob(value)
    const bytes = Uint8Array.from(bin, (c) => c.charCodeAt(0))
    const text = new TextDecoder('utf-8', {fatal: true}).decode(bytes)
    for (let i = 0; i < text.length; i++) {
      const code = text.charCodeAt(i)
      if (code === 0) return null // 空字节
      if (code < 32 && code !== 9 && code !== 10 && code !== 13) return null // 控制字符（除 \t\n\r）
    }
    return text
  } catch {
    return null
  }
}

/** 是否为可读文本。 */
export function isLikelyText(value: string): boolean {
  return decodeUtf8(value) !== null
}

/** 展示摘要：可读文本原样返回；二进制返回「[二进制 · N 字节]」中性标签。 */
export function binarySummary(value: string): string {
  const text = decodeUtf8(value)
  if (text !== null) return text
  return `[二进制 · ${base64ByteLen(value)} 字节]`
}

/** 计算 base64 对应的字节数（剔除 padding）。 */
function base64ByteLen(base64: string): number {
  if (!base64) return 0
  const pad = base64.endsWith('==') ? 2 : base64.endsWith('=') ? 1 : 0
  return Math.floor((base64.length * 3) / 4) - pad
}

// 元素类型字典 → MIME；含 / 的类型视为自身即 MIME。
const EXT_MIME: Record<string, string> = {
  text: 'text/plain',
  json: 'application/json',
  xml: 'application/xml',
  yaml: 'text/yaml',
  png: 'image/png',
  jpg: 'image/jpeg',
  jpeg: 'image/jpeg',
  gif: 'image/gif',
  webp: 'image/webp',
  ico: 'image/x-icon',
  bmp: 'image/bmp',
  svg: 'image/svg+xml',
  avif: 'image/avif',
  mp4: 'video/mp4',
  mp3: 'audio/mpeg',
}

/** 由 dataType 推断 MIME；非文本类型默认 octet-stream。 */
export function mimeOf(dataType?: string): string {
  if (!dataType) return 'application/octet-stream'
  const t = dataType.toLowerCase()
  if (t.includes('/')) return t
  return EXT_MIME[t] ?? 'application/octet-stream'
}

/** 该 dataType 是否可当作图片预览（用于 <img>/<el-image>）。 */
export function isImageType(dataType?: string): boolean {
  if (!dataType) return false
  const t = dataType.toLowerCase()
  if (t.startsWith('image/')) return true
  return ['png', 'jpg', 'jpeg', 'gif', 'webp', 'bmp', 'ico', 'svg', 'avif'].includes(t)
}

/** 构造 data URL，供预览 <img> 使用。 */
export function dataUrl(value: string, mime?: string): string {
  return `data:${mime ?? 'application/octet-stream'};base64,${value}`
}

/** 把 base64 解码为二进制并作为文件下载。 */
export function downloadBase64(value: string, filename: string, mime?: string): void {
  if (!value) return
  const bin = atob(value)
  const bytes = Uint8Array.from(bin, (c) => c.charCodeAt(0))
  const blob = new Blob([bytes], {type: mime ?? 'application/octet-stream'})
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  a.click()
  // 延迟回收 URL，避免部分浏览器在下载尚未触发时报错
  setTimeout(() => URL.revokeObjectURL(url), 1000)
}
/**
 * polyform-tools 前端封装
 *
 * - 统一注册五个格式插件（json / yaml / csv / xml / cbor）
 * - 屏蔽插件注册表的全局性：每次转换前按当前选项重新注册
 * - 处理二进制格式（CBOR）与 HEX 文本的互转，便于在文本框中编辑
 */
import {convert, use} from 'polyform-tools'
import {json} from 'polyform-tools/formats/json'
import {yaml} from 'polyform-tools/formats/yaml'
import {csv} from 'polyform-tools/formats/csv'
import {xml} from 'polyform-tools/formats/xml'
import {cbor} from 'polyform-tools/formats/cbor'

export type FormatId = 'json' | 'yaml' | 'csv' | 'xml' | 'cbor'

export interface FormatMeta {
  id: FormatId
  label: string
  ext: string
  /** 二进制格式：界面以 HEX 文本承载 */
  binary: boolean
  hint: string
}

export const FORMAT_META: Record<FormatId, FormatMeta> = {
  json: {id: 'json', label: 'JSON', ext: 'json', binary: false, hint: '对象 / 数组'},
  yaml: {id: 'yaml', label: 'YAML', ext: 'yaml', binary: false, hint: '缩进表示层级'},
  csv: {id: 'csv', label: 'CSV', ext: 'csv', binary: false, hint: '对象数组 ↔ 二维表'},
  xml: {id: 'xml', label: 'XML', ext: 'xml', binary: false, hint: '对象根最佳；数组根会生成 <0>/<1> 数字标签'},
  cbor: {id: 'cbor', label: 'CBOR', ext: 'cbor', binary: true, hint: '二进制，界面用 HEX 承载'},
}

export const FORMAT_LIST: FormatMeta[] = [
  FORMAT_META.json,
  FORMAT_META.yaml,
  FORMAT_META.csv,
  FORMAT_META.xml,
  FORMAT_META.cbor,
]

export interface JsonOptions {
  indent: number
}

export interface YamlOptions {
  version: '1.1' | '1.2'
  indent: number
  lineWidth: number
}

export interface CsvOptions {
  delimiter: string
  hasHeaders: boolean
  quote: string
  escape: string
  recordDelimiter: string
  escapeFormulas: boolean
}

export interface XmlOptions {
  attributePrefix: string
  format: boolean
  ignoreDeclaration: boolean
  ignoreAttributes: boolean
}

export interface PolyformOptions {
  json: JsonOptions
  yaml: YamlOptions
  csv: CsvOptions
  xml: XmlOptions
}

export function defaultOptions(): PolyformOptions {
  return {
    json: {indent: 2},
    yaml: {version: '1.2', indent: 2, lineWidth: 0},
    csv: {delimiter: ',', hasHeaders: true, quote: '"', escape: '"', recordDelimiter: '\n', escapeFormulas: true},
    xml: {attributePrefix: '@_', format: true, ignoreDeclaration: false, ignoreAttributes: false},
  }
}

/** 按当前选项重新注册全部插件（注册表是全局的，重复注册即覆盖） */
function registerPlugins(options: PolyformOptions) {
  use(json({indent: options.json.indent}))
  use(yaml({
    version: options.yaml.version,
    indent: options.yaml.indent,
    lineWidth: options.yaml.lineWidth,
  }))
  use(csv({
    delimiter: options.csv.delimiter || ',',
    hasHeaders: options.csv.hasHeaders,
    quote: options.csv.quote || '"',
    escape: options.csv.escape || '"',
    recordDelimiter: options.csv.recordDelimiter || '\n',
    escapeFormulas: options.csv.escapeFormulas,
  }))
  use(xml({
    attributePrefix: options.xml.attributePrefix || '@_',
    format: options.xml.format,
    ignoreDeclaration: options.xml.ignoreDeclaration,
    ignoreAttributes: options.xml.ignoreAttributes,
  }))
  use(cbor())
}

export interface ConvertResult {
  /** 展示用文本：二进制目标为 HEX 转储 */
  text: string
  /** 二进制目标的原始字节，供下载使用 */
  bytes: Uint8Array | null
  /** 耗时（毫秒） */
  duration: number
}

/**
 * 执行一次转换。失败时抛出 Error（message 已由 polyform 包装，含阶段信息）。
 */
export function runConvert(
    input: string | Uint8Array,
    from: FormatId,
    to: FormatId,
    options: PolyformOptions,
): ConvertResult {
  registerPlugins(options)
  const start = performance.now()
  const output = convert(input).from(from).to<string | Uint8Array>(to)
  const duration = performance.now() - start
  if (output instanceof Uint8Array) {
    return {text: bytesToHex(output), bytes: output, duration}
  }
  return {text: String(output ?? ''), bytes: null, duration}
}

/** 用同格式往返一次，实现“格式化 / 美化” */
export function reformat(input: string, format: FormatId, options: PolyformOptions): ConvertResult {
  const pretty: PolyformOptions = {
    ...options,
    json: {indent: options.json.indent},
    xml: {...options.xml, format: true},
  }
  return runConvert(input, format, format, pretty)
}

/* ===== HEX / 字节工具 ===== */

/** 宽松解析 HEX：允许空格、换行、逗号、冒号、0x 前缀 */
export function hexToBytes(hex: string): Uint8Array {
  const clean = hex.replace(/(0x|0X)/g, '').replace(/[\s,:;\-_]/g, '')
  if (!clean) return new Uint8Array(0)
  if (clean.length % 2 !== 0) throw new Error('HEX 长度必须是偶数（每 2 个字符 1 字节）')
  if (!/^[0-9a-fA-F]+$/.test(clean)) throw new Error('HEX 含非法字符，仅允许 0-9 / a-f')
  const bytes = new Uint8Array(clean.length / 2)
  for (let i = 0; i < bytes.length; i++) {
    bytes[i] = parseInt(clean.substr(i * 2, 2), 16)
  }
  return bytes
}

/** 字节转 HEX 转储：每字节空格分隔，每行 16 字节 */
export function bytesToHex(bytes: Uint8Array, perLine = 16): string {
  if (bytes.length === 0) return ''
  const lines: string[] = []
  for (let i = 0; i < bytes.length; i += perLine) {
    const slice = bytes.subarray(i, Math.min(i + perLine, bytes.length))
    lines.push(Array.from(slice, b => b.toString(16).padStart(2, '0')).join(' '))
  }
  return lines.join('\n')
}

/** 紧凑 HEX（无分隔，便于复制） */
export function bytesToHexCompact(bytes: Uint8Array): string {
  return Array.from(bytes, b => b.toString(16).padStart(2, '0')).join('')
}

export function utf8Length(text: string): number {
  return new TextEncoder().encode(text).length
}

/* ===== 示例数据 ===== */

const SAMPLE_JSON = `{
  "name": "chaos",
  "version": "1.0.0",
  "enabled": true,
  "items": [
    {"id": 1, "name": "JSON", "score": 9.5},
    {"id": 2, "name": "YAML", "score": 8.5},
    {"id": 3, "name": "CSV", "score": 7.5}
  ]
}`

const SAMPLE_YAML = `name: chaos
version: 1.0.0
enabled: true
items:
  - id: 1
    name: JSON
    score: 9.5
  - id: 2
    name: YAML
    score: 8.5
  - id: 3
    name: CSV
    score: 7.5
`

const SAMPLE_CSV = `id,name,score
1,JSON,9.5
2,YAML,8.5
3,CSV,7.5
`

const SAMPLE_XML = `<root>
  <name>chaos</name>
  <version>1.0.0</version>
  <enabled>true</enabled>
  <items>
    <id>1</id>
    <name>JSON</name>
    <score>9.5</score>
  </items>
  <items>
    <id>2</id>
    <name>YAML</name>
    <score>8.5</score>
  </items>
</root>
`

export const SAMPLE: Record<FormatId, string> = {
  json: SAMPLE_JSON,
  yaml: SAMPLE_YAML,
  csv: SAMPLE_CSV,
  xml: SAMPLE_XML,
  cbor: '',
}

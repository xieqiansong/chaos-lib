/**
 * 自定义 polyform 插件范例（INI）
 *
 * 背景：polyform-tools 只对外导出 convert / use，AST 桥接函数（fromNative / toNative）
 * 并未导出，因此自定义插件要自己完成「原生 JS 值 ↔ AST」的双向转换。
 * 本文件实现：
 *   1. AST 类型与桥接工具（对齐官方 dist/core 的内部约定）
 *   2. 一个零依赖的 ini 插件，可直接 use(ini()) 注册
 *
 * 再新增一种格式的最小步骤：
 *   const myFormat = (options) => ({
 *     name: 'myformat',                       // convert().from('myformat') 用的名字
 *     parse: (input) => toAst(yourParse(input)),        // string | Uint8Array -> AST
 *     serialize: (ast) => yourStringify(fromAst(ast)),  // AST -> string | Uint8Array
 *   })
 * 只提供 parse 即「只读格式」，只提供 serialize 即「只写格式」。
 */

/* ===== AST 类型（官方 types.d.ts 的本地副本，因包未导出） ===== */

export type Scalar = string | number | boolean | null | bigint | Uint8Array

export interface LiteralNode {
  type: 'Literal'
  start: number
  end: number
  value: Scalar
  raw: string
}

export interface PropertyNode {
  type: 'Property'
  start: number
  end: number
  key: LiteralNode
  value: ObjectNode | ArrayNode | LiteralNode
}

export interface ObjectNode {
  type: 'Object'
  start: number
  end: number
  properties: PropertyNode[]
}

export interface ArrayNode {
  type: 'Array'
  start: number
  end: number
  elements: (ObjectNode | ArrayNode | LiteralNode)[]
}

export interface RootNode {
  type: 'Root'
  start: number
  end: number
  body: ObjectNode | ArrayNode | LiteralNode
}

export type ASTNode = RootNode | ObjectNode | ArrayNode | PropertyNode | LiteralNode

export interface PolyformPlugin {
  name: string
  parse?: (input: string | Uint8Array, options?: unknown) => ASTNode
  serialize?: (ast: ASTNode, options?: unknown) => string | Uint8Array
}

/** 程序生成的节点没有位置信息，官方约定用 -1 */
const NO_POS = -1

function literal(value: Scalar): LiteralNode {
  return {type: 'Literal', start: NO_POS, end: NO_POS, value, raw: String(value)}
}

function keyLiteral(key: string): LiteralNode {
  return {type: 'Literal', start: NO_POS, end: NO_POS, value: key, raw: `"${key}"`}
}

/** 原生值 -> AST（等价官方 fromNative） */
export function toAst(value: unknown): ObjectNode | ArrayNode | LiteralNode {
  if (value === null || value instanceof Uint8Array || typeof value !== 'object') {
    return literal(value as Scalar)
  }
  if (Array.isArray(value)) {
    return {type: 'Array', start: NO_POS, end: NO_POS, elements: value.map(toAst)}
  }
  const entries: [string, unknown][] = value instanceof Map
      ? Array.from(value.entries())
      : Object.entries(value as object)
  return {
    type: 'Object',
    start: NO_POS,
    end: NO_POS,
    properties: entries.map(([key, val]) => ({
      type: 'Property',
      start: NO_POS,
      end: NO_POS,
      key: keyLiteral(key),
      value: toAst(val),
    })),
  }
}

/** AST -> 原生值（等价官方 toNative，额外兼容 Root 包装） */
export function fromAst(node: ASTNode): unknown {
  switch (node.type) {
    case 'Root':
      return fromAst(node.body)
    case 'Literal':
      return node.value
    case 'Array':
      return node.elements.map(fromAst)
    case 'Object': {
      const obj: Record<string, unknown> = {}
      for (const prop of node.properties) obj[String(prop.key.value)] = fromAst(prop.value)
      return obj
    }
    case 'Property':
      return [String(node.key.value), fromAst(node.value)]
    default:
      throw new Error(`Unsupported AST node type: ${(node as ASTNode).type}`)
  }
}

/* ===== INI 插件 ===== */

export interface IniOptions {
  /** 数组元素分隔符（序列化时拼接、解析时切分） */
  delimiter: string
  /** 数组写法：join = 一行拼齐，repeat = 重复键 */
  arrayMode: 'join' | 'repeat'
  /** 解析时推断数字 / 布尔 / 数组，关闭则全部保留字符串 */
  inferTypes: boolean
  /** section 前是否补一个空行 */
  blankLineBeforeSection: boolean
}

export function defaultIniOptions(): IniOptions {
  return {delimiter: ',', arrayMode: 'join', inferTypes: true, blankLineBeforeSection: true}
}

const NEED_QUOTE = /[\r\n"';#]/

/** 尝试解析内联 JSON；不是 JSON 返回 undefined */
function tryJson(text: string): unknown | undefined {
  if (!((text.startsWith('{') && text.endsWith('}')) || (text.startsWith('[') && text.endsWith(']')))) return undefined
  try {
    return JSON.parse(text)
  } catch {
    return undefined
  }
}

/** 值解析：内联 JSON > 分隔符数组 > 标量（顺序不能反，否则 JSON 会被切碎） */
function parseValue(raw: string, options: IniOptions): unknown {
  if (!options.inferTypes) return raw
  const inline = tryJson(raw)
  if (inline !== undefined) return inline
  if (options.delimiter && raw.includes(options.delimiter)) {
    return raw.split(options.delimiter).map(part => scalarOf(part.trim()))
  }
  return scalarOf(raw)
}

function scalarOf(text: string): unknown {
  const inline = tryJson(text)
  if (inline !== undefined) return inline
  if (text === 'true') return true
  if (text === 'false') return false
  if (text === 'null') return null
  if (/^-?\d+(\.\d+)?$/.test(text)) return Number(text)
  if (text.length >= 2) {
    const head = text[0]
    if ((head === '"' || head === "'") && text[text.length - 1] === head) return text.slice(1, -1)
  }
  return text
}

function stringify(value: unknown): string {
  if (value === null || value === undefined) return 'null'
  if (typeof value === 'object') return JSON.stringify(value)
  const text = String(value)
  const sep = iniDelimiter
  return text.includes(sep) || NEED_QUOTE.test(text)
      ? `"${text.replace(/"/g, '\\"')}"`
      : text
}

// stringify 需要感知分隔符，用模块级变量承接当前插件配置（插件工厂为闭包，序列化时同步）
let iniDelimiter = ','

function isSection(value: unknown): boolean {
  return value !== null && typeof value === 'object' && !Array.isArray(value)
}

export function ini(pluginOptions: Partial<IniOptions> = {}): PolyformPlugin {
  const options: IniOptions = {...defaultIniOptions(), ...pluginOptions}

  return {
    name: 'ini',

    parse: (input) => {
      const text = input instanceof Uint8Array ? new TextDecoder().decode(input) : input
      const root: Record<string, unknown> = {}
      let section: Record<string, unknown> = root

      for (const rawLine of text.split(/\r?\n/)) {
        const line = rawLine.trim()
        if (!line || line.startsWith(';') || line.startsWith('#')) continue

        if (line.startsWith('[') && line.endsWith(']')) {
          const name = line.slice(1, -1).trim()
          const existing = root[name]
          section = isSection(existing)
              ? existing as Record<string, unknown>
              : (root[name] = {}) as Record<string, unknown>
          continue
        }

        const sep = line.search(/[=:]/)
        if (sep < 0) continue
        const key = line.slice(0, sep).trim()
        const rawValue = line.slice(sep + 1).trim()

        const value = parseValue(rawValue, options)

        const prev = section[key]
        if (prev === undefined) section[key] = value
        else if (Array.isArray(prev)) prev.push(value)
        else section[key] = [prev, value]
      }

      return toAst(root)
    },

    serialize: (ast) => {
      iniDelimiter = options.delimiter
      const value = fromAst(ast)
      const lines: string[] = []

      const writeEntry = (key: string, val: unknown) => {
        if (Array.isArray(val)) {
          // 元素含对象 / 数组时只能用重复键：JSON 内联后含分隔符，拼接会破坏结构
          const hasContainer = val.some(item => item !== null && typeof item === 'object')
          if (options.arrayMode === 'repeat' || hasContainer) {
            for (const item of val) lines.push(`${key} = ${stringify(item)}`)
          } else {
            lines.push(`${key} = ${val.map(stringify).join(options.delimiter)}`)
          }
          return
        }
        lines.push(`${key} = ${stringify(val)}`)
      }

      if (isSection(value)) {
        const obj = value as Record<string, unknown>
        // 先写标量 / 数组，再写 section，保证无 section 的键聚在文件头部
        for (const [key, val] of Object.entries(obj)) {
          if (!isSection(val)) writeEntry(key, val)
        }
        for (const [key, val] of Object.entries(obj)) {
          if (!isSection(val)) continue
          if (lines.length > 0 && options.blankLineBeforeSection) lines.push('')
          lines.push(`[${key}]`)
          for (const [subKey, subVal] of Object.entries(val as Record<string, unknown>)) {
            writeEntry(subKey, subVal)
          }
        }
      } else if (Array.isArray(value)) {
        // 顶层数组：一行一项（对象用 JSON 表示，保证可往返）
        for (const item of value) lines.push(stringify(item))
      } else {
        lines.push(`value = ${stringify(value)}`)
      }

      return lines.join('\n') + '\n'
    },
  }
}

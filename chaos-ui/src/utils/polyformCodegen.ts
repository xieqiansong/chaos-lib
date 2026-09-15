/**
 * 代码生成型插件：JSON -> Java class / Go struct
 *
 * 与 INI 不同，这两个是「只写格式」（仅提供 serialize，不提供 parse）：
 * 源码 -> AST 的逆向解析没有稳定语义，因此它们只能作为转换目标。
 *
 * 实现思路：
 *   1. fromAst(ast) 取回原生值
 *   2. inferSchema(value) 推导结构（合并数组内多样本的类型）
 *   3. 按目标语言渲染
 */
import {fromAst, type PolyformPlugin} from '@/utils/polyformCustom'

/* ===== 结构推导 ===== */

type SchemaKind = 'string' | 'integer' | 'number' | 'boolean' | 'unknown' | 'object' | 'array'

interface SchemaField {
  name: string
  type: Schema
  /** 数组合并时并非所有样本都存在该字段 */
  optional: boolean
}

interface Schema {
  kind: SchemaKind
  fields?: SchemaField[]
  element?: Schema
}

const UNKNOWN: Schema = {kind: 'unknown'}

function infer(value: unknown): Schema {
  if (value === null || value === undefined) return UNKNOWN
  if (Array.isArray(value)) {
    if (value.length === 0) return {kind: 'array', element: UNKNOWN}
    return {kind: 'array', element: value.map(infer).reduce(merge)}
  }
  if (typeof value === 'object') {
    return {
      kind: 'object',
      fields: Object.entries(value as Record<string, unknown>)
          .map(([name, val]) => ({name, type: infer(val), optional: false})),
    }
  }
  if (typeof value === 'boolean') return {kind: 'boolean'}
  if (typeof value === 'number') return {kind: Number.isInteger(value) ? 'integer' : 'number'}
  return {kind: 'string'}
}

/** 合并两个样本的类型（数组元素、对象字段） */
function merge(a: Schema, b: Schema): Schema {
  if (a.kind === 'unknown') return b
  if (b.kind === 'unknown') return a
  if (a.kind === b.kind) {
    if (a.kind === 'object') return {kind: 'object', fields: mergeFields(a.fields ?? [], b.fields ?? [])}
    if (a.kind === 'array') return {kind: 'array', element: merge(a.element ?? UNKNOWN, b.element ?? UNKNOWN)}
    return a
  }
  // int + double 提升为 double
  if ((a.kind === 'integer' && b.kind === 'number') || (a.kind === 'number' && b.kind === 'integer')) {
    return {kind: 'number'}
  }
  return UNKNOWN
}

function mergeFields(a: SchemaField[], b: SchemaField[]): SchemaField[] {
  const result: SchemaField[] = []
  const index = new Map<string, number>()
  const push = (field: SchemaField) => {
    index.set(field.name, result.length)
    result.push({...field})
  }
  for (const field of a) push(field)
  const seenInB = new Set<string>()
  for (const field of b) {
    seenInB.add(field.name)
    const i = index.get(field.name)
    if (i === undefined) {
      push({...field, optional: true})
    } else {
      result[i] = {
        name: field.name,
        type: merge(result[i].type, field.type),
        optional: result[i].optional || field.optional,
      }
    }
  }
  for (let i = 0; i < result.length; i++) {
    if (!seenInB.has(result[i].name)) result[i].optional = true
  }
  return result
}

/* ===== 命名工具 ===== */

function words(name: string): string[] {
  return name
      .replace(/([a-z0-9])([A-Z])/g, '$1 $2')
      .split(/[^a-zA-Z0-9]+/)
      .filter(Boolean)
}

/** Go 惯例的缩写大写化 */
const ACRONYMS: Record<string, string> = {
  id: 'ID', url: 'URL', uri: 'URI', api: 'API', http: 'HTTP', https: 'HTTPS',
  json: 'JSON', xml: 'XML', sql: 'SQL', ip: 'IP', uuid: 'UUID', db: 'DB',
}

function pascal(name: string, acronyms = false): string {
  const list = words(name)
  if (list.length === 0) return 'Field'
  return list.map(w => {
    if (acronyms && ACRONYMS[w.toLowerCase()]) return ACRONYMS[w.toLowerCase()]
    return w[0].toUpperCase() + w.slice(1)
  }).join('')
}

function camel(name: string): string {
  const p = pascal(name)
  return p[0].toLowerCase() + p.slice(1)
}

/** 数组的类名取单数：items -> item */
function singular(name: string): string {
  if (/ies$/i.test(name)) return name.slice(0, -3) + 'y'
  if (/(ss|us|is|as)$/i.test(name)) return name
  if (/s$/i.test(name)) return name.slice(0, -1)
  return name
}

function uniqueName(base: string, used: Set<string>): string {
  let name = base
  let i = 2
  while (used.has(name)) name = `${base}${i++}`
  used.add(name)
  return name
}

/* ===== Java ===== */

export interface JavaOptions {
  /** 外层类名 */
  className: string
  /** 包名，留空则不输出 package 声明 */
  packageName: string
  /** 生成 lombok @Data 注解 */
  lombok: boolean
  /** 字段名转小驼峰 */
  camelCase: boolean
}

export function defaultJavaOptions(): JavaOptions {
  return {className: 'Root', packageName: '', lombok: true, camelCase: true}
}

function javaScalar(schema: Schema): string {
  switch (schema.kind) {
    case 'string':
      return 'String'
    case 'integer':
      return 'int'
    case 'number':
      return 'double'
    case 'boolean':
      return 'boolean'
    case 'array':
      return `List<${javaScalar(schema.element ?? UNKNOWN)}>`
    default:
      return 'Object'
  }
}

function hasArray(schema: Schema): boolean {
  if (schema.kind === 'array') return true
  if (schema.kind === 'object') return (schema.fields ?? []).some(f => hasArray(f.type))
  return false
}

function renderJavaClass(
    schema: Schema,
    className: string,
    depth: number,
    out: string[],
    options: JavaOptions,
    used: Set<string>,
) {
  const pad = '    '.repeat(depth)
  if (options.lombok) out.push(`${pad}@Data`)
  out.push(`${pad}public ${depth > 0 ? 'static ' : ''}class ${className} {`)

  const nested: Array<{name: string; schema: Schema}> = []
  for (const field of schema.fields ?? []) {
    const fieldName = options.camelCase ? camel(field.name) : field.name
    let typeName: string
    if (field.type.kind === 'object') {
      typeName = uniqueName(pascal(field.name), used)
      nested.push({name: typeName, schema: field.type})
    } else if (field.type.kind === 'array') {
      const element = field.type.element ?? UNKNOWN
      if (element.kind === 'object') {
        const inner = uniqueName(pascal(singular(field.name)), used)
        typeName = `List<${inner}>`
        nested.push({name: inner, schema: element})
      } else {
        typeName = `List<${javaScalar(element)}>`
      }
    } else {
      typeName = javaScalar(field.type)
    }
    out.push(`${pad}    private ${typeName} ${fieldName};`)
  }

  for (const item of nested) {
    out.push('')
    renderJavaClass(item.schema, item.name, depth + 1, out, options, used)
  }
  out.push(`${pad}}`)
}

export function java(pluginOptions: Partial<JavaOptions> = {}): PolyformPlugin {
  const options: JavaOptions = {...defaultJavaOptions(), ...pluginOptions}

  return {
    name: 'java',
    serialize: (ast) => {
      const value = fromAst(ast)
      let root = infer(value)
      // 非对象根包一层，保证一定能生成类
      if (root.kind !== 'object') {
        root = {
          kind: 'object',
          fields: [{name: root.kind === 'array' ? 'items' : 'value', type: root, optional: false}],
        }
      }

      const out: string[] = []
      if (options.packageName.trim()) out.push(`package ${options.packageName.trim()};`, '')
      if (options.lombok) out.push('import lombok.Data;')
      if (hasArray(root)) out.push('import java.util.List;')
      if (out.length > 0) out.push('')

      const used = new Set<string>()
      renderJavaClass(root, uniqueName(pascal(options.className) || 'Root', used), 0, out, options, used)
      return out.join('\n') + '\n'
    },
  }
}

/* ===== Go ===== */

export interface GoOptions {
  /** 根结构体名 */
  structName: string
  /** 包名 */
  packageName: string
  /** 生成 json tag */
  jsonTag: boolean
  /** json tag 追加 omitempty（仅对可选字段） */
  omitempty: boolean
}

export function defaultGoOptions(): GoOptions {
  return {structName: 'Root', packageName: 'main', jsonTag: true, omitempty: false}
}

function goScalar(schema: Schema): string {
  switch (schema.kind) {
    case 'string':
      return 'string'
    case 'integer':
      return 'int'
    case 'number':
      return 'float64'
    case 'boolean':
      return 'bool'
    case 'array':
      return `[]${goScalar(schema.element ?? UNKNOWN)}`
    default:
      return 'interface{}'
  }
}

interface GoStruct {
  name: string
  fields: Array<{name: string; type: string; json: string; optional: boolean}>
}

function collectGoStructs(schema: Schema, name: string, out: GoStruct[], used: Set<string>) {
  const struct: GoStruct = {name, fields: []}
  out.push(struct)
  for (const field of schema.fields ?? []) {
    let typeName: string
    if (field.type.kind === 'object') {
      typeName = uniqueName(pascal(field.name, true), used)
      collectGoStructs(field.type, typeName, out, used)
    } else if (field.type.kind === 'array') {
      const element = field.type.element ?? UNKNOWN
      if (element.kind === 'object') {
        const inner = uniqueName(pascal(singular(field.name), true), used)
        typeName = `[]${inner}`
        collectGoStructs(element, inner, out, used)
      } else {
        typeName = `[]${goScalar(element)}`
      }
    } else {
      typeName = goScalar(field.type)
    }
    struct.fields.push({name: pascal(field.name, true), type: typeName, json: field.name, optional: field.optional})
  }
}

function renderGoStruct(struct: GoStruct, options: GoOptions): string[] {
  const lines = [`type ${struct.name} struct {`]
  const nameWidth = Math.max(0, ...struct.fields.map(f => f.name.length))
  const typeWidth = Math.max(0, ...struct.fields.map(f => f.type.length))
  for (const field of struct.fields) {
    const tag = options.jsonTag
        ? ` \`json:"${field.json}${options.omitempty && field.optional ? ',omitempty' : ''}"\``
        : ''
    lines.push(`    ${field.name.padEnd(nameWidth)} ${field.type.padEnd(typeWidth)}${tag}`.trimEnd())
  }
  lines.push('}')
  return lines
}

export function go(pluginOptions: Partial<GoOptions> = {}): PolyformPlugin {
  const options: GoOptions = {...defaultGoOptions(), ...pluginOptions}

  return {
    name: 'go',
    serialize: (ast) => {
      const root = infer(fromAst(ast))
      const used = new Set<string>()
      const structs: GoStruct[] = []
      let alias = ''

      const rootName = pascal(options.structName) || 'Root'

      if (root.kind === 'array') {
        const element = root.element ?? UNKNOWN
        if (element.kind === 'object') {
          // 先占住根名，避免元素类型与别名 type X []X 重名
          used.add(rootName)
          const singularName = pascal(singular(options.structName), true)
          const inner = uniqueName(singularName === rootName ? `${rootName}Item` : singularName, used)
          collectGoStructs(element, inner, structs, used)
          alias = `type ${rootName} []${inner}`
        } else {
          alias = `type ${rootName} []${goScalar(element)}`
        }
      } else if (root.kind !== 'object') {
        alias = `type ${rootName} ${goScalar(root)}`
      } else {
        collectGoStructs(root, uniqueName(rootName, used), structs, used)
      }

      const out = [`package ${options.packageName.trim() || 'main'}`, '']
      for (const struct of structs) {
        out.push(...renderGoStruct(struct, options), '')
      }
      if (alias) out.push(alias, '')
      return out.join('\n').replace(/\n+$/, '\n')
    },
  }
}

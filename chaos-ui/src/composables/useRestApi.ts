// 通用 REST 数据接口：把一个「标准 CRUD 资源」封装成 DataTable / 表单可直接消费的对象。
//
// 与后端 internal/crud 严格对应：GET 列表（page/size/sort/order/<搜索字段>）、
// POST 创建、PATCH 部分更新、DELETE 软删。
// 状态切换等扩展接口不属于标准基线，由各业务 api 文件（如 api/standardData.ts）自行追加。
// 视图层只需 `const api = useRestApi<Xxx>('xxx')`，无需再写 fetchApi 样板。
import {sendMessage} from '@/utils/api'
import type {DataTableApiParams} from '@/components/dataTable/types'

// 驼峰字段名 → snake_case 列名，用于把前端列字段名翻译成后端查询参数。
function toSnake(s: string): string {
  return s.replace(/([a-z0-9])([A-Z])/g, '$1_$2').toLowerCase()
}

export interface RestApi<T extends { ID: number }> {
  /** DataTable 的 api 函数：翻译分页/搜索/排序参数并调用 GET */
  fetch: (params: DataTableApiParams) => Promise<{ rows: T[]; total: number }>
  create: (data: Partial<T>) => Promise<any>
  update: (id: number, data: Partial<T>) => Promise<any>
  remove: (id: number) => Promise<any>
}

export function useRestApi<T extends { ID: number }>(prefix: string): RestApi<T> {
  async function fetch(params: DataTableApiParams) {
    const query: Record<string, any> = {
      page: params.page,
      size: params.pageSize,
    }
    if (params.sort?.field) {
      query.sort = toSnake(params.sort.field)
      query.order = params.sort.order === 'ascending' ? 'asc' : 'desc'
    }
    for (const [k, v] of Object.entries(params.search || {})) {
      if (v !== '' && v !== null && v !== undefined) {
        query[toSnake(k)] = v
      }
    }
    const res = await sendMessage(prefix, 'GET', query)
    // 兼容不同后端构建的响应形态：分页包可能是 {items}（crud 基线）或 {rows}（旧接口），
    // 主键字段可能是 ID 或 Id；统一规整为 {rows, total} 且每行必含 ID。
    const raw = (res?.items ?? res?.rows ?? res?.data?.items ?? res?.data?.rows ?? []) as any[]
    const rows = raw.map((it: any) => ({...it, ID: it.ID ?? it.Id})) as T[]
    const total = res?.total ?? res?.data?.total ?? rows.length
    return {rows, total}
  }

  function create(data: Partial<T>) {
    return sendMessage(prefix, 'POST', data)
  }

  function update(id: number, data: Partial<T>) {
    return sendMessage(`${prefix}/${id}`, 'PATCH', data)
  }

  function remove(id: number) {
    return sendMessage(`${prefix}/${id}`, 'DELETE')
  }

  return {fetch, create, update, remove}
}

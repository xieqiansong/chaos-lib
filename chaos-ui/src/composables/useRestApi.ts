// 通用 REST 数据接口：把一个「标准 CRUD 资源」封装成 DataTable / 表单可直接消费的对象。
//
// 接口重构（见仓库《接口规范.md》）：底层统一走 action()（POST + Action，/api/v1/{prefix}/{action}），
// 与后端 internal/crud 的 RegisterActions 严格对应（动作：list / get / create / update / delete / status）。
// 状态切换等扩展接口不属于标准基线，由各业务 api 文件（如 api/standardData.ts）自行追加 action 调用。
// 视图层只需 `const api = useRestApi<Xxx>('xxx')`，无需关心传输细节。
import {action} from '@/utils/request'
import type {DataTableApiParams} from '@/components/dataTable/types'

// 驼峰字段名 → snake_case 列名，用于把前端列字段名翻译成后端查询参数。
// 自定义取数的页面（如项目管理需追加 groupId）也复用它，避免各自复制一份。
export function toSnake(s: string): string {
    return s.replace(/([a-z0-9])([A-Z])/g, '$1_$2').toLowerCase()
}

export interface RestApi<T extends { ID: number }> {
    /** DataTable 的 api 函数：翻译分页/搜索/排序参数（信封 meta）并调用 POST /api/v1/{prefix}/list */
    fetch: (params: DataTableApiParams) => Promise<{ rows: T[]; total: number }>
    create: (data: Partial<T>) => Promise<any>
    update: (id: number, data: Partial<T>) => Promise<any>
    remove: (id: number) => Promise<any>
}

export function useRestApi<T extends { ID: number }>(prefix: string): RestApi<T> {
    async function fetch(params: DataTableApiParams) {
        const meta: Record<string, any> = {
            page: params.page,
            pageSize: params.pageSize,
        }
        if (params.sort?.field) {
            meta.sort = toSnake(params.sort.field)
            meta.order = params.sort.order === 'ascending' ? 'asc' : 'desc'
        }
        // 搜索：字段级模糊匹配（对应后端 listAction 的 meta.like，仅对 Searchable 列生效），
        // 与存量 GET 的「按字段 LIKE」语义一致。
        const like: Record<string, any> = {}
        for (const [k, v] of Object.entries(params.search || {})) {
            if (v !== '' && v !== null && v !== undefined) {
                like[toSnake(k)] = v
            }
        }
        if (Object.keys(like).length > 0) {
            meta.like = like
        }
        const res = await action<{ list: T[]; pagination: { total: number } }>(
            prefix, 'list', {}, meta,
        )
        // 统一信封解包后 res 即 data：分页时为 { list, pagination }。
        const raw = (res?.list ?? []) as any[]
        const rows = raw.map((it: any) => ({...it, ID: it.ID ?? it.Id})) as T[]
        const total = res?.pagination?.total ?? rows.length
        return {rows, total}
    }

    function create(data: Partial<T>) {
        return action<unknown>(prefix, 'create', data)
    }

    function update(id: number, data: Partial<T>) {
        return action<unknown>(prefix, 'update', {id, ...data})
    }

    function remove(id: number) {
        return action<unknown>(prefix, 'delete', {id})
    }

    return {fetch, create, update, remove}
}

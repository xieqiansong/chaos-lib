// DataTable 配置驱动表格的列定义与接口约定。
// 设计思想借鉴 jezetek-common-framework 的 TablePage/SearchForm（配置驱动 + 列派生多视图），
// 但技术栈改为 Vue3 + Element Plus，并简化为「列上标 searchable 即自动出搜索框」的直觉模型。

export interface DataTableSearchConfig {
  /** 搜索控件类型，默认 input */
  type?: 'input' | 'select'
  /** 搜索框前的标签，默认用列标题 */
  label?: string
  placeholder?: string
  /** type=select 时的选项 */
  options?: { label: string; value: any }[]
  width?: number | string
}

export interface DataTableColumn {
  /** 字段名，同时作为表格 prop 与搜索 state 的 key */
  field: string
  /** 列标题 */
  title: string
  width?: number | string
  minWidth?: number | string
  align?: 'left' | 'center' | 'right'
  fixed?: 'left' | 'right' | boolean
  /** 是否可排序（服务端模式自动转为 custom 排序并向 api 透传 sort） */
  sortable?: boolean
  /** 文本格式化（优先级低于具名插槽 #field） */
  formatter?: (row: any, value: any) => string
  /** 为 true 时自动在工具栏生成对应搜索项 */
  searchable?: boolean
  /** 搜索项细节 */
  search?: DataTableSearchConfig
  /** 标记为操作列，渲染 #actions 插槽 */
  type?: 'actions' | 'switch' | 'datetime'
  /** type=switch 时由消费方通过 switchHandler 回调处理（如调用业务自定义 setStatus 接口）；该字段应为行内的布尔状态字段 */
  /** type=datetime 时单元格按标准格式序列化（默认 yyyy-MM-dd HH:mm:ss，可用 datetimeFormat 覆盖，遵循 date-fns token） */
  datetimeFormat?: string
}

/** 服务端模式：请求参数 */
export interface DataTableApiParams {
  page: number
  pageSize: number
  /** 仅含 searchable 列中已填写的字段 */
  search: Record<string, any>
  sort?: { field: string; order: 'ascending' | 'descending' | null }
}

/** 服务端模式：响应约定 */
export interface DataTableApiResult {
  rows: any[]
  total: number
}

/** 内置 CRUD 动作标识（英文，避免魔法字符串），用于 enabledActions 逐项显隐 */
export const CRUD_ACTION = {
  CREATE: 'create',
  VIEW: 'view',
  EDIT: 'edit',
  DELETE: 'delete',
} as const
export type CrudAction = (typeof CRUD_ACTION)[keyof typeof CRUD_ACTION]

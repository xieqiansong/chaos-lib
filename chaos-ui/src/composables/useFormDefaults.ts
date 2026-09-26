// 表单字段默认值：根据 FormField 类型推导新建时的初始值，供 DataTable 初始化 form。
// 逻辑与 DataFormDialog 的字段类型契约保持一致（switch→true、number→0、datetime→null、其余→''）。
import type {FormField} from '@/components/dataTable/types'

export function defaultFieldValue(field: FormField): any {
  if (field.defaultValue !== undefined) return field.defaultValue
  if (field.type === 'switch') return true
  if (field.type === 'number') return 0
  if (field.type === 'datetime') return null
  return ''
}

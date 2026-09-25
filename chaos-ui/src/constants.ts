// 跨视图共享的常量：消除 PendingTask / TaskPlan / ReviewDialog 之间的重复定义。
// 单一可信源，任何文案/取值调整只需改这里。

/** 任务计划类型映射（待办 / 周期 / 间隔）—— 表格标签与下拉选项共用 */
export const PLAN_TYPE_MAP: Record<string, { text: string; type: string }> = {
  todo: { text: '待办', type: 'primary' },
  cron: { text: '周期', type: 'success' },
  interval: { text: '间隔', type: 'warning' },
}

/** 任务类型下拉选项（新建 / 编辑 / 添加子任务三处表单共用） */
export const PLAN_TYPE_OPTIONS = [
  { value: 'todo', label: '待办任务' },
  { value: 'cron', label: '周期重复任务' },
  { value: 'interval', label: '间隔任务' },
]

/**
 * FSRS 评分四档。
 * - label：标准 FSRS 术语，亦为后端 rating 取值（1=Again 2=Hard 3=Good 4=Easy）
 * - desc：中文释义，供按钮/选项展示
 * - type：Element Plus 语义色，供按钮使用
 */
export const FSRS_RATING_OPTIONS = [
  { value: 1, label: 'Again', desc: '完全想不起来 / 忘记了', type: 'danger' },
  { value: 2, label: 'Hard', desc: '记得但很吃力', type: 'warning' },
  { value: 3, label: 'Good', desc: '正常记得', type: 'primary' },
  { value: 4, label: 'Easy', desc: '太简单 / 毫不费力', type: 'success' },
] as const

/** 复习评分弹窗统一说明（原 PendingTask / TaskPlan / ReviewDialog 各写一份，现归并） */
export const FSRS_RATING_TIP =
  '阅读场景：提示下次重读的紧迫度。Easy→已烂熟，Good→按节奏，Hard→值得回顾，Again→完全没印象。不影响学习难度，只影响下次出现时间。'

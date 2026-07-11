import { ref } from 'vue'

// 全局待办任务刷新信号。
// 任一 PendingTasks 实例在完成/取消/评分/延期等操作后使版本号自增，
// 所有实例监听该信号并重新拉取，从而让侧边栏与任务表格实时同步。
export const pendingTasksVersion = ref(0)

export function refreshPendingTasks() {
  pendingTasksVersion.value++
}

import { ref } from 'vue'

// 全局任务计划刷新信号。
// 在居中面板完成复习后由 App.vue 触发自增，Task.vue 监听后重新拉取任务计划列表，
// 以便复习次数等复习相关字段能及时反映在「任务管理 / 任务计划」表中。
export const taskPlansVersion = ref(0)

export function refreshTaskPlans() {
  taskPlansVersion.value++
}
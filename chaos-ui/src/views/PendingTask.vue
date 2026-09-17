<script setup lang="ts">
import PendingTasks from '@/components/PendingTasks.vue'

// App.vue 会向所有路由组件注入 searchText，此处声明以接收该 prop（待办列表不做关键字过滤）
defineProps<{ searchText?: string }>()
</script>

<template>
  <div>
    <div class="section-toolbar flex items-center justify-between">
      <span class="text-primary text-base section-title">待办任务</span>
    </div>

    <PendingTasks/>
  </div>
</template>

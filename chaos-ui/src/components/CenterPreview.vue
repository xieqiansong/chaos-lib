<script setup lang="ts">
import {onMounted, ref} from 'vue'
import {sendMessage} from '@/utils/api'
import {ElMessage} from 'element-plus'
import MarkdownPreview from '@/components/MarkdownPreview.vue'

const props = defineProps<{
  planId: number
  planName: string
}>()

const loading = ref(false)
const rawLink = ref('')
const content = ref('')

onMounted(async () => {
  loading.value = true
  try {
    const res = await sendMessage(`taskPlans/${props.planId}/raw`, 'GET')
    rawLink.value = res?.rawLink ?? ''
    content.value = res?.content ?? ''
  } catch (e: any) {
    ElMessage.error(e?.message || '获取原文失败')
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div class="center-preview" v-loading="loading">
    <MarkdownPreview :raw-link="rawLink" :content="content" centered />
  </div>
</template>

<style scoped>
.center-preview {
  height: 100%;
  overflow: auto;
}
</style>

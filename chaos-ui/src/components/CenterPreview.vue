<script setup lang="ts">
import {computed, ref, onMounted} from 'vue'
import {sendMessage} from '@/utils/api'
import {ElMessage} from 'element-plus'
import MarkdownIt from 'markdown-it'
import DOMPurify from 'dompurify'

const props = defineProps<{
  planId: number
  planName: string
}>()

const md = new MarkdownIt({html: true, linkify: true, breaks: true})

const loading = ref(false)
const rawLink = ref('')
const content = ref('')

// raw_link 以 .md / .markdown 结尾时按 Markdown 渲染
const isMarkdown = computed(() => {
  const url = rawLink.value.split(/[?#]/)[0].toLowerCase()
  return url.endsWith('.md') || url.endsWith('.markdown')
})

const renderedHtml = computed(() => {
  if (!isMarkdown.value) return ''
  return DOMPurify.sanitize(md.render(content.value || ''))
})

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
    <!-- Markdown：渲染为 HTML（已 sanitize） -->
    <div v-if="isMarkdown" class="markdown-body" v-html="renderedHtml"></div>
    <!-- 其它格式：原样展示 -->
    <pre v-else class="preview-content">{{ content || '（空内容）' }}</pre>
  </div>
</template>

<style scoped>
.center-preview {
  height: 100%;
  overflow: auto;
}

.preview-content {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-word;
  font-family: var(--el-font-family);
  font-size: var(--el-font-size-base);
  line-height: 1.7;
  color: var(--el-text-color-primary);
  text-align: center;
}

.markdown-body {
  max-width: 880px;
  margin: 0 auto;
  padding: 0 var(--space-md);
  font-family: var(--el-font-family);
  font-size: var(--el-font-size-base);
  line-height: 1.7;
  color: var(--el-text-color-primary);
  text-align: left;
  word-break: break-word;
}

.markdown-body :deep(h1),
.markdown-body :deep(h2),
.markdown-body :deep(h3),
.markdown-body :deep(h4) {
  margin: 1.2em 0 0.6em;
  line-height: 1.3;
}

.markdown-body :deep(h1) {
  font-size: 1.6em;
}

.markdown-body :deep(h2) {
  font-size: 1.35em;
}

.markdown-body :deep(h3) {
  font-size: 1.15em;
}

.markdown-body :deep(p) {
  margin: 0.6em 0;
}

.markdown-body :deep(a) {
  color: var(--el-color-primary);
}

.markdown-body :deep(code) {
  background: var(--el-fill-color-light);
  padding: 0.1em 0.4em;
  border-radius: 4px;
  font-size: 0.9em;
}

.markdown-body :deep(pre) {
  background: var(--el-fill-color-light);
  padding: 12px;
  border-radius: var(--el-border-radius-base);
  overflow: auto;
}

.markdown-body :deep(pre code) {
  background: transparent;
  padding: 0;
}

.markdown-body :deep(blockquote) {
  margin: 0.6em 0;
  padding-left: 12px;
  border-left: 3px solid var(--el-border-color);
  color: var(--el-text-color-secondary);
}

.markdown-body :deep(table) {
  border-collapse: collapse;
  width: 100%;
  margin: 0.6em 0;
}

.markdown-body :deep(th),
.markdown-body :deep(td) {
  border: 1px solid var(--el-border-color-lighter);
  padding: 6px 10px;
}

.markdown-body :deep(img) {
  max-width: 100%;
}

.markdown-body :deep(ul),
.markdown-body :deep(ol) {
  padding-left: 1.4em;
}
</style>

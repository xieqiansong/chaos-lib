<script setup lang="ts">
// 统一 Markdown 预览：替代 CenterPreview 与 ReviewDialog 中重复的
// new MarkdownIt / isMarkdown / renderedHtml / .markdown-body 样式定义。
import {computed} from 'vue'
import MarkdownIt from 'markdown-it'
import DOMPurify from 'dompurify'

const props = withDefaults(defineProps<{
  /** 原文链接；以 .md / .markdown 结尾时按 Markdown 渲染，否则按纯文本展示 */
  rawLink?: string
  content: string
  /** 纯文本模式是否水平居中（CenterPreview 用，ReviewDialog 用左对齐） */
  centered?: boolean
  /** 复习场景下未「显示答案」前的模糊态 */
  blurred?: boolean
}>(), {
  rawLink: '',
  centered: false,
  blurred: false,
})

const md = new MarkdownIt({html: true, linkify: true, breaks: true})

const isMarkdown = computed(() => {
  const url = props.rawLink.split(/[?#]/)[0].toLowerCase()
  return url.endsWith('.md') || url.endsWith('.markdown')
})

const renderedHtml = computed(() =>
  isMarkdown.value ? DOMPurify.sanitize(md.render(props.content || '')) : '',
)
</script>

<template>
  <div class="markdown-preview">
    <div
      v-if="isMarkdown"
      class="markdown-body"
      :class="{blurred}"
      v-html="renderedHtml"
    />
    <pre
      v-else
      class="preview-content"
      :class="{blurred, centered}"
    >{{ content || '（空内容）' }}</pre>
  </div>
</template>

<style scoped>
.markdown-preview {
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
  text-align: left;
}

.preview-content.centered {
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

/* 模糊态（复习未显示答案前） */
.markdown-body.blurred,
.preview-content.blurred {
  filter: blur(8px);
  user-select: none;
}

.markdown-body :deep(h1),
.markdown-body :deep(h2),
.markdown-body :deep(h3),
.markdown-body :deep(h4) {
  margin: 1.2em 0 0.6em;
  line-height: 1.3;
}

.markdown-body :deep(h1) { font-size: 1.6em; }
.markdown-body :deep(h2) { font-size: 1.35em; }
.markdown-body :deep(h3) { font-size: 1.15em; }
.markdown-body :deep(p) { margin: 0.6em 0; }
.markdown-body :deep(a) { color: var(--el-color-primary); }
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
.markdown-body :deep(pre code) { background: transparent; padding: 0; }
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
.markdown-body :deep(img) { max-width: 100%; }
.markdown-body :deep(ul),
.markdown-body :deep(ol) { padding-left: 1.4em; }
</style>

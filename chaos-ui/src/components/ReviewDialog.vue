<script setup lang="ts">
import {ref, computed, watch, onMounted} from 'vue'
import {sendMessage} from '@/utils/api'
import {ElMessage} from 'element-plus'
import MarkdownIt from 'markdown-it'
import DOMPurify from 'dompurify'

const props = defineProps<{
  visible: boolean
  planId: number
  planName: string
}>()

const emit = defineEmits<{
  'update:visible': [boolean]
  done: []
}>()

const visible = computed({
  get: () => props.visible,
  set: (v: boolean) => emit('update:visible', v),
})

const loading = ref(false)
const rawLink = ref('')
const content = ref('')
const answer = ref('')

const md = new MarkdownIt({html: true, linkify: true, breaks: true})

// raw_link 以 .md / .markdown 结尾时按 Markdown 渲染
const isMarkdown = computed(() => {
  const url = rawLink.value.split(/[?#]/)[0].toLowerCase()
  return url.endsWith('.md') || url.endsWith('.markdown')
})

const renderedHtml = computed(() => {
  if (!isMarkdown.value) return ''
  return DOMPurify.sanitize(md.render(content.value || ''))
})
const revealed = ref(false)
const submitting = ref(false)
const selectedRating = ref<number | null>(null)

const aiLoading = ref(false)
const aiError = ref('')
const aiResult = ref<{
  points: { text: string; covered: boolean; reason: string }[]
  coverage: number
  suggestedRating: number
} | null>(null)

const ratingOptions = [
  {value: 1, label: 'Again', desc: '完全想不起来', type: 'danger'},
  {value: 2, label: 'Hard', desc: '记得但很吃力', type: 'warning'},
  {value: 3, label: 'Good', desc: '基本完整', type: 'primary'},
  {value: 4, label: 'Easy', desc: '毫不费力', type: 'success'},
]

watch(() => props.visible, (v) => {
  if (v) {
    reset()
    loadRaw()
  }
})

onMounted(() => {
  // 内联模式下由父组件控制挂载生命周期：挂载即展示并加载原文
  reset()
  loadRaw()
})

function reset() {
  loading.value = false
  content.value = ''
  answer.value = ''
  revealed.value = false
  selectedRating.value = null
  submitting.value = false
  aiLoading.value = false
  aiError.value = ''
  aiResult.value = null
}

async function aiScore() {
  if (!content.value.trim()) {
    ElMessage.warning('尚无原文内容，无法评分')
    return
  }
  aiLoading.value = true
  aiError.value = ''
  aiResult.value = null
  try {
    const res = await sendMessage('ai/review-score', 'POST', {
      original: content.value,
      answer: answer.value,
    })
    aiResult.value = {
      points: res.points || [],
      coverage: res.coverage ?? 0,
      suggestedRating: res.suggestedRating ?? 3,
    }
  } catch (e: any) {
    aiError.value = e?.message || 'AI 评分失败'
  } finally {
    aiLoading.value = false
  }
}

function ratingLabel(v: number): string {
  return ratingOptions.find(o => o.value === v)?.label || 'Good'
}

async function loadRaw() {
  loading.value = true
  try {
    const res = await sendMessage(`taskPlans/${props.planId}/raw`, 'GET')
    rawLink.value = res?.rawLink ?? ''
    content.value = res.content || ''
  } catch (e: any) {
    ElMessage.warning(e?.message || '获取原文失败')
    content.value = ''
  } finally {
    loading.value = false
  }
}

function reveal() {
  revealed.value = true
}

async function submit() {
  if (selectedRating.value === null) {
    ElMessage.error('请选择评分')
    return
  }
  submitting.value = true
  try {
    await sendMessage(`taskPlans/${props.planId}/review`, 'POST', {
      rating: selectedRating.value,
      answer: answer.value,
      ai: aiResult.value,
    })
    ElMessage.success('复习完成')
    visible.value = false
    emit('done')
  } catch (e: any) {
    ElMessage.error(e?.message || '提交失败')
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="center-review" v-loading="loading">
    <div class="review-layout">
      <div class="review-pane pane-raw">
        <div class="pane-header">
          <span>原文</span>
          <el-tag v-if="revealed" size="small" type="success">已显示</el-tag>
          <el-tag v-else size="small" type="info">已隐藏</el-tag>
        </div>
        <div class="pane-body">
          <div v-if="isMarkdown" class="markdown-body" :class="{blurred: !revealed}" v-html="renderedHtml"></div>
          <pre v-else class="raw-content" :class="{blurred: !revealed}">{{ content || '（无原文内容）' }}</pre>
          <div v-if="!revealed" class="mask" @click="reveal">
            <el-button type="primary" size="large" @click.stop="reveal">点击显示答案</el-button>
            <p class="mask-tip">先尽量回忆，再对照原文检查完整性</p>
          </div>
        </div>
      </div>

      <div class="review-pane pane-answer">
        <div class="pane-header">
          <span>我的回忆（关键词 / 要点）</span>
          <el-button
              size="small"
              :loading="aiLoading"
              @click="aiScore"
          >AI 评分</el-button>
        </div>
        <div class="pane-body review-answer-body">
          <div class="answer-half">
            <el-input
                v-model="answer"
                type="textarea"
                resize="none"
                placeholder="写下你能回忆起的内容，可以是关键词、要点或短句…"
                class="answer-input"
            />
          </div>
          <div class="ai-half">
            <div class="ai-block" v-if="aiResult || aiLoading || aiError">
              <div class="ai-head">
                <span class="ai-title">AI 覆盖度评分</span>
                <el-button
                    size="small"
                    :loading="aiLoading"
                    @click="aiScore"
                >重新评分</el-button>
              </div>
              <el-alert v-if="aiError" :title="aiError" type="error" show-icon :closable="false" />
              <template v-else-if="aiResult">
                <div class="ai-summary">
                  <span>覆盖度：<b>{{ aiResult.coverage }}%</b></span>
                  <span>建议：<b>{{ ratingLabel(aiResult.suggestedRating) }}</b></span>
                  <el-button size="small" type="primary" plain @click="selectedRating = aiResult!.suggestedRating">
                    采纳建议
                  </el-button>
                </div>
                <ul class="ai-points">
                  <li v-for="(p, i) in aiResult.points" :key="i">
                    <el-tag size="small" :type="p.covered ? 'success' : 'danger'">
                      {{ p.covered ? '命中' : '遗漏' }}
                    </el-tag>
                    <span class="pt-text">{{ p.text }}</span>
                    <span class="pt-reason">{{ p.reason }}</span>
                  </li>
                </ul>
              </template>
            </div>
            <div v-else class="ai-placeholder">点击「AI 评分」对照原文检查覆盖度</div>
          </div>
        </div>
      </div>
    </div>

    <div class="review-footer">
      <div class="rating-area">
        <div class="rating-buttons">
          <el-button
              v-for="opt in ratingOptions"
              :key="opt.value"
              :type="opt.type"
              :plain="selectedRating !== opt.value"
              @click="selectedRating = opt.value"
          >
            {{ opt.label }}
            <span class="rating-desc">{{ opt.desc }}</span>
          </el-button>
        </div>
      </div>
      <div class="footer-actions">
        <el-button @click="visible = false">关闭</el-button>
        <el-button type="primary" :loading="submitting" @click="submit">提交评分</el-button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.center-review {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
}

.review-layout {
  display: flex;
  gap: 16px;
  flex: 1;
  min-height: 0;
}

.review-pane {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
}

/* 原文 : 答案 = 2 : 1 */
.review-pane.pane-raw {
  flex: 2;
}

.review-pane.pane-answer {
  flex: 1;
}

.pane-header {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 600;
  margin-bottom: 8px;
}

.pane-body {
  position: relative;
  flex: 1;
  overflow: hidden;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: var(--el-border-radius-base);
}

/* 答案 + AI 评分结果 共用同一区域，上下各占一半 */
.review-answer-body {
  display: flex;
  flex-direction: column;
}

.answer-half {
  flex: 1;
  min-height: 0;
  display: flex;
  overflow: hidden;
}

.ai-half {
  flex: 1;
  min-height: 0;
  overflow: auto;
  border-top: 1px solid var(--el-border-color-lighter);
  padding: 10px 12px;
}

.ai-placeholder {
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--el-text-color-secondary);
  font-size: 13px;
  text-align: center;
}

.raw-content {
  margin: 0;
  padding: 12px;
  height: 100%;
  overflow: auto;
  white-space: pre-wrap;
  word-break: break-word;
  font-family: var(--el-font-family);
  font-size: 14px;
  line-height: 1.6;
  box-sizing: border-box;
}

.raw-content.blurred {
  filter: blur(8px);
  user-select: none;
}

.markdown-body {
  height: 100%;
  overflow: auto;
  padding: 12px;
  box-sizing: border-box;
  font-family: var(--el-font-family);
  font-size: 14px;
  line-height: 1.6;
  word-break: break-word;
  text-align: left;
}

.markdown-body.blurred {
  filter: blur(8px);
  user-select: none;
}

.markdown-body :deep(h1),
.markdown-body :deep(h2),
.markdown-body :deep(h3),
.markdown-body :deep(h4) {
  margin: 1.1em 0 0.5em;
  line-height: 1.3;
}

.markdown-body :deep(h1) {
  font-size: 1.5em;
}

.markdown-body :deep(h2) {
  font-size: 1.3em;
}

.markdown-body :deep(h3) {
  font-size: 1.15em;
}

.markdown-body :deep(p) {
  margin: 0.5em 0;
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
  margin: 0.5em 0;
  padding-left: 12px;
  border-left: 3px solid var(--el-border-color);
  color: var(--el-text-color-secondary);
}

.markdown-body :deep(table) {
  border-collapse: collapse;
  width: 100%;
  margin: 0.5em 0;
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

.mask {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  background: rgba(var(--el-bg-color-rgb, 255, 255, 255), 0.65);
  cursor: pointer;
}

.mask-tip {
  color: var(--el-text-color-secondary);
  font-size: 13px;
  margin: 0;
}

.answer-input {
  flex: 1;
  min-height: 0;
}

.answer-input :deep(.el-textarea),
.answer-input :deep(.el-textarea__inner) {
  height: 100%;
}

.review-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.rating-buttons {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.rating-desc {
  margin-left: 4px;
  opacity: 0.8;
  font-size: 12px;
}

.footer-actions {
  display: flex;
  gap: 8px;
}

.rating-area {
  display: flex;
  flex-direction: column;
  gap: 10px;
  min-width: 0;
  flex: 1;
}

.ai-block {
  border: 1px solid var(--el-border-color-lighter);
  border-radius: var(--el-border-radius-base);
  padding: 10px 12px;
  background: var(--el-fill-color-light);
}

.ai-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}

.ai-title {
  font-weight: 600;
}

.ai-summary {
  display: flex;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;
  margin-bottom: 8px;
}

.ai-points {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.ai-points li {
  display: flex;
  align-items: baseline;
  gap: 8px;
  font-size: 13px;
  line-height: 1.5;
}

.pt-text {
  font-weight: 500;
}

.pt-reason {
  color: var(--el-text-color-secondary);
  font-size: 12px;
}
</style>

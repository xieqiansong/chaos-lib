<script setup lang="ts">
import {ref, computed, watch} from 'vue'
import {sendMessage} from '@/utils/api'
import {ElMessage} from 'element-plus'

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
const content = ref('')
const answer = ref('')
const revealed = ref(false)
const submitting = ref(false)
const selectedRating = ref<number | null>(null)

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

function reset() {
  loading.value = false
  content.value = ''
  answer.value = ''
  revealed.value = false
  selectedRating.value = null
  submitting.value = false
}

async function loadRaw() {
  loading.value = true
  try {
    const res = await sendMessage(`taskPlans/${props.planId}/raw`, 'GET')
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
  <el-dialog
      v-model="visible"
      :title="`复习 — ${planName}`"
      width="92%"
      top="4vh"
      :close-on-click-modal="false"
      class="review-dialog"
  >
    <div v-loading="loading" class="review-layout">
      <div class="review-pane">
        <div class="pane-header">
          <span>原文</span>
          <el-tag v-if="revealed" size="small" type="success">已显示</el-tag>
          <el-tag v-else size="small" type="info">已隐藏</el-tag>
        </div>
        <div class="pane-body">
          <pre class="raw-content" :class="{blurred: !revealed}">{{ content || '（无原文内容）' }}</pre>
          <div v-if="!revealed" class="mask" @click="reveal">
            <el-button type="primary" size="large" @click.stop="reveal">点击显示答案</el-button>
            <p class="mask-tip">先尽量回忆，再对照原文检查完整性</p>
          </div>
        </div>
      </div>

      <div class="review-pane">
        <div class="pane-header">我的回忆（关键词 / 要点）</div>
        <div class="pane-body">
          <el-input
              v-model="answer"
              type="textarea"
              resize="none"
              placeholder="写下你能回忆起的内容，可以是关键词、要点或短句…"
              class="answer-input"
          />
        </div>
      </div>
    </div>

    <template #footer>
      <div class="review-footer">
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
        <div class="footer-actions">
          <el-button @click="visible = false">关闭</el-button>
          <el-button type="primary" :loading="submitting" @click="submit">提交评分</el-button>
        </div>
      </div>
    </template>
  </el-dialog>
</template>

<style scoped>
.review-layout {
  display: flex;
  gap: 16px;
  height: 70vh;
}

.review-pane {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
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
  height: 100%;
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
</style>

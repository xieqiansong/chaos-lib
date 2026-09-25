<script setup lang="ts">
// 统一 FSRS 评分弹窗：替代 PendingTask / TaskPlan 中结构完全相同的评分对话框。
// 评分选项、说明文案均来自 src/constants.ts，确保单一可信源。
import {computed} from 'vue'
import {ElMessage} from 'element-plus'
import {FSRS_RATING_OPTIONS, FSRS_RATING_TIP} from '@/constants'

const props = withDefaults(defineProps<{
  modelValue: boolean
  /** 当前评分（v-model:rating），未选为 null */
  rating?: number | null
  /** 弹窗标题前缀，如「完成间隔任务」「开启间隔任务」 */
  title?: string
  /** 目标名称（任务 / 计划名），拼接在标题后 */
  targetName?: string
  /** 确认按钮 loading */
  loading?: boolean
}>(), {
  rating: null,
  title: '评分',
  targetName: '',
  loading: false,
})

const emit = defineEmits<{
  (e: 'update:modelValue', v: boolean): void
  (e: 'update:rating', v: number | null): void
  (e: 'submit', rating: number): void
}>()

const visible = computed({
  get: () => props.modelValue,
  set: (v) => emit('update:modelValue', v),
})

const rating = computed({
  get: () => props.rating,
  set: (v) => emit('update:rating', v),
})

function onConfirm() {
  if (rating.value == null) {
    ElMessage.error('请选择评分')
    return
  }
  emit('submit', rating.value)
}
</script>

<template>
  <el-dialog
    v-model="visible"
    :title="targetName ? `${title} — ${targetName}` : title"
    width="30rem"
  >
    <el-alert
      type="info"
      :closable="false"
      show-icon
      :title="FSRS_RATING_TIP"
      class="mb-sm"
    />
    <!-- .rating-group 为全局工具类（style.css），无需此处重复定义 -->
    <div class="rating-group">
      <el-radio-group v-model="rating">
        <el-radio
          v-for="opt in FSRS_RATING_OPTIONS"
          :key="opt.value"
          :value="opt.value"
          :label="opt.value"
        >
          {{ opt.label }}（{{ opt.desc }}）
        </el-radio>
      </el-radio-group>
    </div>
    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button type="primary" :loading="loading" @click="onConfirm">确认</el-button>
    </template>
  </el-dialog>
</template>

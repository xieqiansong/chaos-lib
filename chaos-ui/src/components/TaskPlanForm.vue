<script setup lang="ts">
// 任务计划表单（新建 / 添加子任务 / 修改三处复用）。
// 直接绑定父级传入的 formData 响应式对象，字段改动即时回写父级，
// 因此父级的 createPlan / updatePlan 逻辑与校验保持不变。
import {PLAN_TYPE_OPTIONS} from '@/constants'

defineProps<{
  formData: {
    Name: string
    PlanType: string
    CronExpr: string
    StartedAt: string
    Remark: string
    Link: string
    ParentId: number | null
    OrderNum: number | null
    Priority: number | null
  }
  /** 是否展示「父任务」选择（仅修改模式） */
  showParentSelect?: boolean
  /** 修改模式下父任务树形选项 */
  selectableParents?: any[]
}>()
</script>

<template>
  <el-form label-position="top">
    <el-form-item v-if="showParentSelect" label="父任务">
      <el-tree-select
        v-model="formData.ParentId"
        :data="selectableParents"
        :props="{ label: 'Name', value: 'ID', children: 'Children' }"
        node-key="ID"
        check-strictly
        clearable
        placeholder="根节点（无父任务）"
        style="width: 100%"
      />
    </el-form-item>
    <el-form-item label="任务名称">
      <el-input v-model="formData.Name" placeholder="请输入任务名称"/>
    </el-form-item>
    <el-form-item label="任务类型">
      <el-select v-model="formData.PlanType">
        <el-option v-for="f in PLAN_TYPE_OPTIONS" :key="f.value" :label="f.label" :value="f.value"/>
      </el-select>
    </el-form-item>
    <el-form-item v-if="formData.PlanType === 'cron'" label="Cron 表达式">
      <el-input v-model="formData.CronExpr" placeholder="如: 0 8 * * * (每天8:00)"/>
    </el-form-item>
    <el-form-item v-if="formData.PlanType === 'todo'" label="开始时间">
      <el-date-picker
        v-model="formData.StartedAt"
        type="datetime"
        format="YYYY-MM-DD HH:mm:ss"
        value-format="YYYY-MM-DD[T]HH:mm:ssZ"
        placeholder="选择开始时间"
        style="width: 100%"
      />
    </el-form-item>
    <div class="form-row">
      <el-form-item label="优先级">
        <el-input-number v-model="formData.Priority" :min="0" :controls="false" placeholder="数值越大越优先" style="width: 100%"/>
      </el-form-item>
      <el-form-item label="排序">
        <el-input-number v-model="formData.OrderNum" :min="0" :controls="false" placeholder="控制显示顺序" style="width: 100%"/>
      </el-form-item>
    </div>
    <el-form-item label="关联链接">
      <el-input v-model="formData.Link" placeholder="可选"/>
    </el-form-item>
    <el-form-item label="备注">
      <el-input v-model="formData.Remark" type="textarea" :rows="2" placeholder="可选备注"/>
    </el-form-item>
  </el-form>
</template>

<style scoped>
.form-row {
  display: flex;
  gap: var(--space-md);
}

.form-row .el-form-item {
  flex: 1;
  margin-bottom: 18px;
}
</style>

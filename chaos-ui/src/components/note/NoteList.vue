<script setup lang="ts">
// 笔记列表：按目录过滤 + 关键词 + 星标/缺失筛选；点击行 → 触发预览。
import {computed, ref, watch} from 'vue'
import type {DataTableApiParams} from '@/components/dataTable/types'
import type {Note} from '@/api/note'
import {noteApi} from '@/api/note'

const props = defineProps<{
  currentDir: string
  keyword: string
  onlyStarred: boolean
  onlyMissing: boolean
}>()

const emit = defineEmits<{
  (e: 'open', note: Note): void
  (e: 'loaded', rows: Note[]): void
}>()

const rows = ref<Note[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(50)
const loading = ref(false)

const search = computed(() => ({
  dir: props.currentDir,
  q: props.keyword,
  starred: props.onlyStarred,
  missing: props.onlyMissing,
}))

async function load() {
  loading.value = true
  try {
    const params: DataTableApiParams = {
      page: page.value,
      pageSize: pageSize.value,
      search: search.value,
      sort: {field: 'updatedAt', order: 'descending'},
    }
    const res = await noteApi.fetch(params)
    rows.value = res.rows
    total.value = res.total
    emit('loaded', res.rows)
  } finally {
    loading.value = false
  }
}

function formatSize(n: number): string {
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`
  return `${(n / 1024 / 1024).toFixed(1)} MB`
}

watch(
    () => [props.currentDir, props.keyword, props.onlyStarred, props.onlyMissing, page.value],
    () => load(),
    {immediate: true},
)

defineExpose({load})
</script>

<template>
  <div class="note-list">
    <el-table
        v-loading="loading"
        :data="rows"
        height="100%"
        highlight-current-row
        @row-click="(row: Note) => emit('open', row)"
    >
      <el-table-column prop="Title" label="标题" min-width="160" show-overflow-tooltip>
        <template #default="{row}">
          <span :class="{ 'note-list__missing': row.DiskMissing }">{{ row.Title || row.Name }}</span>
        </template>
      </el-table-column>
      <el-table-column prop="Format" label="格式" width="80"/>
      <el-table-column prop="SizeBytes" label="大小" width="100">
        <template #default="{row}">{{ formatSize(row.SizeBytes) }}</template>
      </el-table-column>
      <el-table-column prop="UpdatedAt" label="更新" width="170" show-overflow-tooltip/>
    </el-table>
    <div class="note-list__footer">
      <el-pagination
          v-model:current-page="page"
          :page-size="pageSize"
          :total="total"
          layout="total, prev, pager, next"
          small
          background
      />
    </div>
  </div>
</template>

<style scoped>
.note-list {
  height: 100%;
  display: flex;
  flex-direction: column;
  border-right: 1px solid var(--el-border-color);
}

.note-list :deep(.el-table) {
  flex: 1;
}

.note-list__footer {
  padding: 6px;
  display: flex;
  justify-content: flex-end;
  border-top: 1px solid var(--el-border-color);
}

.note-list__missing {
  color: var(--el-color-danger);
  text-decoration: line-through;
}
</style>

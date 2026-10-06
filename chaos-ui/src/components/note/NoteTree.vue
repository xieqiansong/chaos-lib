<script setup lang="ts">
// 目录树：文件夹节点点击 → 设置列表过滤目录；笔记节点点击 → 触发预览。
import {onMounted, ref} from 'vue'
import type {TreeNode} from '@/api/note'
import {noteApi} from '@/api/note'

const props = defineProps<{
  /** 当前选中的目录相对路径（用于高亮） */
  currentDir: string
}>()

const emit = defineEmits<{
  (e: 'select-dir', relPath: string): void
  (e: 'open-note', node: TreeNode): void
}>()

const treeData = ref<TreeNode[]>([])
const loading = ref(false)

async function load() {
  loading.value = true
  try {
    treeData.value = await noteApi.tree()
  } finally {
    loading.value = false
  }
}

// 叶节点（笔记）直接回传整个节点：它已带ID / Title，父组件据此打开编辑器即可。
function onNodeClick(node: TreeNode) {
  if (node.isLeaf) {
    emit('open-note', node)
  } else {
    emit('select-dir', node.relPath)
  }
}

onMounted(load)
defineExpose({load})
</script>

<template>
  <div class="note-tree">
    <div class="note-tree__head">
      <span>目录</span>
      <el-button text size="small" :loading="loading" @click="load">刷新</el-button>
    </div>
    <el-scrollbar class="note-tree__body">
      <el-tree
          :data="treeData"
          :props="{label: 'name', children: 'children', isLeaf: 'isLeaf'}"
          node-key="relPath"
          highlight-current
          :expand-on-click-node="false"
          @node-click="(_, node) => onNodeClick(node.data as TreeNode)"
      >
        <template #default="{node}">
          <span class="note-tree__label">
            <el-icon v-if="!(node.data as TreeNode).isLeaf"><Folder /></el-icon>
            <el-icon v-else><Document /></el-icon>
            <span class="note-tree__name">{{ node.label }}</span>
          </span>
        </template>
      </el-tree>
    </el-scrollbar>
  </div>
</template>

<style scoped>
.note-tree {
  height: 100%;
  display: flex;
  flex-direction: column;
  border-right: 1px solid var(--el-border-color);
}

.note-tree__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
  font-weight: 600;
  border-bottom: 1px solid var(--el-border-color);
}

.note-tree__body {
  flex: 1;
  padding: 4px;
}

.note-tree__label {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.note-tree__name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>

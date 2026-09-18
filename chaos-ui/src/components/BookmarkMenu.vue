<script setup lang="ts">
import {ref} from 'vue'
import {Folder} from '@element-plus/icons-vue'

const props = defineProps<{
  nodes: any[]
}>()

// 已展开的子文件夹 id 集合（下拉内可继续展开多层）
const expanded = ref<Set<string>>(new Set())

function toggle(id: string) {
  const s = new Set(expanded.value)
  if (s.has(id)) s.delete(id)
  else s.add(id)
  expanded.value = s
}

function favicon(url: string): string {
  try {
    return `https://www.google.com/s2/favicons?domain=${new URL(url).hostname}&sz=32`
  } catch {
    return ''
  }
}
</script>

<template>
  <div class="bm-menu">
    <template v-for="node in nodes" :key="node.id">
      <a
          v-if="node.url"
          class="bm-menu-item"
          :href="node.url"
          target="_blank"
          rel="noopener"
          :title="node.url"
      >
        <img class="bm-menu-fav" :src="favicon(node.url)" alt="" @error="($event.target as HTMLImageElement).style.visibility = 'hidden'"/>
        <span class="bm-menu-label">{{ node.title || node.url }}</span>
      </a>
      <div
          v-else
          class="bm-menu-item bm-menu-folder"
          :class="{open: expanded.has(node.id)}"
          @click="toggle(node.id)"
      >
        <el-icon class="bm-menu-fav"><Folder/></el-icon>
        <span class="bm-menu-label">{{ node.title || '(无标题)' }}</span>
        <span class="bm-menu-caret">▸</span>
        <BookmarkMenu v-if="expanded.has(node.id)" class="bm-menu-nested" :nodes="node.children || []"/>
      </div>
    </template>
    <div v-if="!nodes || !nodes.length" class="bm-menu-empty">空文件夹</div>
  </div>
</template>

<style scoped>
.bm-menu {
  display: flex;
  flex-direction: column;
  gap: 1px;
  min-width: 100%;
}

.bm-menu-item {
  display: flex;
  align-items: center;
  gap: var(--space-05);
  padding: var(--space-05) var(--space-sm);
  border-radius: 3px;
  text-decoration: none;
  color: var(--el-text-color-regular);
  font-size: var(--el-font-size-small);
  cursor: pointer;
  transition: background 0.12s ease;
}

.bm-menu-item:hover {
  background: var(--el-fill-color-light);
  color: var(--el-text-color-primary);
}

.bm-menu-fav {
  width: 14px;
  height: 14px;
  flex-shrink: 0;
  border-radius: 2px;
  color: var(--el-color-warning);
}

.bm-menu-label {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  flex: 1;
  min-width: 0;
}

/* 文件夹：右箭头 + 展开后旋转 90° */
.bm-menu-folder {
  position: relative;
}

.bm-menu-caret {
  flex-shrink: 0;
  color: var(--el-text-color-secondary);
  font-size: 10px;
  transition: transform 0.12s ease;
}

.bm-menu-folder.open > .bm-menu-caret {
  transform: rotate(90deg);
}

/* 子文件夹展开后竖向下挂（缩进一层） */
.bm-menu-nested {
  margin-left: var(--space-md);
  padding-left: var(--space-sm);
  border-left: 1px solid var(--el-border-color-lighter);
}

.bm-menu-empty {
  padding: var(--space-sm);
  font-size: var(--el-font-size-small);
  color: var(--el-text-color-secondary);
}
</style>

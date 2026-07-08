<script setup lang="ts">
import {computed, onMounted, ref, watch} from 'vue'

interface TreeItem {
  id: string
  label: string
  url: string
}

const props = defineProps<{
  searchText: string
}>()

const browserHistories = ref<TreeItem[]>([])

function loadBrowserHistory() {
  // browser history API is not available in web context
  browserHistories.value = []
}

const treeData = computed<TreeItem[]>(() => {
  return browserHistories.value
})

watch(() => props.searchText, () => {
  loadBrowserHistory()
})

onMounted(() => {
  loadBrowserHistory()
})
</script>

<template>
  <el-row>
    <el-col :span="12">
      <el-empty description="浏览器历史记录功能在 Web 端不可用"/>
    </el-col>
  </el-row>
</template>
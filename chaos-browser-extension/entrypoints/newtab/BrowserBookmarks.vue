<script lang="ts" setup>

//  @ts-ignore
import type {BookmarkTreeNode} from 'webextension-polyfill'

const props = defineProps<{
  searchText: string
}>()


const handleNodeClick = (data: BookmarkTreeNode) => {
  if (data.children?.length) return
  if (data.url) browser.tabs.create({url: data.url})
}

const browserBookmarks = ref<BookmarkTreeNode[]>([])

const defaultProps = {
  children: 'children',
  label: 'title',
}

function loadBrowserBookmarks() {
  browser.bookmarks.getTree().then((rootBookmarks) => {
    browserBookmarks.value = (rootBookmarks[0].children || []).filter(o => o.title === '书签栏')
  })
}


watch(() => props.searchText, () => {
  loadBrowserBookmarks()
})

onMounted(() => {
  loadBrowserBookmarks()
})


</script>

<template>
  <el-tree
      :data="browserBookmarks"
      :props="defaultProps"
      node-key="id"
      :default-expanded-keys="['1']"
      @node-click="handleNodeClick"
  />
</template>
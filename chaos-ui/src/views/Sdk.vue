<script setup lang="ts">
import {computed, onMounted, ref} from 'vue'
import {API_BASE} from '@/utils/api'

interface SdkInfo {
  CurrentVersion: string
  VersionList: string[]
}

type SdkData = Record<string, SdkInfo>

const sdkData = ref<SdkData | null>(null)
const error = ref('')
const switchingSdk = ref<string | null>(null)

const sdkTypes = computed(() => {
  if (!sdkData.value) return []
  return Object.entries(sdkData.value).map(([key, value]) => ({
    key,
    label: key,
    info: value
  }))
})

async function fetchSdks() {
  error.value = ''
  try {
    const response = await fetch(API_BASE + '/sdks')
    if (!response.ok) {
      throw new Error('Failed to fetch SDKs')
    }
    sdkData.value = await response.json()
  } catch (e) {
    error.value = '获取SDK信息失败'
    console.error(e)
  }
}

async function switchVersion(type: string, version: string) {
  switchingSdk.value = type
  try {
    const response = await fetch(`${API_BASE}/sdks/${type}/switch`, {
      method: 'PATCH',
      headers: {
        'Content-Type': 'application/json'
      },
      body: JSON.stringify({version})
    })
    if (!response.ok) {
      throw new Error('Failed to switch version')
    }
    const result = await response.json()
    if (sdkData.value) {
      sdkData.value[type].CurrentVersion = result.version
    }
  } catch (e) {
    console.error(e)
  } finally {
    switchingSdk.value = null
  }
}

onMounted(() => {
  fetchSdks()
})
</script>

<template>
  <div>
    <div class="section-toolbar">
      <span class="text-primary text-base section-title">SDK 版本管理</span>
      <div class="section-actions">
        <el-button size="small" @click="fetchSdks" plain>刷新</el-button>
      </div>
    </div>

    <el-alert
        v-if="error"
        type="error"
        :message="error"
        show-icon
        class="mb-sm"
        @close="error = ''"
    />

    <div v-if="sdkTypes.length === 0" class="empty-wrap">
      <el-empty description="暂无 SDK 数据"/>
    </div>

    <div v-else class="sdk-list">
      <div v-for="sdkType in sdkTypes" :key="sdkType.key" class="sdk-row">
        <span class="text-primary text-sm sdk-label">{{ sdkType.label }}</span>
        <div class="sdk-tags">
          <el-tag
              v-for="version in sdkType.info.VersionList"
              :key="version"
              :type="version === sdkType.info.CurrentVersion ? 'primary' : 'info'"
              :class="['sdk-tag', { 'is-current': version === sdkType.info.CurrentVersion }]"
              :effect="version === sdkType.info.CurrentVersion ? 'dark' : 'plain'"
              @click="switchVersion(sdkType.key, version)"
          >
            {{ version }}
          </el-tag>
        </div>
        <span v-if="switchingSdk === sdkType.key" class="text-secondary text-xs">切换中...</span>
      </div>
    </div>
  </div>
</template>

<style scoped>
.sdk-list {
  display: flex;
  flex-direction: column;
  gap: var(--space-md);
}

.sdk-row {
  display: flex;
  align-items: center;
  gap: var(--space-md);
  flex-wrap: wrap;
  padding: var(--space-sm) var(--space-md);
  background: var(--el-fill-color-lighter);
  border-radius: var(--el-border-radius-base);
}

.sdk-label {
  font-weight: 600;
  white-space: nowrap;
  min-width: 3rem;
}

.sdk-tags {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-05);
}

.sdk-tag {
  cursor: pointer;
}

.sdk-tag.is-current {
  cursor: default;
}
</style>
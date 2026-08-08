<script setup lang="ts">
import {onMounted, onUnmounted, ref} from 'vue'
import {format} from 'date-fns'
import {zhCN} from 'date-fns/locale'
import {ElMessage, ElMessageBox} from 'element-plus'

const now = ref(new Date())
let timer: ReturnType<typeof setInterval>
let timeTimer: ReturnType<typeof setInterval>
let weatherTimer: ReturnType<typeof setInterval>

// 标准时钟偏移量（标准时间 - 本地时间），默认 0
const timeOffset = ref(0)

function goBack() {
  window.location.hash = 'dashboard'
}

// ---- 全屏切换 ----
const isFullscreen = ref(false)

function getFullscreenElement(): Element | null {
  return document.fullscreenElement ||
      (document as any).webkitFullscreenElement ||
      null
}

async function toggleFullscreen() {
  try {
    const el = getFullscreenElement()
    if (!el) {
      const target = document.documentElement as any
      if (target.requestFullscreen) {
        await target.requestFullscreen()
      } else if (target.webkitRequestFullscreen) {
        await target.webkitRequestFullscreen()
      }
    } else {
      const doc = document as any
      if (doc.exitFullscreen) {
        await doc.exitFullscreen()
      } else if (doc.webkitExitFullscreen) {
        await doc.webkitExitFullscreen()
      }
    }
  } catch (e) {
    // 部分浏览器（如 iOS Safari 普通元素）不支持，静默忽略
  }
}

function onFullscreenChange() {
  isFullscreen.value = !!getFullscreenElement()
}

// 全屏时点击屏幕任意处退出全屏（按钮已隐藏）
async function onScreenClick() {
  if (!isFullscreen.value) return
  try {
    const doc = document as any
    if (doc.exitFullscreen) {
      await doc.exitFullscreen()
    } else if (doc.webkitExitFullscreen) {
      await doc.webkitExitFullscreen()
    }
  } catch (e) {
    // 忽略
  }
}

// ---- 屏幕分辨率（物理像素 + 设备像素比）----
const screenRes = ref<string>('')

function updateScreenRes() {
  const sw = window.screen.width
  const sh = window.screen.height
  const dpr = window.devicePixelRatio || 1
  screenRes.value = `${sw}×${sh} @ ${dpr.toFixed(2)}x`
}

// ---- 标准时间校准（每小时一次）----
// 以本机后端响应的 Date 头作为标准时钟，算出偏移后本地继续走秒，避免抖动/闪烁
async function syncTime() {
  try {
    const res = await fetch('/api/tasks/activeStats', {cache: 'no-store'})
    const dateStr = res.headers.get('Date')
    if (!dateStr) return
    const serverMs = new Date(dateStr).getTime()
    if (isNaN(serverMs)) return
    timeOffset.value = serverMs - Date.now()
    now.value = new Date(Date.now() + timeOffset.value)
  } catch (e) {
    // 拉取失败不修改
  }
}

// ---- 天气 ----
// 默认位置（成都市中心），如支持地理定位则优先使用
const LOC_KEY = 'chaos_board_location'
interface SavedPos { lat: number; lon: number; name: string }
interface GeoPos { lat: number; lon: number; name?: string }

const DEFAULT_POS = {lat: 30.5728, lon: 104.0668}
const weatherRegion = ref<string>('')
const weatherTemp = ref<number | null>(null)
const weatherHumidity = ref<number | null>(null)
const weatherDesc = ref<string>('')

let cachedPos: GeoPos | null = null

// 读取浏览器保存的手动位置
function loadSavedPos(): SavedPos | null {
  try {
    const raw = localStorage.getItem(LOC_KEY)
    if (!raw) return null
    const p = JSON.parse(raw)
    if (typeof p.lat === 'number' && typeof p.lon === 'number' && typeof p.name === 'string') {
      return p as SavedPos
    }
  } catch (e) {
    // 解析失败忽略
  }
  return null
}

// 保存手动位置到浏览器（持久化）
function savePos(p: SavedPos) {
  try {
    localStorage.setItem(LOC_KEY, JSON.stringify(p))
  } catch (e) {
    // 忽略
  }
}

// 天气由后端 /api/weather 代理（AK 在后端 .env，前端不持有），规避跨域/ORB 问题
async function getPosition(): Promise<GeoPos> {
  if (cachedPos) return cachedPos
  // 优先使用手动保存的位置
  const saved = loadSavedPos()
  if (saved) {
    const pos: GeoPos = {lat: saved.lat, lon: saved.lon, name: saved.name}
    cachedPos = pos
    return pos
  }
  let pos: GeoPos = {...DEFAULT_POS}
  if (navigator.geolocation) {
    pos = await new Promise<GeoPos>(resolve => {
      navigator.geolocation.getCurrentPosition(
          p => resolve({lat: p.coords.latitude, lon: p.coords.longitude}),
          () => resolve({...DEFAULT_POS}),
          {timeout: 3000},
      )
    })
  }
  cachedPos = pos
  return pos
}

// 天气数据：后端代理请求，拉取失败不修改原值（不闪烁、无加载感）
async function fetchWeatherNow(pos: GeoPos) {
  try {
    const url = `/api/weather?lat=${pos.lat}&lon=${pos.lon}`
    const res = await fetch(url, {cache: 'no-store'})
    if (!res.ok) return
    const data = await res.json()
    if (data.status !== 0 || !data.result || !data.result.now) return
    const now = data.result.now
    weatherTemp.value = now.temp
    weatherHumidity.value = now.rh
    weatherDesc.value = now.text
    // 区域名：手动设置（pos.name 存在）时保留；否则用百度返回的区域
    if (!pos.name && data.result.location) {
      const loc = data.result.location
      weatherRegion.value = loc.name || loc.city || loc.province || '本地'
    }
  } catch (e) {
    // 拉取失败不修改
  }
}

async function refreshWeather() {
  const pos = await getPosition()
  await fetchWeatherNow(pos)
}

// 手动设置位置：直接输入「纬度,经度」（如 30.57,104.07），也兼容输入城市名
async function setCity() {
  try {
    const {value} = await ElMessageBox.prompt(
        '输入经纬度，例如：30.57,104.07\n（也可直接输入城市名，如：北京）',
        '设置位置',
        {
          inputValue: cachedPos ? `${cachedPos.lat}, ${cachedPos.lon}` : '',
          confirmButtonText: '确定',
          cancelButtonText: '取消',
        },
    )
    const text = (value || '').trim()
    if (!text) return

    // 先尝试按「纬度,经度」解析
    const parts = text.split(/[ ,，]+/).filter(Boolean).map(Number)
    const isCoord = parts.length === 2 && parts.every(n => !isNaN(n)) &&
        parts[0] >= -90 && parts[0] <= 90 && parts[1] >= -180 && parts[1] <= 180

    let pos: GeoPos
    if (isCoord) {
      pos = {lat: parts[0], lon: parts[1], name: '手动设置'}
    } else {
      // 否则当作城市名，用 Open-Meteo 地理编码解析
      const url = `https://geocoding-api.open-meteo.com/v1/search?name=${encodeURIComponent(text)}&language=zh&count=1`
      const data = await (await fetch(url)).json()
      const r = data.results && data.results[0]
      if (!r) {
        ElMessage.error('未找到该城市，请换个名称或输入经纬度')
        return
      }
      pos = {
        lat: r.latitude,
        lon: r.longitude,
        name: (r.admin1 && r.name !== r.admin1) ? `${r.name}·${r.admin1}` : (r.name || text),
      }
    }

    cachedPos = pos
    savePos({lat: pos.lat, lon: pos.lon, name: pos.name})
    weatherRegion.value = pos.name
    fetchWeatherNow(pos)
  } catch (e) {
    // 取消或失败，忽略
  }
}

onMounted(async () => {
  document.addEventListener('fullscreenchange', onFullscreenChange)
  document.addEventListener('webkitfullscreenchange', onFullscreenChange)
  onFullscreenChange()
  updateScreenRes()
  now.value = new Date(Date.now() + timeOffset.value)
  syncTime()
  const pos = await getPosition()
  if (pos.name) weatherRegion.value = pos.name
  fetchWeatherNow(pos)
  // 本地每秒走字（基于校准偏移）
  timer = setInterval(() => {
    now.value = new Date(Date.now() + timeOffset.value)
  }, 1000)
  // 标准时间每小时校准一次
  timeTimer = setInterval(syncTime, 60 * 60 * 1000)
  // 天气每 5 分钟刷新一次
  weatherTimer = setInterval(refreshWeather, 5 * 60 * 1000)
})

onUnmounted(() => {
  document.removeEventListener('fullscreenchange', onFullscreenChange)
  document.removeEventListener('webkitfullscreenchange', onFullscreenChange)
  clearInterval(timer)
  clearInterval(timeTimer)
  clearInterval(weatherTimer)
})
</script>

<template>
  <div class="board-screen" @click="onScreenClick">
    <button v-if="!isFullscreen" class="back-btn" @click="goBack" aria-label="返回">←</button>
    <button v-if="!isFullscreen" class="fs-btn" @click="toggleFullscreen" :title="isFullscreen ? '退出全屏' : '全屏'">
      {{ isFullscreen ? '退出' : '全屏' }}
    </button>



    <div class="clock-time">
      <span class="hm">{{ format(now, 'HH:mm') }}</span><span class="sec">:{{ format(now, 'ss') }}</span>
    </div>
    <div class="clock-date">{{ format(now, 'yyyy-MM-dd EEEE', {locale: zhCN}) }}</div>

    <div class="stat-row">
      <div class="stat-card weather">
        <span class="w-region" @click.stop="setCity" title="点击设置城市">{{ weatherRegion || '定位中…' }}</span>
        <span class="w-set" @click.stop="setCity" title="点击设置城市">✎</span>
        <span class="w-temp">{{ weatherTemp != null ? weatherTemp + '°' : '—' }}</span>
        <span class="w-desc">{{ weatherDesc || '天气' }}</span>
        <span class="w-hum" v-if="weatherHumidity != null">湿度 {{ weatherHumidity }}%</span>
      </div>
    </div>



    <div class="rotate-hint">请将手机横屏以获得最佳体验</div>
    <div class="screen-res">{{ screenRes }}</div>
  </div>
</template>

<style scoped>
@import url('https://cdn.jsdelivr.net/npm/dseg@0.46.0/css/dseg.css');

.board-screen {
  position: fixed;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  background: var(--board-bg);
  color: var(--board-fg);
  width: 100vw;
  height: 100vh;
  overflow: hidden;
  user-select: none;
}

.back-btn {
  position: absolute;
  top: 16px;
  left: 16px;
  width: 48px;
  height: 48px;
  border: none;
  border-radius: 50%;
  background: var(--board-surface);
  color: var(--board-fg);
  font-size: var(--board-back-size);
  line-height: 1;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: background 0.2s;
}

.back-btn:hover {
  background: var(--board-surface-hover);
}

.fs-btn {
  position: absolute;
  top: 16px;
  right: 16px;
  height: 48px;
  padding: 0 16px;
  border: none;
  border-radius: 24px;
  background: var(--board-surface);
  color: var(--board-fg);
  font-size: var(--board-btn-size);
  line-height: 1;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: background 0.2s;
}

.fs-btn:hover {
  background: var(--board-surface-hover);
}

.clock-time {
  display: flex;
  align-items: baseline;
  font-family: 'DSEG7 Classic', 'Courier New', monospace;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
  line-height: 1;
  letter-spacing: 0.02em;
  color: var(--board-fg-strong);
  text-shadow: none;
}

.clock-time .hm {
  font-size: min(15.84vw, 31.68vh);
}

.clock-time .sec {
  font-size: min(7.92vw, 15.84vh);
  margin-left: 0.08em;
  opacity: 0.8;
}

.clock-date {
  margin-top: 1.5vh;
  font-size: min(4.5vw, 5.5vh);
  color: var(--board-fg-muted);
}

.stat-row {
  margin-top: 3vh;
  display: flex;
  justify-content: center;
  padding: 0 16px;
}

.stat-card.weather {
  display: flex;
  align-items: baseline;
  gap: 2.5vw;
  flex-wrap: nowrap;
  padding: 2vh 4vw;
  border-radius: 16px;
  background: var(--board-weather-bg);
  white-space: nowrap;
}

.w-region {
  font-size: min(4vw, 5vh);
  color: var(--board-fg-soft);
  cursor: pointer;
  text-decoration: underline dotted var(--board-underline);
  text-underline-offset: 4px;
}

.w-region:hover {
  color: var(--board-fg-bright);
}

.w-set {
  font-size: min(3.4vw, 4vh);
  color: var(--board-fg-faint);
  cursor: pointer;
  margin-left: -1.6vw;
}

.w-set:hover {
  color: var(--board-fg-hover);
}

.w-temp {
  font-size: min(6.75vw, 12vh);
  font-weight: 700;
  font-variant-numeric: tabular-nums;
  line-height: 1;
}

.w-desc {
  font-size: min(4vw, 5vh);
  color: var(--board-fg-hover);
}

.w-hum {
  font-size: min(3.4vw, 4.2vh);
  color: var(--board-fg-mid);
}

.rotate-hint {
  display: none;
}

.screen-res {
  position: absolute;
  bottom: 12px;
  left: 50%;
  transform: translateX(-50%);
  font-size: min(2.8vw, 3.2vh);
  color: var(--board-fg-subtle);
  font-variant-numeric: tabular-nums;
  letter-spacing: 0.05em;
  white-space: nowrap;
}

/* 竖屏时提示用户横屏 */
@media (orientation: portrait) {
  .rotate-hint {
    display: block;
    position: absolute;
    bottom: 24px;
    font-size: 4vw;
    color: var(--board-fg-subtle);
  }
}
</style>

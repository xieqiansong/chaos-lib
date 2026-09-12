<script setup lang="ts">
import {computed, ref, watch} from 'vue'

interface ContributionDay {
  date: string
  count: number
}

interface ContributionItem {
  id: number
  name: string
  total: number
  days: ContributionDay[]
}

interface WeekCell {
  date: string
  count: number
  inRange: boolean
}

const props = defineProps<{
  /** 统计范围说明（根计划名） */
  rootName: string
  /** 可切换的统计项（每项独立统计，不合并） */
  items: ContributionItem[]
}>()

const MONTHS = ['1月', '2月', '3月', '4月', '5月', '6月', '7月', '8月', '9月', '10月', '11月', '12月']
// 行序对齐「周日为一周起点」：0=周日 … 6=周六，仅在周一/三/五显示标签（同 GitHub）
const WEEKDAYS = ['', '周一', '', '周三', '', '周五', '']

const activeId = ref<number | null>(null)

// 首次加载或当前选中项失效时，回退到第一项
watch(() => props.items, (items) => {
  if (items.length > 0 && !items.some(i => i.id === activeId.value)) {
    activeId.value = items[0].id
  }
}, {immediate: true})

const activeItem = computed<ContributionItem | null>(() =>
    props.items.find(i => i.id === activeId.value) ?? props.items[0] ?? null
)

const days = computed(() => activeItem.value?.days ?? [])

const countMap = computed(() => {
  const map = new Map<string, number>()
  for (const d of days.value) map.set(d.date, d.count)
  return map
})

const maxCount = computed(() => days.value.reduce((max, d) => Math.max(max, d.count), 0))

function formatDate(d: Date): string {
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${y}-${m}-${day}`
}

function parseDate(s: string): Date {
  const [y, m, d] = s.split('-').map(Number)
  return new Date(y, m - 1, d)
}

// 把 [起始日, 结束日] 切成整周（每列 7 行，周日 → 周六）
const weeks = computed<WeekCell[][]>(() => {
  if (days.value.length === 0) return []
  const first = parseDate(days.value[0].date)
  const end = parseDate(days.value[days.value.length - 1].date)
  const start = new Date(first)
  start.setDate(start.getDate() - start.getDay())

  const result: WeekCell[][] = []
  const cur = new Date(start)
  while (cur <= end) {
    const week: WeekCell[] = []
    for (let i = 0; i < 7; i++) {
      const key = formatDate(cur)
      week.push({
        date: key,
        count: countMap.value.get(key) ?? 0,
        inRange: cur >= first && cur <= end,
      })
      cur.setDate(cur.getDate() + 1)
    }
    result.push(week)
  }
  return result
})

// 月份标签：对齐到该月首次出现的那个周列
const monthLabels = computed<(string | null)[]>(() => {
  const labels: (string | null)[] = []
  let lastMonth = -1
  for (const week of weeks.value) {
    const anchor = week.find(c => c.inRange && Number(c.date.slice(8)) <= 7)
    if (anchor) {
      const month = Number(anchor.date.slice(5, 7))
      if (month !== lastMonth) {
        labels.push(MONTHS[month - 1])
        lastMonth = month
        continue
      }
    }
    labels.push(null)
  }
  return labels
})

// 5 档色阶：0 为空，1~4 按当日完成数相对最大值等分
function level(count: number): number {
  if (count <= 0) return 0
  const max = maxCount.value || 1
  if (count >= max) return 4
  return Math.max(1, Math.ceil((count / max) * 4))
}
</script>

<template>
  <div class="contrib">
    <div class="contrib-head">
      <el-radio-group v-model="activeId" size="small" class="contrib-tabs">
        <el-radio-button v-for="item in items" :key="item.id" :value="item.id">
          {{ item.name }}
        </el-radio-button>
      </el-radio-group>
      <span class="contrib-total">
        {{ activeItem?.total ?? 0 }} 次完成 · 最近一年
        <span class="contrib-scope">（仅统计「{{ activeItem?.name ?? rootName }}」）</span>
      </span>
    </div>

    <div v-if="!activeItem" class="contrib-empty">暂无可统计数据</div>

    <div v-else class="contrib-body">
      <div class="contrib-weekdays">
        <span v-for="(label, i) in WEEKDAYS" :key="i" class="contrib-weekday">{{ label }}</span>
      </div>

      <div class="contrib-scroll">
        <div class="contrib-months">
          <span
              v-for="(label, i) in monthLabels"
              :key="i"
              class="contrib-month"
          >{{ label || '' }}</span>
        </div>

        <div class="contrib-grid">
          <div v-for="(week, wi) in weeks" :key="wi" class="contrib-week">
            <span
                v-for="cell in week"
                :key="cell.date"
                class="contrib-cell"
                :class="cell.inRange ? `contrib-l${level(cell.count)}` : 'contrib-out'"
                :title="cell.inRange ? `${cell.date} · 完成 ${cell.count} 个` : ''"
            />
          </div>
        </div>
      </div>
    </div>

<!--    <div class="contrib-legend">-->
<!--      <span>少</span>-->
<!--      <span class="contrib-cell contrib-l0"/>-->
<!--      <span class="contrib-cell contrib-l1"/>-->
<!--      <span class="contrib-cell contrib-l2"/>-->
<!--      <span class="contrib-cell contrib-l3"/>-->
<!--      <span class="contrib-cell contrib-l4"/>-->
<!--      <span>多</span>-->
<!--    </div>-->
  </div>
</template>

<style scoped>
.contrib {
  /* 色阶：默认（终端暗色）与浅黄护眼分别给值 */
  --contrib-cell-size: 11px;
  --contrib-gap: 3px;
  --contrib-l0: var(--el-fill-color-light);
  --contrib-l1: rgba(51, 255, 102, 0.16);
  --contrib-l2: rgba(51, 255, 102, 0.36);
  --contrib-l3: rgba(51, 255, 102, 0.62);
  --contrib-l4: var(--term-green);
}

html.paper .contrib {
  --contrib-l1: rgba(122, 138, 69, 0.22);
  --contrib-l2: rgba(122, 138, 69, 0.45);
  --contrib-l3: rgba(122, 138, 69, 0.7);
  --contrib-l4: var(--term-green);
}

.contrib-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: var(--space-md);
  margin-bottom: var(--space-md);
}

.contrib-tabs {
  flex-wrap: wrap;
}

.contrib-total {
  color: var(--el-text-color-primary);
  font-size: var(--font-sm);
}

.contrib-scope {
  color: var(--el-text-color-secondary);
  font-size: var(--font-xs);
}

.contrib-empty {
  height: 6rem;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--el-text-color-secondary);
  font-size: var(--font-sm);
}

.contrib-body {
  display: flex;
  gap: var(--space-sm);
}

.contrib-weekdays {
  display: flex;
  flex-direction: column;
  gap: var(--contrib-gap);
  padding-top: 18px;
  flex-shrink: 0;
}

.contrib-weekday {
  height: var(--contrib-cell-size);
  line-height: var(--contrib-cell-size);
  font-size: 10px;
  color: var(--el-text-color-secondary);
  white-space: nowrap;
}

.contrib-scroll {
  flex: 1;
  min-width: 0;
  overflow-x: auto;
}

.contrib-months {
  display: flex;
  gap: var(--contrib-gap);
  height: 14px;
  margin-bottom: 4px;
}

.contrib-month {
  width: var(--contrib-cell-size);
  flex-shrink: 0;
  overflow: visible;
  white-space: nowrap;
  font-size: 10px;
  line-height: 14px;
  color: var(--el-text-color-secondary);
  text-align: left;
}

.contrib-grid {
  display: flex;
  gap: var(--contrib-gap);
}

.contrib-week {
  display: flex;
  flex-direction: column;
  gap: var(--contrib-gap);
}

.contrib-cell {
  display: block;
  width: var(--contrib-cell-size);
  height: var(--contrib-cell-size);
  border-radius: 2px;
  background: var(--contrib-l0);
}

.contrib-l0 { background: var(--contrib-l0); }
.contrib-l1 { background: var(--contrib-l1); }
.contrib-l2 { background: var(--contrib-l2); }
.contrib-l3 { background: var(--contrib-l3); }
.contrib-l4 { background: var(--contrib-l4); }

/* 区间外的补位格：留空不显示 */
.contrib-out { background: transparent; }

.contrib-legend {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: var(--contrib-gap);
  margin-top: var(--space-sm);
  font-size: var(--font-xs);
  color: var(--el-text-color-secondary);
}
</style>

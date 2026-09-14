// 统计图表通用工具：主题取色 + 数值封顶（避免极端值压扁坐标轴）

export const MAX_VISUAL = 99

export type CappedItem = { visual: number; raw: number }

function cssVar(name: string): string {
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim()
}

/** 从当前主题读取图表用色，随主题切换实时取色，避免游离硬编码 */
export function themeColors() {
  return {
    green: cssVar('--term-green'),
    red: cssVar('--term-red'),
    amber: cssVar('--term-amber'),
    areaTop: cssVar('--term-area-top'),
    areaBottom: cssVar('--term-area-bottom'),
  }
}

export function capValues(counts: number[]): CappedItem[] {
  return counts.map(c => ({visual: Math.min(c, MAX_VISUAL), raw: c}))
}

export function capYAxis() {
  return {
    max: MAX_VISUAL,
    axisLabel: {
      formatter: (v: number) => (v >= MAX_VISUAL ? `${MAX_VISUAL}+` : v),
    },
  }
}

export function capTooltip(prefix: string, capped: CappedItem[]) {
  return (params: any) => {
    const d = params[0]
    const item = capped[d.dataIndex]
    const extra = item && item.raw > MAX_VISUAL ? `（实际: ${item.raw}）` : ''
    return `${d.name}<br/>${d.marker} ${prefix}: ${d.value}${extra}`
  }
}

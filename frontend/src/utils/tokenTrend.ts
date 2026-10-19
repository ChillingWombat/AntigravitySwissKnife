import type { TokenRange, TokenTrendBucket, TokenTrendSegment } from '../types'

/** Unit the histogram is currently rendered in (#129). */
export type TrendUnit = 'usd' | 'tokens' | 'requests'

/** Which segment series is stacked inside every bar. */
export type TrendGroup = 'model' | 'project'

export const TREND_COLORS = [
  '#0b57d0',
  '#137333',
  '#b06000',
  '#7e22ce',
  '#b3261e',
  '#0f766e',
  '#0369a1',
  '#be185d',
  '#c2410c',
  '#475569',
]
export const OTHERS_COLOR = '#9aa0a6'

export interface StackedSegment {
  name: string
  value: number
  color: string
}

export interface TrendBar {
  start: number
  segments: StackedSegment[]
  /** Bar height: the bucket's real aggregated value in the selected unit. */
  total: number
  requests: number
}

export interface TrendBarsResult {
  bars: TrendBar[]
  /** Every entity ranked over the whole window, used by the legend. */
  entities: { name: string; total: number }[]
  /** False when the whole window is empty (chart renders axes + "No data"). */
  hasData: boolean
}

/** Metric of a whole bucket in the active unit. */
export function bucketMetric(b: TokenTrendBucket, unit: TrendUnit): number {
  return unit === 'usd' ? b.cost_usd : unit === 'tokens' ? b.total_tokens : b.requests
}

/** Metric of one stacked segment (model or project) in the active unit. */
export function segmentMetric(s: TokenTrendSegment, unit: TrendUnit): number {
  return unit === 'usd' ? s.cost_usd : unit === 'tokens' ? s.tokens : s.requests
}

/** X-axis / tooltip label for a bucket start timestamp, honouring the range width. */
export function bucketLabel(start: number, range: TokenRange): string {
  const d = new Date(start)
  if (range === '24h') return `${d.getHours()}:00`
  if (range === '7d') return d.toLocaleDateString(undefined, { weekday: 'short' })
  return `${d.getMonth() + 1}/${d.getDate()}`
}

/**
 * Turns the backend's real per-bucket aggregates (#125) into stacked histogram
 * bars: each bucket becomes one bar whose height is the bucket total, split into
 * the top-N entities plus an aggregated "Others" segment.
 *
 * There is deliberately no synthetic interpolation or random fallback: when the
 * window holds no usage, every bar is zero and `hasData` is false so the chart
 * renders empty axes with a "No data in this range" note.
 */
export function buildTrendBars(
  buckets: TokenTrendBucket[],
  opts: { unit: TrendUnit; group: TrendGroup; topN: number }
): TrendBarsResult {
  const { unit, group, topN } = opts

  const totals = new Map<string, number>()
  for (const b of buckets) {
    const segs = group === 'model' ? b.models : b.projects
    for (const s of segs) {
      totals.set(s.name, (totals.get(s.name) || 0) + segmentMetric(s, unit))
    }
  }
  const entities = Array.from(totals.entries())
    .map(([name, total]) => ({ name, total }))
    .sort((a, b) => b.total - a.total)

  const topEntityNames = entities.slice(0, topN).map((e) => e.name)
  const wrapOthers = entities.length > topN
  const othersNames = new Set(entities.slice(topN).map((e) => e.name))

  const bars = buckets.map((b) => {
    const segs = group === 'model' ? b.models : b.projects
    const byName = new Map<string, number>()
    for (const s of segs) byName.set(s.name, segmentMetric(s, unit))

    const stacked: StackedSegment[] = topEntityNames.map((name, i) => ({
      name,
      value: byName.get(name) || 0,
      color: TREND_COLORS[i % TREND_COLORS.length],
    }))

    if (wrapOthers) {
      let others = 0
      for (const [name, value] of byName.entries()) {
        if (othersNames.has(name)) others += value
      }
      stacked.push({ name: 'Others', value: others, color: OTHERS_COLOR })
    }

    return {
      start: b.start,
      segments: stacked,
      total: bucketMetric(b, unit),
      requests: b.requests,
    }
  })

  return { bars, entities, hasData: bars.some((b) => b.total > 0) }
}

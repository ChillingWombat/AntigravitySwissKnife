/**
 * tokenTrend.test.ts — #125/#129 histogram aggregation tests
 *
 * Locks the contract of the stacked histogram: bar height = real bucket total
 * from the backend window, per-entity segments + aggregated "Others", the
 * Requests unit, and the anti-Math.sin guarantee that an empty window yields
 * an empty chart (hasData === false) instead of a synthesised curve.
 */

import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import type { TokenTrendBucket, TokenTrendSegment } from '../../types'
import { buildTrendBars, bucketLabel, bucketMetric, segmentMetric } from '../tokenTrend'

function seg(name: string, tokens: number, cost: number, requests: number): TokenTrendSegment {
  return { name, tokens, cost_usd: cost, requests }
}

function bucket(
  start: number,
  fresh: number,
  cached: number,
  output: number,
  requests: number,
  models: TokenTrendSegment[],
  projects: TokenTrendSegment[]
): TokenTrendBucket {
  return {
    start,
    fresh_tokens: fresh,
    cached_tokens: cached,
    output_tokens: output,
    total_tokens: fresh + cached + output,
    cost_usd: models.reduce((a, m) => a + m.cost_usd, 0),
    requests,
    models,
    projects,
  }
}

// Two real hourly buckets: 10:00 has GPT-4o only, 11:00 stacks GPT-4o + Claude.
const FIXTURE: TokenTrendBucket[] = [
  bucket(1000, 600, 300, 100, 4, [seg('GPT-4o', 1000, 0.5, 4)], [seg('proj-a', 1000, 0.5, 4)]),
  bucket(2000, 400, 100, 50, 3, [seg('GPT-4o', 300, 0.15, 1), seg('Claude 3.7 Sonnet', 250, 0.35, 2)], [
    seg('proj-a', 150, 0.05, 1),
    seg('proj-b', 400, 0.45, 2),
  ]),
]

describe('buildTrendBars — stacked histogram (#125)', () => {
  it('builds one bar per real bucket whose height equals the bucket total', () => {
    const { bars, hasData } = buildTrendBars(FIXTURE, { unit: 'tokens', group: 'model', topN: 5 })
    assert.equal(bars.length, 2, 'one bar per backend bucket')
    assert.equal(bars[0].total, 1000)
    assert.equal(bars[1].total, 550)
    assert.equal(hasData, true)
  })

  it('stacks per-entity segments that sum to the bar total', () => {
    const { bars } = buildTrendBars(FIXTURE, { unit: 'tokens', group: 'model', topN: 5 })
    const second = bars[1]
    const sums = second.segments.reduce((a, s) => a + s.value, 0)
    assert.equal(sums, second.total, 'segments must reconstruct the bar total')
    const gpt = second.segments.find((s) => s.name === 'GPT-4o')
    const claude = second.segments.find((s) => s.name === 'Claude 3.7 Sonnet')
    assert.equal(gpt?.value, 300)
    assert.equal(claude?.value, 250)
    assert.notEqual(gpt?.color, claude?.color, 'stacked segments must be distinctly coloured')
  })

  it('ranks entities over the whole window for the legend', () => {
    const { entities } = buildTrendBars(FIXTURE, { unit: 'tokens', group: 'model', topN: 5 })
    assert.deepEqual(
      entities.map((e) => e.name),
      ['GPT-4o', 'Claude 3.7 Sonnet'],
      'entities must be ranked by total usage across the window'
    )
    assert.equal(entities[0].total, 1000 + 300)
  })

  it('collapses the tail into an aggregated Others segment when Top-N is smaller', () => {
    const { bars, entities } = buildTrendBars(FIXTURE, { unit: 'tokens', group: 'model', topN: 1 })
    assert.equal(entities.length, 2)
    const second = bars[1]
    assert.deepEqual(
      second.segments.map((s) => s.name),
      ['GPT-4o', 'Others'],
      'Top-1 keeps the leader plus one Others segment'
    )
    assert.equal(second.segments[1].value, 250, 'Others carries the remaining entities')
    assert.equal(
      second.segments.reduce((a, s) => a + s.value, 0),
      second.total,
      'Others must keep the stack equal to the bar total'
    )
  })

  it('honours the Requests unit (#129) using the same real records', () => {
    const { bars } = buildTrendBars(FIXTURE, { unit: 'requests', group: 'model', topN: 5 })
    assert.equal(bars[0].total, 4)
    assert.equal(bars[1].total, 3)
    const { bars: projectBars } = buildTrendBars(FIXTURE, { unit: 'requests', group: 'project', topN: 5 })
    assert.equal(projectBars[1].total, 3)
    assert.equal(projectBars[1].segments.find((s) => s.name === 'proj-b')?.value, 2)
  })

  it('reports an empty chart (no synthetic curve) when the window has no data', () => {
    // 24 empty hourly buckets: exactly what the backend returns for an empty
    // window (previous Math.sin fallback fabricated values here — must not).
    const empty: TokenTrendBucket[] = Array.from({ length: 24 }, (_, i) => bucket(i * 3600_000, 0, 0, 0, 0, [], []))
    const { bars, entities, hasData } = buildTrendBars(empty, { unit: 'tokens', group: 'model', topN: 5 })

    assert.equal(bars.length, 24, 'axes keep one zero-height bar per bucket')
    assert.equal(hasData, false, 'empty window must render the "No data in this range" state')
    assert.equal(entities.length, 0, 'no fabricated entities')
    for (const b of bars) {
      assert.equal(b.total, 0)
      assert.deepEqual(b.segments, [], 'no zero-value or invented segments')
    }
  })

  it('keeps real data visible when only part of the window has usage', () => {
    const partial: TokenTrendBucket[] = [
      bucket(0, 0, 0, 0, 0, [], []),
      bucket(3600_000, 100, 0, 10, 1, [seg('GPT-4o', 110, 0.1, 1)], [seg('proj-a', 110, 0.1, 1)]),
    ]
    const { bars, hasData } = buildTrendBars(partial, { unit: 'tokens', group: 'model', topN: 5 })
    assert.equal(hasData, true)
    assert.equal(bars[0].total, 0)
    assert.equal(bars[1].total, 110)
  })
})

describe('bucket/segment metrics and labels', () => {
  it('maps each unit to its bucket metric', () => {
    const b = FIXTURE[0]
    assert.equal(bucketMetric(b, 'usd'), b.cost_usd)
    assert.equal(bucketMetric(b, 'tokens'), b.total_tokens)
    assert.equal(bucketMetric(b, 'requests'), b.requests)
  })

  it('maps each unit to its segment metric', () => {
    const s = seg('GPT-4o', 100, 1.5, 7)
    assert.equal(segmentMetric(s, 'usd'), 1.5)
    assert.equal(segmentMetric(s, 'tokens'), 100)
    assert.equal(segmentMetric(s, 'requests'), 7)
  })

  it('formats hour labels for 24h and calendar dates otherwise', () => {
    const start = new Date(2026, 9, 19, 14, 0, 0).getTime()
    assert.equal(bucketLabel(start, '24h'), '14:00')
    assert.equal(bucketLabel(start, '30d'), '10/19')
    assert.equal(bucketLabel(start, 'all'), '10/19')
    assert.ok(bucketLabel(start, '7d').length > 0, '7d labels must be non-empty weekday names')
  })
})

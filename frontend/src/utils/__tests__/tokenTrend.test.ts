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
import { buildTrendBars, bucketLabel, bucketMetric, segmentMetric, generate24HourlyBuckets } from '../tokenTrend'

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

describe('generate24HourlyBuckets — 24H hourly granularity (#125)', () => {
  it('generates exactly 24 hourly buckets spaced 1 hour apart', () => {
    const now = new Date(2026, 9, 25, 15, 30, 0).getTime()
    const buckets = generate24HourlyBuckets(now)
    assert.equal(buckets.length, 24, 'must have exactly 24 buckets')

    // Verify spacing is 1 hour between each bucket
    for (let i = 1; i < buckets.length; i++) {
      assert.equal(buckets[i].start - buckets[i - 1].start, 3600_000, 'buckets must be 1 hour apart')
    }

    // Last bucket should be the current clock hour (15:00)
    const lastDate = new Date(buckets[23].start)
    assert.equal(lastDate.getHours(), 15, 'last bucket should be 15:00')

    // First bucket should be 23 hours prior (16:00 yesterday)
    const firstDate = new Date(buckets[0].start)
    assert.equal(firstDate.getHours(), 16, 'first bucket should be 16:00 yesterday')
  })

  it('correctly attributes events to corresponding hourly buckets and stacks models/projects', () => {
    const now = new Date(2026, 9, 25, 15, 30, 0).getTime()
    const currentHour = Math.floor(now / 3600_000) * 3600_000

    const events = [
      {
        timestamp: new Date(currentHour + 10 * 60_000).toISOString(),
        model: 'gemini-3-pro',
        project: 'project-alpha',
        input_tokens: 1000,
        cached_tokens: 200,
        output_tokens: 300,
        cost_usd: 0.05,
      },
      {
        timestamp: new Date(currentHour - 2 * 3600_000 + 15 * 60_000).toISOString(),
        model: 'claude-3-7-sonnet',
        project: 'project-beta',
        input_tokens: 2000,
        cached_tokens: 500,
        output_tokens: 400,
        cost_usd: 0.12,
      },
    ]

    const buckets = generate24HourlyBuckets(now, events)
    assert.equal(buckets.length, 24)

    // Current hour bucket (index 23)
    const lastBucket = buckets[23]
    assert.equal(lastBucket.requests, 1)
    assert.equal(lastBucket.total_tokens, 800 + 200 + 300) // fresh (1000-200) + cached (200) + out (300) = 1300
    assert.equal(lastBucket.models[0].name, 'gemini-3-pro')
    assert.equal(lastBucket.projects[0].name, 'project-alpha')

    // Two hours ago (index 21)
    const twoHoursAgo = buckets[21]
    assert.equal(twoHoursAgo.requests, 1)
    assert.equal(twoHoursAgo.models[0].name, 'claude-3-7-sonnet')
    assert.equal(twoHoursAgo.projects[0].name, 'project-beta')

    // Build trend bars with model grouping
    const { bars: modelBars } = buildTrendBars(buckets, { unit: 'tokens', group: 'model', topN: 5 })
    assert.equal(modelBars.length, 24)
    assert.equal(modelBars[23].total, 1300)
    assert.equal(modelBars[21].total, 1500 + 500 + 400) // fresh (1500) + cached (500) + out (400) = 2400

    // Build trend bars with project grouping
    const { bars: projectBars } = buildTrendBars(buckets, { unit: 'tokens', group: 'project', topN: 5 })
    assert.equal(projectBars.length, 24)
    const alphaSeg = projectBars[23].segments.find((s) => s.name === 'project-alpha')
    assert.equal(alphaSeg?.value, 1300)
    const betaSeg = projectBars[21].segments.find((s) => s.name === 'project-beta')
    assert.equal(betaSeg?.value, 2400)
  })
})

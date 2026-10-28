import { describe, it, afterEach } from 'node:test'
import assert from 'node:assert/strict'
import React, { act } from 'react'
import { initDOM, mount, type RenderResult } from '../test-dom'
import { TokenMonitorPage } from '../pages/TokenMonitorPage'
import { cancelAllInFlightRequests } from '../api'

const OVERVIEW_FIXTURE = {
  total_tokens: 25000000,
  input_tokens: 20000000,
  cached_input_tokens: 15000000,
  output_tokens: 5000000,
  total_cost_usd: 12.5,
  saved_cost_usd: 4.2,
  avg_tps: 53.3,
  requests_count: 8220,
  unpriced_models: [],
  model_breakdowns: [
    {
      model_id: 'gemini-3-pro',
      canonical_id: 'gemini-3-pro',
      model_name: 'Gemini 3 Pro',
      name: 'Gemini 3 Pro',
      provider: 'google',
      classification: 'native',
      total_tokens: 18000000,
      input_tokens: 15000000,
      cached_tokens: 12000000,
      output_tokens: 3000000,
      cost_usd: 8.5,
      avg_tps: 53.3,
      requests: 6100,
      percentage: 72.0,
      price_configured: true,
    },
    {
      model_id: 'claude-3-7-sonnet',
      canonical_id: 'claude-3-7-sonnet',
      model_name: 'Claude 3.7 Sonnet',
      name: 'Claude 3.7 Sonnet',
      provider: 'anthropic',
      classification: 'custom',
      total_tokens: 7000000,
      input_tokens: 5000000,
      cached_tokens: 3000000,
      output_tokens: 2000000,
      cost_usd: 4.0,
      avg_tps: 42.1,
      requests: 2120,
      percentage: 28.0,
      price_configured: true,
    },
  ],
  project_breakdowns: [
    {
      project_name: 'Swiss Knife',
      workspace_path: '/path/to/swiss-knife',
      total_tokens: 20000000,
      input_tokens: 16000000,
      cached_tokens: 15344000,
      output_tokens: 4000000,
      cost_usd: 10.0,
      saved_cost_usd: 3.5,
      cache_hit_ratio: 95.9,
      requests: 6500,
      last_active: 'Just now',
    },
  ],
  telemetry_events: [],
  trend: {
    bucket_ms: 3600000,
    buckets: Array.from({ length: 24 }, (_, i) => ({
      start: Date.now() - (23 - i) * 3600000,
      fresh_tokens: 1000,
      cached_tokens: 2000,
      output_tokens: 500,
      total_tokens: 3500,
      cost_usd: 0.1,
      requests: 5,
      models: [{ name: 'Gemini 3 Pro', tokens: 3500, cost_usd: 0.1, requests: 5 }],
      projects: [{ name: 'Swiss Knife', tokens: 3500, cost_usd: 0.1, requests: 5 }],
    })),
  },
}

describe('TokenMonitorPage: Hover Details & 24H Hourly Trend', () => {
  const originalFetch = globalThis.fetch
  let activeView: RenderResult | null = null

  afterEach(() => {
    activeView?.unmount()
    activeView = null
    globalThis.fetch = originalFetch
    cancelAllInFlightRequests()
  })

  it('removes permanently visible metrics below progress bars and provides them via hover/title', async () => {
    initDOM()
    globalThis.fetch = (async (url: any) => {
      const target = String(url)
      const payload = target.includes('/api/tokens/summary') ? OVERVIEW_FIXTURE : {}
      return { ok: true, status: 200, json: async () => payload } as any
    }) as any

    activeView = await mount(React.createElement(TokenMonitorPage, { activeTab: 0 }))
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 50))
    })

    const bodyText = activeView.container.textContent || ''

    // 1. Permanently visible lines below progress bars must NOT be in the regular text flow
    assert.equal(
      bodyText.includes('Speed: 53.3 TPS'),
      false,
      'Speed metrics should not be permanently visible below progress bars'
    )
    assert.equal(
      bodyText.includes('Cache hit: 95.9%'),
      false,
      'Cache hit metrics should not be permanently visible below progress bars'
    )

    // 2. Instead, detailed metrics must be accessible via hover / title attributes
    const elementsWithTitle = activeView.querySelectorAll('[title]')
    const titles = elementsWithTitle.map((el) => el.getAttribute('title') || '')

    const modelTitle = titles.find((t) => t.includes('Speed: 53.3 TPS'))
    assert.ok(modelTitle, 'Model row must have hover title with Speed details')
    assert.ok(modelTitle?.includes('6100 requests'), 'Model hover title must include requests count')
    assert.ok(modelTitle?.includes('cached'), 'Model hover title must include cached token details')

    const projectTitle = titles.find((t) => t.includes('Cache hit: 95.9%'))
    assert.ok(projectTitle, 'Project row must have hover title with Cache hit ratio')
    assert.ok(projectTitle?.includes('Active Just now'), 'Project hover title must include active timestamp')
  })

  it('renders 24 hourly bars when 24H view is selected', async () => {
    initDOM()
    globalThis.fetch = (async (url: any) => {
      const target = String(url)
      const payload = target.includes('/api/tokens/summary') ? OVERVIEW_FIXTURE : {}
      return { ok: true, status: 200, json: async () => payload } as any
    }) as any

    activeView = await mount(React.createElement(TokenMonitorPage, { activeTab: 0 }))
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 50))
    })

    // Click 24H timeRange button
    const buttons = activeView.querySelectorAll('button')
    const btn24h = buttons.find((b) => b.textContent?.trim() === '24H' || b.textContent?.trim() === '24h')
    assert.ok(btn24h, '24H filter button should exist')

    await act(async () => {
      btn24h.dispatchEvent(new MouseEvent('click', { bubbles: true }))
      await new Promise((resolve) => setTimeout(resolve, 50))
    })

    // Inspect SVG stacked histogram bars
    const svg = activeView.querySelector('svg[data-testid="token-trend-svg"]')
    assert.ok(svg, 'Trend SVG should exist')

    // Find all bar groups (<g> containing the hover rect and segment rects)
    const barGroups = Array.from(svg.querySelectorAll('g'))
    assert.equal(barGroups.length, 24, '24H view must render exactly 24 hourly bars')
  })
})

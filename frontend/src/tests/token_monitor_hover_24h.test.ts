import { describe, it, afterEach } from 'node:test'
import assert from 'node:assert/strict'
import React, { act } from 'react'
import { initDOM, mount, type RenderResult } from '../test-dom'
import { TokenMonitorPage, formatHoverCost, formatCount } from '../pages/TokenMonitorPage'
import { fireEvent } from '@testing-library/react'
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
      avg_tps: 53.3,
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

  it('removes the duplicate native title tooltip and renders the built-in metrics window on hover', async () => {
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

    // 2. Model and Project rows must NOT have a native title tooltip (second explain text field eliminated)
    const modelRow = activeView.querySelector('[data-testid="model-row-gemini-3-pro"]')
    assert.ok(modelRow, 'Model row must exist')
    assert.equal(modelRow?.getAttribute('title'), null, 'Model row must not have title attribute')

    const projectRow = activeView.querySelector('[data-testid="project-row-Swiss Knife"]')
    assert.ok(projectRow, 'Project row must exist')
    assert.equal(projectRow?.getAttribute('title'), null, 'Project row must not have title attribute')

    // 3. Hovering Model row displays built-in little window with: Input Tokens, Cache Hit Ratio, Output Tokens, Requests, Cost
    await act(async () => {
      if (modelRow) fireEvent.mouseEnter(modelRow)
      await new Promise((resolve) => setTimeout(resolve, 50))
    })

    const modelHover = activeView.querySelector('[data-testid="model-hover-metrics"]')
    assert.ok(modelHover, 'Model hover metrics window must appear on mouseEnter')
    const modelHoverText = modelHover?.textContent || ''

    assert.ok(modelHoverText.includes('Input Tokens: 15.00M'), 'Model hover must show Input Tokens')
    assert.ok(modelHoverText.includes('Cache Hit Ratio: 80.0%'), 'Model hover must show Cache Hit Ratio')
    assert.ok(modelHoverText.includes('Output Tokens: 3.00M'), 'Model hover must show Output Tokens')
    assert.ok(modelHoverText.includes('Requests: 6.10k'), 'Model hover must show Requests count')
    assert.ok(modelHoverText.includes('Cost: $8.50'), 'Model hover must show Cost')

    // Mouse leave model row
    await act(async () => {
      if (modelRow) fireEvent.mouseLeave(modelRow)
      await new Promise((resolve) => setTimeout(resolve, 20))
    })

    // 4. Hovering Project row displays built-in little window with: Input Tokens, Cache Hit Ratio, Output Tokens, Requests, Cost, Speed, Input/Output Ratio
    await act(async () => {
      if (projectRow) fireEvent.mouseEnter(projectRow)
      await new Promise((resolve) => setTimeout(resolve, 50))
    })

    const projectHover = activeView.querySelector('[data-testid="project-hover-metrics"]')
    assert.ok(projectHover, 'Project hover metrics window must appear on mouseEnter')
    const projectHoverText = projectHover?.textContent || ''

    assert.ok(projectHoverText.includes('Input Tokens: 16.00M'), 'Project hover must show Input Tokens')
    assert.ok(projectHoverText.includes('Cache Hit Ratio: 95.9%'), 'Project hover must show Cache Hit Ratio')
    assert.ok(projectHoverText.includes('Output Tokens: 4.00M'), 'Project hover must show Output Tokens')
    assert.ok(projectHoverText.includes('Requests: 6.50k'), 'Project hover must show Requests count')
    assert.ok(projectHoverText.includes('Cost: $10.00'), 'Project hover must show Cost')
    assert.ok(projectHoverText.includes('Speed: 53.3 TPS'), 'Project hover must show Speed')
    assert.ok(projectHoverText.includes('Input/Output Ratio: 4.0x'), 'Project hover must show Input/Output Ratio')
  })

  it('formatHoverCost formats costs, sub-cents, and unpriced models correctly', () => {
    assert.equal(formatHoverCost(null), 'Unpriced')
    assert.equal(formatHoverCost(undefined), 'Unpriced')
    assert.equal(formatHoverCost(12.5, true), 'Unpriced')
    assert.equal(formatHoverCost(0), '$0.00')
    assert.equal(formatHoverCost(0.0042), '$0.0042')
    assert.equal(formatHoverCost(5.39), '$5.39')
    assert.equal(formatHoverCost(184.4), '$184.40')
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

  it("formats requests and completed chats in unit k and M with 2 decimal places once >= 1000", async () => {
    initDOM()
    globalThis.fetch = (async (url: any) => {
      const target = String(url)
      const payload = target.includes("/api/tokens/summary") ? OVERVIEW_FIXTURE : {}
      return { ok: true, status: 200, json: async () => payload } as any
    }) as any

    activeView = await mount(React.createElement(TokenMonitorPage, { activeTab: 0 }))
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 50))
    })

    const bodyText = activeView.container.textContent || ""

    // Completed Chat Turns card (Card 5) displays 8220 formatted as 8.22k
    assert.ok(bodyText.includes("8.22k"), "Card 5 must format 8220 requests as 8.22k")

    // Click Requests unitMode button
    const buttons = activeView.querySelectorAll("button")
    const btnRequests = buttons.find((b) => b.textContent?.trim() === "Requests")
    assert.ok(btnRequests, "Requests mode button should exist")

    await act(async () => {
      btnRequests.dispatchEvent(new MouseEvent("click", { bubbles: true }))
      await new Promise((resolve) => setTimeout(resolve, 50))
    })

    const updatedText = activeView.container.textContent || ""
    assert.ok(updatedText.includes("Total Requests"), "Header must update to Total Requests")
  })

  it("formatCount formats units k, M, B with 2 decimal places once >= 1000", () => {
    assert.equal(formatCount(0), "0")
    assert.equal(formatCount(null), "0")
    assert.equal(formatCount(undefined), "0")
    assert.equal(formatCount(500), "500")
    assert.equal(formatCount(1000), "1.00k")
    assert.equal(formatCount(6100), "6.10k")
    assert.equal(formatCount(8220), "8.22k")
    assert.equal(formatCount(10740), "10.74k")
    assert.equal(formatCount(1500000), "1.50M")
    assert.equal(formatCount(1000000000), "1.00B")
  })

  it("renders Native, Custom, All filter switcher with Native as default and filters models", async () => {
    initDOM()
    globalThis.fetch = (async (url: any) => {
      const target = String(url)
      const payload = target.includes("/api/tokens/summary") ? OVERVIEW_FIXTURE : {}
      return { ok: true, status: 200, json: async () => payload } as any
    }) as any

    activeView = await mount(React.createElement(TokenMonitorPage, { activeTab: 0 }))
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 50))
    })

    // Filter buttons exist
    const btnNative = activeView.querySelector('[data-testid="model-filter-native"]')
    const btnCustom = activeView.querySelector('[data-testid="model-filter-custom"]')
    const btnAll = activeView.querySelector('[data-testid="model-filter-all"]')

    assert.ok(btnNative, "Native filter button must exist")
    assert.ok(btnCustom, "Custom filter button must exist")
    assert.ok(btnAll, "All filter button must exist")

    // Default is Native: native model is displayed, custom model is filtered out
    let nativeModel = activeView.querySelector('[data-testid="model-row-gemini-3-pro"]')
    let customModel = activeView.querySelector('[data-testid="model-row-claude-3-7-sonnet"]')
    assert.ok(nativeModel, "Native model must be visible by default")
    assert.equal(customModel, null, "Custom model must be hidden under Native filter")

    // Switch to Custom
    await act(async () => {
      btnCustom.dispatchEvent(new MouseEvent("click", { bubbles: true }))
      await new Promise((resolve) => setTimeout(resolve, 50))
    })

    nativeModel = activeView.querySelector('[data-testid="model-row-gemini-3-pro"]')
    customModel = activeView.querySelector('[data-testid="model-row-claude-3-7-sonnet"]')
    assert.equal(nativeModel, null, "Native model must be hidden under Custom filter")
    assert.ok(customModel, "Custom model must be visible under Custom filter")

    // Switch to All
    await act(async () => {
      btnAll.dispatchEvent(new MouseEvent("click", { bubbles: true }))
      await new Promise((resolve) => setTimeout(resolve, 50))
    })

    nativeModel = activeView.querySelector('[data-testid="model-row-gemini-3-pro"]')
    customModel = activeView.querySelector('[data-testid="model-row-claude-3-7-sonnet"]')
    assert.ok(nativeModel, "Native model must be visible under All filter")
    assert.ok(customModel, "Custom model must be visible under All filter")
  })
})

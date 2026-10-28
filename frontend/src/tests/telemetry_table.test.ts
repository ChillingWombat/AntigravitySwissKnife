/**
 * telemetry_table.test.ts — Verifies the "Recent Agent Session Telemetry Log" table layout.
 *
 * User-reported defect: the telemetry log table exposed a dedicated "Type" column
 * (Native / Custom) that cluttered the audit view. The classification must instead be
 * surfaced as a tooltip on the Model cell, and the empty-state colSpan must match the
 * reduced column count.
 */

import { describe, it, afterEach } from 'node:test'
import assert from 'node:assert/strict'
import React, { act } from 'react'
import { mount, initDOM } from '../test-dom'
import TokenMonitorPage from '../pages/TokenMonitorPage'
import { cancelAllInFlightRequests } from '../api'

const TELEMETRY_FIXTURE = {
  total_tokens: 12000,
  input_tokens: 9000,
  cached_input_tokens: 3000,
  output_tokens: 3000,
  total_cost_usd: 0.12,
  saved_cost_usd: 0.01,
  avg_tps: 42,
  requests_count: 2,
  unpriced_models: [],
  model_breakdowns: [],
  project_breakdowns: [],
  pricing_records: [],
  telemetry_events: [
    {
      id: 'turn-1',
      timestamp: '2026-01-01 10:00:00',
      project: 'swiss-knife',
      canonical_id: 'gemini-3-pro',
      model: 'Gemini 3 Pro',
      classification: 'native',
      input_tokens: 1000,
      cached_tokens: 200,
      output_tokens: 300,
      duration_ms: 1500,
      tps: 45,
      cost_usd: 0.02,
      agent_role: 'Main Agent',
    },
    {
      id: 'turn-2',
      timestamp: '2026-01-01 10:05:00',
      project: 'swiss-knife',
      canonical_id: 'custom-local-llm',
      model: 'Custom Local LLM',
      classification: 'custom',
      input_tokens: 800,
      cached_tokens: 0,
      output_tokens: 120,
      duration_ms: 2200,
      tps: 30,
      cost_usd: null,
      is_subagent: true,
      agent_role: 'Sub-Agent',
    },
  ],
}

const EMPTY_TELEMETRY_FIXTURE = { ...TELEMETRY_FIXTURE, telemetry_events: [] }

const UNPRICED_FIXTURE = {
  ...TELEMETRY_FIXTURE,
  telemetry_events: [],
  pricing_records: [
    {
      internal_id: 1,
      canonical_id: 'unpriced-model',
      model_id: 'unpriced-model',
      model_name: 'Unpriced Model',
      name: 'Unpriced Model',
      provider: 'custom',
      classification: 'custom',
      input_price_per_m: null,
      cached_input_price_per_m: null,
      output_price_per_m: null,
      source: 'unconfigured',
      updated_at: '2026-01-01',
    },
  ],
}

const stubFetch = (summaryFixture: unknown) => {
  globalThis.fetch = (async (url: any) => {
    const target = String(url)
    const payload = target.includes('/api/tokens/summary') ? summaryFixture : {}
    return {
      ok: true,
      status: 200,
      json: async () => payload,
    } as any
  }) as any
}

const mountPage = async (summaryFixture: unknown, tab = 1) => {
  stubFetch(summaryFixture)
  // Tab 1 hosts the telemetry stream log; tab 2 hosts the pricing registry table.
  const rendered = await mount(React.createElement(TokenMonitorPage, { activeTab: tab }))
  // Let the initial useEffect fetch resolve.
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 20))
  })
  return rendered
}

/** Finds the telemetry log table (unique by its "Time" + "Tokens (In / Cache / Out)" header combination). */
const findTelemetryTable = (querySelectorAll: (sel: string) => Element[]) => {
  const tables = querySelectorAll('table')
  return tables.find((table) => {
    const headers = Array.from(table.querySelectorAll('thead th')).map((th) => th.textContent?.trim())
    return headers.includes('Time') && headers.includes('Tokens (In / Cache / Out)')
  }) as HTMLTableElement | undefined
}

describe('Telemetry log table: Type column removal', () => {
  const originalFetch = globalThis.fetch

  afterEach(() => {
    globalThis.fetch = originalFetch
    cancelAllInFlightRequests()
  })

  it('renders exactly 7 columns and no dedicated Type or Status header', async () => {
    initDOM()
    const { querySelectorAll, unmount } = await mountPage(TELEMETRY_FIXTURE)

    try {
      const table = findTelemetryTable(querySelectorAll)
      assert.ok(table, 'Telemetry log table must be rendered')

      const headers = Array.from(table!.querySelectorAll('thead th')).map((th) => th.textContent?.trim())
      assert.equal(headers.length, 7, `Expected 7 telemetry columns, got ${headers.length}: ${headers.join(', ')}`)
      assert.ok(!headers.includes('Type'), 'The dedicated Type column must be removed')
      assert.ok(!headers.includes('Status'), 'The Status column must be removed')
      assert.ok(headers.includes('Model'), 'Model column must remain')
    } finally {
      unmount()
    }
  })

  it('surfaces the classification as a tooltip on the Model cell instead of a column', async () => {
    initDOM()
    const { querySelectorAll, unmount } = await mountPage(TELEMETRY_FIXTURE)

    try {
      const table = findTelemetryTable(querySelectorAll)
      assert.ok(table, 'Telemetry log table must be rendered')

      const rows = Array.from(table!.querySelectorAll('tbody tr'))
      assert.equal(rows.length, 2, 'Both telemetry events must render as rows')

      const modelTitles = rows.map((row) => {
        const cells = Array.from(row.querySelectorAll('td'))
        // Manually select the Model cell (4th column) and find its tooltip span.
        const modelCell = cells[3]
        const span = modelCell?.querySelector('span[title]')
        return span?.getAttribute('title') || ''
      })

      assert.ok(
        modelTitles[0].includes('Gemini 3 Pro') && modelTitles[0].includes('Native'),
        `Native classification must appear in the Model tooltip, got: "${modelTitles[0]}"`
      )
      assert.ok(
        modelTitles[1].includes('Custom Local LLM') && modelTitles[1].includes('Custom'),
        `Custom classification must appear in the Model tooltip, got: "${modelTitles[1]}"`
      )

      // No standalone "Native"/"Custom" classification badge text in the row body.
      const rowText = rows[0].textContent || ''
      assert.ok(!/(^|[^A-Za-z])Native([^A-Za-z]|$)/.test(rowText), 'Classification must not render as standalone cell text')
    } finally {
      unmount()
    }
  })

  it('empty state colSpan matches the reduced 7-column layout', async () => {
    initDOM()
    const { querySelectorAll, unmount } = await mountPage(EMPTY_TELEMETRY_FIXTURE)

    try {
      const table = findTelemetryTable(querySelectorAll)
      assert.ok(table, 'Telemetry log table must be rendered even when empty')

      const emptyCell = table!.querySelector('tbody td[colspan]')
      assert.ok(emptyCell, 'Empty-state cell must exist')
      assert.equal(emptyCell!.getAttribute('colspan'), '7', 'Empty-state colSpan must be 7')
    } finally {
      unmount()
    }
  })
})

describe('Pricing registry table: empty price cells render an em dash', () => {
  const originalFetch = globalThis.fetch

  afterEach(() => {
    globalThis.fetch = originalFetch
    cancelAllInFlightRequests()
  })

  /** Finds the pricing registry table (unique by its $/1M headers). */
  const findPricingTable = (querySelectorAll: (sel: string) => Element[]) => {
    const tables = querySelectorAll('table')
    return tables.find((table) => {
      const headers = Array.from(table.querySelectorAll('thead th')).map((th) => th.textContent?.trim())
      return headers.includes('Prompt Input ($/1M)') && headers.includes('Cached Input ($/1M)')
    }) as HTMLTableElement | undefined
  }

  it('renders "—" for null input/cached/output prices and for an unconfigured source', async () => {
    initDOM()
    const { querySelectorAll, unmount } = await mountPage(UNPRICED_FIXTURE, 2)

    try {
      const table = findPricingTable(querySelectorAll)
      assert.ok(table, 'Pricing registry table must be rendered')

      const rows = Array.from(table!.querySelectorAll('tbody tr'))
      assert.equal(rows.length, 1, 'The single pricing record must render one row')

      const cells = Array.from(rows[0].querySelectorAll('td'))
      // Columns: Model, Type, Provider, Prompt Input, Cached Input, Output, Source, Action.
      assert.equal(cells[3]?.textContent?.trim(), '—', 'Null prompt-input price must render an em dash')
      assert.equal(cells[4]?.textContent?.trim(), '—', 'Null cached-input price must render an em dash')
      assert.equal(cells[5]?.textContent?.trim(), '—', 'Null output price must render an em dash')
      assert.equal(cells[6]?.textContent?.trim(), '—', 'Unconfigured source must render an em dash')

      const rowText = rows[0].textContent || ''
      assert.ok(!rowText.includes('Null —'), 'Legacy "Null — …" labels must not be rendered')
      assert.ok(!rowText.includes('Manual Required'), 'Legacy "Manual Required" text must not be rendered')
      assert.ok(!rowText.toLowerCase().includes('manual entry'), 'Legacy "Manual Entry" text must not be rendered')
    } finally {
      unmount()
    }
  })
})

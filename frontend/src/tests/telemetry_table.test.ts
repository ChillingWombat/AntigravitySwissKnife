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
import TokenMonitorPage, { formatPricingRateHelper } from '../pages/TokenMonitorPage'
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
      routed_model: 'gemini-3.8-flash-n',
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
    return headers.includes('Time') && headers.includes('Project') && headers.includes('Input')
  }) as HTMLTableElement | undefined
}

describe('Telemetry log table: Type column removal', () => {
  const originalFetch = globalThis.fetch

  afterEach(() => {
    globalThis.fetch = originalFetch
    cancelAllInFlightRequests()
  })

  it('renders exactly 9 columns with Project, Input, Output, Duration and no dedicated Type or Status header', async () => {
    initDOM()
    const { querySelectorAll, unmount } = await mountPage(TELEMETRY_FIXTURE)

    try {
      const table = findTelemetryTable(querySelectorAll)
      assert.ok(table, 'Telemetry log table must be rendered')

      const headers = Array.from(table!.querySelectorAll('thead th')).map((th) => th.textContent?.trim())
      assert.equal(headers.length, 9, `Expected 9 telemetry columns, got ${headers.length}: ${headers.join(', ')}`)
      assert.deepEqual(headers, ['Time', 'Project', 'Agent', 'Model', 'Input', 'Output', 'Duration', 'Speed', 'Cost'])
      assert.ok(!headers.includes('Session / Project'), 'Session / Project header must be renamed to Project')
      assert.ok(!headers.includes('Tokens (In / Cache / Out)'), 'Tokens column must be replaced')
      assert.ok(!headers.includes('Type'), 'The dedicated Type column must be removed')
      assert.ok(!headers.includes('Status'), 'The Status column must be removed')
    } finally {
      unmount()
    }
  })

  it('renders Input tokens with cache hit ratio, Output, and Duration cells', async () => {
    initDOM()
    const { querySelectorAll, unmount } = await mountPage(TELEMETRY_FIXTURE)

    try {
      const table = findTelemetryTable(querySelectorAll)
      assert.ok(table, 'Telemetry log table must be rendered')

      const rows = Array.from(table!.querySelectorAll('tbody tr'))
      assert.equal(rows.length, 2, 'Both telemetry events must render as rows')

      const cells0 = Array.from(rows[0].querySelectorAll('td'))
      // Turn 1: Input=1000, Cache=200 -> cache hit 20.0%, Output=300, Duration=1500ms (1.5s)
      assert.equal(cells0[1]?.textContent?.trim(), 'swiss-knife')
      assert.ok(cells0[4]?.textContent?.includes('1.0k'), 'Input tokens formatted')
      assert.ok(cells0[4]?.textContent?.includes('(20.0%)'), 'Cache hit ratio in parenthesis')
      assert.equal(cells0[5]?.textContent?.trim(), '300', 'Output tokens formatted')
      assert.equal(cells0[6]?.textContent?.trim(), '1.5s', 'Duration formatted')

      const cells1 = Array.from(rows[1].querySelectorAll('td'))
      // Turn 2: Input=800, Cache=0 -> cache hit 0.0%, Output=120, Duration=2200ms (2.2s)
      assert.ok(cells1[4]?.textContent?.includes('800'), 'Input tokens formatted')
      assert.ok(cells1[4]?.textContent?.includes('(0.0%)'), 'Cache hit ratio in parenthesis')
      assert.equal(cells1[5]?.textContent?.trim(), '120', 'Output tokens formatted')
      assert.equal(cells1[6]?.textContent?.trim(), '2.2s', 'Duration formatted')
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

  it('shows the upstream routed model in the Model tooltip', async () => {
    initDOM()
    const { querySelectorAll, unmount } = await mountPage(TELEMETRY_FIXTURE)

    try {
      const table = findTelemetryTable(querySelectorAll)
      assert.ok(table, 'Telemetry log table must be rendered')

      const firstRow = table!.querySelectorAll('tbody tr')[0]
      const modelCell = firstRow.querySelectorAll('td')[3]
      const title = modelCell?.querySelector('span[title]')?.getAttribute('title') || ''

      assert.ok(
        title.includes('Routed: gemini-3.8-flash-n'),
        `Model tooltip must show the server-routed model, got: "${title}"`
      )
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
      assert.equal(emptyCell!.getAttribute('colspan'), '9', 'Empty-state colSpan must be 9')
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

  /** Finds the pricing registry table (unique by its Prompt Input and Cached Input headers). */
  const findPricingTable = (querySelectorAll: (sel: string) => Element[]) => {
    const tables = querySelectorAll('table')
    return tables.find((table) => {
      const headers = Array.from(table.querySelectorAll('thead th')).map((th) => th.textContent?.trim())
      return headers.includes('Prompt Input') && headers.includes('Cached Input')
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
      // Columns: Model, Type, Prompt Input, Cached Input, Output, Source, Action.
      assert.equal(cells[2]?.textContent?.trim(), '—', 'Null prompt-input price must render an em dash')
      assert.equal(cells[3]?.textContent?.trim(), '—', 'Null cached-input price must render an em dash')
      assert.equal(cells[4]?.textContent?.trim(), '—', 'Null output price must render an em dash')
      assert.equal(cells[5]?.textContent?.trim(), '—', 'Unconfigured source must render an em dash')

      const rowText = rows[0].textContent || ''
      assert.ok(!rowText.includes('Null —'), 'Legacy "Null — …" labels must not be rendered')
      assert.ok(!rowText.includes('Manual Required'), 'Legacy "Manual Required" text must not be rendered')
      assert.ok(!rowText.toLowerCase().includes('manual entry'), 'Legacy "Manual Entry" text must not be rendered')
    } finally {
      unmount()
    }
  })

  it('renders exactly 7 columns without Provider or ($/1M) and displays Official and 3rd Party sources', async () => {
    const PRICED_FIXTURE = {
      ...TELEMETRY_FIXTURE,
      telemetry_events: [],
      pricing_records: [
        {
          internal_id: 1,
          canonical_id: 'gemini-3.8-flash',
          model_name: 'Gemini 3.8 Flash',
          provider: 'Gemini',
          classification: 'native',
          input_price_per_m: 0.750,
          cached_input_price_per_m: 0.075,
          output_price_per_m: 3.75,
          source: 'provider',
          updated_at: '2026-01-01',
        },
        {
          internal_id: 2,
          canonical_id: 'deepseek-flash',
          model_name: 'DeepSeek V4.1 Flash',
          provider: 'Deepseek',
          classification: 'custom',
          input_price_per_m: 2.000,
          cached_input_price_per_m: 0.0182,
          output_price_per_m: 12.00,
          source: 'third_party',
          updated_at: '2026-01-01',
        },
      ],
    }

    initDOM()
    const { querySelectorAll, unmount } = await mountPage(PRICED_FIXTURE, 2)

    try {
      const table = findPricingTable(querySelectorAll)
      assert.ok(table, 'Pricing registry table must be rendered')

      const headers = Array.from(table!.querySelectorAll('thead th')).map((th) => th.textContent?.trim())
      assert.equal(headers.length, 7, 'Must have exactly 7 columns')
      assert.deepEqual(headers, ['Model', 'Type', 'Prompt Input', 'Cached Input', 'Output', 'Source', 'Action'])
      assert.ok(!headers.includes('Provider'), 'Provider column header must be removed')
      assert.ok(!headers.some((h) => h?.includes('($/1M)')), '($/1M) text must be removed from price columns')

      const rows = Array.from(table!.querySelectorAll('tbody tr'))
      assert.equal(rows.length, 2, 'Should render 2 rows')

      // Row 1: Gemini 3.8 Flash (provider source -> 'Official', 0.750 -> $0.75, 0.075 -> $0.075, 3.75 -> $3.75)
      const cells0 = Array.from(rows[0].querySelectorAll('td'))
      assert.equal(cells0[0]?.textContent?.trim(), 'Gemini 3.8 Flash')
      assert.equal(cells0[1]?.textContent?.trim(), 'native')
      assert.equal(cells0[2]?.textContent?.trim(), '$0.75', '0.750 must format to 2 decimals ($0.75)')
      assert.equal(cells0[3]?.textContent?.trim(), '$0.075', '0.075 must format with 3 decimals ($0.075)')
      assert.equal(cells0[4]?.textContent?.trim(), '$3.75', '3.75 must format to $3.75')
      assert.equal(cells0[5]?.textContent?.trim(), 'Official', 'Provider source must display as Official')
      // Verify cached input price cell color is plain text (var(--text)), not #137333 green
      const cachedCellColor0 = (cells0[3] as HTMLElement)?.style?.color
      assert.equal(cachedCellColor0, 'var(--text)', 'Cached input price must use plain text color')

      // Row 2: DeepSeek V4.1 Flash (third_party source -> '3rd Party', 2.000 -> $2.00, 0.0182 -> $0.018, 12.00 -> $12.00)
      const cells1 = Array.from(rows[1].querySelectorAll('td'))
      assert.equal(cells1[0]?.textContent?.trim(), 'DeepSeek V4.1 Flash')
      assert.equal(cells1[1]?.textContent?.trim(), 'custom')
      assert.equal(cells1[2]?.textContent?.trim(), '$2.00', '2.000 must format to 2 decimals ($2.00)')
      assert.equal(cells1[3]?.textContent?.trim(), '$0.018', '0.0182 rounded to 3 decimals must format to $0.018')
      assert.equal(cells1[4]?.textContent?.trim(), '$12.00', '12.00 must format to $12.00')
      assert.equal(cells1[5]?.textContent?.trim(), '3rd Party', 'Third party source must display as 3rd Party')
    } finally {
      unmount()
    }
  })

  it('formatPricingRateHelper correctly rounds to at most 3 decimals and drops trailing 0 at 3rd decimal', () => {
    assert.equal(formatPricingRateHelper(null), '—')
    assert.equal(formatPricingRateHelper(undefined), '—')
    assert.equal(formatPricingRateHelper(0), '$0.00')
    assert.equal(formatPricingRateHelper(2.000), '$2.00')
    assert.equal(formatPricingRateHelper(2), '$2.00')
    assert.equal(formatPricingRateHelper(0.750), '$0.75')
    assert.equal(formatPricingRateHelper(0.75), '$0.75')
    assert.equal(formatPricingRateHelper(0.075), '$0.075')
    assert.equal(formatPricingRateHelper(0.0182), '$0.018')
    assert.equal(formatPricingRateHelper(0.091), '$0.091')
    assert.equal(formatPricingRateHelper(0.1), '$0.10')
    assert.equal(formatPricingRateHelper(0.125), '$0.125')
    assert.equal(formatPricingRateHelper(12), '$12.00')
    assert.equal(formatPricingRateHelper(15.000), '$15.00')
    assert.equal(formatPricingRateHelper(1.500), '$1.50')
    assert.equal(formatPricingRateHelper(75.00), '$75.00')
  })
})

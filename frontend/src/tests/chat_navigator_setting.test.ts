/**
 * chat_navigator_setting.test.ts — Chat navigator display-mode selector (#162)
 *
 * Covers the "Chat navigator" select added to the in-chat settings section of the
 * Token Monitor page:
 *   - the select renders with a Panel (default) / Single line choice;
 *   - the daemon value seeds the control and the default is Panel;
 *   - changing it POSTs `chat_navigator_mode: "line"` through the existing
 *     GUI-config save path (no dedicated save button).
 */

import { describe, it, afterEach } from 'node:test'
import assert from 'node:assert/strict'
import React, { act } from 'react'
import { initDOM, mount, type RenderResult } from '../test-dom'
import TokenMonitorPage from '../pages/TokenMonitorPage'
import { cancelAllInFlightRequests } from '../api'

interface RecordedCall {
  url: string
  method: string
  body?: any
}

const TELEMETRY_FIXTURE = {
  total_tokens: 0,
  input_tokens: 0,
  cached_input_tokens: 0,
  output_tokens: 0,
  total_cost_usd: 0,
  saved_cost_usd: 0,
  avg_tps: 0,
  requests_count: 0,
  unpriced_models: [],
  model_breakdowns: [],
  project_breakdowns: [],
  telemetry_events: [],
}

/** Installs a fetch stub that answers the endpoints TokenMonitorPage hits on mount. */
function installFetch(guiConfig: Record<string, unknown>): RecordedCall[] {
  const calls: RecordedCall[] = []
  globalThis.fetch = (async (url: any, opts: any) => {
    const target = String(url)
    const method = (opts?.method || 'GET').toUpperCase()
    const body = opts?.body ? JSON.parse(String(opts.body)) : undefined
    calls.push({ url: target, method, body })

    let payload: any = {}
    if (target.includes('/api/gui/config')) {
      payload = method === 'POST' ? { ...guiConfig, ...body } : guiConfig
    } else if (target.includes('/api/tokens/summary')) {
      payload = TELEMETRY_FIXTURE
    }
    return { ok: true, status: 200, json: async () => payload } as any
  }) as any
  return calls
}

const flush = async () => {
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 20))
  })
}

const mountPage = async (guiConfig: Record<string, unknown>) => {
  const calls = installFetch(guiConfig)
  // Tab 1 hosts the in-chat settings gadget.
  const view = await mount(React.createElement(TokenMonitorPage, { activeTab: 1 }))
  await flush()
  return { view, calls }
}

const guiConfigPosts = (calls: RecordedCall[]) =>
  calls.filter((c) => c.url.includes('/api/gui/config') && c.method === 'POST')

const findNavigatorSelect = (view: RenderResult) =>
  view.querySelector<HTMLSelectElement>('select[aria-label="Chat navigator"]')

describe('Chat navigator display-mode selector', () => {
  const originalFetch = globalThis.fetch
  let view: RenderResult | null = null

  afterEach(() => {
    view?.unmount()
    view = null
    globalThis.fetch = originalFetch
    cancelAllInFlightRequests()
  })

  it('renders the select with Panel (default) / Single line and defaults to Panel', async () => {
    initDOM()
    const { view: mounted } = await mountPage({})
    view = mounted

    const select = findNavigatorSelect(view!)
    assert.ok(select, 'Chat navigator select must render')
    assert.equal(select!.value, 'panel', 'Chat navigator select must default to panel')

    const options = Array.from(select!.querySelectorAll('option')).map((o) => (o.textContent || '').trim())
    assert.deepEqual(options, ['Panel (default)', 'Single line'], `Unexpected options: ${options.join(', ')}`)

    const text = view!.container.textContent || ''
    assert.ok(text.includes('Chat navigator'), 'Chat navigator label must render')
    assert.ok(
      text.includes('Hover the left rail to expand the prompt list.'),
      'panel mode description must render'
    )
  })

  it('seeds the control from the daemon value and keeps Panel for an unknown value', async () => {
    initDOM()
    const first = await mountPage({ chat_navigator_mode: 'line' })
    view = first.view
    assert.equal(findNavigatorSelect(view!)!.value, 'line', 'daemon-reported line mode must seed the select')
    assert.ok(
      (view!.container.textContent || '').includes('Show the rail as plain dashes.'),
      'line mode description must render'
    )
    view!.unmount()

    const second = await mountPage({ chat_navigator_mode: 'bogus' })
    view = second.view
    assert.equal(findNavigatorSelect(view!)!.value, 'panel', 'unknown daemon value must fall back to panel')
  })

  it('saves chat_navigator_mode: "line" through the GUI-config path when changed', async () => {
    initDOM()
    const { view: mounted, calls } = await mountPage({ chat_navigator_mode: 'panel' })
    view = mounted

    assert.equal(guiConfigPosts(calls).length, 0, 'no GUI-config POST may happen on mount')

    const select = findNavigatorSelect(view!)!
    await act(async () => {
      select.value = 'line'
      select.dispatchEvent(new (globalThis as any).Event('change', { bubbles: true }))
    })
    await flush()

    const posts = guiConfigPosts(calls)
    assert.equal(posts.length, 1, 'changing the select must POST exactly once')
    assert.equal(posts[0].body.chat_navigator_mode, 'line', 'the partial payload must carry chat_navigator_mode: "line"')

    assert.equal(findNavigatorSelect(view!)!.value, 'line', 'the select must reflect the saved value')
    assert.ok(
      (view!.container.textContent || '').includes('Show the rail as plain dashes.'),
      'description must switch with the mode'
    )
  })
})

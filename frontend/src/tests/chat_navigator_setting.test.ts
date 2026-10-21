/**
 * chat_navigator_setting.test.ts - Chat Navigator display-mode selector (#162)
 *
 * Covers the Chat Navigator select under App Enhancements > Chat View >
 * Quick Prompt Jump Bar:
 *   - the select renders with Panel (Default) / Single Line choices;
 *   - the daemon value seeds the control and defaults to panel;
 *   - changing it POSTs `chat_navigator_mode: "line"` immediately.
 */

import { describe, it, afterEach } from 'node:test'
import assert from 'node:assert/strict'
import React, { act } from 'react'
import { initDOM, mount, type RenderResult } from '../test-dom'
import { AppEnhancementsPage } from '../pages/AppEnhancementsPage'
import { cancelAllInFlightRequests } from '../api'

interface RecordedCall {
  url: string
  method: string
  body?: any
}

const ENHANCEMENTS_FIXTURE = {
  version: '1.0.0',
  enabled: true,
  prompt_jump_bar: {
    enabled: true,
    show_tooltip: true,
    focus_pulse: true,
    sync_scroll: true,
    position: 'gutter',
    dash_width: 14,
    dash_thickness: 2.5,
    inactive_thickness: 1.5,
    color_mode: 'default',
    custom_color: '#0b57d0',
  },
  overview_panel: {
    enabled: true,
    division_style: 'divider_line',
    zone_border_radius: 8,
    zone_border_color: '#e2e8f0',
    zone_background_contrast: 'whiter',
    zone_padding: 10,
    zone_gap: 10,
    replace_see_all_triangle: true,
    aux_tabs_format: 'icon',
  },
  tool_density_mode: 'normal',
  breaker_line_enabled: false,
  scroll_to_bottom: false,
  turn_counter: false,
}

/** Installs a fetch stub that answers the endpoints AppEnhancementsPage hits on mount. */
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
    } else if (target.includes('/api/gui/projects')) {
      payload = []
    } else if (target.includes('/api/enhancements')) {
      payload = ENHANCEMENTS_FIXTURE
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
  const view = await mount(
    React.createElement(AppEnhancementsPage, { activeCategoryTab: 0 })
  )
  await flush()
  return { view, calls }
}

const guiConfigPosts = (calls: RecordedCall[]) =>
  calls.filter((c) => c.url.includes('/api/gui/config') && c.method === 'POST')

const findNavigatorSelect = (view: RenderResult) =>
  view.querySelector<HTMLSelectElement>('select#chat-navigator-mode')

describe('Chat Navigator display-mode selector', () => {
  const originalFetch = globalThis.fetch
  let view: RenderResult | null = null

  afterEach(() => {
    view?.unmount()
    view = null
    globalThis.fetch = originalFetch
    cancelAllInFlightRequests()
  })

  it('renders under Quick Prompt Jump Bar with Title Case options and defaults to Panel', async () => {
    initDOM()
    const { view: mounted } = await mountPage({})
    view = mounted

    const select = findNavigatorSelect(view!)
    assert.ok(select, 'Chat Navigator select must render')
    assert.equal(select!.value, 'panel', 'Chat Navigator select must default to panel')

    const card = select!.closest('.google-card')
    assert.ok(
      card?.textContent?.includes('Quick Prompt Jump Bar'),
      'Chat Navigator select must render inside the Quick Prompt Jump Bar card'
    )

    const label = view!.querySelector<HTMLLabelElement>('label[for="chat-navigator-mode"]')
    assert.equal(label?.textContent?.trim(), 'Chat Navigator')

    const options = Array.from(select!.querySelectorAll('option')).map((o) => (o.textContent || '').trim())
    assert.deepEqual(options, ['Panel (Default)', 'Single Line'])
    assert.ok(
      (view!.container.textContent || '').includes('Hover the left rail to expand the prompt list.'),
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

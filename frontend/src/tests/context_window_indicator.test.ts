import { describe, it, afterEach } from 'node:test'
import assert from 'node:assert/strict'
import React, { act } from 'react'
import { initDOM, mount, type RenderResult } from '../test-dom'
import { AppEnhancementsPage } from '../pages/AppEnhancementsPage'
import { cancelAllInFlightRequests } from '../api'

const BASE_ENHANCEMENTS = {
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

function installFetch(enhancements: Record<string, unknown>) {
  globalThis.fetch = (async (url: any, opts: any) => {
    const target = String(url)
    const method = (opts?.method || 'GET').toUpperCase()
    const body = opts?.body ? JSON.parse(String(opts.body)) : undefined

    let payload: any = {}
    if (target.includes('/api/gui/config')) {
      payload = method === 'POST' ? body : {}
    } else if (target.includes('/api/gui/projects')) {
      payload = []
    } else if (target.includes('/api/enhancements')) {
      payload = enhancements
    }

    return { ok: true, status: 200, json: async () => payload } as any
  }) as any
}

const flush = async () => {
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 20))
  })
}

const mountPage = async (enhancements: Record<string, unknown>) => {
  installFetch(enhancements)
  const view = await mount(
    React.createElement(AppEnhancementsPage, { activeCategoryTab: 0 })
  )
  await flush()
  return view
}

const findCard = (view: RenderResult) =>
  view.querySelectorAll<HTMLElement>('.google-card').find((card) =>
    card.textContent?.includes('Context Window Indicator')
  ) || null

describe('Context Window Indicator settings', () => {
  const originalFetch = globalThis.fetch
  let view: RenderResult | null = null

  afterEach(() => {
    view?.unmount()
    view = null
    globalThis.fetch = originalFetch
    cancelAllInFlightRequests()
  })

  it('defaults both legacy and new configs to enabled', async () => {
    initDOM()
    view = await mountPage(BASE_ENHANCEMENTS)

    const card = findCard(view)
    assert.ok(card, 'Context Window Indicator card must render')
    assert.ok(card!.textContent?.includes('Click Ring To Compact'))

    const switches = Array.from(card!.querySelectorAll<HTMLInputElement>('input[role="switch"]'))
    assert.equal(switches.length, 2, 'indicator and compact switches must render')
    assert.ok(switches.every((toggle) => toggle.checked), 'both switches must default to enabled')
  })

  it('hides the compact switch when the daemon disables the ring', async () => {
    initDOM()
    view = await mountPage({
      ...BASE_ENHANCEMENTS,
      context_window_indicator_enabled: false,
      context_window_click_to_compact_enabled: true,
    })

    const card = findCard(view)!
    const switches = Array.from(card.querySelectorAll<HTMLInputElement>('input[role="switch"]'))
    assert.equal(switches.length, 1, 'compact switch must be nested under the enabled indicator')
    assert.equal(switches[0].checked, false, 'indicator switch must reflect the disabled state')
    assert.ok(!card.textContent?.includes('Click Ring To Compact'))
  })

  it('keeps the ring visible while disabling only click-to-compact', async () => {
    initDOM()
    view = await mountPage({
      ...BASE_ENHANCEMENTS,
      context_window_indicator_enabled: true,
      context_window_click_to_compact_enabled: false,
    })

    const card = findCard(view)!
    const switches = Array.from(card.querySelectorAll<HTMLInputElement>('input[role="switch"]'))
    assert.equal(switches.length, 2)
    assert.equal(switches[0].checked, true)
    assert.equal(switches[1].checked, false)
  })
})

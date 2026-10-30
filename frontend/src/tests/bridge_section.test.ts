import { afterEach, beforeEach, describe, it } from 'node:test'
import assert from 'node:assert/strict'
import React, { act } from 'react'
import { BRIDGE_TABS, parseHash, routeToTool, serializeRoute } from '../navigation/routes'
import { initDOM, mount, type RenderResult } from '../test-dom'

const originalFetch = globalThis.fetch
let originalWindowFetch: typeof fetch
let originalElectronAPI: unknown
let requestController: AbortController
let view: RenderResult | null = null
const requests: Array<{ method: string; path: string; body: string }> = []

const jsonResponse = (body: unknown): Response => ({
  ok: true,
  status: 200,
  headers: {
    get: (name: string) => name.toLowerCase() === 'content-type' ? 'application/json' : null,
  },
  json: async () => body,
  text: async () => JSON.stringify(body),
}) as unknown as Response

const installFetchMock = () => {
  requestController = new AbortController()
  const fetchMock = async (
    input: RequestInfo | URL,
    init?: RequestInit,
  ): Promise<Response> => {
    const raw = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url
    const url = new URL(raw, 'http://localhost')
    const body = typeof init?.body === 'string' ? init.body : ''
    requests.push({ method: init?.method || 'GET', path: url.pathname, body })

    if (requestController.signal.aborted) {
      throw new DOMException('Request aborted', 'AbortError')
    }

    const path = url.pathname
    if (path.endsWith('/api/auth/status')) return jsonResponse({ password_required: false })
    if (path.endsWith('/api/status')) {
      return jsonResponse({ daemon_running: true, active_account: 'bridge-test@example.com' })
    }
    if (path.includes('/api/quota/fleet')) return jsonResponse({ accounts: [], active_account: null })
    if (path.includes('/api/rules')) return jsonResponse({})
    if (path.includes('/api/gui/projects') || path.includes('/api/projects')) return jsonResponse([])
    if (path.includes('/api/accounts')) return jsonResponse([])
    if (path.endsWith('/api/bridge/remote/status')) {
      return jsonResponse({
        success: true,
        status: {
          tunnel: { active: false, type: 'loopback', logs: [] },
          app_password_enabled: false,
          use_app_password: false,
        },
        config: { use_app_password: false },
      })
    }
    if (path.endsWith('/api/bridge/remote/config')) {
      return jsonResponse({ success: true, config: { use_app_password: false } })
    }
    if (path.endsWith('/api/bridge/status')) {
      return jsonResponse({
        tunnel: { active: false, type: 'loopback' },
        enabled: false,
        operating_mode: 'manual',
        agents: {},
      })
    }
    if (path.endsWith('/api/bridge/http-allowlist')) {
      return jsonResponse({
        success: true,
        mode: 'always_allow',
        allowlist: [],
        pending: [],
      })
    }
    if (path.endsWith('/api/bridge/channels/status')) {
      return jsonResponse({
        success: true,
        status: { whatsapp: {}, telegram: {}, discord: {} },
      })
    }
    if (path.endsWith('/api/bridge/channels/config')) {
      return jsonResponse({
        success: true,
        config: {
          whatsapp: { enabled: false, allowed_phone_numbers: [] },
          telegram: { enabled: false, allowed_chat_ids: [] },
          discord: { enabled: false, allowed_channel_ids: [] },
        },
      })
    }
    return jsonResponse({ success: true })
  }

  globalThis.fetch = fetchMock as typeof fetch
  window.fetch = fetchMock as typeof window.fetch
}

const flush = async () => {
  for (let i = 0; i < 3; i += 1) {
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 0))
    })
  }
}

const clickTab = async (tab: string) => {
  assert.ok(view)
  await view.click(view.querySelector(`[data-route-tab="${tab}"]`))
  await flush()
  assert.equal(view.querySelector(`[data-route-tab="${tab}"]`)?.getAttribute('aria-pressed'), 'true')
  assert.equal(
    window.location.hash,
    tab === 'remote' ? '#/remote-connect' : `#/remote-connect/${tab}`,
  )
}

beforeEach(() => {
  initDOM()
  requests.length = 0
  originalWindowFetch = window.fetch
  originalElectronAPI = (window as any).electronAPI
  window.localStorage.clear()
  window.history.replaceState(null, '', '#/remote-connect')
  ;(window as any).electronAPI = { isElectron: false }
  installFetchMock()
})

afterEach(() => {
  view?.unmount()
  view = null
  requestController?.abort()
  globalThis.fetch = originalFetch
  window.fetch = originalWindowFetch
  ;(window as any).electronAPI = originalElectronAPI
})

describe('Bridge Section rendered behavior', () => {
  it('round-trips every typed bridge route', () => {
    assert.deepEqual([...BRIDGE_TABS], ['remote', 'gemini', 'whatsapp', 'telegram', 'discord'])
    for (const tab of BRIDGE_TABS) {
      const hash = serializeRoute({ tool: 'remote-connect', tab })
      assert.equal(routeToTool(parseHash(hash)), 'remote-connect')
      assert.deepEqual(parseHash(hash), { tool: 'remote-connect', tab })
    }
  })

  it('mounts the App, renders all bridge tabs, and swaps the active panel', async () => {
    const { App } = await import('../App')
    view = await mount(React.createElement(App))
    await flush()

    const tabs = view.querySelectorAll<HTMLElement>('[data-route-tab]')
    assert.deepEqual(tabs.map((tab) => tab.dataset.routeTab), [...BRIDGE_TABS])
    assert.deepEqual(
      tabs.map((tab) => tab.textContent?.trim()),
      ['Remote', 'Gemini', 'WhatsApp', 'Telegram', 'Discord'],
    )
    assert.equal(view.querySelector('[data-route-tab="remote"]')?.getAttribute('aria-pressed'), 'true')
    assert.match(view.container.textContent || '', /Security PIN/)
    assert.ok(requests.some((request) => request.path.endsWith('/api/bridge/remote/status')))

    await clickTab('gemini')
    assert.match(view.container.textContent || '', /Gemini Agent Chat Bridge/)
    assert.doesNotMatch(view.container.textContent || '', /Security PIN/)
    assert.ok(requests.some((request) => request.path.endsWith('/api/bridge/status')))

    await clickTab('whatsapp')
    assert.match(view.container.textContent || '', /WhatsApp Agent Chat Bridge/)
    assert.ok(requests.some((request) => request.path.endsWith('/api/bridge/channels/status')))

    await clickTab('telegram')
    assert.match(view.container.textContent || '', /Telegram Agent Chat Bridge/)

    await clickTab('discord')
    assert.match(view.container.textContent || '', /Discord Agent Chat Bridge/)

    await clickTab('remote')
    assert.match(view.container.textContent || '', /Security PIN/)
  })
})

import { afterEach, beforeEach, describe, it } from 'node:test'
import assert from 'node:assert/strict'
import React, { act } from 'react'
import { serializeRoute, type Route } from '../navigation/routes'
import { initDOM, mount, type RenderResult } from '../test-dom'

const ALL_ROUTES: Route[] = [
  { tool: 'switcher', tab: 'dashboard' },
  { tool: 'switcher', tab: 'fingerprints' },
  { tool: 'switcher', tab: 'switcher-settings' },
  { tool: 'system-settings', tab: 'general' },
  { tool: 'system-settings', tab: 'storage' },
  { tool: 'system-settings', tab: 'privacy' },
  { tool: 'system-settings', tab: 'about' },
  { tool: 'custom-models' },
  { tool: 'enhancements', tab: 'chat-view' },
  { tool: 'enhancements', tab: 'project-panel' },
  { tool: 'enhancements', tab: 'auxiliary-panel' },
  { tool: 'extensions', tab: 'built-in' },
  { tool: 'extensions', tab: 'community' },
  { tool: 'token-monitor', tab: 'overview' },
  { tool: 'token-monitor', tab: 'telemetry' },
  { tool: 'token-monitor', tab: 'token-price' },
  { tool: 'utilities', tab: 'cache' },
  { tool: 'utilities', tab: 'archives' },
  { tool: 'utilities', tab: 'import' },
  { tool: 'remote-connect', tab: 'remote' },
  { tool: 'remote-connect', tab: 'gemini' },
  { tool: 'remote-connect', tab: 'whatsapp' },
  { tool: 'remote-connect', tab: 'telegram' },
  { tool: 'remote-connect', tab: 'discord' },
  { tool: 'acp-adapter' },
]

const TOOL_NAV_ID: Record<Route['tool'], string> = {
  'switcher': '#btnNavAccountSwitcher',
  'system-settings': '#btnNavSystemSettings',
  'custom-models': '#btnNavCustomModels',
  'enhancements': '#btnNavEnhancements',
  'extensions': '#btnNavExtensions',
  'token-monitor': '#btnNavTokenMonitor',
  'utilities': '#btnNavUtilities',
  'remote-connect': '#btnNavBridge',
  'acp-adapter': '#btnNavACPAgentMesh',
}

const originalFetch = globalThis.fetch
let originalElectronAPI: unknown

const jsonResponse = (body: unknown): Response => ({
  ok: true,
  status: 200,
  headers: { get: () => 'application/json' },
  json: async () => body,
  text: async () => JSON.stringify(body),
}) as unknown as Response

const installFetchMock = () => {
  const fetchMock = async (input: RequestInfo | URL): Promise<Response> => {
    const raw = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url
    const path = new URL(raw, 'http://localhost').pathname
    if (path.endsWith('/api/auth/status')) return jsonResponse({ password_required: false })
    if (path.endsWith('/api/status')) {
      return jsonResponse({ daemon_running: true, active_account: 'route-test@example.com' })
    }
    if (path.includes('/api/quota/fleet')) return jsonResponse({ accounts: [], active_account: null })
    if (path.includes('/api/rules')) return jsonResponse({})
    if (path.includes('/api/gui/projects') || path.includes('/api/projects')) return jsonResponse([])
    if (path.includes('/api/accounts')) return jsonResponse([])
    return jsonResponse({})
  }
  globalThis.fetch = fetchMock as typeof fetch
  window.fetch = fetchMock as typeof window.fetch
}

const flush = async () => {
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 0))
  })
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 0))
  })
}

const activeTabId = (view: RenderResult): string | null => {
  const pressed = view.querySelector<HTMLElement>('[data-route-tab][aria-pressed="true"]')
  return pressed?.dataset.routeTab ?? null
}

const renderAt = async (route: Route) => {
  window.history.replaceState(null, '', serializeRoute(route))
  const { App } = await import('../App')
  const view = await mount(React.createElement(App))
  await flush()
  return view
}

const moveHistory = async (direction: 'back' | 'forward') => {
  await act(async () => {
    window.history[direction]()
    await new Promise((resolve) => setTimeout(resolve, 25))
  })
  await flush()
}

describe('App URL routing', () => {
  let view: RenderResult | null = null
  let popStateCleanup: (() => void) | null = null

  beforeEach(() => {
    initDOM()
    originalElectronAPI = (window as any).electronAPI
    window.localStorage.clear()
    window.history.replaceState(null, '', '#/switcher/dashboard')
    installFetchMock()
  })

  afterEach(() => {
    popStateCleanup?.()
    popStateCleanup = null
    view?.unmount()
    view = null
    globalThis.fetch = originalFetch
    ;(window as any).electronAPI = originalElectronAPI
  })

  it('mounts the real App and restores every tool and tab across a reload', async () => {
    for (const route of ALL_ROUTES) {
      view?.unmount()
      view = await renderAt(route)

      assert.ok(view.querySelector(TOOL_NAV_ID[route.tool]), `${route.tool} nav button`)
      assert.equal(
        view.querySelector<HTMLElement>('[data-active-tool]')?.dataset.activeTool,
        route.tool,
        `${route.tool} rendered tool`,
      )

      const expectedTab = 'tab' in route ? route.tab : null
      assert.equal(activeTabId(view), expectedTab, `${serializeRoute(route)} active tab`)
    }
  })

  it('navigates through App controls and restores back/forward history without synthetic popstate', async () => {
    view = await renderAt({ tool: 'extensions', tab: 'built-in' })

    await view.click(view.querySelector('[data-route-tab="community"]'))
    await flush()
    assert.equal(window.location.hash, '#/extensions/community')
    assert.equal(activeTabId(view), 'community')

    await view.click(view.querySelector('#btnNavSystemSettings'))
    await flush()
    assert.equal(window.location.hash, '#/system-settings')
    assert.equal(activeTabId(view), 'general')

    await view.click(view.querySelector('[data-route-tab="privacy"]'))
    await flush()
    assert.equal(window.location.hash, '#/system-settings/privacy')
    assert.equal(activeTabId(view), 'privacy')

    let popstateCount = 0
    const onPopState = () => { popstateCount += 1 }
    window.addEventListener('popstate', onPopState)
    popStateCleanup = () => window.removeEventListener('popstate', onPopState)

    await moveHistory('back')
    assert.equal(window.location.hash, '#/system-settings')
    assert.equal(activeTabId(view), 'general')

    await moveHistory('back')
    assert.equal(window.location.hash, '#/extensions/community')
    assert.equal(activeTabId(view), 'community')

    await moveHistory('forward')
    assert.equal(window.location.hash, '#/system-settings')
    assert.equal(activeTabId(view), 'general')
    assert.ok(popstateCount >= 3, `expected popstate events, got ${popstateCount}`)
    popStateCleanup()
    popStateCleanup = null
  })

  it('routes semantic and legacy tray targets through the mounted App', async () => {
    const callbacks: Array<(target: unknown) => void> = []
    ;(window as any).electronAPI = {
      isElectron: true,
      onNavigate(callback: (target: unknown) => void) {
        callbacks.push(callback)
        return () => {}
      },
    }
    view = await renderAt({ tool: 'switcher', tab: 'dashboard' })
    assert.equal(callbacks.length, 1)

    await act(async () => { callbacks[0]('utilities/archives') })
    await flush()
    assert.equal(window.location.hash, '#/utilities/archives')
    assert.equal(activeTabId(view), 'archives')

    await act(async () => { callbacks[0](2) })
    await flush()
    assert.equal(window.location.hash, '#/system-settings')
    assert.equal(activeTabId(view), 'general')
  })
})

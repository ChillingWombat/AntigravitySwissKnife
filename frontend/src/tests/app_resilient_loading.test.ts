import { afterEach, beforeEach, describe, it } from 'node:test'
import assert from 'node:assert/strict'
import React, { act } from 'react'
import { App } from '../App'
import { useDashboardData, type DashboardLoaders } from '../hooks/useDashboardData'
import type { FleetQuotaSummary, RuleConfig, SystemStatus } from '../types'
import { initDOM, mount, type RenderResult } from '../test-dom'

const status: SystemStatus = {
  daemon_running: true,
  daemon_pid: 42,
  active_account: 'known@example.com',
}

const fleet: FleetQuotaSummary = {
  fleet_5h_available: 1,
  fleet_weekly_available: 1,
  total_accounts: 1,
  active_account: 'known@example.com',
  accounts: [{
    email: 'known@example.com',
    is_active: true,
    status: 'ACTIVE',
    reset_horizon_text: '',
    has_mfa: false,
  }],
}

const makeLoaders = (overrides: Partial<DashboardLoaders> = {}): DashboardLoaders => ({
  status: async () => status,
  quota: async () => fleet,
  rules: async () => ({}) as RuleConfig,
  projects: async () => [],
  accounts: async () => [{ email: 'known@example.com' }],
  ...overrides,
})

const jsonResponse = (body: unknown): Response => ({
  ok: true,
  status: 200,
  headers: { get: () => 'application/json' },
  json: async () => body,
  text: async () => JSON.stringify(body),
}) as unknown as Response

const flush = async () => {
  for (let index = 0; index < 4; index += 1) {
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 0))
    })
  }
}

describe('App resilient dashboard loading', () => {
  const originalFetch = globalThis.fetch
  let view: RenderResult | null = null

  beforeEach(() => {
    initDOM()
    const fetchMock = async (input: RequestInfo | URL): Promise<Response> => {
      const raw = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url
      const path = new URL(raw, 'http://localhost').pathname
      if (path.endsWith('/api/auth/status')) return jsonResponse({ password_required: false })
      return jsonResponse({})
    }
    globalThis.fetch = fetchMock as typeof fetch
    window.fetch = fetchMock as typeof window.fetch
  })

  afterEach(() => {
    view?.unmount()
    view = null
    globalThis.fetch = originalFetch
  })

  it('renders good data when one endpoint fails without exposing backend text', async () => {
    const loaders = makeLoaders({
      rules: async () => { throw new Error('secret backend body') },
    })
    view = await mount(React.createElement(App, { dashboardLoaders: loaders }))
    await flush()

    const text = view.container.textContent ?? ''
    assert.match(text, /known@example\.com/)
    assert.match(text, /1 source unavailable/)
    assert.equal(text.includes('secret backend body'), false)
    assert.ok(view.querySelector('[data-dashboard-health][role="status"][aria-live="polite"]'))
    assert.ok(view.querySelector('main[aria-busy="false"]'))
  })

  it('clears degraded health after retrying only the failed endpoints', async () => {
    let rulesFail = true
    const loaders = makeLoaders({
      rules: async () => {
        if (rulesFail) throw new Error('offline')
        return {} as RuleConfig
      },
    })
    view = await mount(React.createElement(App, { dashboardLoaders: loaders }))
    await flush()
    assert.ok(view.querySelector('[data-dashboard-health]'))

    rulesFail = false
    await view.click(view.querySelector('[data-dashboard-retry]'))
    await flush()
    assert.equal(view.querySelector('[data-dashboard-health]'), null)
  })

  it('preserves last-known data after a later refresh fails', async () => {
    let statusFails = false
    const loaders = makeLoaders({
      status: async () => {
        if (statusFails) throw new Error('offline')
        return status
      },
    })
    view = await mount(React.createElement(App, { dashboardLoaders: loaders }))
    await flush()
    assert.match(view.container.textContent ?? '', /known@example\.com/)

    statusFails = true
    await act(async () => {
      document.dispatchEvent(new window.Event('visibilitychange'))
      await new Promise((resolve) => setTimeout(resolve, 0))
    })
    await flush()

    assert.match(view.container.textContent ?? '', /known@example\.com/)
    assert.match(view.container.textContent ?? '', /1 source unavailable/)
  })

  it('ignores an older completion after a newer retry supersedes it', async () => {
    let resolveOlder: ((value: SystemStatus) => void) | null = null
    let resolveNewer: ((value: SystemStatus) => void) | null = null
    let calls = 0
    const loaders = makeLoaders({
      status: () => new Promise((resolve) => {
        calls += 1
        if (calls === 1) resolveOlder = resolve
        else resolveNewer = resolve
      }),
    })

    let dashboard: ReturnType<typeof useDashboardData> | null = null
    const Probe = () => {
      dashboard = useDashboardData({ loaders, autoLoad: false })
      return null
    }
    view = await mount(React.createElement(Probe))

    await act(async () => {
      void dashboard!.load()
      await new Promise((resolve) => setTimeout(resolve, 0))
    })
    await act(async () => {
      void dashboard!.retry(['status'])
      await new Promise((resolve) => setTimeout(resolve, 0))
    })

    await act(async () => {
      resolveNewer!({ ...status, active_account: 'newer@example.com' })
      await new Promise((resolve) => setTimeout(resolve, 0))
    })
    await act(async () => {
      resolveOlder!({ ...status, active_account: 'older@example.com' })
      await new Promise((resolve) => setTimeout(resolve, 0))
    })

    assert.equal(dashboard!.state.status.data?.active_account, 'newer@example.com')
  })
})

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

const laterFleet: FleetQuotaSummary = {
  ...fleet,
  active_account: 'fleet@example.com',
  accounts: [{ ...fleet.accounts[0], email: 'fleet@example.com' }],
}

function deferred<T>() {
  let resolve!: (value: T | PromiseLike<T>) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise
    reject = rejectPromise
  })
  return { promise, resolve, reject }
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

const assertDegradedFleetAccounts = (view: RenderResult) => {
  const text = view.container.textContent ?? ''
  assert.match(text, /fleet@example\.com/)
  assert.match(text, /1 source unavailable/)
  assert.ok(view.querySelector('[data-dashboard-health][role="status"][aria-live="polite"]'))
  assert.ok(view.querySelector('[data-dashboard-retry]'))
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

  it('keeps accounts failed when fleet quota fulfills after the accounts request fails', async () => {
    const accounts = deferred<Awaited<ReturnType<DashboardLoaders['accounts']>>>()
    const quota = deferred<FleetQuotaSummary>()
    const loaders = makeLoaders({
      accounts: () => accounts.promise,
      quota: () => quota.promise,
    })
    view = await mount(React.createElement(App, { dashboardLoaders: loaders }))
    await flush()

    await act(async () => {
      accounts.reject(new Error('accounts offline'))
      await new Promise((resolve) => setTimeout(resolve, 0))
    })
    await flush()
    await act(async () => {
      quota.resolve(laterFleet)
      await new Promise((resolve) => setTimeout(resolve, 0))
    })
    await flush()

    assertDegradedFleetAccounts(view)
  })

  it('keeps accounts degraded when the accounts request fails after fleet quota succeeds', async () => {
    const accounts = deferred<Awaited<ReturnType<DashboardLoaders['accounts']>>>()
    const quota = deferred<FleetQuotaSummary>()
    const loaders = makeLoaders({
      accounts: () => accounts.promise,
      quota: () => quota.promise,
    })
    view = await mount(React.createElement(App, { dashboardLoaders: loaders }))
    await flush()

    await act(async () => {
      quota.resolve(laterFleet)
      await new Promise((resolve) => setTimeout(resolve, 0))
    })
    await flush()
    await act(async () => {
      accounts.reject(new Error('accounts offline'))
      await new Promise((resolve) => setTimeout(resolve, 0))
    })
    await flush()

    assertDegradedFleetAccounts(view)
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
  it('background refresh requests only status and fleet', async () => {
    const calls: string[] = []
    const loaders = makeLoaders({
      status: async () => { calls.push('status'); return status },
      quota: async () => { calls.push('quota'); return fleet },
      rules: async () => { calls.push('rules'); return {} as RuleConfig },
      projects: async () => { calls.push('projects'); return [] },
      accounts: async () => { calls.push('accounts'); return [{ email: 'known@example.com' }] },
    })
    view = await mount(React.createElement(App, { dashboardLoaders: loaders }))
    await flush()
    calls.length = 0
    await act(async () => {
      document.dispatchEvent(new window.Event('visibilitychange'))
      await new Promise((resolve) => setTimeout(resolve, 0))
    })
    await flush()
    assert.deepEqual(calls.sort(), ['quota', 'status'])
  })

  it('fetches accounts when the fleet refresh fails', async () => {
    let fleetFails = false
    let accountCalls = 0
    const loaders = makeLoaders({
      quota: async () => {
        if (fleetFails) throw new Error('fleet offline')
        return fleet
      },
      accounts: async () => {
        accountCalls += 1
        return [{ email: 'known@example.com' }]
      },
    })
    view = await mount(React.createElement(App, { dashboardLoaders: loaders }))
    await flush()
    accountCalls = 0
    fleetFails = true
    await act(async () => {
      document.dispatchEvent(new window.Event('visibilitychange'))
      await new Promise((resolve) => setTimeout(resolve, 0))
    })
    await flush()
    assert.equal(accountCalls, 1)
  })

  it('forces a fresh status read after a successful account switch', async () => {
    let statusCalls = 0
    const loaders = makeLoaders({
      status: async () => { statusCalls += 1; return status },
    })
    view = await mount(React.createElement(App, { dashboardLoaders: loaders }))
    await flush()
    statusCalls = 0
    await act(async () => {
      window.dispatchEvent(new CustomEvent('swiss:account-switched', { detail: { active_account: 'next@example.com' } }))
      await new Promise((resolve) => setTimeout(resolve, 0))
    })
    await flush()
    assert.ok(statusCalls >= 1)
  })

  it('refreshes rules after a successful mutation event', async () => {
    let ruleCalls = 0
    const loaders = makeLoaders({
      rules: async () => { ruleCalls += 1; return {} as RuleConfig },
    })
    view = await mount(React.createElement(App, { dashboardLoaders: loaders }))
    await flush()
    ruleCalls = 0
    await act(async () => {
      window.dispatchEvent(new CustomEvent('swiss:dashboard-mutated', { detail: { endpoint: 'rules' } }))
      await new Promise((resolve) => setTimeout(resolve, 0))
    })
    await flush()
    assert.equal(ruleCalls, 1)
  })

})

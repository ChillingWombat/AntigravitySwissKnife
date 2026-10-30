import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import type { FleetQuotaSummary, SystemStatus } from '../types'
import {
  createInitialDashboardLoadState,
  dashboardReducer,
  failedDashboardEndpoints,
} from './dashboardLoadState'

const status: SystemStatus = { daemon_running: true, daemon_pid: 1, active_account: 'one@example.com' }
const fleet: FleetQuotaSummary = {
  fleet_5h_available: 1,
  fleet_weekly_available: 1,
  total_accounts: 1,
  active_account: 'one@example.com',
  accounts: [{
    email: 'one@example.com',
    is_active: true,
    status: 'ACTIVE',
    reset_horizon_text: '',
    has_mfa: false,
  }],
}

describe('dashboardLoadState', () => {
  it('starts every endpoint in initial state', () => {
    const state = createInitialDashboardLoadState()
    assert.equal(failedDashboardEndpoints(state).length, 0)
    assert.equal(state.accounts.status, 'initial')
    assert.equal(state.accounts.source, 'direct')
  })

  it('preserves retained data while loading and turns a failed refresh into degraded', () => {
    let state = dashboardReducer(createInitialDashboardLoadState(), {
      type: 'load/fulfilled',
      endpoint: 'status',
      data: status,
    })
    state = dashboardReducer(state, { type: 'load/start', endpoints: ['status'] })
    assert.equal(state.status.status, 'loading')
    assert.equal(state.status.data, status)

    state = dashboardReducer(state, {
      type: 'load/rejected',
      endpoints: ['status'],
      error: 'request_failed',
    })
    assert.equal(state.status.status, 'degraded')
    assert.equal(state.status.data, status)
    assert.equal(state.status.error, 'request_failed')
  })

  it('uses error when a failed endpoint has no retained data', () => {
    const state = dashboardReducer(createInitialDashboardLoadState(), {
      type: 'load/rejected',
      endpoints: ['rules'],
      error: 'invalid_response',
    })
    assert.equal(state.rules.status, 'error')
    assert.equal(state.rules.error, 'invalid_response')
  })

  it('keeps fulfilled endpoints when only one endpoint fails', () => {
    let state = dashboardReducer(createInitialDashboardLoadState(), {
      type: 'load/start',
      endpoints: ['status', 'rules'],
    })
    state = dashboardReducer(state, {
      type: 'load/fulfilled',
      endpoint: 'status',
      data: status,
    })
    state = dashboardReducer(state, {
      type: 'load/rejected',
      endpoints: ['rules'],
      error: 'request_failed',
    })
    assert.equal(state.status.status, 'ready')
    assert.equal(state.status.data, status)
    assert.deepEqual(failedDashboardEndpoints(state), ['rules'])
  })

  it('lets a fleet response satisfy accounts while retaining an accounts failure', () => {
    let state = dashboardReducer(createInitialDashboardLoadState(), {
      type: 'load/fulfilled',
      endpoint: 'quota',
      data: fleet,
    })
    assert.equal(state.accounts.status, 'ready')
    assert.equal(state.accounts.source, 'fleet')
    assert.deepEqual(state.accounts.data, fleet.accounts)

    state = dashboardReducer(state, {
      type: 'load/rejected',
      endpoints: ['accounts'],
      error: 'request_failed',
    })
    assert.equal(state.accounts.status, 'degraded')
    assert.equal(state.accounts.error, 'request_failed')
    assert.deepEqual(state.accounts.data, fleet.accounts)
  })
})

import { describe, it } from 'node:test'
import assert from 'node:assert'
import { sortAccounts } from './accountSorting.ts'
import type { AccountState } from '../types.ts'

function createAccount(overrides: Partial<AccountState>): AccountState {
  return {
    email: 'test@example.com',
    label: 'Test Account',
    priority: 'High',
    plan_tier: 'Pro',
    is_active: false,
    status: 'STANDBY',
    quota_5h_current: 1.0,
    quota_5h_available: 1.0,
    quota_weekly: 0.5,
    reset_horizon_text: 'Ready',
    reset_horizon_weekly_text: 'Ready',
    has_mfa: false,
    credits: 0,
    ...overrides,
  }
}

describe('accountSorting utility', () => {
  it('pins the active account to Row 1 even when quota is below threshold', () => {
    const depletedActive = createAccount({
      email: 'active@gmail.com',
      label: 'Depleted Active Account',
      is_active: true,
      status: 'ACTIVE',
      quota_5h_current: 0.0,
      quota_5h_available: 0.0,
      quota_weekly: 0.04, // 4%, below 5% threshold
    })

    const healthyStandby = createAccount({
      email: 'standby@gmail.com',
      label: 'Healthy Standby Account',
      is_active: false,
      status: 'STANDBY',
      quota_5h_current: 1.0,
      quota_5h_available: 1.0,
      quota_weekly: 0.8,
    })

    const accounts = [healthyStandby, depletedActive]
    const sorted = sortAccounts(accounts, 'active@gmail.com', 0.05, 'auto')

    assert.strictEqual(sorted[0].email, 'active@gmail.com', 'Active account must be in Row 1')
    assert.strictEqual(sorted[1].email, 'standby@gmail.com', 'Standby account must be in Row 2')
  })

  it('ranks healthy standby accounts above depleted standby accounts', () => {
    const active = createAccount({
      email: 'active@gmail.com',
      is_active: true,
      quota_5h_current: 0.5,
      quota_weekly: 0.5,
    })

    const healthy = createAccount({
      email: 'healthy@gmail.com',
      is_active: false,
      quota_5h_current: 1.0,
      quota_weekly: 0.9,
    })

    const depleted = createAccount({
      email: 'depleted@gmail.com',
      is_active: false,
      quota_5h_current: 0.02, // Below threshold
      quota_weekly: 0.03,
    })

    const banned = createAccount({
      email: 'banned@gmail.com',
      is_active: false,
      status: 'BANNED',
      quota_5h_current: 1.0,
      quota_weekly: 1.0,
    })

    const sorted = sortAccounts([banned, depleted, healthy, active], 'active@gmail.com', 0.05, 'auto')

    assert.strictEqual(sorted[0].email, 'active@gmail.com')
    assert.strictEqual(sorted[1].email, 'healthy@gmail.com')
    assert.strictEqual(sorted[2].email, 'depleted@gmail.com')
    assert.strictEqual(sorted[3].email, 'banned@gmail.com')
  })

  it('resolves active account using activeEmail over is_active flag when activeEmail is provided', () => {
    const acc1 = createAccount({
      email: 'first@gmail.com',
      is_active: true, // Stale is_active in item
    })

    const acc2 = createAccount({
      email: 'second@gmail.com',
      is_active: false,
    })

    // Suppose system activeEmail is second@gmail.com
    const sorted = sortAccounts([acc1, acc2], 'second@gmail.com', 0.05, 'auto')
    assert.strictEqual(sorted[0].email, 'second@gmail.com', 'second@gmail.com should be active')
    assert.strictEqual(sorted[1].email, 'first@gmail.com')
  })

  it('sorts by identity / name properly', () => {
    const b = createAccount({ email: 'beta@gmail.com', label: 'Beta' })
    const a = createAccount({ email: 'alpha@gmail.com', label: 'Alpha' })

    const sorted = sortAccounts([b, a], '', 0.05, 'identity')
    assert.strictEqual(sorted[0].label, 'Alpha')
    assert.strictEqual(sorted[1].label, 'Beta')
  })

  it('sorts by priority properly', () => {
    const low = createAccount({ email: 'low@gmail.com', priority: 'Low' })
    const high = createAccount({ email: 'high@gmail.com', priority: 'High' })
    const mid = createAccount({ email: 'mid@gmail.com', priority: 'Mid' })

    const sorted = sortAccounts([low, high, mid], '', 0.05, 'priority')
    assert.strictEqual(sorted[0].priority, 'High')
    assert.strictEqual(sorted[1].priority, 'Mid')
    assert.strictEqual(sorted[2].priority, 'Low')
  })
})

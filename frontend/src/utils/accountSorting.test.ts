import { describe, it } from 'node:test'
import assert from 'node:assert'
import { sortAccounts, getNextSortState } from './accountSorting.ts'
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
  describe('3-state column header sorting transitions (getNextSortState)', () => {
    it('cycles: 1st click -> asc, 2nd click -> desc, 3rd click -> auto', () => {
      // 1st click on 5H Quota
      const click1 = getNextSortState('auto', 'asc', 'quota_5h')
      assert.deepStrictEqual(click1, { mode: 'quota_5h', direction: 'asc' })

      // 2nd click on 5H Quota
      const click2 = getNextSortState(click1.mode, click1.direction, 'quota_5h')
      assert.deepStrictEqual(click2, { mode: 'quota_5h', direction: 'desc' })

      // 3rd click on 5H Quota (exit to auto)
      const click3 = getNextSortState(click2.mode, click2.direction, 'quota_5h')
      assert.deepStrictEqual(click3, { mode: 'auto', direction: 'asc' })
    })

    it('resets to 1st click (asc) when clicking a different column', () => {
      // Currently on 5H Quota descending
      const next = getNextSortState('quota_5h', 'desc', 'quota_weekly')
      assert.deepStrictEqual(next, { mode: 'quota_weekly', direction: 'asc' })
    })
  })

  describe('sortAccounts bidirectional sorting', () => {
    it('sorts identity ascending (A->Z) and descending (Z->A)', () => {
      const a = createAccount({ email: 'a@example.com', label: 'Alpha' })
      const b = createAccount({ email: 'b@example.com', label: 'Beta' })
      const c = createAccount({ email: 'c@example.com', label: 'Charlie' })

      const sortedAsc = sortAccounts([c, a, b], '', 0.05, 'identity', undefined, 0.05, false, 'asc')
      assert.deepStrictEqual(sortedAsc.map(x => x.label), ['Alpha', 'Beta', 'Charlie'])

      const sortedDesc = sortAccounts([c, a, b], '', 0.05, 'identity', undefined, 0.05, false, 'desc')
      assert.deepStrictEqual(sortedDesc.map(x => x.label), ['Charlie', 'Beta', 'Alpha'])
    })

    it('sorts 5H quota ascending (0% -> 100%) and descending (100% -> 0%)', () => {
      const low = createAccount({ email: 'low@example.com', quota_5h_available: 0.10 })
      const mid = createAccount({ email: 'mid@example.com', quota_5h_available: 0.50 })
      const high = createAccount({ email: 'high@example.com', quota_5h_available: 0.90 })

      const sortedAsc = sortAccounts([mid, high, low], '', 0.05, 'quota_5h', undefined, 0.05, false, 'asc')
      assert.deepStrictEqual(sortedAsc.map(x => x.email), ['low@example.com', 'mid@example.com', 'high@example.com'])

      const sortedDesc = sortAccounts([mid, high, low], '', 0.05, 'quota_5h', undefined, 0.05, false, 'desc')
      assert.deepStrictEqual(sortedDesc.map(x => x.email), ['high@example.com', 'mid@example.com', 'low@example.com'])
    })

    it('sorts 7D quota ascending (0% -> 100%) and descending (100% -> 0%)', () => {
      const low = createAccount({ email: 'low@example.com', quota_weekly: 0.05 })
      const mid = createAccount({ email: 'mid@example.com', quota_weekly: 0.40 })
      const high = createAccount({ email: 'high@example.com', quota_weekly: 0.85 })

      const sortedAsc = sortAccounts([mid, high, low], '', 0.05, 'quota_weekly', undefined, 0.05, false, 'asc')
      assert.deepStrictEqual(sortedAsc.map(x => x.email), ['low@example.com', 'mid@example.com', 'high@example.com'])

      const sortedDesc = sortAccounts([mid, high, low], '', 0.05, 'quota_weekly', undefined, 0.05, false, 'desc')
      assert.deepStrictEqual(sortedDesc.map(x => x.email), ['high@example.com', 'mid@example.com', 'low@example.com'])
    })

    it('sorts credits ascending (lowest first) and descending (highest first)', () => {
      const low = createAccount({ email: 'low@example.com', credits: 0 })
      const mid = createAccount({ email: 'mid@example.com', credits: 5 })
      const high = createAccount({ email: 'high@example.com', credits: 20 })

      const sortedAsc = sortAccounts([mid, high, low], '', 0.05, 'credits', undefined, 0.05, false, 'asc')
      assert.deepStrictEqual(sortedAsc.map(x => x.email), ['low@example.com', 'mid@example.com', 'high@example.com'])

      const sortedDesc = sortAccounts([mid, high, low], '', 0.05, 'credits', undefined, 0.05, false, 'desc')
      assert.deepStrictEqual(sortedDesc.map(x => x.email), ['high@example.com', 'mid@example.com', 'low@example.com'])
    })

    it('sorts priority ascending (Low->Mid->High) and descending (High->Mid->Low)', () => {
      const low = createAccount({ email: 'low@example.com', priority: 'Low' })
      const mid = createAccount({ email: 'mid@example.com', priority: 'Mid' })
      const high = createAccount({ email: 'high@example.com', priority: 'High' })

      const sortedAsc = sortAccounts([mid, high, low], '', 0.05, 'priority', undefined, 0.05, false, 'asc')
      assert.deepStrictEqual(sortedAsc.map(x => x.priority), ['Low', 'Mid', 'High'])

      const sortedDesc = sortAccounts([mid, high, low], '', 0.05, 'priority', undefined, 0.05, false, 'desc')
      assert.deepStrictEqual(sortedDesc.map(x => x.priority), ['High', 'Mid', 'Low'])
    })
  })
})

import { describe, it } from 'node:test'
import assert from 'node:assert'
import {
  parseHorizonTextSeconds,
  normalizeSwitchMode,
  isFreePlanTier,
  planTierRank,
  planTierCapacityMultiplier,
  extractAccountMetrics,
  rankStandbyAccounts,
  shouldSwitchProactivelyMaxTokens,
  evaluateAutoSwitch,
  sortAccounts,
  computeEffectiveWeeklyAvailable,
} from './accountRanking.ts'
import type { AccountState } from '../types.ts'

describe('accountRanking utility', () => {
  describe('parseHorizonTextSeconds', () => {
    it('parses zero and readiness strings', () => {
      assert.strictEqual(parseHorizonTextSeconds(''), 0)
      assert.strictEqual(parseHorizonTextSeconds('Ready'), 0)
      assert.strictEqual(parseHorizonTextSeconds('Resets now'), 0)
      assert.strictEqual(parseHorizonTextSeconds('Not Polled'), 0)
    })

    it('parses seconds, minutes, hours, days', () => {
      assert.strictEqual(parseHorizonTextSeconds('Resets in 45s'), 45)
      assert.strictEqual(parseHorizonTextSeconds('Resets in 12m'), 720)
      assert.strictEqual(parseHorizonTextSeconds('Resets in 2h 15m'), 8100)
      assert.strictEqual(parseHorizonTextSeconds('Resets in 3d 5h'), 277200)
    })
  })

  describe('normalizeSwitchMode', () => {
    it('standardizes aliases and defaults to balanced', () => {
      assert.strictEqual(normalizeSwitchMode('balanced'), 'balanced')
      assert.strictEqual(normalizeSwitchMode('default'), 'balanced')
      assert.strictEqual(normalizeSwitchMode(''), 'balanced')
      assert.strictEqual(normalizeSwitchMode('max_tokens'), 'max_tokens')
      assert.strictEqual(normalizeSwitchMode('max_total_tokens'), 'max_tokens')
      assert.strictEqual(normalizeSwitchMode('tokens'), 'max_tokens')
      assert.strictEqual(normalizeSwitchMode('max_continuous'), 'max_continuous')
      assert.strictEqual(normalizeSwitchMode('continuous'), 'max_continuous')
    })
  })

  describe('plan tier metrics and free detection', () => {
    it('ranks plan tiers correctly and classifies free accounts', () => {
      assert.strictEqual(isFreePlanTier('free@gmail.com', 'Free'), true)
      assert.strictEqual(isFreePlanTier('user@gmail.com', ''), true)
      assert.strictEqual(isFreePlanTier('dev@gmail.com', 'Pro'), false)
      assert.strictEqual(isFreePlanTier('ultra@gmail.com', 'Ultra 20X'), false)

      assert.ok(planTierRank('Enterprise') > planTierRank('Ultra 20X'))
      assert.ok(planTierRank('Ultra 20X') > planTierRank('Ultra 10X'))
      assert.ok(planTierRank('Ultra 10X') > planTierRank('Ultra 5X'))
      assert.ok(planTierRank('Ultra 5X') > planTierRank('Pro'))
      assert.ok(planTierRank('Pro') > planTierRank('Edu'))
      assert.ok(planTierRank('Edu') > planTierRank('Pro - Trial'))
      assert.ok(planTierRank('Pro - Trial') > planTierRank('Plus'))
      assert.ok(planTierRank('Plus') > planTierRank('Free'))

      assert.strictEqual(planTierCapacityMultiplier('Ultra 20X'), 1.4)
      assert.strictEqual(planTierCapacityMultiplier('Pro'), 1.0)
      assert.strictEqual(planTierCapacityMultiplier('Free'), 0.25)

      const metrics = extractAccountMetrics(
        { email: 'user@test.com', plan_tier: 'Pro', quota_5h_current: 0.7, quota_weekly: 0.8, is_active: false, status: 'STANDBY', quota_5h_available: 0.7, reset_horizon_text: 'Ready', has_mfa: false },
        'balanced'
      )
      assert.strictEqual(metrics.tierRank, 4)
      assert.strictEqual(metrics.isFree, false)
    })
  })

  describe('rankStandbyAccounts', () => {
    it('strictly demotes 100% Free accounts behind healthy paid accounts', () => {
      const freeAcc: AccountState = {
        email: 'free@gmail.com',
        label: 'Free Account',
        plan_tier: 'Free',
        is_active: false,
        status: 'STANDBY',
        quota_5h_current: 1.0,
        quota_5h_available: 1.0,
        quota_weekly: 1.0,
        reset_horizon_text: 'Ready',
        has_mfa: false,
      }
      const proAcc: AccountState = {
        email: 'pro@gmail.com',
        label: 'Pro Account',
        plan_tier: 'Pro',
        is_active: false,
        status: 'STANDBY',
        quota_5h_current: 0.6,
        quota_5h_available: 0.6,
        quota_weekly: 0.7,
        reset_horizon_text: 'Ready',
        has_mfa: false,
      }

      const ranked = rankStandbyAccounts([freeAcc, proAcc], 0.05, 'balanced')
      assert.strictEqual(ranked.length, 2)
      assert.strictEqual(ranked[0].email, 'pro@gmail.com')
      assert.strictEqual(ranked[1].email, 'free@gmail.com')
    })

    it('prefers Ultra 20X over Pro in max_continuous mode when quotas are full', () => {
      const proAcc: AccountState = {
        email: 'pro@gmail.com',
        label: 'Pro Account',
        plan_tier: 'Pro',
        is_active: false,
        status: 'STANDBY',
        quota_5h_current: 0.95,
        quota_5h_available: 0.95,
        quota_weekly: 0.9,
        reset_horizon_text: 'Ready',
        has_mfa: false,
      }
      const ultraAcc: AccountState = {
        email: 'ultra@gmail.com',
        label: 'Ultra Account',
        plan_tier: 'Ultra 20X',
        is_active: false,
        status: 'STANDBY',
        quota_5h_current: 0.95,
        quota_5h_available: 0.95,
        quota_weekly: 0.9,
        reset_horizon_text: 'Ready',
        has_mfa: false,
      }

      const ranked = rankStandbyAccounts([proAcc, ultraAcc], 0.05, 'max_continuous')
      assert.strictEqual(ranked[0].email, 'ultra@gmail.com')
    })

    it('prefers earlier expiring reset windows in max_tokens mode', () => {
      const expiringSoon: AccountState = {
        email: 'soon@gmail.com',
        plan_tier: 'Pro',
        is_active: false,
        status: 'STANDBY',
        quota_5h_current: 0.8,
        reset_seconds: 1800, // 30m
        quota_5h_available: 0.96,
        quota_weekly: 0.8,
        reset_horizon_text: 'Resets in 30m',
        has_mfa: false,
      }
      const expiringLate: AccountState = {
        email: 'late@gmail.com',
        plan_tier: 'Pro',
        is_active: false,
        status: 'STANDBY',
        quota_5h_current: 0.8,
        reset_seconds: 14400, // 4h
        quota_5h_available: 0.84,
        quota_weekly: 0.8,
        reset_horizon_text: 'Resets in 4h',
        has_mfa: false,
      }

      const ranked = rankStandbyAccounts([expiringLate, expiringSoon], 0.05, 'max_tokens')
      assert.strictEqual(ranked[0].email, 'soon@gmail.com')
    })

    it('excludes BANNED, ERROR, and COOLDOWN accounts from standby candidates', () => {
      const healthyStandby: AccountState = {
        email: 'healthy@gmail.com',
        label: 'Healthy Standby',
        plan_tier: 'Pro',
        is_active: false,
        status: 'STANDBY',
        quota_5h_current: 0.9,
        quota_5h_available: 0.9,
        quota_weekly: 0.9,
        reset_horizon_text: 'Ready',
        has_mfa: false,
      }
      const cooldownAcc: AccountState = {
        email: 'cooldown@gmail.com',
        label: 'Cooldown Account',
        plan_tier: 'Pro',
        is_active: false,
        status: 'COOLDOWN',
        quota_5h_current: 0.95,
        quota_5h_available: 0.95,
        quota_weekly: 0.95,
        reset_horizon_text: 'Ready',
        has_mfa: false,
      }
      const bannedAcc: AccountState = {
        email: 'banned@gmail.com',
        label: 'Banned Account',
        plan_tier: 'Pro',
        is_active: false,
        status: 'BANNED',
        quota_5h_current: 0.95,
        quota_5h_available: 0.95,
        quota_weekly: 0.95,
        reset_horizon_text: 'Ready',
        has_mfa: false,
      }
      const errorAcc: AccountState = {
        email: 'error@gmail.com',
        label: 'Error Account',
        plan_tier: 'Pro',
        is_active: false,
        status: 'ERROR',
        quota_5h_current: 0.95,
        quota_5h_available: 0.95,
        quota_weekly: 0.95,
        reset_horizon_text: 'Ready',
        has_mfa: false,
      }

      const ranked = rankStandbyAccounts([cooldownAcc, bannedAcc, errorAcc, healthyStandby], 0.05, 'balanced')
      assert.strictEqual(ranked.length, 1)
      assert.strictEqual(ranked[0].email, 'healthy@gmail.com')

      const coolingAcc: AccountState = {
        ...cooldownAcc,
        email: 'cooling@gmail.com',
        status: 'COOLING',
      }
      const rankedWithCooling = rankStandbyAccounts([coolingAcc, healthyStandby], 0.05, 'balanced')
      assert.strictEqual(rankedWithCooling.length, 1)
      assert.strictEqual(rankedWithCooling[0].email, 'healthy@gmail.com')
    })

    it('excludes standby accounts below thresholdWeekly unless credit overages are enabled', () => {
      const lowWeeklyStandby: AccountState = {
        email: 'lowweekly@gmail.com',
        label: 'Low Weekly Account',
        plan_tier: 'Pro',
        is_active: false,
        status: 'STANDBY',
        quota_5h_current: 1.0,
        quota_5h_available: 1.0,
        quota_weekly: 0.04,
        reset_horizon_text: 'Ready',
        has_mfa: false,
      }
      const ranked = rankStandbyAccounts([lowWeeklyStandby], 0.05, 'balanced', 0.05)
      assert.strictEqual(ranked.length, 0)

      const overageStandby: AccountState = {
        ...lowWeeklyStandby,
        enable_credit_overages: true,
        credits: 50,
      }
      const rankedOverage = rankStandbyAccounts([overageStandby], 0.05, 'balanced', 0.05)
      assert.strictEqual(rankedOverage.length, 1)
    })
  })

  describe('evaluateAutoSwitch & proactive rotation', () => {
    const active: AccountState = {
      email: 'active@gmail.com',
      is_active: true,
      plan_tier: 'Pro',
      status: 'ACTIVE',
      quota_5h_current: 0.8,
      reset_seconds: 7200,
      quota_5h_available: 0.8,
      quota_weekly: 0.85,
      reset_horizon_text: 'Resets in 2h',
      has_mfa: false,
    }
    const standbyReady: AccountState = {
      email: 'standby@gmail.com',
      is_active: false,
      plan_tier: 'Pro',
      status: 'STANDBY',
      quota_5h_current: 1.0,
      reset_seconds: 0,
      quota_5h_available: 1.0,
      quota_weekly: 0.9,
      reset_horizon_text: 'Ready',
      has_mfa: false,
    }

    it('does not proactively switch in balanced mode with healthy quota', () => {
      const res = evaluateAutoSwitch([active, standbyReady], active.email, 0.05, 'balanced', 1200)
      assert.strictEqual(res.shouldSwitch, false)
    })

    it('does not proactively switch in max_tokens when dwell is under 10 minutes', () => {
      const res = evaluateAutoSwitch([active, standbyReady], active.email, 0.05, 'max_tokens', 300)
      assert.strictEqual(res.shouldSwitch, false)
      assert.ok(res.reason.includes('healthy') || res.reason.includes('dwell'))
    })

    it('proactively switches in max_tokens to ignite 100% idle clock when dwell >= 600s', () => {
      const direct = shouldSwitchProactivelyMaxTokens(active, standbyReady, 0.05, 700)
      assert.strictEqual(direct.shouldSwitch, true)
      assert.ok(direct.reason.includes('ignite'))

      const res = evaluateAutoSwitch([active, standbyReady], active.email, 0.05, 'max_tokens', 700)
      assert.strictEqual(res.shouldSwitch, true)
      assert.strictEqual(res.successor?.email, 'standby@gmail.com')
      assert.ok(res.reason.includes('ignite'))
    })

    it('does not switch in max_tokens when active 5h reset is imminent (<= 45m)', () => {
      const imminentActive: AccountState = {
        ...active,
        reset_seconds: 1200, // 20m remaining
        reset_horizon_text: 'Resets in 20m',
      }
      const res = evaluateAutoSwitch([imminentActive, standbyReady], active.email, 0.05, 'max_tokens', 800)
      assert.strictEqual(res.shouldSwitch, false)
    })

    it('switches on threshold breach across all switch modes', () => {
      const exhaustedActive: AccountState = {
        ...active,
        quota_5h_current: 0.03, // breached <= 0.05
      }
      const res = evaluateAutoSwitch([exhaustedActive, standbyReady], active.email, 0.05, 'max_continuous', 30)
      assert.strictEqual(res.shouldSwitch, true)
      assert.strictEqual(res.successor?.email, 'standby@gmail.com')
      assert.ok(res.reason.includes('dropped below threshold'))
    })

    it('switches on weekly quota threshold breach even when 5h quota is 100%', () => {
      const weeklyDepletedActive: AccountState = {
        ...active,
        quota_5h_current: 1.0,
        quota_5h_available: 1.0,
        quota_weekly: 0.04, // breached <= 0.05
      }
      const res = evaluateAutoSwitch([weeklyDepletedActive, standbyReady], active.email, 0.05, 'balanced', 0, 0.05)
      assert.strictEqual(res.shouldSwitch, true)
      assert.strictEqual(res.successor?.email, 'standby@gmail.com')
      assert.ok(res.reason.includes('Active weekly quota (4.0%)'))
      assert.ok(res.reason.includes('dropped below threshold'))
    })
  })

  describe('sortAccounts', () => {
    it('maintains 6 structural tiers in auto mode with switchMode', () => {
      const active: AccountState = {
        email: 'active@gmail.com',
        is_active: true,
        plan_tier: 'Pro',
        quota_5h_current: 0.8,
        quota_5h_available: 0.8,
        quota_weekly: 0.8,
        reset_horizon_text: 'Ready',
        has_mfa: false,
        status: 'ACTIVE',
      }
      const ultraStandby: AccountState = {
        email: 'ultra@gmail.com',
        is_active: false,
        plan_tier: 'Ultra 20X',
        quota_5h_current: 0.9,
        quota_5h_available: 0.9,
        quota_weekly: 0.9,
        reset_horizon_text: 'Ready',
        has_mfa: false,
        status: 'STANDBY',
      }
      const proStandby: AccountState = {
        email: 'pro@gmail.com',
        is_active: false,
        plan_tier: 'Pro',
        quota_5h_current: 0.85,
        quota_5h_available: 0.85,
        quota_weekly: 0.85,
        reset_horizon_text: 'Ready',
        has_mfa: false,
        status: 'STANDBY',
      }
      const freeStandby: AccountState = {
        email: 'free@gmail.com',
        is_active: false,
        plan_tier: 'Free',
        quota_5h_current: 1.0,
        quota_5h_available: 1.0,
        quota_weekly: 1.0,
        reset_horizon_text: 'Ready',
        has_mfa: false,
        status: 'STANDBY',
      }
      const cooling: AccountState = {
        email: 'cooling@gmail.com',
        is_active: false,
        plan_tier: 'Pro',
        quota_5h_current: 0.02,
        quota_5h_available: 0.8,
        quota_weekly: 0.8,
        reset_horizon_text: 'Ready',
        has_mfa: false,
        status: 'STANDBY',
      }
      const errAcc: AccountState = {
        email: 'err@gmail.com',
        is_active: false,
        status: 'ERROR',
        quota_5h_current: 1.0,
        quota_5h_available: 1.0,
        quota_weekly: 1.0,
        reset_horizon_text: 'Ready',
        has_mfa: false,
      }
      const banAcc: AccountState = {
        email: 'ban@gmail.com',
        is_active: false,
        status: 'BANNED',
        quota_5h_current: 1.0,
        quota_5h_available: 1.0,
        quota_weekly: 1.0,
        reset_horizon_text: 'Ready',
        has_mfa: false,
      }

      const list = [banAcc, freeStandby, proStandby, errAcc, cooling, active, ultraStandby]
      const sorted = sortAccounts(list, active.email, 0.05, 'auto', 'max_continuous')

      assert.strictEqual(sorted.length, 7)
      assert.strictEqual(sorted[0].email, 'active@gmail.com') // Tier 0
      assert.strictEqual(sorted[1].email, 'ultra@gmail.com') // Tier 1 (Ultra over Pro)
      assert.strictEqual(sorted[2].email, 'pro@gmail.com') // Tier 1
      assert.strictEqual(sorted[3].email, 'free@gmail.com') // Tier 2 (Free ranked last among standbys)
      assert.strictEqual(sorted[4].email, 'cooling@gmail.com') // Tier 3
      assert.strictEqual(sorted[5].email, 'err@gmail.com') // Tier 4
      assert.strictEqual(sorted[6].email, 'ban@gmail.com') // Tier 5
    })

    it('ranks accounts with status COOLDOWN strictly in Tier 3', () => {
      const active: AccountState = {
        email: 'active@gmail.com',
        is_active: true,
        plan_tier: 'Pro',
        status: 'ACTIVE',
        quota_5h_current: 0.8,
        quota_5h_available: 0.8,
        quota_weekly: 0.8,
        reset_horizon_text: 'Ready',
        has_mfa: false,
      }
      const proStandby: AccountState = {
        email: 'pro@gmail.com',
        is_active: false,
        plan_tier: 'Pro',
        status: 'STANDBY',
        quota_5h_current: 0.8,
        quota_5h_available: 0.8,
        quota_weekly: 0.8,
        reset_horizon_text: 'Ready',
        has_mfa: false,
      }
      const freeStandby: AccountState = {
        email: 'free@gmail.com',
        is_active: false,
        plan_tier: 'Free',
        status: 'STANDBY',
        quota_5h_current: 0.8,
        quota_5h_available: 0.8,
        quota_weekly: 0.8,
        reset_horizon_text: 'Ready',
        has_mfa: false,
      }
      const cooldownAcc: AccountState = {
        email: 'cooldown@gmail.com',
        is_active: false,
        plan_tier: 'Pro',
        status: 'COOLDOWN',
        quota_5h_current: 0.02,
        quota_5h_available: 0.7,
        quota_weekly: 0.7,
        reset_horizon_text: 'Resets in 3h',
        has_mfa: false,
      }
      const errorAcc: AccountState = {
        email: 'error@gmail.com',
        is_active: false,
        plan_tier: 'Pro',
        status: 'ERROR',
        quota_5h_current: 0.8,
        quota_5h_available: 0.8,
        quota_weekly: 0.8,
        reset_horizon_text: 'Ready',
        has_mfa: false,
      }
      const bannedAcc: AccountState = {
        email: 'banned@gmail.com',
        is_active: false,
        plan_tier: 'Pro',
        status: 'BANNED',
        quota_5h_current: 0.8,
        quota_5h_available: 0.8,
        quota_weekly: 0.8,
        reset_horizon_text: 'Ready',
        has_mfa: false,
      }

      const sorted = sortAccounts(
        [bannedAcc, errorAcc, cooldownAcc, freeStandby, proStandby, active],
        active.email,
        0.05,
        'auto',
        'balanced'
      )

      assert.strictEqual(sorted.length, 6)
      assert.strictEqual(sorted[0].email, 'active@gmail.com')   // Tier 0
      assert.strictEqual(sorted[1].email, 'pro@gmail.com')      // Tier 1
      assert.strictEqual(sorted[2].email, 'free@gmail.com')     // Tier 2
      assert.strictEqual(sorted[3].email, 'cooldown@gmail.com') // Tier 3
      assert.strictEqual(sorted[4].email, 'error@gmail.com')    // Tier 4
      assert.strictEqual(sorted[5].email, 'banned@gmail.com')   // Tier 5

      const coolingOnlyAcc = { ...cooldownAcc, email: 'cooling_only@gmail.com', status: 'COOLING' as const }
      const sortedCooling = sortAccounts(
        [bannedAcc, errorAcc, coolingOnlyAcc, freeStandby, proStandby, active],
        'active@gmail.com',
        0.05,
        'auto',
        'balanced'
      )
      assert.strictEqual(sortedCooling[3].email, 'cooling_only@gmail.com') // Tier 3
    })

    it('demotes standby accounts below thresholdWeekly into Tier 3', () => {
      const active: AccountState = {
        email: 'active@gmail.com',
        is_active: true,
        plan_tier: 'Pro',
        status: 'ACTIVE',
        quota_5h_current: 0.8,
        quota_5h_available: 0.8,
        quota_weekly: 0.8,
        reset_horizon_text: 'Ready',
        has_mfa: false,
      }
      const healthyStandby: AccountState = {
        email: 'healthy@gmail.com',
        is_active: false,
        plan_tier: 'Pro',
        status: 'STANDBY',
        quota_5h_current: 0.6,
        quota_5h_available: 0.6,
        quota_weekly: 0.8,
        reset_horizon_text: 'Ready',
        has_mfa: false,
      }
      const lowWeeklyStandby: AccountState = {
        email: 'lowweekly@gmail.com',
        is_active: false,
        plan_tier: 'Pro',
        status: 'STANDBY',
        quota_5h_current: 0.9,
        quota_5h_available: 0.9,
        quota_weekly: 0.04, // <= 0.05
        reset_horizon_text: 'Ready',
        has_mfa: false,
      }

      const sorted = sortAccounts(
        [lowWeeklyStandby, healthyStandby, active],
        active.email,
        0.05,
        'auto',
        'balanced',
        0.05
      )

      assert.strictEqual(sorted[0].email, 'active@gmail.com')
      assert.strictEqual(sorted[1].email, 'healthy@gmail.com')
      assert.strictEqual(sorted[2].email, 'lowweekly@gmail.com')
    })

    it('ranks 5h-cooling accounts with healthy weekly quota (Satya, PRWH) strictly above weekly-depleted accounts (Albert)', () => {
      const active: AccountState = {
        email: 'jose@gmail.com',
        is_active: true,
        plan_tier: 'Pro',
        status: 'COOLDOWN',
        quota_5h_current: 0.64,
        quota_5h_available: 0.64,
        quota_weekly: 0.0,
        reset_horizon_text: 'Ready',
        has_mfa: false,
      }
      const albert: AccountState = {
        email: 'alberto.carey8718@gmail.com',
        is_active: false,
        plan_tier: 'Pro',
        status: 'COOLDOWN',
        quota_5h_current: 0.0,
        quota_5h_available: 0.86,
        quota_weekly: 0.01, // 1% weekly - exhausted!
        reset_seconds: 2400, // resets in 40m
        reset_horizon_text: 'Resets in 40m',
        has_mfa: false,
      }
      const satya: AccountState = {
        email: 'satyaprakash78447@gmail.com',
        is_active: false,
        plan_tier: 'Pro',
        status: 'COOLDOWN',
        quota_5h_current: 0.0,
        quota_5h_available: 0.46,
        quota_weekly: 0.33, // 33% weekly - healthy!
        reset_seconds: 9720, // resets in 2h 42m
        reset_horizon_text: 'Resets in 2h 42m',
        has_mfa: false,
      }
      const prwh: AccountState = {
        email: 'prwh.dpl@gmail.com',
        is_active: false,
        plan_tier: 'Pro',
        status: 'STANDBY',
        quota_5h_current: 0.0,
        quota_5h_available: 0.34,
        quota_weekly: 0.83, // 83% weekly - tons of quota!
        reset_seconds: 12000, // resets in 3h 20m
        reset_horizon_text: 'Resets in 3h 20m',
        has_mfa: false,
      }

      // In balanced mode: PRWH (83% weekly) ranks top among cooling, Satya second, Albert last
      const sortedBal = sortAccounts(
        [albert, satya, prwh, active],
        active.email,
        0.05,
        'auto',
        'balanced',
        0.05
      )
      assert.strictEqual(sortedBal[0].email, 'jose@gmail.com', 'Active account pinned to row 0')
      assert.strictEqual(sortedBal[1].email, 'prwh.dpl@gmail.com', 'PRWH with 83% weekly quota ranks ahead')
      assert.strictEqual(sortedBal[2].email, 'satyaprakash78447@gmail.com', 'Satya with 33% weekly quota ranks ahead of Albert')
      assert.strictEqual(sortedBal[3].email, 'alberto.carey8718@gmail.com', 'Albert with 1% weekly quota demoted to weekly-depleted tier')

      // In max_continuous mode: Satya (sooner 5h recovery, 46% projected) ranks ahead of PRWH (34%), Albert still last
      const sortedCont = sortAccounts(
        [albert, satya, prwh, active],
        active.email,
        0.05,
        'auto',
        'max_continuous',
        0.05
      )
      assert.strictEqual(sortedCont[0].email, 'jose@gmail.com')
      assert.strictEqual(sortedCont[1].email, 'satyaprakash78447@gmail.com', 'Satya recovers 5h capacity earlier in continuous window')
      assert.strictEqual(sortedCont[2].email, 'prwh.dpl@gmail.com', 'PRWH recovers after Satya')
      assert.strictEqual(sortedCont[3].email, 'alberto.carey8718@gmail.com', 'Albert cannot provide continuous quota with 1% weekly limit')
    })

    it('extractAccountMetrics caps 5h available quota by weekly quota when weekly is depleted', () => {
      const albert: AccountState = {
        email: 'alberto.carey8718@gmail.com',
        is_active: false,
        plan_tier: 'Pro',
        status: 'COOLDOWN',
        quota_5h_current: 0.0,
        quota_5h_available: 0.86,
        quota_weekly: 0.01,
        reset_seconds: 2400,
        reset_horizon_text: 'Resets in 40m',
        reset_seconds_weekly: 300000, // 3.5 days away
        has_mfa: false,
      }
      const metrics = extractAccountMetrics(albert, 'balanced')
      assert.strictEqual(metrics.q7d, 0.01)
      assert.ok(metrics.q5hAvail <= 0.01, `Expected effective 5h available to be capped at weekly quota 0.01, got ${metrics.q5hAvail}`)
    })

    it('ranks Jose Antonio (84% 5h, 0% weekly) ahead of Albert (0% 5h, 1% weekly) due to 5h readiness in Tier 4', () => {
      const jose: AccountState = {
        email: 'jose@gmail.com',
        is_active: false,
        plan_tier: 'Pro',
        status: 'COOLDOWN',
        quota_5h_current: 0.84,
        quota_5h_available: 0.84,
        quota_weekly: 0.0,
        reset_seconds_weekly: 300000,
        reset_horizon_text: 'Ready',
        has_mfa: false,
      }
      const albert: AccountState = {
        email: 'albert@gmail.com',
        is_active: false,
        plan_tier: 'Pro',
        status: 'COOLDOWN',
        quota_5h_current: 0.0,
        quota_5h_available: 0.86,
        quota_weekly: 0.01,
        reset_seconds: 2400,
        reset_seconds_weekly: 300000,
        reset_horizon_text: 'Resets in 40m',
        has_mfa: false,
      }

      const sorted = sortAccounts([albert, jose], '', 0.05, 'auto', 'balanced', 0.05)
      assert.strictEqual(sorted[0].email, 'jose@gmail.com', 'Jose Antonio with 84% 5h quota should outrank Albert with 0% 5h quota')
      assert.strictEqual(sorted[1].email, 'albert@gmail.com')
    })

    it('places account with weekly quota recovering within 5h into Tier 3 rather than Tier 4', () => {
      const weeklyRecovering: AccountState = {
        email: 'recovering@gmail.com',
        is_active: false,
        plan_tier: 'Pro',
        status: 'COOLDOWN',
        quota_5h_current: 0.0,
        quota_5h_available: 0.5,
        quota_weekly: 0.0,
        reset_seconds: 3600,
        reset_seconds_weekly: 1800, // Weekly resets in 30 minutes!
        reset_horizon_text: 'Resets in 1h',
        has_mfa: false,
      }
      const weeklyDepleted: AccountState = {
        email: 'depleted@gmail.com',
        is_active: false,
        plan_tier: 'Pro',
        status: 'COOLDOWN',
        quota_5h_current: 0.0,
        quota_5h_available: 0.5,
        quota_weekly: 0.01,
        reset_seconds: 3600,
        reset_seconds_weekly: 400000, // 4.5 days away
        reset_horizon_text: 'Resets in 1h',
        has_mfa: false,
      }

      const sorted = sortAccounts([weeklyDepleted, weeklyRecovering], '', 0.05, 'auto', 'balanced', 0.05)
      assert.strictEqual(sorted[0].email, 'recovering@gmail.com', 'Account recovering weekly quota in 30m enters Tier 3')
      assert.strictEqual(sorted[1].email, 'depleted@gmail.com', 'Account depleted for 4.5 days remains in Tier 4')
    })

    it('bypasses weekly cap when enable_credit_overages is true with positive credits', () => {
      const creditAcc: AccountState = {
        email: 'credits@gmail.com',
        is_active: false,
        plan_tier: 'Pro',
        status: 'STANDBY',
        quota_5h_current: 0.8,
        quota_5h_available: 0.8,
        quota_weekly: 0.0,
        enable_credit_overages: true,
        credits: 100,
        reset_horizon_text: 'Ready',
        has_mfa: false,
      }
      const metrics = extractAccountMetrics(creditAcc, 'balanced')
      assert.strictEqual(metrics.q5hAvail, 0.8, 'Credits allow 5h available to not be capped by weekly 0')

      const sorted = sortAccounts([creditAcc], '', 0.05, 'auto', 'balanced', 0.05)
      // Should be in Tier 1 (Healthy Paid Standby) because hasWeekly is true via credits
      assert.strictEqual(sorted[0].email, 'credits@gmail.com')
    })

    it('computes computeEffectiveWeeklyAvailable smoothly at 18000s boundary', () => {
      const atBoundary = computeEffectiveWeeklyAvailable(0.02, 18000)
      assert.strictEqual(atBoundary, 0.02, 'At exactly 5h boundary boost is 0')
      const insideWindow = computeEffectiveWeeklyAvailable(0.0, 3600)
      assert.strictEqual(insideWindow, 0.8, '1 hour reset provides 80% replenishment across 5h window')
    })
  })
})

import { describe, it } from 'node:test'
import assert from 'node:assert'
import { toAccountState, isProOrAbovePlan } from '../types.ts'
import type { AccountState } from '../types.ts'
import {
  rankStandbyAccounts,
  evaluateAutoSwitch,
  sortAccounts,
} from './accountRanking.ts'

describe('Dual-Engine Model Routing & Quota Cooldown Switchability (R1)', () => {
  describe('toAccountState transformation', () => {
    it('preserves quota_5h_claude_gpt and quota_weekly_claude_gpt', () => {
      const raw = {
        email: 'user@example.com',
        label: 'Pro Account',
        quota_5h_current: 0.1,
        quota_weekly: 0.2,
        quota_5h_claude_gpt: 0.85,
        quota_weekly_claude_gpt: 0.90,
        status: 'STANDBY',
      }
      const state = toAccountState(raw)
      assert.strictEqual(state.quota_5h_claude_gpt, 0.85)
      assert.strictEqual(state.quota_weekly_claude_gpt, 0.90)
    })

    it('preserves the separate Gemini 4 Argon 5H and weekly quotas', () => {
      const raw = {
        email: 'argon@example.com',
        plan_tier: 'Pro',
        quota_5h_argon: 0.44,
        quota_weekly: 0.72,
        quota_weekly_argon: 0.31,
        reset_seconds_5h_argon: 12000,
        reset_seconds_weekly_argon: 374400,
        reset_horizon_5h_argon_text: 'Resets in 3h 20m',
        reset_horizon_weekly_argon_text: 'Resets in 4d 8h',
        status: 'STANDBY',
      }
      const state = toAccountState(raw)
      assert.strictEqual(state.quota_5h_argon, 0.44)
      assert.strictEqual(state.quota_weekly, 0.72)
      assert.strictEqual(state.quota_weekly_argon, 0.31)
      assert.strictEqual(state.reset_seconds_5h_argon, 12000)
      assert.strictEqual(state.reset_5h_argon_seconds, 12000)
      assert.strictEqual(state.reset_seconds_weekly_argon, 374400)
      assert.strictEqual(state.reset_weekly_argon_seconds, 374400)
      assert.strictEqual(state.reset_horizon_5h_argon_text, 'Resets in 3h 20m')
      assert.strictEqual(state.reset_horizon_weekly_argon_text, 'Resets in 4d 8h')
    })

    it('preserves cached 100% full Argon quota with 0 reset seconds immediately on startup', () => {
      const raw = {
        email: 'argon-full@example.com',
        plan_tier: 'Pro',
        quota_5h_argon: 1.0,
        quota_weekly: 1.0,
        quota_weekly_argon: 1.0,
        reset_seconds_5h_argon: 0,
        reset_seconds_weekly_argon: 0,
        status: 'STANDBY',
      }
      const state = toAccountState(raw)
      assert.strictEqual(state.quota_5h_argon, 1.0)
      assert.strictEqual(state.quota_weekly_argon, 1.0)
      assert.strictEqual(state.reset_seconds_5h_argon, 0)
      assert.strictEqual(state.reset_seconds_weekly_argon, 0)
      assert.strictEqual(state.reset_horizon_5h_argon_text, 'Ready')
      assert.strictEqual(state.reset_horizon_weekly_argon_text, 'Ready')
    })

    it('maps raw.quota_5h_argon when nested under raw object', () => {
      const input = {
        email: 'nested-raw@example.com',
        plan_tier: 'Pro',
        raw: {
          quota_5h_argon: 0.88,
          quota_weekly_argon: 0.92,
        },
        status: 'STANDBY',
      }
      const state = toAccountState(input)
      assert.strictEqual(state.quota_5h_argon, 0.88)
      assert.strictEqual(state.quota_weekly_argon, 0.92)
    })

    it('keeps Argon null for Pro accounts when the API omits Argon buckets', () => {
      const raw = {
        email: 'pro@example.com',
        plan_tier: 'Pro',
        quota_5h_available: 1.0,
        quota_weekly: 0.51,
        status: 'STANDBY',
      }
      const state = toAccountState(raw)
      assert.strictEqual(state.quota_5h_argon, null, 'Pro account must not synthesize Argon 5H quota')
      assert.strictEqual(state.quota_weekly_argon, null, 'Pro account must not synthesize Argon 7D quota')
    })

    it('keeps Argon null for Free accounts when Argon is null', () => {
      const raw = {
        email: 'free@example.com',
        plan_tier: 'Free',
        quota_5h_available: 1.0,
        quota_weekly: 0.51,
        status: 'STANDBY',
      }
      const state = toAccountState(raw)
      assert.strictEqual(state.quota_5h_argon, null, 'Free account must not inherit Argon quota')
      assert.strictEqual(state.quota_weekly_argon, null, 'Free account must not inherit Argon quota')
    })

    it('validates isProOrAbovePlan tier classifications', () => {
      assert.strictEqual(isProOrAbovePlan('Pro'), true)
      assert.strictEqual(isProOrAbovePlan('pro'), true)
      assert.strictEqual(isProOrAbovePlan('Google AI Pro'), true)
      assert.strictEqual(isProOrAbovePlan('Pro - Trial'), true)
      assert.strictEqual(isProOrAbovePlan('trial'), true)
      assert.strictEqual(isProOrAbovePlan('Ultra 5X'), true)
      assert.strictEqual(isProOrAbovePlan('Ultra 10X'), true)
      assert.strictEqual(isProOrAbovePlan('Ultra 20X'), true)
      assert.strictEqual(isProOrAbovePlan('Enterprise'), true)
      assert.strictEqual(isProOrAbovePlan('Free'), false)
      assert.strictEqual(isProOrAbovePlan('free'), false)
      assert.strictEqual(isProOrAbovePlan('Plus'), false)
      assert.strictEqual(isProOrAbovePlan('Edu'), false)
    })

    it('falls back to quota_5h_claude and quota_weekly_claude aliases', () => {
      const raw = {
        email: 'user@example.com',
        quota_5h_claude: 0.77,
        quota_weekly_claude: 0.88,
        status: 'STANDBY',
      }
      const state = toAccountState(raw)
      assert.strictEqual(state.quota_5h_claude_gpt, 0.77)
      assert.strictEqual(state.quota_weekly_claude_gpt, 0.88)
    })

    it('recovers COOLING account to STANDBY when non-Gemini routing is enabled and Claude quota is available', () => {
      const rawCooling = {
        email: 'cooling-dual@example.com',
        status: 'COOLING',
        quota_5h_current: 0.01,
        quota_weekly: 0.02,
        quota_5h_claude_gpt: 0.80,
        quota_weekly_claude_gpt: 0.85,
      }

      // 1. Without allowNonGemini: stays COOLING
      const stateStandard = toAccountState(rawCooling, '', false)
      assert.strictEqual(stateStandard.status, 'COOLING')

      // 2. With allowNonGemini = true: recovers to STANDBY
      const stateDual = toAccountState(rawCooling, '', true)
      assert.strictEqual(stateDual.status, 'STANDBY')
    })

    it('recovers active COOLING account to ACTIVE when non-Gemini routing is enabled', () => {
      const rawActiveCooling = {
        email: 'active-cooling@example.com',
        is_active: true,
        status: 'COOLING',
        quota_5h_current: 0.01,
        quota_weekly: 0.02,
        quota_5h_claude_gpt: 0.75,
        quota_weekly_claude_gpt: 0.80,
      }
      const state = toAccountState(rawActiveCooling, 'active-cooling@example.com', true)
      assert.strictEqual(state.status, 'ACTIVE')
    })

    it('keeps COOLING status if both Gemini and non-Gemini quotas are depleted', () => {
      const rawDepleted = {
        email: 'depleted@example.com',
        status: 'COOLING',
        quota_5h_current: 0.01,
        quota_weekly: 0.01,
        quota_5h_claude_gpt: 0.02,
        quota_weekly_claude_gpt: 0.03,
      }
      const state = toAccountState(rawDepleted, '', true)
      assert.strictEqual(state.status, 'COOLING')
    })

    it('keeps COOLING status when Gemini is depleted and Claude weekly is 0.00 without credits', () => {
      const rawClaudeWeeklyDepleted = {
        email: 'depleted-claude-weekly@example.com',
        status: 'COOLING',
        quota_5h_current: 0.01,
        quota_weekly: 0.01,
        quota_5h_claude_gpt: 0.85,
        quota_weekly_claude_gpt: 0.00,
      }
      const state = toAccountState(rawClaudeWeeklyDepleted, '', true)
      assert.strictEqual(state.status, 'COOLING', '0% weekly Claude quota must NOT be treated as available without credits')
    })

    it('recovers to STANDBY when Claude weekly is 0.00 but credit overages are enabled with positive credits', () => {
      const rawOverages = {
        email: 'overages@example.com',
        status: 'COOLING',
        quota_5h_current: 0.01,
        quota_weekly: 0.01,
        quota_5h_claude_gpt: 0.85,
        quota_weekly_claude_gpt: 0.00,
        enable_credit_overages: true,
        credits: 15.0,
      }
      const state = toAccountState(rawOverages, '', true)
      assert.strictEqual(state.status, 'STANDBY', 'Credit overages allow standby recovery even with 0% weekly quota')
    })

    it('recovers to STANDBY when Claude weekly is undefined (unconstrained model)', () => {
      const rawUnconstrained = {
        email: 'unconstrained@example.com',
        status: 'COOLING',
        quota_5h_current: 0.01,
        quota_weekly: 0.01,
        quota_5h_claude_gpt: 0.85,
      }
      const state = toAccountState(rawUnconstrained, '', true)
      assert.strictEqual(state.status, 'STANDBY', 'Unconstrained weekly Claude quota recovers to STANDBY')
    })
  })

  describe('rankStandbyAccounts eligibility in dual-engine mode', () => {
    const accGeminiOnly: AccountState = {
      email: 'gemini-only@example.com',
      is_active: false,
      plan_tier: 'Pro',
      status: 'STANDBY',
      quota_5h_current: 0.70,
      quota_5h_available: 0.70,
      quota_weekly: 0.80,
      quota_5h_claude_gpt: 0.0,
      quota_weekly_claude_gpt: 0.0,
      reset_horizon_text: 'Ready',
      has_mfa: false,
    }

    const accClaudeOnly: AccountState = {
      email: 'claude-only@example.com',
      is_active: false,
      plan_tier: 'Pro',
      status: 'STANDBY',
      quota_5h_current: 0.01,
      quota_5h_available: 0.01,
      quota_weekly: 0.01,
      quota_5h_claude_gpt: 0.85,
      quota_weekly_claude_gpt: 0.90,
      reset_horizon_text: 'Ready',
      has_mfa: false,
    }

    const accBothDepleted: AccountState = {
      email: 'both-depleted@example.com',
      is_active: false,
      plan_tier: 'Pro',
      status: 'COOLING',
      quota_5h_current: 0.01,
      quota_5h_available: 0.01,
      quota_weekly: 0.01,
      quota_5h_claude_gpt: 0.01,
      quota_weekly_claude_gpt: 0.01,
      reset_horizon_text: 'Resets in 2h',
      has_mfa: false,
    }

    it('excludes Gemini-depleted accounts when allowNonGemini is false', () => {
      const candidates = rankStandbyAccounts([accGeminiOnly, accClaudeOnly, accBothDepleted], 0.05, 'balanced', 0.05, false)
      assert.strictEqual(candidates.length, 1)
      assert.strictEqual(candidates[0].email, 'gemini-only@example.com')
    })

    it('qualifies Claude-available accounts as eligible standby candidates when allowNonGemini is true', () => {
      const candidates = rankStandbyAccounts([accGeminiOnly, accClaudeOnly, accBothDepleted], 0.05, 'balanced', 0.05, true)
      assert.strictEqual(candidates.length, 2)
      const emails = candidates.map((c) => c.email)
      assert.ok(emails.includes('gemini-only@example.com'))
      assert.ok(emails.includes('claude-only@example.com'))
      assert.ok(!emails.includes('both-depleted@example.com'))
    })

    it('excludes accounts with Claude weekly = 0.00 without credits but includes those with credits', () => {
      const accClaudeWeeklyDepleted: AccountState = {
        email: 'claude-depleted-weekly@example.com',
        is_active: false,
        plan_tier: 'Pro',
        status: 'STANDBY',
        quota_5h_current: 0.01,
        quota_5h_available: 0.01,
        quota_weekly: 0.01,
        quota_5h_claude_gpt: 0.85,
        quota_weekly_claude_gpt: 0.00,
        reset_horizon_text: 'Ready',
        has_mfa: false,
      }
      const accClaudeWithCredits: AccountState = {
        ...accClaudeWeeklyDepleted,
        email: 'claude-with-credits@example.com',
        enable_credit_overages: true,
        credits: 10.0,
      }

      const candidates = rankStandbyAccounts([accClaudeWeeklyDepleted, accClaudeWithCredits], 0.05, 'balanced', 0.05, true)
      assert.strictEqual(candidates.length, 1)
      assert.strictEqual(candidates[0].email, 'claude-with-credits@example.com')
    })
  })

  describe('evaluateAutoSwitch under dual-engine routing', () => {
    const activeDepletedGemini: AccountState = {
      email: 'active@example.com',
      is_active: true,
      plan_tier: 'Pro',
      status: 'ACTIVE',
      quota_5h_current: 0.02,
      quota_5h_available: 0.02,
      quota_weekly: 0.02,
      quota_5h_claude_gpt: 0.80,
      quota_weekly_claude_gpt: 0.85,
      reset_horizon_text: 'Resets in 3h',
      has_mfa: false,
    }

    const standbyAccount: AccountState = {
      email: 'standby@example.com',
      is_active: false,
      plan_tier: 'Pro',
      status: 'STANDBY',
      quota_5h_current: 0.90,
      quota_5h_available: 0.90,
      quota_weekly: 0.95,
      quota_5h_claude_gpt: 0.90,
      quota_weekly_claude_gpt: 0.95,
      reset_horizon_text: 'Ready',
      has_mfa: false,
    }

    it('triggers auto-switch in single-engine mode when Gemini is depleted', () => {
      const result = evaluateAutoSwitch(
        [activeDepletedGemini, standbyAccount],
        'active@example.com',
        0.05,
        'balanced',
        1000,
        0.05,
        false
      )
      assert.strictEqual(result.shouldSwitch, true)
      assert.strictEqual(result.successor?.email, 'standby@example.com')
    })

    it('suppresses auto-switch in dual-engine mode when Claude quota is healthy', () => {
      const result = evaluateAutoSwitch(
        [activeDepletedGemini, standbyAccount],
        'active@example.com',
        0.05,
        'balanced',
        1000,
        0.05,
        true
      )
      assert.strictEqual(result.shouldSwitch, false)
      assert.strictEqual(result.successor, null)
    })

    it('triggers auto-switch in dual-engine mode when BOTH Gemini and Claude are depleted on active account', () => {
      const activeFullyDepleted: AccountState = {
        ...activeDepletedGemini,
        quota_5h_claude_gpt: 0.01,
        quota_weekly_claude_gpt: 0.01,
      }
      const result = evaluateAutoSwitch(
        [activeFullyDepleted, standbyAccount],
        'active@example.com',
        0.05,
        'balanced',
        1000,
        0.05,
        true
      )
      assert.strictEqual(result.shouldSwitch, true)
      assert.strictEqual(result.successor?.email, 'standby@example.com')
    })
  })

  describe('sortAccounts tier placement in dual-engine mode', () => {
    it('places Claude-healthy account into healthy standby tier instead of cooldown tier', () => {
      const claudeHealthy: AccountState = {
        email: 'claude-healthy@example.com',
        is_active: false,
        plan_tier: 'Pro',
        status: 'STANDBY',
        quota_5h_current: 0.0,
        quota_5h_available: 0.0,
        quota_weekly: 0.0,
        quota_5h_claude_gpt: 0.90,
        quota_weekly_claude_gpt: 0.95,
        reset_horizon_text: 'Resets in 3h',
        has_mfa: false,
      }

      const exhaustedBoth: AccountState = {
        email: 'exhausted-both@example.com',
        is_active: false,
        plan_tier: 'Pro',
        status: 'COOLDOWN',
        quota_5h_current: 0.0,
        quota_5h_available: 0.0,
        quota_weekly: 0.0,
        quota_5h_claude_gpt: 0.0,
        quota_weekly_claude_gpt: 0.0,
        reset_horizon_text: 'Resets in 3h',
        has_mfa: false,
      }

      // With allowNonGemini = true: claudeHealthy must rank ahead of exhaustedBoth
      const sortedDual = sortAccounts([exhaustedBoth, claudeHealthy], '', 0.05, 'auto', 'balanced', 0.05, true)
      assert.strictEqual(sortedDual[0].email, 'claude-healthy@example.com')
      assert.strictEqual(sortedDual[1].email, 'exhausted-both@example.com')
    })
  })

  describe('Quota Dashboard UI switchability predicate logic', () => {
    it('enables switch button and suppresses cooling badge when non-Gemini routing is enabled', () => {
      const acc: AccountState = {
        email: 'dual-switchable@example.com',
        is_active: false,
        status: 'COOLING',
        quota_5h_current: 0.01,
        quota_weekly: 0.01,
        quota_5h_claude_gpt: 0.85,
        quota_weekly_claude_gpt: 0.90,
        reset_horizon_text: 'Resets in 2h',
        has_mfa: false,
      }

      const threshold = 0.05
      const thresholdWeekly = 0.05
      const rulesDualEnabled = { allow_non_gemini_native_models: true }
      const rulesDualDisabled = { allow_non_gemini_native_models: false }

      const computeSwitchState = (account: AccountState, rules: any) => {
        const st = account.status?.toUpperCase() || ''
        const isCooling = st === 'COOLDOWN' || st === 'COOLING'
        const claude5h = typeof account.quota_5h_claude_gpt === 'number' ? account.quota_5h_claude_gpt : 0
        const claudeWeekly = typeof account.quota_weekly_claude_gpt === 'number' ? account.quota_weekly_claude_gpt : 0
        const hasNonGemini = Boolean(rules?.allow_non_gemini_native_models) &&
          claude5h > threshold &&
          (claudeWeekly > thresholdWeekly || claudeWeekly === 0 || Boolean(account.enable_credit_overages && (account.credits ?? 0) > 0))

        const rendersCoolingBadge = isCooling && !hasNonGemini
        const canSwitch = !rendersCoolingBadge
        return { rendersCoolingBadge, canSwitch }
      }

      const disabledResult = computeSwitchState(acc, rulesDualDisabled)
      assert.strictEqual(disabledResult.rendersCoolingBadge, true)
      assert.strictEqual(disabledResult.canSwitch, false)

      const enabledResult = computeSwitchState(acc, rulesDualEnabled)
      assert.strictEqual(enabledResult.rendersCoolingBadge, false)
      assert.strictEqual(enabledResult.canSwitch, true)
    })

    it('enables context menu switch actions for non-Gemini ready accounts', () => {
      const acc: AccountState = {
        email: 'context-menu-dual@example.com',
        is_active: false,
        status: 'COOLING',
        quota_5h_current: 0.01,
        quota_weekly: 0.01,
        quota_5h_claude_gpt: 0.80,
        quota_weekly_claude_gpt: 0.85,
        reset_horizon_text: 'Resets in 2h',
        has_mfa: false,
      }

      const threshold = 0.05
      const thresholdWeekly = 0.05
      const rulesDualEnabled = { allow_non_gemini_native_models: true }
      const rulesDualDisabled = { allow_non_gemini_native_models: false }

      const canShowContextMenuSwitch = (account: AccountState, rules: any) => {
        const st = account.status?.trim().toUpperCase() || ''
        const claude5h = typeof account.quota_5h_claude_gpt === 'number' ? account.quota_5h_claude_gpt : 0
        const claudeWeekly = typeof account.quota_weekly_claude_gpt === 'number' ? account.quota_weekly_claude_gpt : 0
        const hasNonGemini = Boolean(rules?.allow_non_gemini_native_models) &&
          claude5h > threshold &&
          (claudeWeekly > thresholdWeekly || claudeWeekly === 0 || Boolean(account.enable_credit_overages && (account.credits ?? 0) > 0))
        const isCooldown = st.includes('COOLDOWN') || st.includes('COOLING')
        const isBanned = st.includes('BANNED')

        if (isBanned || (isCooldown && !hasNonGemini)) {
          return false
        }
        return true
      }

      assert.strictEqual(canShowContextMenuSwitch(acc, rulesDualDisabled), false)
      assert.strictEqual(canShowContextMenuSwitch(acc, rulesDualEnabled), true)
    })
  })
})

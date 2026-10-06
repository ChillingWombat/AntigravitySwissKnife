import type { AccountState, SwitchMode } from '../types.ts'
import { normalizePlanTier } from '../types.ts'

export type SortMode = 'auto' | 'identity' | 'priority' | 'quota_5h' | 'quota_weekly' | 'credits'

export const FIVE_HOUR_WINDOW_SECONDS = 18000.0
export const WEEKLY_WINDOW_SECONDS = 604800.0
export const MIN_PROACTIVE_SWITCH_DWELL_SECONDS = 600.0 // 10 minutes

export function clamp01(val?: number): number {
  if (val === undefined || val === null || isNaN(val)) return 0
  return Math.max(0, Math.min(1, val))
}

export function parseHorizonTextSeconds(raw?: string): number {
  if (!raw) return 0
  const clean = raw.trim().toLowerCase()
  if (clean === '' || clean === 'ready' || clean === 'resets now' || clean === 'not polled') {
    return 0
  }
  const regex = /(\d+)\s*([dhms])/gi
  let match: RegExpExecArray | null
  let totalSec = 0
  while ((match = regex.exec(clean)) !== null) {
    const val = parseFloat(match[1])
    if (isNaN(val)) continue
    const unit = match[2].toLowerCase()
    switch (unit) {
      case 'd':
        totalSec += val * 86400
        break
      case 'h':
        totalSec += val * 3600
        break
      case 'm':
        totalSec += val * 60
        break
      case 's':
        totalSec += val
        break
    }
  }
  return totalSec
}

export function normalizeSwitchMode(raw?: string): SwitchMode {
  if (!raw) return 'balanced'
  const clean = raw.trim().toLowerCase()
  switch (clean) {
    case 'max_tokens':
    case 'max_total_tokens':
    case 'max-tokens':
    case 'tokens':
      return 'max_tokens'
    case 'max_continuous':
    case 'continuous':
    case 'max_continues':
    case 'max-continuous':
    case 'continues':
      return 'max_continuous'
    case 'balanced':
    case 'default':
    default:
      return 'balanced'
  }
}

export function resolveAccountPlanTier(email: string, tier?: string): string {
  const t = (tier || '').trim()
  if (t !== '') {
    return normalizePlanTier(t)
  }
  const lowerEmail = email.toLowerCase()
  if (lowerEmail.includes('ultra')) return 'Ultra 20X'
  if (lowerEmail.includes('.edu') || lowerEmail.includes('student') || lowerEmail.includes('univ')) return 'Edu'
  if (lowerEmail.includes('trial')) return 'Pro - Trial'
  if (lowerEmail.includes('dev') || lowerEmail.includes('pro')) return 'Pro'
  if (lowerEmail.includes('plus')) return 'Plus'
  return 'Free'
}

export function isFreePlanTier(email: string, tier?: string): boolean {
  return resolveAccountPlanTier(email, tier) === 'Free'
}

export function planTierRank(tier: string): number {
  const norm = normalizePlanTier(tier)
  switch (norm) {
    case 'Enterprise':
      return 8
    case 'Ultra 20X':
      return 7
    case 'Ultra 10X':
      return 6
    case 'Ultra 5X':
      return 5
    case 'Pro':
      return 4
    case 'Edu':
      return 3
    case 'Pro - Trial':
      return 2
    case 'Plus':
      return 1
    case 'Free':
    default:
      return 0
  }
}

export function planTierCapacityMultiplier(tier: string): number {
  const norm = normalizePlanTier(tier)
  switch (norm) {
    case 'Enterprise':
      return 1.45
    case 'Ultra 20X':
      return 1.4
    case 'Ultra 10X':
      return 1.3
    case 'Ultra 5X':
      return 1.2
    case 'Pro':
      return 1.0
    case 'Edu':
      return 0.98
    case 'Pro - Trial':
      return 0.92
    case 'Plus':
      return 0.8
    case 'Free':
      return 0.25
    default:
      return 1.0
  }
}

export function planTierNormalizedWeight(tier: string): number {
  const norm = normalizePlanTier(tier)
  switch (norm) {
    case 'Enterprise':
      return 1.0
    case 'Ultra 20X':
      return 0.95
    case 'Ultra 10X':
      return 0.85
    case 'Ultra 5X':
      return 0.75
    case 'Pro':
      return 0.55
    case 'Edu':
      return 0.52
    case 'Pro - Trial':
      return 0.45
    case 'Plus':
      return 0.3
    case 'Free':
      return 0.05
    default:
      return 0.5
  }
}

export function priorityRank(priority?: string): number {
  const clean = (priority || '').trim().toUpperCase()
  switch (clean) {
    case 'HIGH':
    case '':
      return 0
    case 'MID':
    case 'MEDIUM':
      return 1
    case 'LOW':
      return 2
    default:
      return 0
  }
}

export function computeEffective5hAvailable(currentFrac: number, resetSeconds: number): number {
  const cur = clamp01(currentFrac)
  if (cur === 0 && resetSeconds <= 0) {
    return 0
  }
  if (resetSeconds > 0) {
    const h = resetSeconds / 3600.0
    if (h <= 5.0) {
      const replenishedBoost = (1.0 - cur) * ((5.0 - h) / 5.0)
      return clamp01(cur + replenishedBoost)
    }
  }
  return cur
}

export interface AccountMetrics {
  q5hCur: number
  q5hAvail: number
  r5hSoonness: number
  q7d: number
  r7dSoonness: number
  tier: string
  tierRank: number
  tierMult: number
  tierNorm: number
  isFree: boolean
  priorityRank: number
}

export function extractAccountMetrics(acc: AccountState, mode: SwitchMode): AccountMetrics {
  const q5hCur = clamp01(acc.quota_5h_current ?? acc.quota_5h_available)
  let sec5h = acc.reset_seconds ?? 0
  if (sec5h <= 0 && acc.reset_horizon_text) {
    sec5h = parseHorizonTextSeconds(acc.reset_horizon_text)
  }

  let q5hAvail = computeEffective5hAvailable(q5hCur, sec5h)
  if (acc.quota_5h_available > q5hAvail && sec5h <= 0) {
    q5hAvail = clamp01(acc.quota_5h_available)
  }

  let r5hSoonness = 0.5
  if (sec5h > 0 && sec5h <= FIVE_HOUR_WINDOW_SECONDS) {
    r5hSoonness = clamp01(1.0 - sec5h / FIVE_HOUR_WINDOW_SECONDS)
  } else if (sec5h > FIVE_HOUR_WINDOW_SECONDS) {
    r5hSoonness = 0.0
  } else {
    // sec5h <= 0 (Ready / unstarted)
    if (mode === 'max_tokens' && q5hCur >= 0.98) {
      r5hSoonness = 0.85 // Clock ignition bonus
    } else {
      r5hSoonness = 0.5
    }
  }

  const q7d = clamp01(acc.quota_weekly)
  let sec7d = acc.reset_seconds_weekly ?? 0
  if (sec7d <= 0 && acc.reset_horizon_weekly_text) {
    sec7d = parseHorizonTextSeconds(acc.reset_horizon_weekly_text)
  }

  let r7dSoonness = 0.35
  if (sec7d > 0 && sec7d <= WEEKLY_WINDOW_SECONDS) {
    r7dSoonness = clamp01(1.0 - sec7d / WEEKLY_WINDOW_SECONDS)
  } else if (sec7d > WEEKLY_WINDOW_SECONDS) {
    r7dSoonness = 0.0
  } else {
    r7dSoonness = 0.35
  }

  const tier = resolveAccountPlanTier(acc.email, acc.plan_tier)
  const isFree = tier === 'Free'

  return {
    q5hCur,
    q5hAvail,
    r5hSoonness,
    q7d,
    r7dSoonness,
    tier,
    tierRank: planTierRank(tier),
    tierMult: planTierCapacityMultiplier(tier),
    tierNorm: planTierNormalizedWeight(tier),
    isFree,
    priorityRank: priorityRank(acc.priority),
  }
}

export function computeAccountRankingScore(acc: AccountState, _threshold: number, mode: SwitchMode): number {
  const m = normalizeSwitchMode(mode)
  const metrics = extractAccountMetrics(acc, m)

  switch (m) {
    case 'max_continuous': {
      return (
        0.75 * (metrics.tierMult * metrics.q5hAvail) +
        0.08 * metrics.r5hSoonness +
        0.1 * metrics.q7d +
        0.05 * metrics.r7dSoonness +
        0.02 * metrics.tierNorm
      )
    }

    case 'max_tokens': {
      let sec5h = acc.reset_seconds ?? 0
      if (sec5h <= 0 && acc.reset_horizon_text) {
        sec5h = parseHorizonTextSeconds(acc.reset_horizon_text)
      }

      let urgency5h = 0
      if (sec5h > 0 && sec5h <= FIVE_HOUR_WINDOW_SECONDS) {
        urgency5h = metrics.q5hCur * (0.45 + 0.55 * metrics.r5hSoonness)
      } else if (sec5h <= 0 && metrics.q5hCur >= 0.98) {
        urgency5h = metrics.q5hCur * 0.85
      } else {
        urgency5h = metrics.q5hCur * 0.5
      }

      const urgency7d = metrics.q7d * (0.55 + 0.45 * metrics.r7dSoonness)

      return (
        metrics.tierMult *
        (0.45 * urgency5h + 0.15 * metrics.q5hAvail + 0.25 * metrics.q7d + 0.15 * urgency7d)
      )
    }

    case 'balanced':
    default: {
      return (
        metrics.tierMult *
        (0.42 * metrics.q5hCur +
          0.12 * metrics.q5hAvail +
          0.06 * metrics.r5hSoonness +
          0.32 * metrics.q7d +
          0.08 * metrics.r7dSoonness)
      )
    }
  }
}

export function compareStandbyCandidates(
  a: AccountState,
  b: AccountState,
  threshold: number,
  mode: SwitchMode
): number {
  const m = normalizeSwitchMode(mode)
  const mA = extractAccountMetrics(a, m)
  const mB = extractAccountMetrics(b, m)

  // Invariant 1: Free accounts are always ranked last among healthy standby candidates
  if (mA.isFree !== mB.isFree) {
    return mA.isFree ? 1 : -1
  }

  // Invariant 2: Within same class, respect user Priority (High > Mid > Low)
  if (mA.priorityRank !== mB.priorityRank) {
    return mA.priorityRank - mB.priorityRank
  }

  // Invariant 3: Mode-specific ranking across 4 metrics + plan tier
  if (m === 'max_continuous') {
    const cap5hA = mA.tierMult * mA.q5hAvail
    const cap5hB = mB.tierMult * mB.q5hAvail
    if (Math.abs(cap5hA - cap5hB) > 0.02) {
      return cap5hB - cap5hA
    }

    // Tie-break when 5h available capacities are about the same (<= 2%)
    const tieA =
      0.3 * mA.r5hSoonness +
      0.2 * mA.q5hCur +
      0.3 * mA.q7d +
      0.15 * mA.r7dSoonness +
      0.05 * mA.tierNorm
    const tieB =
      0.3 * mB.r5hSoonness +
      0.2 * mB.q5hCur +
      0.3 * mB.q7d +
      0.15 * mB.r7dSoonness +
      0.05 * mB.tierNorm
    if (Math.abs(tieA - tieB) > 0.0005) {
      return tieB - tieA
    }
  } else {
    const scoreA = computeAccountRankingScore(a, threshold, m)
    const scoreB = computeAccountRankingScore(b, threshold, m)
    if (Math.abs(scoreA - scoreB) > 0.0005) {
      return scoreB - scoreA
    }
  }

  // Fallback deterministic tie-breakers
  if (mA.tierRank !== mB.tierRank) {
    return mB.tierRank - mA.tierRank
  }
  if (Math.abs(mA.q5hAvail - mB.q5hAvail) > 0.0005) {
    return mB.q5hAvail - mA.q5hAvail
  }
  if (Math.abs(mA.q5hCur - mB.q5hCur) > 0.0005) {
    return mB.q5hCur - mA.q5hCur
  }
  if (Math.abs(mA.r5hSoonness - mB.r5hSoonness) > 0.0005) {
    return mB.r5hSoonness - mA.r5hSoonness
  }
  if (Math.abs(mA.q7d - mB.q7d) > 0.0005) {
    return mB.q7d - mA.q7d
  }
  if (Math.abs(mA.r7dSoonness - mB.r7dSoonness) > 0.0005) {
    return mB.r7dSoonness - mA.r7dSoonness
  }

  const nameA = (a.label || a.email).toLowerCase()
  const nameB = (b.label || b.email).toLowerCase()
  return nameA.localeCompare(nameB)
}

export function rankStandbyAccounts(
  accounts: AccountState[],
  threshold: number,
  mode: SwitchMode = 'balanced',
  thresholdWeekly: number = 0.05
): AccountState[] {
  const m = normalizeSwitchMode(mode)
  const candidates: AccountState[] = []

  for (const acc of accounts) {
    if (acc.is_active) continue
    const st = (acc.status || '').toUpperCase()
    if (st === 'BANNED' || st === 'ERROR' || st === 'COOLDOWN') continue

    const cur5h = acc.quota_5h_current ?? acc.quota_5h_available ?? 0
    if (cur5h <= threshold) continue

    const hasWeekly =
      (acc.quota_weekly ?? 0) > thresholdWeekly ||
      Boolean(acc.enable_credit_overages && (acc.credits ?? 0) > 0)
    if (!hasWeekly) continue

    candidates.push(acc)
  }

  return candidates.sort((a, b) => compareStandbyCandidates(a, b, threshold, m))
}

export function shouldSwitchProactivelyMaxTokens(
  active: AccountState,
  bestStandby: AccountState,
  threshold: number,
  activeDwellSec: number
): { shouldSwitch: boolean; reason: string } {
  let sec5hActive = active.reset_seconds ?? 0
  if (sec5hActive <= 0 && active.reset_horizon_text) {
    sec5hActive = parseHorizonTextSeconds(active.reset_horizon_text)
  }

  let sec5hStandby = bestStandby.reset_seconds ?? 0
  if (sec5hStandby <= 0 && bestStandby.reset_horizon_text) {
    sec5hStandby = parseHorizonTextSeconds(bestStandby.reset_horizon_text)
  }

  let sec7dActive = active.reset_seconds_weekly ?? 0
  if (sec7dActive <= 0 && active.reset_horizon_weekly_text) {
    sec7dActive = parseHorizonTextSeconds(active.reset_horizon_weekly_text)
  }

  let sec7dStandby = bestStandby.reset_seconds_weekly ?? 0
  if (sec7dStandby <= 0 && bestStandby.reset_horizon_weekly_text) {
    sec7dStandby = parseHorizonTextSeconds(bestStandby.reset_horizon_weekly_text)
  }

  // Guardrail 1: Minimum active use dwell time (default 10 minutes = 600s)
  let hasSufficientDwell = activeDwellSec >= MIN_PROACTIVE_SWITCH_DWELL_SECONDS
  if (!hasSufficientDwell && activeDwellSec <= 0) {
    if (
      sec5hActive > 0 &&
      sec5hActive <= FIVE_HOUR_WINDOW_SECONDS - MIN_PROACTIVE_SWITCH_DWELL_SECONDS
    ) {
      hasSufficientDwell = true
    }
  }
  if (!hasSufficientDwell) {
    return { shouldSwitch: false, reason: 'Minimum 10-minute active use dwell not reached' }
  }

  // Guardrail 2: Do NOT switch away if active account is about to reset soon (harvesting expiring quota)
  if (sec5hActive > 0 && sec5hActive <= 2700.0) {
    return {
      shouldSwitch: false,
      reason: 'Active account 5h reset is imminent (harvesting active expiring quota)',
    }
  }

  // Guardrail 3: Best standby candidate must be a healthy paid account (or active is also Free)
  if (isFreePlanTier(bestStandby.email, bestStandby.plan_tier) && !isFreePlanTier(active.email, active.plan_tier)) {
    return { shouldSwitch: false, reason: 'Standby is Free tier while active is paid tier' }
  }

  const standby5hCur = bestStandby.quota_5h_current ?? bestStandby.quota_5h_available ?? 0

  // Trigger 1: Idle Clock Ignition
  if (sec5hActive > 0 && standby5hCur >= 0.98 && sec5hStandby <= 0) {
    return {
      shouldSwitch: true,
      reason: `Proactive rotation to ignite standby 5h reset clock (${bestStandby.email})`,
    }
  }

  // Trigger 2: Standby 5h reset is significantly earlier than active's
  if (sec5hStandby > 0) {
    if (sec5hActive <= 0 || sec5hStandby + 1800.0 < sec5hActive) {
      return {
        shouldSwitch: true,
        reason: `Proactive rotation to harvest standby expiring 5h window (${bestStandby.email} resets in ${Math.round(sec5hStandby)}s vs active ${Math.round(sec5hActive)}s)`,
      }
    }
  }

  // Trigger 3: Standby weekly reset expiring within 24h while active does not
  if (sec7dStandby > 0 && sec7dStandby <= 86400.0) {
    if (sec7dActive <= 0 || sec7dActive > 86400.0) {
      return {
        shouldSwitch: true,
        reason: `Proactive rotation to harvest standby expiring weekly quota (${bestStandby.email})`,
      }
    }
  }

  // Trigger 4: Standby has meaningful harvest score advantage (> 8%)
  const scoreActive = computeAccountRankingScore(active, threshold, 'max_tokens')
  const scoreStandby = computeAccountRankingScore(bestStandby, threshold, 'max_tokens')
  if (scoreStandby > scoreActive * 1.08) {
    return {
      shouldSwitch: true,
      reason: `Proactive rotation: standby harvest score (${scoreStandby.toFixed(3)}) exceeds active (${scoreActive.toFixed(3)}) by >8%`,
    }
  }

  return { shouldSwitch: false, reason: 'Active account remains optimal for current window' }
}

export function evaluateAutoSwitch(
  accounts: AccountState[],
  activeEmail: string,
  threshold: number,
  mode: SwitchMode = 'balanced',
  activeDwellSec = 0,
  thresholdWeekly: number = 0.05
): { shouldSwitch: boolean; successor: AccountState | null; reason: string } {
  const m = normalizeSwitchMode(mode)
  const normActive = activeEmail.toLowerCase().trim()
  const active = accounts.find(
    (a) => a.is_active || (normActive !== '' && a.email.toLowerCase().trim() === normActive)
  )

  const ranked = rankStandbyAccounts(accounts, threshold, m, thresholdWeekly)
  if (ranked.length === 0) {
    return { shouldSwitch: false, successor: null, reason: 'No eligible standby accounts above threshold' }
  }

  const best = ranked[0]

  // 1. Mandatory Threshold Trigger (applies in all modes)
  if (active) {
    const cur5h = active.quota_5h_current ?? active.quota_5h_available ?? 0
    const weekly = active.quota_weekly ?? 0
    const hasWeekly = weekly > thresholdWeekly || Boolean(active.enable_credit_overages && (active.credits ?? 0) > 0)
    const is5hBreached = cur5h <= threshold
    const isWeeklyBreached = !hasWeekly

    if (is5hBreached || isWeeklyBreached) {
      let reason: string
      if (is5hBreached && isWeeklyBreached) {
        reason = `Active quota (5h: ${(cur5h * 100).toFixed(1)}%, weekly: ${(weekly * 100).toFixed(1)}%) dropped below thresholds (5h: ${(threshold * 100).toFixed(1)}%, weekly: ${(thresholdWeekly * 100).toFixed(1)}%)`
      } else if (is5hBreached) {
        reason = `Active 5h quota (${(cur5h * 100).toFixed(1)}%) dropped below threshold (${(threshold * 100).toFixed(1)}%)`
      } else {
        reason = `Active weekly quota (${(weekly * 100).toFixed(1)}%) dropped below threshold (${(thresholdWeekly * 100).toFixed(1)}%)`
      }
      return { shouldSwitch: true, successor: best, reason }
    }
  }

  // 2. Proactive rotation only in max_tokens mode
  if (m === 'max_tokens' && active) {
    const proactive = shouldSwitchProactivelyMaxTokens(active, best, threshold, activeDwellSec)
    if (proactive.shouldSwitch) {
      return { shouldSwitch: true, successor: best, reason: proactive.reason }
    }
  }

  return { shouldSwitch: false, successor: null, reason: 'Active account quota is healthy' }
}

export function sortAccounts(
  accounts: AccountState[],
  activeEmail: string,
  threshold: number,
  mode: SortMode,
  switchMode?: SwitchMode,
  thresholdWeekly: number = 0.05
): AccountState[] {
  const copy = [...accounts]
  const swMode = normalizeSwitchMode(switchMode)

  if (mode === 'identity') {
    return copy.sort((a, b) => {
      const nameA = (a.label || a.email).toLowerCase()
      const nameB = (b.label || b.email).toLowerCase()
      if (nameA !== nameB) {
        return nameA.localeCompare(nameB)
      }
      return a.email.localeCompare(b.email)
    })
  }

  if (mode === 'priority') {
    return copy.sort((a, b) => {
      const pa = priorityRank(a.priority)
      const pb = priorityRank(b.priority)
      if (pa !== pb) return pa - pb
      const diff = (b.quota_5h_available ?? 0) - (a.quota_5h_available ?? 0)
      if (Math.abs(diff) > 0.0001) return diff
      return (b.quota_weekly ?? 0) - (a.quota_weekly ?? 0)
    })
  }

  if (mode === 'credits') {
    return copy.sort((a, b) => {
      const credA = a.credits !== undefined && a.credits !== null ? Number(a.credits) : 0
      const credB = b.credits !== undefined && b.credits !== null ? Number(b.credits) : 0
      return credB - credA
    })
  }

  if (mode === 'quota_5h') {
    return copy.sort((a, b) => {
      const diff = (b.quota_5h_available ?? 0) - (a.quota_5h_available ?? 0)
      if (Math.abs(diff) > 0.0001) return diff
      return (b.quota_weekly ?? 0) - (a.quota_weekly ?? 0)
    })
  }

  if (mode === 'quota_weekly') {
    return copy.sort((a, b) => {
      const diff = (b.quota_weekly ?? 0) - (a.quota_weekly ?? 0)
      if (Math.abs(diff) > 0.0001) return diff
      return (b.quota_5h_available ?? 0) - (a.quota_5h_available ?? 0)
    })
  }

  // mode === 'auto' (Default)
  // Structural tiers:
  // Tier 0: Active healthy account (Row 1 pinned)
  // Tier 1: Healthy Paid Standby successors above threshold (ordered by switch mode)
  // Tier 2: Healthy Free Standby successors (Free ranked last among eligible standbys)
  // Tier 3: Cooling down / below threshold accounts
  // Tier 4: Error accounts
  // Tier 5: Banned accounts
  const normActive = activeEmail.toLowerCase().trim()

  const getTier = (a: AccountState): number => {
    const st = (a.status || '').toUpperCase()
    if (st === 'BANNED') return 5
    if (st === 'ERROR') return 4
    if (st === 'COOLDOWN') return 3

    const cur5h = a.quota_5h_current ?? a.quota_5h_available ?? 0
    const weekly = a.quota_weekly ?? 0
    const hasWeekly = weekly > thresholdWeekly || Boolean(a.enable_credit_overages && (a.credits ?? 0) > 0)
    const isBelow = cur5h <= threshold || !hasWeekly

    if (!isBelow) {
      if (isFreePlanTier(a.email, a.plan_tier)) {
        return 2
      }
      return 1
    }
    return 3
  }

  return copy.sort((a, b) => {
    const isActA = normActive !== '' ? a.email.toLowerCase().trim() === normActive : Boolean(a.is_active)
    const isActB = normActive !== '' ? b.email.toLowerCase().trim() === normActive : Boolean(b.is_active)

    if (isActA && !isActB) return -1
    if (!isActA && isActB) return 1

    const tA = getTier(a)
    const tB = getTier(b)
    if (tA !== tB) {
      return tA - tB
    }

    // Within Tier 1 or Tier 2: compare by switch mode
    if (tA === 1 || tA === 2) {
      return compareStandbyCandidates(a, b, threshold, swMode)
    }

    // Within Tier 3 (cooling down): prioritize paid over free, then highest available recovery
    if (tA === 3) {
      const freeA = isFreePlanTier(a.email, a.plan_tier)
      const freeB = isFreePlanTier(b.email, b.plan_tier)
      if (freeA !== freeB) {
        return freeA ? 1 : -1
      }
      const diff5h = (b.quota_5h_available ?? 0) - (a.quota_5h_available ?? 0)
      if (Math.abs(diff5h) > 0.001) {
        return diff5h
      }
      return (b.quota_weekly ?? 0) - (a.quota_weekly ?? 0)
    }

    return (a.label || a.email).localeCompare(b.label || b.email)
  })
}

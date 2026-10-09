// Module for filtering, ranking, and building the tray Switch Google Account menu items.
// Excludes accounts in cool down, with error, or banned.
// Preserves ranking identical to the Account Fleet table.

const CANONICAL_TIERS = {
  'enterprise': { rank: 8, mult: 1.45 },
  'ultra 20x': { rank: 7, mult: 1.40 },
  'ultra 10x': { rank: 6, mult: 1.30 },
  'ultra 5x': { rank: 5, mult: 1.20 },
  'pro': { rank: 4, mult: 1.00 },
  'edu': { rank: 3, mult: 0.98 },
  'pro - trial': { rank: 2, mult: 0.92 },
  'plus': { rank: 1, mult: 0.80 },
  'free': { rank: 0, mult: 0.25 },
};

function resolveTierInfo(email = '', rawTier = '') {
  const t = (rawTier || '').trim().toLowerCase();
  if (t) {
    if (t.includes('enterprise')) return CANONICAL_TIERS['enterprise'];
    if (t.includes('20x')) return CANONICAL_TIERS['ultra 20x'];
    if (t.includes('10x')) return CANONICAL_TIERS['ultra 10x'];
    if (t.includes('5x')) return CANONICAL_TIERS['ultra 5x'];
    if (t.includes('ultra')) return CANONICAL_TIERS['ultra 20x'];
    if (t.includes('edu') || t.includes('student')) return CANONICAL_TIERS['edu'];
    if (t.includes('trial') || t.includes('promo') || t.includes('jio')) return CANONICAL_TIERS['pro - trial'];
    if (t.includes('pro')) return CANONICAL_TIERS['pro'];
    if (t.includes('plus')) return CANONICAL_TIERS['plus'];
    if (t.includes('free')) return CANONICAL_TIERS['free'];
  }
  const lowEmail = email.toLowerCase();
  if (lowEmail.includes('ultra')) return CANONICAL_TIERS['ultra 20x'];
  if (lowEmail.includes('.edu') || lowEmail.includes('student')) return CANONICAL_TIERS['edu'];
  if (lowEmail.includes('trial') || lowEmail.includes('promo')) return CANONICAL_TIERS['pro - trial'];
  if (lowEmail.includes('pro') || lowEmail.includes('dev')) return CANONICAL_TIERS['pro'];
  if (lowEmail.includes('plus')) return CANONICAL_TIERS['plus'];
  return CANONICAL_TIERS['free'];
}

function priorityRank(priority) {
  const p = (priority || '').trim().toUpperCase();
  if (p === 'LOW') return 2;
  if (p === 'MID' || p === 'MEDIUM') return 1;
  return 0; // Default HIGH
}

function computeStandbyScore(acc) {
  const email = (acc.email || (typeof acc === 'string' ? acc : '')).trim();
  const tierInfo = resolveTierInfo(email, acc.plan_tier);
  const q5hCur = typeof acc.quota_5h_current === 'number'
    ? acc.quota_5h_current
    : (typeof acc.quota_5h_available === 'number' ? acc.quota_5h_available : 1.0);
  const q5hAvail = typeof acc.quota_5h_available === 'number' ? acc.quota_5h_available : q5hCur;
  const q7d = typeof acc.quota_weekly === 'number' ? acc.quota_weekly : 1.0;
  const r5hSoonness = 0.5;
  const r7dSoonness = 0.35;
  return tierInfo.mult * (0.42 * q5hCur + 0.12 * q5hAvail + 0.06 * r5hSoonness + 0.32 * q7d + 0.08 * r7dSoonness);
}

function compareFleetAccounts(a, b, activeEmail = '') {
  const emailA = (a.email || (typeof a === 'string' ? a : '')).trim().toLowerCase();
  const emailB = (b.email || (typeof b === 'string' ? b : '')).trim().toLowerCase();
  const normActive = (activeEmail || '').trim().toLowerCase();
  const effectiveActive = (normActive && normActive !== 'not logged in') ? normActive : '';

  const isActA = effectiveActive !== '' ? emailA === effectiveActive : Boolean(a.is_active);
  const isActB = effectiveActive !== '' ? emailB === effectiveActive : Boolean(b.is_active);

  if (isActA && !isActB) return -1;
  if (!isActA && isActB) return 1;

  // Free accounts rank last among healthy standbys
  const tierA = resolveTierInfo(emailA, a.plan_tier);
  const tierB = resolveTierInfo(emailB, b.plan_tier);
  const isFreeA = tierA.rank === 0;
  const isFreeB = tierB.rank === 0;
  if (isFreeA !== isFreeB) {
    return isFreeA ? 1 : -1;
  }

  // Priority rank (High=0, Mid=1, Low=2)
  const prioA = priorityRank(a.priority);
  const prioB = priorityRank(b.priority);
  if (prioA !== prioB) {
    return prioA - prioB;
  }

  // Score comparison based on capacity multiplier and quota
  const scoreA = computeStandbyScore(a);
  const scoreB = computeStandbyScore(b);
  if (Math.abs(scoreA - scoreB) > 0.0005) {
    return scoreB - scoreA;
  }

  // Tie-breaker 1: Tier rank
  if (tierA.rank !== tierB.rank) {
    return tierB.rank - tierA.rank;
  }

  // Tie-breaker 2: Available 5h quota
  const q5hA = typeof a.quota_5h_available === 'number' ? a.quota_5h_available : (a.quota_5h_current || 0);
  const q5hB = typeof b.quota_5h_available === 'number' ? b.quota_5h_available : (b.quota_5h_current || 0);
  if (Math.abs(q5hA - q5hB) > 0.001) {
    return q5hB - q5hA;
  }

  // Tie-breaker 3: Weekly quota
  const qWkA = a.quota_weekly || 0;
  const qWkB = b.quota_weekly || 0;
  if (Math.abs(qWkA - qWkB) > 0.001) {
    return qWkB - qWkA;
  }

  // Tie-breaker 4: Alphabetical label or email
  const nameA = (a.label || a.email || '').toLowerCase();
  const nameB = (b.label || b.email || '').toLowerCase();
  return nameA.localeCompare(nameB);
}

function isAccountExcludedFromSwitchMenu(acc, threshold5h = 0.10, thresholdWeekly = 0.05) {
  if (!acc) return true;
  const email = (acc.email || (typeof acc === 'string' ? acc : '')).trim();
  if (!email) return true;

  const st = (acc.status || '').trim().toUpperCase();

  // 1. Banned accounts
  if (st === 'BANNED' || st.includes('BANNED')) return true;
  if (acc.status_reason && acc.status_reason.toUpperCase().includes('BANNED')) return true;

  // 2. Error accounts
  if (st === 'ERROR' || st.includes('ERROR') || st.includes('FAIL') || st.includes('INVALID') || st.includes('REVOKED')) return true;
  if (Boolean(acc.error_status)) return true;
  if (Boolean(acc.error)) return true;
  if (typeof acc.error_message === 'string' && acc.error_message.trim() !== '') return true;
  if (Boolean(acc.has_error)) return true;
  if (acc.status_reason && (acc.status_reason.toUpperCase().includes('ERROR') || acc.status_reason.toUpperCase().includes('FAIL'))) return true;

  // 3. Cool down / cooling accounts
  if (st === 'COOLDOWN' || st === 'COOLING' || st.includes('COOL')) return true;
  if (acc.status_reason && (acc.status_reason.toUpperCase().includes('COOL') || acc.status_reason.toUpperCase().includes('COOLDOWN'))) return true;

  // 4. Quota exhaustion below threshold counts as cool down
  const cur5h = typeof acc.quota_5h_current === 'number'
    ? acc.quota_5h_current
    : (typeof acc.quota_5h_available === 'number' ? acc.quota_5h_available : undefined);
  if (cur5h !== undefined && cur5h <= threshold5h) return true;

  const weekly = typeof acc.quota_weekly === 'number' ? acc.quota_weekly : undefined;
  const hasCredits = Boolean(acc.enable_credit_overages && (acc.credits || 0) > 0);
  if (weekly !== undefined && weekly <= thresholdWeekly && !hasCredits) return true;

  return false;
}

function rankAndFilterAccountsForTray(accounts, activeEmail = '', threshold5h = 0.10, thresholdWeekly = 0.05, isPreRanked = false) {
  if (!Array.isArray(accounts)) return [];

  // 1. Filter out accounts that are in cool down, have error, or are banned
  const eligible = accounts.filter((acc) => !isAccountExcludedFromSwitchMenu(acc, threshold5h, thresholdWeekly));

  // 2. If the accounts are already pre-ranked by /api/quota/fleet (daemon SortAccountQuotaStatesWithThresholds),
  // preserve that exact ranking order. Otherwise, sort them to strictly match the Account Fleet table.
  if (isPreRanked) {
    return eligible;
  }

  return eligible.slice().sort((a, b) => compareFleetAccounts(a, b, activeEmail));
}

function buildSwitchMenuItems(accounts, activeEmail = '', onSwitch, threshold5h = 0.10, thresholdWeekly = 0.05, isPreRanked = false) {
  const eligible = rankAndFilterAccountsForTray(accounts, activeEmail, threshold5h, thresholdWeekly, isPreRanked);
  const normActive = (activeEmail || '').trim().toLowerCase();
  const effectiveActive = (normActive && normActive !== 'not logged in') ? normActive : '';

  return eligible.map((acc) => {
    const email = (acc.email || (typeof acc === 'string' ? acc : '')).trim();
    const isCurrent = effectiveActive !== '' ? email.toLowerCase() === effectiveActive : Boolean(acc.is_active);
    return {
      label: isCurrent ? `✓ ${email}` : email,
      enabled: !isCurrent,
      click: () => {
        if (typeof onSwitch === 'function') {
          onSwitch(email);
        }
      },
    };
  });
}

function formatQuotaPercentage(val) {
  let num = val;
  if (typeof num === 'string') {
    const cleaned = num.replace('%', '').trim();
    const parsed = parseFloat(cleaned);
    if (!isNaN(parsed)) {
      num = parsed;
    }
  }
  if (typeof num !== 'number' || isNaN(num) || !isFinite(num)) {
    return '0%';
  }
  const pct = (num >= 0 && num <= 1.0) ? num * 100 : num;
  const clamped = Math.max(0, Math.min(100, Math.round(pct)));
  return `${clamped}%`;
}

function formatQuotaInfo(accOrQ5h, maybeQ7d) {
  if (typeof accOrQ5h === 'string' && accOrQ5h.includes('5H:')) {
    return accOrQ5h.trim();
  }

  let q5h;
  let q7d;

  if (typeof accOrQ5h === 'number') {
    q5h = accOrQ5h;
    q7d = maybeQ7d;
  } else if (accOrQ5h && typeof accOrQ5h === 'object') {
    const acc = accOrQ5h;
    if (typeof acc.quota_5h_current === 'number') {
      q5h = acc.quota_5h_current;
    } else if (typeof acc.quota_5h_available === 'number') {
      q5h = acc.quota_5h_available;
    } else if (typeof acc.quota_5h_fraction === 'number') {
      q5h = acc.quota_5h_fraction;
    } else if (typeof acc.quota_5h === 'number') {
      q5h = acc.quota_5h;
    } else if (typeof acc.min_fraction === 'number') {
      q5h = acc.min_fraction;
    }

    if (typeof acc.quota_weekly === 'number') {
      q7d = acc.quota_weekly;
    } else if (typeof acc.quota_weekly_fraction === 'number') {
      q7d = acc.quota_weekly_fraction;
    } else if (typeof acc.quota_weekly_available === 'number') {
      q7d = acc.quota_weekly_available;
    } else if (typeof acc.quota_7d === 'number') {
      q7d = acc.quota_7d;
    } else if (typeof acc.quota_7d_available === 'number') {
      q7d = acc.quota_7d_available;
    }
  }

  return `5H: ${formatQuotaPercentage(q5h)} | 7D: ${formatQuotaPercentage(q7d)}`;
}

function resolveActiveAccount(accounts, activeEmail = '') {
  const normActive = (activeEmail || '').trim().toLowerCase();
  let matched = null;

  if (Array.isArray(accounts)) {
    if (normActive && normActive !== 'not logged in') {
      matched = accounts.find((a) => {
        const email = (a && a.email ? a.email : (typeof a === 'string' ? a : '')).trim().toLowerCase();
        return email === normActive;
      });
    }
    if (!matched) {
      matched = accounts.find((a) => a && typeof a === 'object' && Boolean(a.is_active));
    }
  }

  return matched || null;
}

function resolveActiveAccountAlias(accounts, activeEmail = '') {
  const normActive = (activeEmail || '').trim().toLowerCase();
  const matched = resolveActiveAccount(accounts, activeEmail);

  if (matched) {
    if (typeof matched === 'string' && matched.trim()) {
      return matched.trim();
    }
    if (typeof matched === 'object') {
      const label = (matched.label || matched.alias || '').trim();
      if (label) return label;
      const email = (matched.email || '').trim();
      if (email) return email;
    }
  }

  if (normActive && normActive !== 'not logged in') {
    return (activeEmail || '').trim();
  }

  return 'Not Logged In';
}

function buildTrayMenuTemplate({
  activeAlias = 'Not Logged In',
  quotaInfo = '5H: 0% | 7D: 0%',
  switchMenuItems = [],
  accountsCount = 0,
  onOpenDashboard,
  onSystemSettings,
  onQuit,
} = {}) {
  const safeAlias = (typeof activeAlias === 'string' && activeAlias.trim())
    ? activeAlias.trim()
    : 'Not Logged In';
  const safeQuota = (typeof quotaInfo === 'string' && quotaInfo.trim())
    ? quotaInfo.trim()
    : '5H: 0% | 7D: 0%';

  return [
    {
      label: safeAlias,
      enabled: false,
    },
    {
      label: safeQuota,
      enabled: false,
    },
    { type: 'separator' },
    {
      label: 'Open Dashboard',
      click: () => {
        if (typeof onOpenDashboard === 'function') {
          onOpenDashboard();
        }
      },
    },
    {
      label: 'Switch Google Account',
      submenu: switchMenuItems.length > 0
        ? switchMenuItems
        : [{ label: accountsCount > 0 ? 'No accounts available' : 'No accounts configured', enabled: false }],
    },
    { type: 'separator' },
    {
      label: 'System Settings',
      click: () => {
        if (typeof onSystemSettings === 'function') {
          onSystemSettings();
        }
      },
    },
    { type: 'separator' },
    {
      label: 'Quit Antigravity Swiss Knife',
      click: () => {
        if (typeof onQuit === 'function') {
          onQuit();
        }
      },
    },
  ];
}

module.exports = {
  isAccountExcludedFromSwitchMenu,
  rankAndFilterAccountsForTray,
  buildSwitchMenuItems,
  compareFleetAccounts,
  resolveTierInfo,
  computeStandbyScore,
  formatQuotaPercentage,
  formatQuotaInfo,
  resolveActiveAccount,
  resolveActiveAccountAlias,
  buildTrayMenuTemplate,
};

const { describe, it } = require('node:test');
const assert = require('node:assert');
const {
  isAccountExcludedFromSwitchMenu,
  rankAndFilterAccountsForTray,
  buildSwitchMenuItems,
  compareFleetAccounts,
  resolveTierInfo,
  formatQuotaPercentage,
  formatQuotaInfo,
  resolveActiveAccount,
  resolveActiveAccountAlias,
  buildTrayMenuTemplate,
} = require('./tray-account-menu');

describe('Tray Switch Google Account Menu', () => {
  describe('isAccountExcludedFromSwitchMenu', () => {
    it('excludes accounts with null, undefined or empty input', () => {
      assert.strictEqual(isAccountExcludedFromSwitchMenu(null), true);
      assert.strictEqual(isAccountExcludedFromSwitchMenu(undefined), true);
      assert.strictEqual(isAccountExcludedFromSwitchMenu({}), true);
      assert.strictEqual(isAccountExcludedFromSwitchMenu({ email: '   ' }), true);
      assert.strictEqual(isAccountExcludedFromSwitchMenu(''), true);
      assert.strictEqual(isAccountExcludedFromSwitchMenu('   '), true);
    });

    it('excludes banned accounts', () => {
      assert.strictEqual(isAccountExcludedFromSwitchMenu({ email: 'banned@gmail.com', status: 'BANNED' }), true);
      assert.strictEqual(isAccountExcludedFromSwitchMenu({ email: 'banned2@gmail.com', status: 'banned_suspicious' }), true);
      assert.strictEqual(isAccountExcludedFromSwitchMenu({ email: 'banned3@gmail.com', status_reason: 'Account is banned by admin' }), true);
    });

    it('excludes accounts with errors (including STANDBY or ACTIVE status with error_message)', () => {
      assert.strictEqual(isAccountExcludedFromSwitchMenu({ email: 'err@gmail.com', status: 'ERROR' }), true);
      assert.strictEqual(isAccountExcludedFromSwitchMenu({ email: 'err2@gmail.com', error_status: 'auth_failed' }), true);
      assert.strictEqual(isAccountExcludedFromSwitchMenu({ email: 'err3@gmail.com', status: 'ERROR_AUTH', error_message: 'Token expired' }), true);
      // Critical regression: STANDBY account with error_message MUST be excluded
      assert.strictEqual(isAccountExcludedFromSwitchMenu({ email: 'err4@gmail.com', status: 'STANDBY', error_message: 'Token expired' }), true);
      // ACTIVE account with error_message MUST be excluded
      assert.strictEqual(isAccountExcludedFromSwitchMenu({ email: 'err5@gmail.com', status: 'ACTIVE', error_message: 'OAuth refresh revoked' }), true);
      // Account with error property
      assert.strictEqual(isAccountExcludedFromSwitchMenu({ email: 'err6@gmail.com', error: 'Service unavailable' }), true);
      assert.strictEqual(isAccountExcludedFromSwitchMenu({ email: 'err7@gmail.com', has_error: true }), true);
      assert.strictEqual(isAccountExcludedFromSwitchMenu({ email: 'err8@gmail.com', status_reason: 'Auth failure 401' }), true);
      assert.strictEqual(isAccountExcludedFromSwitchMenu({ email: 'err9@gmail.com', status: 'REVOKED' }), true);
    });

    it('excludes accounts in cool down / cooling', () => {
      assert.strictEqual(isAccountExcludedFromSwitchMenu({ email: 'cool1@gmail.com', status: 'COOLDOWN' }), true);
      assert.strictEqual(isAccountExcludedFromSwitchMenu({ email: 'cool2@gmail.com', status: 'COOLING' }), true);
      assert.strictEqual(isAccountExcludedFromSwitchMenu({ email: 'cool3@gmail.com', status: 'cooling_reset' }), true);
      assert.strictEqual(isAccountExcludedFromSwitchMenu({ email: 'cool4@gmail.com', status_reason: 'Cooldown until 5pm' }), true);
    });

    it('excludes accounts whose 5h quota is exhausted below threshold', () => {
      const acc = {
        email: 'depleted5h@gmail.com',
        status: 'STANDBY',
        quota_5h_available: 0.05, // 5% <= 10% threshold
        quota_weekly: 0.80,
      };
      assert.strictEqual(isAccountExcludedFromSwitchMenu(acc, 0.10, 0.05), true);

      const accZero = {
        email: 'zero5h@gmail.com',
        status: 'STANDBY',
        quota_5h_current: 0,
        quota_weekly: 0.80,
      };
      assert.strictEqual(isAccountExcludedFromSwitchMenu(accZero, 0.10, 0.05), true);
    });

    it('excludes accounts whose weekly quota is exhausted below threshold without credits', () => {
      const acc = {
        email: 'depletedWeekly@gmail.com',
        status: 'STANDBY',
        quota_5h_available: 0.90,
        quota_weekly: 0.02, // 2% <= 5% threshold
        credits: 0,
        enable_credit_overages: false,
      };
      assert.strictEqual(isAccountExcludedFromSwitchMenu(acc, 0.10, 0.05), true);
    });

    it('allows accounts with weekly quota below threshold if credit overages are enabled and credits > 0', () => {
      const acc = {
        email: 'credits@gmail.com',
        status: 'STANDBY',
        quota_5h_available: 0.90,
        quota_weekly: 0.02,
        credits: 50,
        enable_credit_overages: true,
      };
      assert.strictEqual(isAccountExcludedFromSwitchMenu(acc, 0.10, 0.05), false);
    });

    it('allows healthy accounts', () => {
      const healthy = {
        email: 'healthy@gmail.com',
        status: 'STANDBY',
        quota_5h_available: 0.85,
        quota_weekly: 0.90,
      };
      assert.strictEqual(isAccountExcludedFromSwitchMenu(healthy, 0.10, 0.05), false);

      // Plain string email fallback (e.g. unpolled account from keyring)
      assert.strictEqual(isAccountExcludedFromSwitchMenu('plain@gmail.com'), false);
    });
  });

  describe('rankAndFilterAccountsForTray', () => {
    it('filters out cooldown, error, and banned accounts while preserving healthy accounts', () => {
      const accounts = [
        { email: 'active@gmail.com', is_active: true, status: 'ACTIVE', quota_5h_available: 0.90, quota_weekly: 0.80, plan_tier: 'Pro' },
        { email: 'cooling@gmail.com', status: 'COOLING', quota_5h_available: 0.02, quota_weekly: 0.50, plan_tier: 'Pro' },
        { email: 'error@gmail.com', status: 'ERROR', quota_5h_available: 0.80, quota_weekly: 0.80, plan_tier: 'Pro' },
        { email: 'error_msg@gmail.com', status: 'STANDBY', error_message: 'Expired', quota_5h_available: 0.95, quota_weekly: 0.90, plan_tier: 'Pro' },
        { email: 'banned@gmail.com', status: 'BANNED', quota_5h_available: 1.0, quota_weekly: 1.0, plan_tier: 'Ultra 20X' },
        { email: 'standby1@gmail.com', status: 'STANDBY', quota_5h_available: 0.95, quota_weekly: 0.90, plan_tier: 'Pro' },
        { email: 'standby2@gmail.com', status: 'STANDBY', quota_5h_available: 0.70, quota_weekly: 0.60, plan_tier: 'Free' },
      ];

      const ranked = rankAndFilterAccountsForTray(accounts, 'active@gmail.com');
      const emails = ranked.map((a) => a.email);

      assert.deepStrictEqual(emails, [
        'active@gmail.com',
        'standby1@gmail.com',
        'standby2@gmail.com',
      ]);
    });

    it('ranks healthy accounts matching the Account Fleet table order (Active first, then Paid above Free, then Quota)', () => {
      const accounts = [
        { email: 'free_high_quota@gmail.com', status: 'STANDBY', quota_5h_available: 1.0, quota_weekly: 1.0, plan_tier: 'Free', priority: 'High' },
        { email: 'pro_mid_quota@gmail.com', status: 'STANDBY', quota_5h_available: 0.80, quota_weekly: 0.80, plan_tier: 'Pro', priority: 'High' },
        { email: 'active@gmail.com', status: 'ACTIVE', quota_5h_available: 0.50, quota_weekly: 0.50, plan_tier: 'Pro', priority: 'High' },
        { email: 'pro_top_quota@gmail.com', status: 'STANDBY', quota_5h_available: 0.95, quota_weekly: 0.95, plan_tier: 'Pro', priority: 'High' },
      ];

      const ranked = rankAndFilterAccountsForTray(accounts, 'active@gmail.com');
      const emails = ranked.map((a) => a.email);

      // Active pinned to Row 1
      // pro_top_quota (95% Pro) ranks ahead of pro_mid_quota (80% Pro)
      // free_high_quota (100% Free) ranks last among healthy standbys
      assert.strictEqual(emails[0], 'active@gmail.com');
      assert.strictEqual(emails[1], 'pro_top_quota@gmail.com');
      assert.strictEqual(emails[2], 'pro_mid_quota@gmail.com');
      assert.strictEqual(emails[3], 'free_high_quota@gmail.com');
    });

    it('ranks Ultra accounts above Pro accounts when Ultra capacity score is higher', () => {
      const accounts = [
        { email: 'pro@example.com', plan_tier: 'Pro', status: 'STANDBY', quota_5h_current: 0.85, quota_5h_available: 0.85, quota_weekly: 0.90, priority: 'High' },
        { email: 'ultra@example.com', plan_tier: 'Ultra 20X', status: 'STANDBY', quota_5h_current: 0.70, quota_5h_available: 0.70, quota_weekly: 0.90, priority: 'High' },
      ];

      // Ultra 20X has 1.4x capacity multiplier, so score ~1.01 outranks Pro score ~0.80
      const ranked = rankAndFilterAccountsForTray(accounts, '');
      const emails = ranked.map((a) => a.email);

      assert.deepStrictEqual(emails, [
        'ultra@example.com',
        'pro@example.com',
      ]);
    });

    it('respects priority rankings (High > Mid > Low)', () => {
      const accounts = [
        { email: 'low@gmail.com', plan_tier: 'Pro', priority: 'Low', status: 'STANDBY', quota_5h_available: 1.0, quota_weekly: 1.0 },
        { email: 'high@gmail.com', plan_tier: 'Pro', priority: 'High', status: 'STANDBY', quota_5h_available: 0.5, quota_weekly: 0.5 },
        { email: 'mid@gmail.com', plan_tier: 'Pro', priority: 'Mid', status: 'STANDBY', quota_5h_available: 0.8, quota_weekly: 0.8 },
      ];

      const ranked = rankAndFilterAccountsForTray(accounts, '');
      const emails = ranked.map((a) => a.email);

      assert.deepStrictEqual(emails, [
        'high@gmail.com',
        'mid@gmail.com',
        'low@gmail.com',
      ]);
    });

    it('preserves pre-ranked order from /api/quota/fleet when isPreRanked is true', () => {
      const preRankedAccounts = [
        { email: 'active@gmail.com', is_active: true, status: 'ACTIVE', quota_5h_available: 0.90 },
        { email: 'custom_first@gmail.com', status: 'STANDBY', quota_5h_available: 0.70 },
        { email: 'custom_second@gmail.com', status: 'STANDBY', quota_5h_available: 0.85 },
        { email: 'banned@gmail.com', status: 'BANNED', quota_5h_available: 1.0 },
      ];

      const result = rankAndFilterAccountsForTray(preRankedAccounts, 'active@gmail.com', 0.10, 0.05, true);
      const emails = result.map((a) => a.email);

      assert.deepStrictEqual(emails, [
        'active@gmail.com',
        'custom_first@gmail.com',
        'custom_second@gmail.com',
      ]);
    });

    it('excludes active account if the active account itself is in cool down, error, or banned', () => {
      const accounts = [
        { email: 'active_cooling@gmail.com', status: 'COOLING', quota_5h_available: 0.01, quota_weekly: 0.01, plan_tier: 'Pro' },
        { email: 'standby_healthy@gmail.com', status: 'STANDBY', quota_5h_available: 0.90, quota_weekly: 0.90, plan_tier: 'Pro' },
      ];

      const ranked = rankAndFilterAccountsForTray(accounts, 'active_cooling@gmail.com');
      assert.strictEqual(ranked.length, 1);
      assert.strictEqual(ranked[0].email, 'standby_healthy@gmail.com');
    });
  });

  describe('buildSwitchMenuItems', () => {
    it('creates menu items with checkmark for active account and click handler for standbys', () => {
      const accounts = [
        { email: 'active@gmail.com', is_active: true, status: 'ACTIVE', quota_5h_available: 0.85, quota_weekly: 0.85 },
        { email: 'standby@gmail.com', is_active: false, status: 'STANDBY', quota_5h_available: 0.90, quota_weekly: 0.90, plan_tier: 'Pro' },
      ];

      let switchedTo = null;
      const items = buildSwitchMenuItems(accounts, 'active@gmail.com', (email) => {
        switchedTo = email;
      });

      assert.strictEqual(items.length, 2);
      // Active item
      assert.strictEqual(items[0].label, '✓ active@gmail.com');
      assert.strictEqual(items[0].enabled, false);

      // Standby item
      assert.strictEqual(items[1].label, 'standby@gmail.com');
      assert.strictEqual(items[1].enabled, true);

      items[1].click();
      assert.strictEqual(switchedTo, 'standby@gmail.com');
    });

    it('returns empty array when all accounts are in cooldown, error, or banned', () => {
      const accounts = [
        { email: 'cool@gmail.com', status: 'COOLDOWN' },
        { email: 'banned@gmail.com', status: 'BANNED' },
        { email: 'err@gmail.com', status: 'ERROR' },
        { email: 'err_msg@gmail.com', status: 'STANDBY', error_message: 'Revoked' },
      ];

      const items = buildSwitchMenuItems(accounts, 'none@gmail.com');
      assert.strictEqual(items.length, 0);
    });
  });

  describe('formatQuotaPercentage', () => {
    it('formats numbers into rounded percentages with % symbol', () => {
      assert.strictEqual(formatQuotaPercentage(0), '0%');
      assert.strictEqual(formatQuotaPercentage(1.0), '100%');
      assert.strictEqual(formatQuotaPercentage(0.85), '85%');
      assert.strictEqual(formatQuotaPercentage(0.854), '85%');
      assert.strictEqual(formatQuotaPercentage(0.856), '86%');
      assert.strictEqual(formatQuotaPercentage(0.05), '5%');
    });

    it('handles pre-scaled percentage numbers', () => {
      assert.strictEqual(formatQuotaPercentage(85), '85%');
      assert.strictEqual(formatQuotaPercentage(100), '100%');
    });

    it('handles out of bounds and non-numeric inputs', () => {
      assert.strictEqual(formatQuotaPercentage(null), '0%');
      assert.strictEqual(formatQuotaPercentage(undefined), '0%');
      assert.strictEqual(formatQuotaPercentage(NaN), '0%');
      assert.strictEqual(formatQuotaPercentage(-0.5), '0%');
      assert.strictEqual(formatQuotaPercentage(150), '100%');
    });
  });

  describe('formatQuotaInfo', () => {
    it('formats quota from account object matching "5H: {percentage} | 7D: {percentage}" format', () => {
      const acc = {
        quota_5h_current: 0.85,
        quota_weekly: 0.90,
      };
      assert.strictEqual(formatQuotaInfo(acc), '5H: 85% | 7D: 90%');
    });

    it('falls back to quota_5h_available if quota_5h_current is not set', () => {
      const acc = {
        quota_5h_available: 0.75,
        quota_weekly: 1.0,
      };
      assert.strictEqual(formatQuotaInfo(acc), '5H: 75% | 7D: 100%');
    });

    it('preserves 0 quota without falling back to available', () => {
      const acc = {
        quota_5h_current: 0,
        quota_5h_available: 0.8,
        quota_weekly: 0.05,
      };
      assert.strictEqual(formatQuotaInfo(acc), '5H: 0% | 7D: 5%');
    });

    it('formats quota from direct numeric arguments', () => {
      assert.strictEqual(formatQuotaInfo(0.65, 0.82), '5H: 65% | 7D: 82%');
      assert.strictEqual(formatQuotaInfo(0, 0), '5H: 0% | 7D: 0%');
    });

    it('handles null, undefined, or empty account objects gracefully', () => {
      assert.strictEqual(formatQuotaInfo(null), '5H: 0% | 7D: 0%');
      assert.strictEqual(formatQuotaInfo(undefined), '5H: 0% | 7D: 0%');
      assert.strictEqual(formatQuotaInfo({}), '5H: 0% | 7D: 0%');
    });

    it('uses " | " vertical divider as required', () => {
      const text = formatQuotaInfo(0.5, 0.5);
      assert.ok(text.includes(' | '));
      const parts = text.split(' | ');
      assert.strictEqual(parts.length, 2);
      assert.strictEqual(parts[0], '5H: 50%');
      assert.strictEqual(parts[1], '7D: 50%');
    });
  });

  describe('resolveActiveAccountAlias', () => {
    it('returns custom account label as alias when present', () => {
      const accounts = [
        { email: 'user@gmail.com', label: 'Work Account' },
      ];
      assert.strictEqual(resolveActiveAccountAlias(accounts, 'user@gmail.com'), 'Work Account');
    });

    it('returns custom account alias property if label is missing', () => {
      const accounts = [
        { email: 'user@gmail.com', alias: 'Personal Pro' },
      ];
      assert.strictEqual(resolveActiveAccountAlias(accounts, 'user@gmail.com'), 'Personal Pro');
    });

    it('falls back to email when label is empty or only whitespace', () => {
      const accounts = [
        { email: 'user@gmail.com', label: '   ' },
      ];
      assert.strictEqual(resolveActiveAccountAlias(accounts, 'user@gmail.com'), 'user@gmail.com');
    });

    it('falls back to activeEmail when accounts list is empty or account not found in list', () => {
      assert.strictEqual(resolveActiveAccountAlias([], 'unlisted@gmail.com'), 'unlisted@gmail.com');
      assert.strictEqual(resolveActiveAccountAlias([{ email: 'other@gmail.com' }], 'unlisted@gmail.com'), 'unlisted@gmail.com');
    });

    it('resolves active account by is_active flag when activeEmail is empty', () => {
      const accounts = [
        { email: 'idle@gmail.com', label: 'Idle' },
        { email: 'active@gmail.com', label: 'Current Runner', is_active: true },
      ];
      assert.strictEqual(resolveActiveAccountAlias(accounts, ''), 'Current Runner');
    });

    it('returns "Not Logged In" when no active email and no active account', () => {
      assert.strictEqual(resolveActiveAccountAlias([], ''), 'Not Logged In');
      assert.strictEqual(resolveActiveAccountAlias([], 'Not Logged In'), 'Not Logged In');
      assert.strictEqual(resolveActiveAccountAlias(null, null), 'Not Logged In');
    });
  });

  describe('buildTrayMenuTemplate', () => {
    it('builds menu with active account alias row and quota info row at the top, both disabled (grey)', () => {
      let openDashboardClicked = false;
      let systemSettingsClicked = false;
      let quitClicked = false;

      const template = buildTrayMenuTemplate({
        activeAlias: 'Primary Pro',
        quotaInfo: '5H: 85% | 7D: 90%',
        switchMenuItems: [{ label: '✓ Primary Pro', enabled: false }],
        accountsCount: 1,
        onOpenDashboard: () => { openDashboardClicked = true; },
        onSystemSettings: () => { systemSettingsClicked = true; },
        onQuit: () => { quitClicked = true; },
      });

      // Row 0: Active account (alias), grey and not clickable
      assert.strictEqual(template[0].label, 'Primary Pro');
      assert.strictEqual(template[0].enabled, false);

      // Row 1: Quota info, grey and not clickable
      assert.strictEqual(template[1].label, '5H: 85% | 7D: 90%');
      assert.strictEqual(template[1].enabled, false);

      // Verify "Antigravity Swiss Knife" header is NOT at top
      assert.notStrictEqual(template[0].label, 'Antigravity Swiss Knife');
      assert.notStrictEqual(template[1].label, 'Antigravity Swiss Knife');

      // Verify no duplicate "Active: ..." item in the menu
      const hasDuplicateActive = template.some((item) => typeof item.label === 'string' && item.label.startsWith('Active:'));
      assert.strictEqual(hasDuplicateActive, false);

      // Row 2: Separator
      assert.strictEqual(template[2].type, 'separator');

      // Row 3: Open Dashboard
      assert.strictEqual(template[3].label, 'Open Dashboard');
      template[3].click();
      assert.strictEqual(openDashboardClicked, true);

      // Row 4: Switch Google Account
      assert.strictEqual(template[4].label, 'Switch Google Account');
      assert.strictEqual(template[4].submenu.length, 1);
      assert.strictEqual(template[4].submenu[0].label, '✓ Primary Pro');

      // Row 5: Separator
      assert.strictEqual(template[5].type, 'separator');

      // Row 6: System Settings
      assert.strictEqual(template[6].label, 'System Settings');
      template[6].click();
      assert.strictEqual(systemSettingsClicked, true);

      // Row 7: Separator
      assert.strictEqual(template[7].type, 'separator');

      // Row 8: Quit Antigravity Swiss Knife
      assert.strictEqual(template[8].label, 'Quit Antigravity Swiss Knife');
      template[8].click();
      assert.strictEqual(quitClicked, true);
    });

    it('shows fallback submenu item when switchMenuItems is empty', () => {
      const templateNoAccs = buildTrayMenuTemplate({
        activeAlias: 'Not Logged In',
        quotaInfo: '5H: 0% | 7D: 0%',
        switchMenuItems: [],
        accountsCount: 0,
      });

      assert.strictEqual(templateNoAccs[4].label, 'Switch Google Account');
      assert.strictEqual(templateNoAccs[4].submenu[0].label, 'No accounts configured');
      assert.strictEqual(templateNoAccs[4].submenu[0].enabled, false);

      const templateAllExcluded = buildTrayMenuTemplate({
        activeAlias: 'Banned User',
        quotaInfo: '5H: 0% | 7D: 0%',
        switchMenuItems: [],
        accountsCount: 2,
      });

      assert.strictEqual(templateAllExcluded[4].submenu[0].label, 'No accounts available');
      assert.strictEqual(templateAllExcluded[4].submenu[0].enabled, false);
    });

    it('safely sanitizes non-string or null activeAlias and quotaInfo to avoid Electron template crash', () => {
      const templateNull = buildTrayMenuTemplate({
        activeAlias: null,
        quotaInfo: null,
      });

      assert.strictEqual(templateNull[0].label, 'Not Logged In');
      assert.strictEqual(templateNull[0].enabled, false);
      assert.strictEqual(templateNull[1].label, '5H: 0% | 7D: 0%');
      assert.strictEqual(templateNull[1].enabled, false);

      const templateEmpty = buildTrayMenuTemplate({
        activeAlias: '   ',
        quotaInfo: '   ',
      });

      assert.strictEqual(templateEmpty[0].label, 'Not Logged In');
      assert.strictEqual(templateEmpty[1].label, '5H: 0% | 7D: 0%');
    });
  });

  describe('Robustness and Edge Cases', () => {
    it('handles activeEmail="Not Logged In" sentinel in buildSwitchMenuItems by falling back to is_active', () => {
      const accounts = [
        { email: 'active@gmail.com', is_active: true, status: 'ACTIVE', quota_5h_available: 0.9, quota_weekly: 0.9 },
        { email: 'standby@gmail.com', is_active: false, status: 'STANDBY', quota_5h_available: 0.8, quota_weekly: 0.8 },
      ];

      const items = buildSwitchMenuItems(accounts, 'Not Logged In');
      assert.strictEqual(items.length, 2);
      assert.strictEqual(items[0].label, '✓ active@gmail.com');
      assert.strictEqual(items[0].enabled, false);
      assert.strictEqual(items[1].label, 'standby@gmail.com');
      assert.strictEqual(items[1].enabled, true);
    });

    it('handles activeEmail="Not Logged In" sentinel in compareFleetAccounts by falling back to is_active', () => {
      const accActive = { email: 'active@gmail.com', is_active: true, plan_tier: 'Pro', quota_5h_available: 0.5, quota_weekly: 0.5 };
      const accStandby = { email: 'standby@gmail.com', is_active: false, plan_tier: 'Ultra 20X', quota_5h_available: 1.0, quota_weekly: 1.0 };

      // Active must rank ahead of Ultra standby even when activeEmail is "Not Logged In"
      const res = compareFleetAccounts(accActive, accStandby, 'Not Logged In');
      assert.strictEqual(res, -1);
    });

    it('parses numeric and percentage strings in formatQuotaPercentage', () => {
      assert.strictEqual(formatQuotaPercentage('85%'), '85%');
      assert.strictEqual(formatQuotaPercentage('0.85'), '85%');
      assert.strictEqual(formatQuotaPercentage('100%'), '100%');
      assert.strictEqual(formatQuotaPercentage('  50 % '), '50%');
      assert.strictEqual(formatQuotaPercentage('invalid'), '0%');
    });

    it('handles pre-formatted strings, min_fraction, and available quota fields in formatQuotaInfo', () => {
      assert.strictEqual(formatQuotaInfo('5H: 80% | 7D: 90%'), '5H: 80% | 7D: 90%');
      assert.strictEqual(formatQuotaInfo({ min_fraction: 0.65, quota_weekly_available: 0.75 }), '5H: 65% | 7D: 75%');
      assert.strictEqual(formatQuotaInfo({ quota_5h_fraction: 0.40, quota_7d_available: 0.55 }), '5H: 40% | 7D: 55%');
    });

    it('resolves active account alias when accounts array contains plain strings', () => {
      const stringAccounts = ['alice@gmail.com', 'bob@gmail.com'];
      assert.strictEqual(resolveActiveAccountAlias(stringAccounts, 'alice@gmail.com'), 'alice@gmail.com');
    });
  });
});

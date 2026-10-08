import { describe, it } from 'node:test'
import assert from 'node:assert'
import {
  getAccountTableDisplay,
  getAccountHeaderDisplay,
  resolveDefaultAlias,
  normalizeMfaSecret,
  ACCOUNT_SETUP_TEXTS,
} from './accountPresentation.ts'

describe('accountPresentation utility', () => {
  describe('getAccountTableDisplay', () => {
    it('shows only the account alias and hides the id when alias equals account id', () => {
      const res = getAccountTableDisplay('alice@example.com', 'alice@example.com')
      assert.strictEqual(res.primaryText, 'alice@example.com')
      assert.strictEqual(res.secondaryText, null)
    })

    it('shows only the account alias and hides id with whitespace differences', () => {
      const res = getAccountTableDisplay('  user@gmail.com  ', 'user@gmail.com')
      assert.strictEqual(res.primaryText, 'user@gmail.com')
      assert.strictEqual(res.secondaryText, null)
    })

    it('shows only the account alias and hides id with case differences in email', () => {
      const res = getAccountTableDisplay('User@Gmail.Com', 'user@gmail.com')
      assert.strictEqual(res.primaryText, 'User@Gmail.Com')
      assert.strictEqual(res.secondaryText, null)
    })

    it('shows only id as primary and hides secondary when alias contains only whitespace', () => {
      const res = getAccountTableDisplay('   ', 'bob@example.com')
      assert.strictEqual(res.primaryText, 'bob@example.com')
      assert.strictEqual(res.secondaryText, null)
    })

    it('shows both alias and account id when alias differs from id', () => {
      const res = getAccountTableDisplay('Primary Account', 'alice@example.com')
      assert.strictEqual(res.primaryText, 'Primary Account')
      assert.strictEqual(res.secondaryText, 'alice@example.com')
    })

    it('shows only id as primary when alias is empty', () => {
      const res = getAccountTableDisplay('', 'bob@example.com')
      assert.strictEqual(res.primaryText, 'bob@example.com')
      assert.strictEqual(res.secondaryText, null)
    })
  })

  describe('getAccountHeaderDisplay', () => {
    it('shows grey Account Alias placeholder for new account when alias is not set', () => {
      const res = getAccountHeaderDisplay(true, '', '')
      assert.strictEqual(res.text, 'Account Alias')
      assert.strictEqual(res.isPlaceholder, true)
      assert.strictEqual(res.showStatusBadge, false)
    })

    it('changes to real account alias in black once value is set for new account', () => {
      const res = getAccountHeaderDisplay(true, 'Secondary Backup', '')
      assert.strictEqual(res.text, 'Secondary Backup')
      assert.strictEqual(res.isPlaceholder, false)
      assert.strictEqual(res.showStatusBadge, false)
    })

    it('shows existing account alias and enables status badge', () => {
      const res = getAccountHeaderDisplay(false, 'Work Account', 'work@company.com')
      assert.strictEqual(res.text, 'Work Account')
      assert.strictEqual(res.isPlaceholder, false)
      assert.strictEqual(res.showStatusBadge, true)
    })

    it('falls back to email for existing account without alias and enables status badge', () => {
      const res = getAccountHeaderDisplay(false, '', 'dev@gmail.com')
      assert.strictEqual(res.text, 'dev@gmail.com')
      assert.strictEqual(res.isPlaceholder, false)
      assert.strictEqual(res.showStatusBadge, true)
    })

    it('shows grey placeholder for existing account with empty alias and empty email', () => {
      const res = getAccountHeaderDisplay(false, '', '')
      assert.strictEqual(res.text, 'Account Alias')
      assert.strictEqual(res.isPlaceholder, true)
      assert.strictEqual(res.showStatusBadge, true)
    })
  })

  describe('resolveDefaultAlias', () => {
    it('defaults to email when alias is empty', () => {
      assert.strictEqual(resolveDefaultAlias('', 'test@gmail.com'), 'test@gmail.com')
    })

    it('defaults to email when alias contains only whitespace', () => {
      assert.strictEqual(resolveDefaultAlias('   ', 'test@gmail.com'), 'test@gmail.com')
    })

    it('preserves user-entered alias when non-empty', () => {
      assert.strictEqual(resolveDefaultAlias('Custom Label', 'test@gmail.com'), 'Custom Label')
    })
  })

  describe('normalizePlanTier', () => {
    it('normalizes various raw tier strings to canonical forms', async () => {
      const { normalizePlanTier } = await import('../types.ts')
      assert.strictEqual(normalizePlanTier(undefined), 'Free')
      assert.strictEqual(normalizePlanTier(''), 'Free')
      assert.strictEqual(normalizePlanTier('free'), 'Free')
      assert.strictEqual(normalizePlanTier('plus'), 'Plus')
      assert.strictEqual(normalizePlanTier('pro'), 'Pro')
      assert.strictEqual(normalizePlanTier('Google AI Pro'), 'Pro')
      assert.strictEqual(normalizePlanTier('trial'), 'Pro - Trial')
      assert.strictEqual(normalizePlanTier('pro-trial'), 'Pro - Trial')
      assert.strictEqual(normalizePlanTier('promo'), 'Pro - Trial')
      assert.strictEqual(normalizePlanTier('starter pro'), 'Pro - Trial')
      assert.strictEqual(normalizePlanTier('jio offer'), 'Pro - Trial')
      assert.strictEqual(normalizePlanTier('partner bundle'), 'Pro - Trial')
      assert.strictEqual(normalizePlanTier('Sonnet 5.5 is now available on paid Pro and Ultra plans. Third-party model access will no longer be available on your current plan starting on November 2, 2026.'), 'Pro - Trial')
      assert.strictEqual(normalizePlanTier('starter quota'), 'Free')
      assert.strictEqual(normalizePlanTier('edu'), 'Edu')
      assert.strictEqual(normalizePlanTier('stanford.edu'), 'Edu')
      assert.strictEqual(normalizePlanTier('teams_tier_enterprise'), 'Enterprise')
      assert.strictEqual(normalizePlanTier('ultra 5x'), 'Ultra 5X')
      assert.strictEqual(normalizePlanTier('ultra 10x'), 'Ultra 10X')
      assert.strictEqual(normalizePlanTier('ultra 20x'), 'Ultra 20X')
      assert.strictEqual(normalizePlanTier('ultra'), 'Ultra 20X')
    })
  })

  describe('normalizeMfaSecret', () => {
    it('auto-removes spaces from spaced grouped 4-characters format', () => {
      assert.strictEqual(normalizeMfaSecret('JBSW Y3DP EHPK 3PXP'), 'JBSWY3DPEHPK3PXP')
      assert.strictEqual(normalizeMfaSecret('HXDM J3VF 9G4W 2ZQA'), 'HXDMJ3VF9G4W2ZQA')
      assert.strictEqual(normalizeMfaSecret('ABCD EFGH 1234 5678'), 'ABCDEFGH12345678')
    })

    it('preserves continuous character string format unchanged', () => {
      assert.strictEqual(normalizeMfaSecret('JBSWY3DPEHPK3PXP'), 'JBSWY3DPEHPK3PXP')
      assert.strictEqual(normalizeMfaSecret('HXDMJ3VF9G4W2ZQA'), 'HXDMJ3VF9G4W2ZQA')
    })

    it('handles irregular and multiple consecutive spaces', () => {
      assert.strictEqual(normalizeMfaSecret('  JBSW   Y3DP  EHPK   3PXP  '), 'JBSWY3DPEHPK3PXP')
    })

    it('handles empty and undefined inputs safely', () => {
      assert.strictEqual(normalizeMfaSecret(''), '')
      assert.strictEqual(normalizeMfaSecret('   '), '')
      assert.strictEqual(normalizeMfaSecret(undefined as any), '')
    })
  })

  describe('ACCOUNT_SETUP_TEXTS', () => {
    it('verifies password label and placeholder contain no vault references', () => {
      assert.ok(!ACCOUNT_SETUP_TEXTS.PASSWORD_LABEL.toLowerCase().includes('vault'))
      assert.strictEqual(ACCOUNT_SETUP_TEXTS.PASSWORD_LABEL, 'Password (Optional):')
      assert.ok(!ACCOUNT_SETUP_TEXTS.PASSWORD_PLACEHOLDER.toLowerCase().includes('vault'))
      assert.strictEqual(ACCOUNT_SETUP_TEXTS.PASSWORD_PLACEHOLDER, 'Optional login password')
    })

    it('verifies OAuth refresh token label and placeholder text', () => {
      assert.strictEqual(ACCOUNT_SETUP_TEXTS.OAUTH_REFRESH_LABEL, 'OAuth Refresh Token:')
      assert.strictEqual(
        ACCOUNT_SETUP_TEXTS.OAUTH_REFRESH_PLACEHOLDER,
        '1//... (Sign in with Google or paste token)'
      )
    })
  })

  describe('Edge cases and model settings', () => {
    it('handles mixed case and symbols in alias resolution', () => {
      assert.strictEqual(resolveDefaultAlias('', 'Developer+Test@Company.CO.UK'), 'Developer+Test@Company.CO.UK')
      assert.strictEqual(resolveDefaultAlias('  My Alias  ', 'test@google.com'), 'My Alias')
    })

    it('verifies getAccountTableDisplay with mixed uppercase and symbols', () => {
      const res = getAccountTableDisplay('Admin+Team@Google.COM', 'admin+team@google.com')
      assert.strictEqual(res.primaryText, 'Admin+Team@Google.COM')
      assert.strictEqual(res.secondaryText, null)
    })
  })

  describe('AccountDetailModal presentation integrity', () => {
    it('verifies Gemini Quota grey-out, weekly reset time, aligned right-column controls, and dual toggle layout', async () => {
      const fs = await import('node:fs')
      const path = await import('node:path')
      const modalSrc = fs.readFileSync(
        path.resolve(import.meta.dirname, '../components/AccountDetailModal.tsx'),
        'utf8'
      )
      assert.ok(modalSrc.includes('Gemini Quota'), 'should contain Gemini Quota section header')
      assert.ok(modalSrc.includes('isQuotaConnected'), 'should gate quota display on connection state')
      assert.ok(modalSrc.includes('disabled={!isQuotaConnected}'), 'should disable quota bars when not connected')
      assert.ok(modalSrc.includes('account.reset_horizon_weekly_text'), 'should display actual weekly reset horizon text')
      assert.ok(!modalSrc.includes('7-day allowance'), 'should not hardcode 7-day allowance text')
      assert.ok(modalSrc.includes("const RIGHT_ACTION_WIDTH = '172px'"), 'should standardize right column width for Priority, Google button, and MFA gadget')
      assert.ok(modalSrc.includes('Allow Claude &amp; GPT models'), 'should contain Allow Claude & GPT models toggle')
      assert.ok(modalSrc.includes('Allow using AI credits'), 'should contain Allow using AI credits toggle')
      assert.ok(!modalSrc.includes('Claude &amp; GPT Models:'), 'should not render header above Claude & GPT models toggle')
      assert.ok(!modalSrc.includes('Plan Tier / Membership:'), 'should not contain manual Plan Tier select in form body')
      assert.ok(!modalSrc.includes('Credit Pool'), 'should not contain manual Credit Pool in form body')
      assert.ok(modalSrc.includes('renderPlanTierBadge(account.plan_tier)'), 'should render read-only plan tier badge in header')
      assert.ok(modalSrc.includes('getAccountHeaderDisplay'), 'should use getAccountHeaderDisplay')
      assert.ok(modalSrc.includes('onBlur={handleEmailBlur}'), 'should trigger handleEmailBlur on account ID blur')
      assert.ok(modalSrc.includes('Show OAuth refresh token'), 'should have refresh token eye toggle tooltip')
      assert.ok(modalSrc.includes("refreshToken.trim().startsWith('ya29.')"), 'should detect temporary access token and advise refresh token')
    })
  })
})

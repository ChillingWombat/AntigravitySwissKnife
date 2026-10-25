import { afterEach, describe, it } from 'node:test'
import assert from 'node:assert'
import {
  getAccountTableDisplay,
  getAccountHeaderDisplay,
  resolveDefaultAlias,
  copyTextWithFallback,
  normalizeMfaSecret,
  parseAccountErrorAlert,
  ACCOUNT_SETUP_TEXTS,
  commitLatest,
  copyWithStatus,
  createRequestGate,
  shouldHydrateAccountCredentials,
  resolveHydratedAccountCredentials,
} from './accountPresentation.ts'

const originalBrowserGlobals = {
  window: Object.getOwnPropertyDescriptor(globalThis, 'window'),
  navigator: Object.getOwnPropertyDescriptor(globalThis, 'navigator'),
  document: Object.getOwnPropertyDescriptor(globalThis, 'document'),
}

function installBrowserGlobals(
  windowValue: unknown,
  navigatorValue: unknown,
  documentValue: unknown
) {
  Object.defineProperty(globalThis, 'window', { configurable: true, value: windowValue })
  Object.defineProperty(globalThis, 'navigator', { configurable: true, value: navigatorValue })
  Object.defineProperty(globalThis, 'document', { configurable: true, value: documentValue })
}

afterEach(() => {
  for (const [key, descriptor] of Object.entries(originalBrowserGlobals)) {
    if (descriptor) {
      Object.defineProperty(globalThis, key, descriptor)
    } else {
      delete (globalThis as any)[key]
    }
  }
})

describe('accountPresentation utility', () => {
  describe('credential hydration', () => {
    it('hydrates a fleet-stripped MFA secret', () => {
      const account = {
        email: 'fleet@example.com',
        has_totp: true,
        totp_secret: '',
      }

      assert.strictEqual(shouldHydrateAccountCredentials(account, false), true)
      const resolved = resolveHydratedAccountCredentials(
        account,
        { totp_secret: 'JBSWY3DPEHPK3PXP' },
        { password: '', totpSecret: '', accessToken: '', refreshToken: '' }
      )

      assert.strictEqual(resolved.totpSecret, 'JBSWY3DPEHPK3PXP')
    })

    it('hydrates a single redacted access token', () => {
      const account = {
        email: 'redacted@example.com',
        access_token: '[REDACTED]',
        refresh_token: 'stored-refresh-token',
      }

      assert.strictEqual(shouldHydrateAccountCredentials(account, false), true)
      const resolved = resolveHydratedAccountCredentials(
        account,
        { access_token: 'fresh-access-token' },
        { password: '', totpSecret: '', accessToken: '', refreshToken: 'stored-refresh-token' }
      )

      assert.strictEqual(resolved.accessToken, 'fresh-access-token')
    })

    it('uses usable fallback credentials when primary values are redacted', () => {
      const account = {
        email: 'fallback@example.com',
        totp_secret: '[REDACTED]',
        access_token: '[REDACTED]',
      }

      const resolved = resolveHydratedAccountCredentials(
        account,
        {
          totp_secret: '[REDACTED]',
          mfa: 'FALLBACK-MFA-SECRET',
          access_token: '[REDACTED]',
          credential: { access_token: 'FALLBACK-ACCESS-TOKEN' },
        },
        { password: '', totpSecret: '', accessToken: '', refreshToken: '' }
      )

      assert.strictEqual(resolved.totpSecret, 'FALLBACK-MFA-SECRET')
      assert.strictEqual(resolved.accessToken, 'FALLBACK-ACCESS-TOKEN')
    })

    it('preserves MFA and access token edits made before hydration resolves', () => {
      const account = {
        email: 'edited@example.com',
        totp_secret: '[REDACTED]',
        access_token: '[REDACTED]',
      }
      const current = {
        password: '',
        totpSecret: 'USER-MFA-SECRET',
        accessToken: 'USER-ACCESS-TOKEN',
        refreshToken: '',
      }

      const resolved = resolveHydratedAccountCredentials(
        account,
        {
          totp_secret: 'EXPORTED-MFA-SECRET',
          access_token: 'EXPORTED-ACCESS-TOKEN',
        },
        current
      )

      assert.deepStrictEqual(resolved, current)
    })
  })

  describe('copyTextWithFallback', () => {
    it('uses window.electronAPI.copyToClipboard when present', async () => {
      let copiedText = ''
      const copyToClipboard = async (text: string) => {
        copiedText = text
        return true
      }
      installBrowserGlobals(
        { electronAPI: { copyToClipboard, writeClipboard: copyToClipboard } },
        { clipboard: { writeText: async () => assert.fail('navigator fallback used') } },
        {}
      )

      assert.strictEqual(await copyTextWithFallback('electron-copy'), true)
      assert.strictEqual(copiedText, 'electron-copy')
    })

    it('falls back to navigator.clipboard.writeText', async () => {
      let copiedText = ''
      let execCommandCalls = 0
      installBrowserGlobals(
        {},
        {
          clipboard: {
            writeText: async (text: string) => {
              copiedText = text
            },
          },
        },
        {
          execCommand: () => {
            execCommandCalls += 1
            return true
          },
        }
      )

      assert.strictEqual(await copyTextWithFallback('navigator-copy'), true)
      assert.strictEqual(copiedText, 'navigator-copy')
      assert.strictEqual(execCommandCalls, 0)
    })

    it('succeeds with document.execCommand when navigator.clipboard.writeText rejects', async () => {
      const textArea = {
        value: '',
        style: {},
        focus() {},
        select() {},
        remove() {},
      }
      let copiedText = ''
      installBrowserGlobals(
        {},
        {
          clipboard: {
            writeText: async () => {
              throw new Error('clipboard denied')
            },
          },
        },
        {
          body: { appendChild() {}, removeChild() {} },
          createElement: () => textArea,
          execCommand: (command: string) => {
            assert.strictEqual(command, 'copy')
            copiedText = textArea.value
            return true
          },
        }
      )

      assert.strictEqual(await copyTextWithFallback('exec-copy'), true)
      assert.strictEqual(copiedText, 'exec-copy')
    })

    it('returns false when document.execCommand returns false', async () => {
      const textArea = {
        value: '',
        style: {},
        focus() {},
        select() {},
        remove() {},
      }
      installBrowserGlobals(
        {},
        {
          clipboard: {
            writeText: async () => {
              throw new Error('clipboard denied')
            },
          },
        },
        {
          body: { appendChild() {}, removeChild() {} },
          createElement: () => textArea,
          execCommand: () => false,
        }
      )

      assert.strictEqual(await copyTextWithFallback('123456'), false)
    })
  })

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
      assert.strictEqual(normalizePlanTier('Google AI Pro'), 'Pro - Trial')
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

    it('falls back to Free when the tier is only a product label', async () => {
      const { normalizePlanTier } = await import('../types.ts')
      // Google answers an unlicensed account with a tier named after the app.
      assert.strictEqual(normalizePlanTier('Antigravity'), 'Free')
      assert.strictEqual(normalizePlanTier('antigravity'), 'Free')
      assert.strictEqual(normalizePlanTier('Antigravity Pro'), 'Pro')
      assert.strictEqual(normalizePlanTier('something unparseable'), 'Free')
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

  describe('Account async race protection', () => {
    it('ignores stale quota and TOTP results after account and secret switches', async () => {
      const quotaGate = createRequestGate()
      const totpGate = createRequestGate()
      let resolveStaleQuota!: (value: string) => void
      let resolveStaleTotp!: (value: string) => void
      const staleQuota = new Promise<string>((resolve) => {
        resolveStaleQuota = resolve
      })
      const staleTotp = new Promise<string>((resolve) => {
        resolveStaleTotp = resolve
      })
      const staleQuotaRequest = quotaGate.begin()
      const staleTotpRequest = totpGate.begin()
      const currentQuotaRequest = quotaGate.begin()
      const currentTotpRequest = totpGate.begin()
      const rendered = { account: 'second@example.com', quota: '', totp: '' }

      assert.strictEqual(
        await commitLatest(quotaGate, currentQuotaRequest, Promise.resolve('87%'), (value) => {
          rendered.quota = value
        }),
        true
      )
      assert.strictEqual(
        await commitLatest(totpGate, currentTotpRequest, Promise.resolve('654321'), (value) => {
          rendered.totp = value
        }),
        true
      )

      resolveStaleQuota('13%')
      resolveStaleTotp('123456')
      assert.strictEqual(
        await commitLatest(quotaGate, staleQuotaRequest, staleQuota, (value) => {
          rendered.quota = value
        }),
        false
      )
      assert.strictEqual(
        await commitLatest(totpGate, staleTotpRequest, staleTotp, (value) => {
          rendered.totp = value
        }),
        false
      )
      assert.deepStrictEqual(rendered, {
        account: 'second@example.com',
        quota: '87%',
        totp: '654321',
      })
    })

    it('leaves copied false when the clipboard write rejects', async () => {
      const copiedStates: boolean[] = []
      let copied = false
      const result = await copyWithStatus(
        async () => {
          throw new Error('clipboard denied')
        },
        (nextCopied) => {
          copied = nextCopied
          copiedStates.push(nextCopied)
        }
      )

      assert.strictEqual(result, false)
      assert.strictEqual(copied, false)
      assert.deepStrictEqual(copiedStates, [false])
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
      assert.ok(
        modalSrc.includes('renderPlanTierBadge(currentPlanTier)'),
        'should render read-only plan tier badge in header'
      )
      assert.ok(
        modalSrc.includes('getAccountHeaderDisplay(isNewAccount, label, email)'),
        'should derive account header presentation through getAccountHeaderDisplay'
      )
      assert.ok(modalSrc.includes('displayAccountName'), 'should use the derived display account name')
      assert.ok(!modalSrc.includes('account.name ='), 'should not mutate account.name')
      assert.ok(!modalSrc.includes('account.plan_tier ='), 'should not mutate account.plan_tier')
      assert.ok(modalSrc.includes('onBlur={handleEmailBlur}'), 'should trigger handleEmailBlur on account ID blur')
      assert.ok(modalSrc.includes('Show OAuth refresh token'), 'should have refresh token eye toggle tooltip')
      assert.ok(modalSrc.includes("refreshToken.trim().startsWith('ya29.')"), 'should detect temporary access token and advise refresh token')
      assert.ok(modalSrc.includes("'COOLING'"), 'should render COOLING status badge')
      assert.ok(!modalSrc.includes("'COOL DOWN'"), 'should not contain legacy COOL DOWN badge')
    })

    it('verifies toAccountState normalizes legacy COOLDOWN status to COOLING', async () => {
      const { toAccountState } = await import('../types.ts')
      const legacyAccount = {
        email: 'legacy@example.com',
        status: 'COOLDOWN',
        is_active: false,
      }
      const state = toAccountState(legacyAccount)
      assert.strictEqual(state.status, 'COOLING', 'legacy COOLDOWN should be normalized to COOLING')

      const coolingAccount = {
        email: 'cooling@example.com',
        status: 'COOLING',
        is_active: false,
      }
      const stateCooling = toAccountState(coolingAccount)
      assert.strictEqual(stateCooling.status, 'COOLING', 'COOLING status should be preserved')

      const standbyAccount = {
        email: 'standby@example.com',
        status: 'STANDBY',
        is_active: false,
      }
      const stateStandby = toAccountState(standbyAccount)
      assert.strictEqual(stateStandby.status, 'STANDBY', 'STANDBY status should be preserved')
    })

    it('verifies QuotaDashboardPage displays Cooling and eliminates legacy Cool Down status', async () => {
      const fs = await import('node:fs')
      const path = await import('node:path')
      const pageSrc = fs.readFileSync(
        path.resolve(import.meta.dirname, '../pages/QuotaDashboardPage.tsx'),
        'utf8'
      )
      assert.ok(pageSrc.includes('Cooling'), 'should display Cooling badge')
      assert.ok(!pageSrc.includes('Cool Down'), 'should not contain legacy Cool Down text')
      assert.ok(!pageSrc.includes('cooling down until reset'), 'should not contain legacy cooling down tooltip')
      assert.ok(pageSrc.includes('cooling until reset'), 'should contain modern cooling tooltip')
    })

    it('verifies QuotaDashboardPage labels the non-Gemini quota bucket as Non-Gemini', async () => {
      const fs = await import('node:fs')
      const path = await import('node:path')
      const pageSrc = fs.readFileSync(
        path.resolve(import.meta.dirname, '../pages/QuotaDashboardPage.tsx'),
        'utf8'
      )
      assert.ok(pageSrc.includes('title="Non-Gemini 5H"'), 'should title the 5h gauge Non-Gemini 5H')
      assert.ok(pageSrc.includes('title="Non-Gemini 7D"'), 'should title the weekly gauge Non-Gemini 7D')
      assert.ok(pageSrc.includes('<span>Non-Gemini</span>'), 'should label detail rows Non-Gemini')
      assert.ok(pageSrc.includes('title="Non-Gemini 5h cycle"'), 'should title the 5h bar Non-Gemini 5h cycle')
      assert.ok(pageSrc.includes('title="Non-Gemini 7D cycle"'), 'should title the weekly bar Non-Gemini 7-day cycle')
      assert.ok(!pageSrc.includes('Claude/GPT'), 'should not contain legacy Claude/GPT quota labels')
      assert.ok(!pageSrc.includes('Claude &amp; GPT'), 'should not contain legacy Claude & GPT quota labels')
    })

    it('verifies separate Gemini and Gemini 4 Argon 5H/7D gauges and bars', async () => {
      const fs = await import('node:fs')
      const path = await import('node:path')
      const pageSrc = fs.readFileSync(
        path.resolve(import.meta.dirname, '../pages/QuotaDashboardPage.tsx'),
        'utf8'
      )
      const modalSrc = fs.readFileSync(
        path.resolve(import.meta.dirname, '../components/AccountDetailModal.tsx'),
        'utf8'
      )
      assert.ok(pageSrc.includes('title="Gemini 7D"'), 'should render a Gemini 7D ring')
      assert.ok(pageSrc.includes('title="Argon 5H"'), 'should render a separate Argon 5H ring')
      assert.ok(pageSrc.includes('title="Argon 7D"'), 'should render a separate Argon 7D ring')
      assert.ok(pageSrc.includes('const hasArgon5h = polledAccounts5hArgon.length > 0'), 'should detect fleet Argon 5H quota')
      assert.ok(pageSrc.includes('const hasArgon7d = polledAccountsWeeklyArgon.length > 0'), 'should detect fleet Argon 7D quota')
      assert.ok(pageSrc.includes('{hasArgon5h && ('), 'should omit the Argon 5H ring when no fleet Argon quota exists')
      assert.ok(pageSrc.includes("flexDirection: 'row'"), 'should render quota rings in horizontal rows')
      assert.ok(pageSrc.includes("flexWrap: 'nowrap'"), 'should keep each quota row on one line')
      assert.ok(pageSrc.includes("gap: '12px'"), 'should use consistent spacing between quota rings and rows')
      assert.ok(pageSrc.includes('title="Argon 7D"'), 'should render Argon 7D in both overview layouts')
      assert.ok(pageSrc.includes('quota_5h_argon'), 'should bind the 5H Argon bar to real quota data')
      assert.ok(pageSrc.includes('quota_weekly_argon'), 'should bind the Argon bar to real Argon quota data')
      assert.ok(pageSrc.includes('if (hasArgon5h)'), 'should split the Gemini 5H cell only when real fleet Argon 5H quota exists')
      assert.ok(modalSrc.includes('Gemini 4 Argon 5H Quota'), 'account details should show Argon 5H quota')
      assert.ok(modalSrc.includes('Gemini 4 Argon 7D Quota'), 'account details should show Argon 7D quota')
    })

    it('verifies QuotaDashboardPage displays Credit column header and reduced column width', async () => {
      const fs = await import('node:fs')
      const path = await import('node:path')
      const pageSrc = fs.readFileSync(
        path.resolve(import.meta.dirname, '../pages/QuotaDashboardPage.tsx'),
        'utf8'
      )
      assert.ok(pageSrc.includes('<span>Credit</span>'), 'should display Credit column title')
      assert.ok(!pageSrc.includes('<span>AI Credits</span>'), 'should not contain legacy AI Credits column header')
      assert.ok(pageSrc.includes("width: '76px'"), 'should use compact 76px column width for Credit')
    })

    it('verifies QuotaDashboardPage removes manual Relaunch IDE button and relaunch status text while keeping automatic relaunch on switch', async () => {
      const fs = await import('node:fs')
      const path = await import('node:path')
      const pageSrc = fs.readFileSync(
        path.resolve(import.meta.dirname, '../pages/QuotaDashboardPage.tsx'),
        'utf8'
      )
      assert.ok(!pageSrc.includes('Relaunch IDE'), 'should not contain manual Relaunch IDE button')
      assert.ok(!pageSrc.includes('Relaunching Antigravity IDE'), 'should not contain Relaunching Antigravity IDE text')
      assert.ok(!pageSrc.includes('Switched active account to'), 'should not contain Switched active account status text')
      assert.ok(pageSrc.includes('api.switchAccount(acc.email, true)'), 'should trigger automatic relaunch on table switch')
      assert.ok(pageSrc.includes('api.switchAccount(target, true)'), 'should trigger automatic relaunch on context menu switch')
    })

    it('parses VALIDATION_REQUIRED verification URLs and populates empty authuser parameter', () => {
      const raw =
        'Verify your account to continue. (VALIDATION_REQUIRED) (Verification: https://accounts.google.com/signin/continue?sarp=1&scc=1&continue=https://developers.google.com/gemini-code-assist/auth/auth_success_gemini&plt=AKgnsbs&flowName=GlifWebSignIn&authuser)'
      const parsed = parseAccountErrorAlert(raw, 'joseantoniocarrarofalchi@gmail.com')
      assert.strictEqual(parsed.isValidationRequired, true)
      assert.strictEqual(parsed.summaryText, 'Verify your account to continue. (VALIDATION_REQUIRED)')
      assert.strictEqual(
        parsed.verificationUrl,
        'https://accounts.google.com/signin/continue?sarp=1&scc=1&continue=https://developers.google.com/gemini-code-assist/auth/auth_success_gemini&plt=AKgnsbs&flowName=GlifWebSignIn&authuser=joseantoniocarrarofalchi%40gmail.com'
      )

      const fallback = parseAccountErrorAlert(null)
      assert.strictEqual(fallback.isValidationRequired, false)
      assert.strictEqual(fallback.verificationUrl, null)
    })

    it('fetches credentials for redacted or fleet-stripped state and does not rerun after hydration', async () => {
      const fs = await import('node:fs')
      const path = await import('node:path')
      const modalSrc = fs.readFileSync(
        path.resolve(import.meta.dirname, '../components/AccountDetailModal.tsx'),
        'utf8'
      )
      const exportEffect = modalSrc.match(
        /useEffect\(\(\) => \{[\s\S]*?shouldHydrateAccountCredentials\(account, isNewAccount\)[\s\S]*?resolveHydratedAccountCredentials\(account, found, \{[\s\S]*?\n  \}, \[([^\]]+)\]\)/
      )

      assert.ok(modalSrc.includes('api.exportAccounts(account.email)'), 'should call api.exportAccounts with account email')
      assert.ok(exportEffect, 'should have a dedicated credential export effect')
      assert.ok(
        modalSrc.includes('shouldHydrateAccountCredentials(account, isNewAccount)'),
        'should gate credential export with the shared hydration check'
      )
      assert.ok(
        modalSrc.includes('resolveHydratedAccountCredentials(account, found, {'),
        'should hydrate exported credentials through the shared resolver'
      )
      assert.ok(
        exportEffect && !exportEffect[1].includes('refreshToken'),
        'should not rerun the export when local credential state is hydrated'
      )
      assert.ok(
        !modalSrc.includes('[account.email, refreshToken]'),
        'should not rerun quota loading when the refresh token is hydrated'
      )
      assert.ok(
        modalSrc.includes('credentialTouchedRef.current.refreshToken = true'),
        'should protect a user-edited refresh token from async hydration'
      )
      assert.ok(
        modalSrc.includes('credentialTouchedRef.current.password = true'),
        'should protect a user-edited password from async hydration'
      )
      assert.ok(
        modalSrc.includes('credentialTouchedRef.current.totpSecret = true'),
        'should protect a user-edited MFA secret from async hydration'
      )
      assert.ok(
        modalSrc.includes(
          'resolveMfaSecretForSave(account, totpSecret, credentialTouchedRef.current.totpSecret)'
        ),
        'should resolve MFA changes through the shared save resolver'
      )
      assert.ok(
        !modalSrc.includes('||\n          exported[0]') && !modalSrc.includes('|| exported[0]'),
        'should never fall back to another account credential'
      )
    })
  })
})
import {
  REDACTED_CREDENTIAL as REDACTED_CREDENTIAL_FOR_SAVE,
  resolveMfaSecretForSave,
} from './accountPresentation'
describe('resolveMfaSecretForSave', () => {
  it('preserves an untouched stored MFA secret and clears a touched empty value', () => {
    const account = { has_totp: true, totp_secret: REDACTED_CREDENTIAL_FOR_SAVE }

    assert.strictEqual(
      resolveMfaSecretForSave(account, '', false),
      REDACTED_CREDENTIAL_FOR_SAVE
    )
    assert.strictEqual(resolveMfaSecretForSave(account, '', true), '[CLEAR]')
  })

  it('normalizes a non-empty MFA secret before saving', () => {
    assert.strictEqual(
      resolveMfaSecretForSave({ has_totp: false }, '  abC123  ', false),
      'ABC123'
    )
  })

  it('uses resolveMfaSecretForSave without a hydration gate', async () => {
    const fs = await import('node:fs')
    const path = await import('node:path')
    const modalSrc = fs.readFileSync(
      path.resolve(import.meta.dirname, '../components/AccountDetailModal.tsx'),
      'utf8'
    )

    assert.ok(
      modalSrc.includes(
        'resolveMfaSecretForSave(account, totpSecret, credentialTouchedRef.current.totpSecret)'
      )
    )
    assert.ok(!modalSrc.includes('hydrationCompleted'))
  })
})

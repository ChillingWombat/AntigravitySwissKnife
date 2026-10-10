import { describe, it } from 'node:test'
import assert from 'node:assert'
import fs from 'node:fs'
import path from 'node:path'

describe('Account Error Details Modal Stability in QuotaDashboardPage', () => {
  const pagePath = path.resolve(import.meta.dirname, '../pages/QuotaDashboardPage.tsx')
  const pageSrc = fs.readFileSync(pagePath, 'utf8')

  it('guarantees error modal does not auto-dismiss on initial render when opening an error/reauth account', () => {
    // Must guard useEffect with isVerifyingErrorAccount
    assert.ok(
      pageSrc.includes('if (!errorDetailAccount?.email || !isVerifyingErrorAccount) return'),
      'useEffect must not auto-dismiss unless user is actively verifying in browser'
    )
  })

  it('checks for complete health before auto-dismissing during active verification', () => {
    assert.ok(
      pageSrc.includes("!matching.status?.toUpperCase().includes('NEEDS_REAUTH')"),
      'Must check NEEDS_REAUTH before auto-dismissing'
    )
    assert.ok(
      pageSrc.includes('Boolean(matching.refresh_token?.trim())'),
      'Must verify refresh token is present before auto-dismissing'
    )
  })

  it('guards pollErrorDetailAccount so polling does not prematurely dismiss modal when not in verification mode', () => {
    assert.ok(
      pageSrc.includes('if (isVerifyingRef.current)'),
      'pollErrorDetailAccount must guard auto-dismiss with isVerifyingRef.current'
    )
  })

  it('renders the Account Error Details modal when errorDetailAccount is set', () => {
    assert.ok(
      pageSrc.includes('Account Error Details'),
      'Modal must display Account Error Details title'
    )
    assert.ok(
      pageSrc.includes('onClick={() => setErrorDetailAccount(null)}'),
      'Modal must allow user-initiated dismissal'
    )
  })
})

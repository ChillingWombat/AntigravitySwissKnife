import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import { waitForGoogleOAuthURL } from './googleOAuth'

describe('waitForGoogleOAuthURL', () => {
  it('returns the first active authorization URL', async () => {
    const controller = new AbortController()
    let calls = 0
    const authURL = await waitForGoogleOAuthURL(
      async () => {
        calls += 1
        if (calls === 1) return { success: true, active: false }
        return { success: true, active: true, auth_url: 'https://accounts.google.com/o/oauth2/auth?state=test' }
      },
      controller.signal,
      { timeoutMs: 100, pollMs: 5 }
    )

    assert.equal(authURL, 'https://accounts.google.com/o/oauth2/auth?state=test')
    assert.equal(calls, 2)
  })

  it('returns null when the caller aborts', async () => {
    const controller = new AbortController()
    controller.abort()

    const authURL = await waitForGoogleOAuthURL(
      async () => ({ success: true, active: true, auth_url: 'unexpected' }),
      controller.signal,
      { timeoutMs: 100, pollMs: 5 }
    )

    assert.equal(authURL, null)
  })

  it('keeps polling through transient request failures', async () => {
    const controller = new AbortController()
    let calls = 0
    const authURL = await waitForGoogleOAuthURL(
      async () => {
        calls += 1
        if (calls === 1) throw new Error('temporary failure')
        return { success: true, active: true, auth_url: 'https://accounts.google.com/o/oauth2/auth?state=retry' }
      },
      controller.signal,
      { timeoutMs: 100, pollMs: 5 }
    )

    assert.equal(authURL, 'https://accounts.google.com/o/oauth2/auth?state=retry')
    assert.equal(calls, 2)
  })

  it('returns null after the polling deadline', async () => {
    const controller = new AbortController()
    const authURL = await waitForGoogleOAuthURL(
      async () => ({ success: true, active: false }),
      controller.signal,
      { timeoutMs: 20, pollMs: 5 }
    )

    assert.equal(authURL, null)
  })
})

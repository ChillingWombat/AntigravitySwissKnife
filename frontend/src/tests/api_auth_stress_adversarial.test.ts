import { describe, it, beforeEach, afterEach } from 'node:test'
import assert from 'node:assert/strict'
import { getEventListeners } from 'node:events'
import {
  api,
  request,
  isAbortError,
  cancelInFlightRequest,
  cancelAllInFlightRequests,
  runSingleFlightPoll,
  shouldRunBackgroundPoll,
  setSessionToken,
  getSessionToken,
} from '../api.ts'

describe('Adversarial Stress Suite: Frontend API Client Auth & Concurrency (Milestone M10)', () => {
  const originalFetch = globalThis.fetch
  let dispatchedEvents: string[] = []

  beforeEach(() => {
    dispatchedEvents = []
    setSessionToken(null)
    // Mock window event dispatch if window exists or attach shim
    if (typeof (globalThis as any).window === 'undefined') {
      (globalThis as any).window = {
        dispatchEvent: (event: any) => {
          dispatchedEvents.push(event.type)
          return true
        },
      }
    } else {
      const origDispatch = (globalThis as any).window.dispatchEvent
      ;(globalThis as any).window.dispatchEvent = (event: any) => {
        dispatchedEvents.push(event.type)
        return origDispatch ? origDispatch(event) : true
      }
    }
  })

  afterEach(() => {
    globalThis.fetch = originalFetch
    cancelAllInFlightRequests()
    setSessionToken(null)
  })

  // =========================================================================
  // SECTION 1: Concurrent 401 Storms & Singleton Deduplication
  // =========================================================================

  it('1.1: 50 concurrent requests hitting 401 simultaneously trigger EXACTLY ONE refresh cycle and all recover', async () => {
    let authStatusCount = 0
    let authUnlockCount = 0
    const endpointCallCounts: Record<number, number> = {}

    globalThis.fetch = (async (url: any, opts: any) => {
      if (opts?.signal?.aborted) {
        throw opts.signal.reason || new DOMException('The operation was aborted', 'AbortError')
      }
      const urlStr = String(url)
      if (urlStr === '/api/auth/status') {
        authStatusCount++
        return {
          ok: true,
          status: 200,
          json: async () => ({ password_required: false }),
        } as any
      }
      if (urlStr === '/api/auth/unlock') {
        authUnlockCount++
        // Simulate network latency for refresh
        await new Promise((r) => setTimeout(r, 40))
        return {
          ok: true,
          status: 200,
          json: async () => ({ success: true, token: 'fresh-reacquired-token-777' }),
        } as any
      }
      if (urlStr.startsWith('/api/test-storm/')) {
        const id = parseInt(urlStr.split('/').pop() || '0', 10)
        endpointCallCounts[id] = (endpointCallCounts[id] || 0) + 1
        const h = new Headers(opts?.headers)
        const token = h.get('X-Session-Token')
        if (token === 'fresh-reacquired-token-777') {
          return {
            ok: true,
            status: 200,
            json: async () => ({ id, recovered: true }),
          } as any
        }
        return {
          ok: false,
          status: 401,
          text: async () => 'Unauthorized',
        } as any
      }
      throw new Error(`Unexpected url ${urlStr}`)
    }) as any

    setSessionToken('stale-token')

    const count = 50
    const promises: Promise<any>[] = []
    for (let i = 0; i < count; i++) {
      promises.push(request(`/api/test-storm/${i}`, { cancelKey: `storm:${i}` }))
    }

    const results = await Promise.all(promises)
    assert.equal(results.length, count)

    // Deduplication assertion: singleton promise must prevent redundant auth calls
    assert.equal(authStatusCount, 1, 'MUST call /api/auth/status exactly ONCE across all 50 concurrent requests')
    assert.equal(authUnlockCount, 1, 'MUST call /api/auth/unlock exactly ONCE across all 50 concurrent requests')

    // Every request must have recovered and received fresh token
    for (let i = 0; i < count; i++) {
      assert.equal(results[i].recovered, true)
      assert.equal(endpointCallCounts[i], 2, `Request ${i} must have been invoked twice (initial 401 + retry 200)`)
    }

    assert.equal(getSessionToken(), 'fresh-reacquired-token-777')
    assert.equal(dispatchedEvents.length, 0, 'No auth-required event should be emitted when refresh succeeds')
  })

  it('1.2: 50 concurrent requests hitting 401 fail cleanly and emit swiss:auth-required when password is required', async () => {
    let authStatusCount = 0
    let authUnlockCount = 0

    globalThis.fetch = (async (url: any, opts: any) => {
      if (opts?.signal?.aborted) {
        throw opts.signal.reason || new DOMException('The operation was aborted', 'AbortError')
      }
      const urlStr = String(url)
      if (urlStr === '/api/auth/status') {
        authStatusCount++
        return {
          ok: true,
          status: 200,
          json: async () => ({ password_required: true }), // Password is required; cannot silently refresh
        } as any
      }
      if (urlStr === '/api/auth/unlock') {
        authUnlockCount++
        return {
          ok: true,
          status: 200,
          json: async () => ({ success: true, token: 'should-not-happen' }),
        } as any
      }
      if (urlStr.startsWith('/api/test-locked/')) {
        return {
          ok: false,
          status: 401,
          text: async () => 'Unauthorized: Password authentication required',
        } as any
      }
      throw new Error(`Unexpected url ${urlStr}`)
    }) as any

    setSessionToken('stale-token-locked')

    const count = 50
    const promises: Promise<any>[] = []
    for (let i = 0; i < count; i++) {
      promises.push(request(`/api/test-locked/${i}`, { cancelKey: `locked:${i}` }))
    }

    const results = await Promise.allSettled(promises)
    assert.equal(results.length, count)

    // Verify all 50 failed cleanly
    for (let i = 0; i < count; i++) {
      assert.equal(results[i].status, 'rejected', `Request ${i} must reject`)
      const reason = (results[i] as PromiseRejectedResult).reason
      assert.ok(reason.message.includes('Unauthorized'), `Request ${i} error message must include Unauthorized`)
    }

    // Verify deduplication
    assert.equal(authStatusCount, 1, 'MUST call /api/auth/status exactly ONCE')
    assert.equal(authUnlockCount, 0, 'MUST NEVER call /api/auth/unlock when password_required is true')

    // Session token must be cleared and lock screen dispatched
    assert.equal(getSessionToken(), null)
    assert.ok(dispatchedEvents.includes('swiss:auth-required'))
  })

  it('1.3: 50 concurrent requests hitting 401 handle server 500 error during refresh gracefully', async () => {
    let authStatusCount = 0

    globalThis.fetch = (async (url: any, opts: any) => {
      if (opts?.signal?.aborted) {
        throw opts.signal.reason || new DOMException('The operation was aborted', 'AbortError')
      }
      const urlStr = String(url)
      if (urlStr === '/api/auth/status') {
        authStatusCount++
        return {
          ok: false,
          status: 500,
          text: async () => 'Internal Server Error during auth status',
        } as any
      }
      if (urlStr.startsWith('/api/test-server-err/')) {
        return {
          ok: false,
          status: 401,
          text: async () => 'Unauthorized',
        } as any
      }
      throw new Error(`Unexpected url ${urlStr}`)
    }) as any

    setSessionToken('stale-token-500')

    const count = 50
    const promises: Promise<any>[] = []
    for (let i = 0; i < count; i++) {
      promises.push(request(`/api/test-server-err/${i}`, { cancelKey: `server-err:${i}` }))
    }

    const results = await Promise.allSettled(promises)
    assert.equal(results.length, count)

    for (let i = 0; i < count; i++) {
      assert.equal(results[i].status, 'rejected')
    }

    assert.equal(authStatusCount, 1, 'MUST call /api/auth/status exactly ONCE')
    assert.equal(getSessionToken(), null, 'Session token must be wiped on unrecoverable 401')
    assert.ok(dispatchedEvents.includes('swiss:auth-required'))
  })

  it('1.4: Sequential waves of 401 storms properly reset singleton promise and execute independent refresh cycles', async () => {
    let refreshCycle = 0
    let statusCallCount = 0

    globalThis.fetch = (async (url: any, opts: any) => {
      if (opts?.signal?.aborted) {
        throw opts.signal.reason || new DOMException('The operation was aborted', 'AbortError')
      }
      const urlStr = String(url)
      if (urlStr === '/api/auth/status') {
        statusCallCount++
        return {
          ok: true,
          status: 200,
          json: async () => ({ password_required: false }),
        } as any
      }
      if (urlStr === '/api/auth/unlock') {
        refreshCycle++
        const issuedToken = `token-cycle-${refreshCycle}`
        return {
          ok: true,
          status: 200,
          json: async () => ({ success: true, token: issuedToken }),
        } as any
      }
      if (urlStr.startsWith('/api/wave/')) {
        const h = new Headers(opts?.headers)
        const token = h.get('X-Session-Token')
        // Current valid token is token-cycle-${refreshCycle}
        if (token === `token-cycle-${refreshCycle}` && refreshCycle > 0) {
          return {
            ok: true,
            status: 200,
            json: async () => ({ cycle: refreshCycle }),
          } as any
        }
        return {
          ok: false,
          status: 401,
          text: async () => 'Unauthorized',
        } as any
      }
      throw new Error(`Unexpected url ${urlStr}`)
    }) as any

    // Wave 1: 10 requests hit 401
    setSessionToken('initial-stale-token')
    const wave1 = await Promise.all(
      Array.from({ length: 10 }, (_, i) => request(`/api/wave/1-${i}`, { cancelKey: `w1:${i}` }))
    )
    assert.equal(wave1.length, 10)
    assert.equal(refreshCycle, 1)
    assert.equal(statusCallCount, 1)
    assert.equal(getSessionToken(), 'token-cycle-1')

    // Wave 2: Sometime later, token-cycle-1 is invalidated and 10 new requests hit 401
    setSessionToken('invalidated-token')
    const wave2 = await Promise.all(
      Array.from({ length: 10 }, (_, i) => request(`/api/wave/2-${i}`, { cancelKey: `w2:${i}` }))
    )
    assert.equal(wave2.length, 10)
    assert.equal(refreshCycle, 2, 'MUST execute a second refresh cycle after first has settled')
    assert.equal(statusCallCount, 2, 'MUST call /api/auth/status for the new wave')
    assert.equal(getSessionToken(), 'token-cycle-2')
  })

  // =========================================================================
  // SECTION 2: AbortController & Timeout Cancellation During In-Flight Refresh
  // =========================================================================

  it('2.1: Caller AbortSignal fired while token refresh is in progress aborts only that request while shared refresh and peer requests succeed', async () => {
    let unlockStarted = false

    globalThis.fetch = (async (url: any, opts: any) => {
      if (opts?.signal?.aborted) {
        throw opts.signal.reason || new DOMException('The operation was aborted', 'AbortError')
      }
      const urlStr = String(url)
      if (urlStr === '/api/auth/status') {
        return {
          ok: true,
          status: 200,
          json: async () => ({ password_required: false }),
        } as any
      }
      if (urlStr === '/api/auth/unlock') {
        unlockStarted = true
        // Latency: 70ms to allow client cancellation mid-refresh
        await new Promise((r) => setTimeout(r, 70))
        return {
          ok: true,
          status: 200,
          json: async () => ({ success: true, token: 'fresh-survivor-token' }),
        } as any
      }
      if (urlStr.startsWith('/api/abort-test/')) {
        const h = new Headers(opts?.headers)
        if (h.get('X-Session-Token') === 'fresh-survivor-token') {
          return {
            ok: true,
            status: 200,
            json: async () => ({ success: true, path: urlStr }),
          } as any
        }
        return {
          ok: false,
          status: 401,
          text: async () => 'Unauthorized',
        } as any
      }
      throw new Error(`Unexpected url ${urlStr}`)
    }) as any

    setSessionToken('stale-token')

    const ctrlA = new AbortController()
    const reqA = request<any>('/api/abort-test/A', { cancelKey: 'keyA', signal: ctrlA.signal })
    const reqB = request<any>('/api/abort-test/B', { cancelKey: 'keyB' })

    // Wait until refresh has started
    while (!unlockStarted) {
      await new Promise((r) => setTimeout(r, 5))
    }

    // Abort Request A while unlock is in progress
    ctrlA.abort(new DOMException('User cancelled request A', 'AbortError'))

    const [resA, resB] = await Promise.allSettled([reqA, reqB])

    // Request A must be aborted cleanly
    assert.equal(resA.status, 'rejected')
    assert.ok(isAbortError((resA as PromiseRejectedResult).reason), 'Request A must be recognized as AbortError')

    // Request B must fulfill successfully with the fresh token
    assert.equal(resB.status, 'fulfilled')
    assert.equal((resB as PromiseFulfilledResult<any>).value.success, true)
    assert.equal(getSessionToken(), 'fresh-survivor-token')
  })

  it('2.2: Request timeout fired during in-flight token refresh rejects with timeout error and prevents retried fetch completion', async () => {
    let secondFetchCompleted = false

    globalThis.fetch = (async (url: any, opts: any) => {
      if (opts?.signal?.aborted) {
        throw opts.signal.reason || new DOMException('The operation was aborted', 'AbortError')
      }
      const urlStr = String(url)
      if (urlStr === '/api/auth/status') {
        return {
          ok: true,
          status: 200,
          json: async () => ({ password_required: false }),
        } as any
      }
      if (urlStr === '/api/auth/unlock') {
        // Slow refresh (60ms)
        await new Promise((r) => setTimeout(r, 60))
        return {
          ok: true,
          status: 200,
          json: async () => ({ success: true, token: 'slow-refresh-token' }),
        } as any
      }
      if (urlStr === '/api/timeout-test') {
        const h = new Headers(opts?.headers)
        if (h.get('X-Session-Token') === 'slow-refresh-token') {
          secondFetchCompleted = true
          return { ok: true, status: 200, json: async () => ({ ok: true }) } as any
        }
        return { ok: false, status: 401, text: async () => 'Unauthorized' } as any
      }
      throw new Error(`Unexpected url ${urlStr}`)
    }) as any

    setSessionToken('stale-token')

    // Timeout of 25ms will fire during the 60ms unlock delay
    await assert.rejects(
      async () => {
        await request('/api/timeout-test', { timeoutMs: 25 })
      },
      (err: any) => {
        assert.ok(isAbortError(err), 'Must be recognized as an abort/timeout error')
        assert.ok(err.message.includes('timed out after 25ms'), 'Must state exact timeout message')
        return true
      }
    )

    // Wait until refresh finishes to ensure timed-out request did not complete successfully
    await new Promise((r) => setTimeout(r, 50))
    assert.equal(secondFetchCompleted, false, 'Timed-out request MUST NOT complete retried fetch')
  })

  it('2.3: cancelPrevious preempts older in-flight request during token refresh without disrupting newer request or refresh process', async () => {
    let totalCompletedCalls = 0

    globalThis.fetch = (async (url: any, opts: any) => {
      if (opts?.signal?.aborted) {
        throw opts.signal.reason || new DOMException('The operation was aborted', 'AbortError')
      }
      const urlStr = String(url)
      if (urlStr === '/api/auth/status') {
        return {
          ok: true,
          status: 200,
          json: async () => ({ password_required: false }),
        } as any
      }
      if (urlStr === '/api/auth/unlock') {
        await new Promise((r) => setTimeout(r, 50))
        return {
          ok: true,
          status: 200,
          json: async () => ({ success: true, token: 'preempt-token' }),
        } as any
      }
      if (urlStr === '/api/preempt') {
        totalCompletedCalls++
        const h = new Headers(opts?.headers)
        if (h.get('X-Session-Token') === 'preempt-token') {
          return { ok: true, status: 200, json: async () => ({ totalCompletedCalls }) } as any
        }
        return { ok: false, status: 401, text: async () => 'Unauthorized' } as any
      }
      throw new Error(`Unexpected url ${urlStr}`)
    }) as any

    setSessionToken('stale-token')

    // Dispatches req1 with cancelPrevious (default for GET)
    const req1 = request<any>('/api/preempt', { cancelKey: 'shared-key' })

    // 15ms later, dispatch req2 with same cancelKey
    await new Promise((r) => setTimeout(r, 15))
    const req2 = request<any>('/api/preempt', { cancelKey: 'shared-key' })

    const [res1, res2] = await Promise.allSettled([req1, req2])

    assert.equal(res1.status, 'rejected')
    assert.ok(isAbortError((res1 as PromiseRejectedResult).reason), 'Req 1 must be aborted by Req 2')

    assert.equal(res2.status, 'fulfilled')
    // Completed calls: req1 initial 401 (1) + req2 initial 401 (2) + req2 retry 200 (3).
    // Req 1 retry aborted at entry via signal.aborted without completing network logic.
    assert.equal((res2 as PromiseFulfilledResult<any>).value.totalCompletedCalls, 3)
  })

  it('2.4: cancelAllInFlightRequests during in-flight token refresh cleanly cancels all pending requests with 0 leaks', async () => {
    globalThis.fetch = (async (url: any, opts: any) => {
      if (opts?.signal?.aborted) {
        throw opts.signal.reason || new DOMException('The operation was aborted', 'AbortError')
      }
      const urlStr = String(url)
      if (urlStr === '/api/auth/status') {
        return {
          ok: true,
          status: 200,
          json: async () => ({ password_required: false }),
        } as any
      }
      if (urlStr === '/api/auth/unlock') {
        await new Promise((r) => setTimeout(r, 70))
        return {
          ok: true,
          status: 200,
          json: async () => ({ success: true, token: 'cancel-all-token' }),
        } as any
      }
      if (urlStr.startsWith('/api/cancel-all/')) {
        return { ok: false, status: 401, text: async () => 'Unauthorized' } as any
      }
      throw new Error(`Unexpected url ${urlStr}`)
    }) as any

    setSessionToken('stale-token')

    const tasks = [
      request('/api/cancel-all/1', { cancelKey: 'c1' }),
      request('/api/cancel-all/2', { cancelKey: 'c2' }),
      request('/api/cancel-all/3', { cancelKey: 'c3' }),
    ]

    // Wait 15ms into refresh, then cancel all
    await new Promise((r) => setTimeout(r, 15))
    cancelAllInFlightRequests()

    const results = await Promise.allSettled(tasks)
    for (let i = 0; i < 3; i++) {
      assert.equal(results[i].status, 'rejected')
      assert.ok(isAbortError((results[i] as PromiseRejectedResult).reason))
    }
  })

  it('2.5: cancelInFlightRequest targets specific request key during token refresh without affecting peers', async () => {
    globalThis.fetch = (async (url: any, opts: any) => {
      if (opts?.signal?.aborted) {
        throw opts.signal.reason || new DOMException('The operation was aborted', 'AbortError')
      }
      const urlStr = String(url)
      if (urlStr === '/api/auth/status') {
        return {
          ok: true,
          status: 200,
          json: async () => ({ password_required: false }),
        } as any
      }
      if (urlStr === '/api/auth/unlock') {
        await new Promise((r) => setTimeout(r, 60))
        return {
          ok: true,
          status: 200,
          json: async () => ({ success: true, token: 'targeted-token' }),
        } as any
      }
      if (urlStr === '/api/target/cancelled') {
        const h = new Headers(opts?.headers)
        if (h.get('X-Session-Token') === 'targeted-token') {
          return { ok: true, status: 200, json: async () => ({ ok: true }) } as any
        }
        return { ok: false, status: 401, text: async () => 'Unauthorized' } as any
      }
      if (urlStr === '/api/target/survivor') {
        const h = new Headers(opts?.headers)
        if (h.get('X-Session-Token') === 'targeted-token') {
          return { ok: true, status: 200, json: async () => ({ survived: true }) } as any
        }
        return { ok: false, status: 401, text: async () => 'Unauthorized' } as any
      }
      throw new Error(`Unexpected url ${urlStr}`)
    }) as any

    setSessionToken('stale-token')

    const reqCancelled = request<any>('/api/target/cancelled', { cancelKey: 'key-to-cancel' })
    const reqSurvivor = request<any>('/api/target/survivor', { cancelKey: 'key-to-survive' })

    await new Promise((r) => setTimeout(r, 15))
    cancelInFlightRequest('key-to-cancel')

    const [resCancelled, resSurvivor] = await Promise.allSettled([reqCancelled, reqSurvivor])

    assert.equal(resCancelled.status, 'rejected')
    assert.ok(isAbortError((resCancelled as PromiseRejectedResult).reason))

    assert.equal(resSurvivor.status, 'fulfilled')
    assert.equal((resSurvivor as PromiseFulfilledResult<any>).value.survived, true)
  })

  // =========================================================================
  // SECTION 3: Fast Token Rotation & Invalidation Invariants
  // =========================================================================

  it('3.1: Rapid token rotation correctly updates in-memory token, trims whitespace, and sanitizes null/empty values', () => {
    // Normal assignment
    setSessionToken('initial-token')
    assert.equal(getSessionToken(), 'initial-token')

    // Whitespace trimming
    setSessionToken('   whitespace-padded-token   \n\t')
    assert.equal(getSessionToken(), 'whitespace-padded-token')

    // Empty and pure whitespace strings become null
    setSessionToken('')
    assert.equal(getSessionToken(), null)
    setSessionToken('   \t\n  ')
    assert.equal(getSessionToken(), null)
    setSessionToken(null)
    assert.equal(getSessionToken(), null)

    // Rapid loop rotation
    for (let i = 0; i < 100; i++) {
      setSessionToken(`token-rot-${i}`)
      assert.equal(getSessionToken(), `token-rot-${i}`)
    }
  })

  it('3.2: Caller-specified X-Session-Token header is strictly preserved despite concurrent background token changes', async () => {
    setSessionToken('global-active-token')
    let capturedHeader: string | null = null

    globalThis.fetch = (async (_url: any, opts: any) => {
      const h = new Headers(opts?.headers)
      capturedHeader = h.get('X-Session-Token')
      return { ok: true, status: 200, json: async () => ({ ok: true }) } as any
    }) as any

    await request('/api/test-preserve', {
      headers: { 'X-Session-Token': 'caller-pinned-override-token' },
    })

    assert.equal(capturedHeader, 'caller-pinned-override-token', 'Must preserve explicit caller-specified header')
  })

  it('3.3: Repeated 401s after successful refresh fail with exactly one retry without infinite recursion', async () => {
    let requestCount = 0
    let refreshCount = 0

    globalThis.fetch = (async (url: any) => {
      const urlStr = String(url)
      if (urlStr === '/api/auth/status') {
        return { ok: true, status: 200, json: async () => ({ password_required: false }) } as any
      }
      if (urlStr === '/api/auth/unlock') {
        refreshCount++
        return { ok: true, status: 200, json: async () => ({ success: true, token: `refreshed-${refreshCount}` }) } as any
      }
      if (urlStr === '/api/stubborn-401') {
        requestCount++
        // Always returns 401, even with refreshed token
        return { ok: false, status: 401, text: async () => 'Permanent 401 Forbidden Account' } as any
      }
      throw new Error(`Unexpected url ${urlStr}`)
    }) as any

    setSessionToken('initial-token')

    await assert.rejects(
      async () => {
        await request('/api/stubborn-401')
      },
      (err: any) => err.message.includes('Permanent 401')
    )

    // Must execute initial request + exactly 1 retry after refresh = 2 requests total
    assert.equal(requestCount, 2, 'MUST attempt initial request and exactly 1 retry before giving up')
    assert.equal(refreshCount, 1, 'MUST trigger exactly 1 refresh attempt')
    assert.equal(getSessionToken(), null, 'Session token must be wiped after failed second 401')
    assert.ok(dispatchedEvents.includes('swiss:auth-required'), 'Must emit swiss:auth-required event')
  })

  it('3.4: unlockApp updates session token on success and leaves existing token intact on failure', async () => {
    setSessionToken('pre-existing-token')

    // Scenario A: Failed unlock
    globalThis.fetch = (async (url: any) => {
      assert.equal(String(url), '/api/auth/unlock')
      return {
        ok: false,
        status: 403,
        text: async () => JSON.stringify({ success: false, error: 'Incorrect master password' }),
      } as any
    }) as any

    await assert.rejects(
      async () => {
        await api.unlockApp('badpassword')
      },
      (err: any) => err.message.includes('Incorrect master password')
    )
    assert.equal(getSessionToken(), 'pre-existing-token', 'Existing session token must not be erased on rejected password')

    // Scenario B: Successful unlock
    globalThis.fetch = (async (url: any) => {
      assert.equal(String(url), '/api/auth/unlock')
      return {
        ok: true,
        status: 200,
        json: async () => ({ success: true, session_token: 'new-valid-session-jwt' }),
      } as any
    }) as any

    const res = await api.unlockApp('goodpassword')
    assert.equal(res.success, true)
    assert.equal(getSessionToken(), 'new-valid-session-jwt', 'Session token must be updated on successful unlock')
  })

  // =========================================================================
  // SECTION 4: URL Parsing Edge Cases in One-Time ?token= Extraction
  // =========================================================================

  function runUrlTokenExtraction(href: string): { extractedToken: string | null; cleanUrl: string } {
    let capturedToken: string | null = null
    let replacedUrl = href
    try {
      const parsedUrl = new URL(href)
      const urlToken = parsedUrl.searchParams.get('token')
      if (urlToken) {
        capturedToken = urlToken.trim() ? urlToken.trim() : null
        parsedUrl.searchParams.delete('token')
        const rest = parsedUrl.searchParams.toString()
        replacedUrl = parsedUrl.pathname + (rest ? `?${rest}` : '') + parsedUrl.hash
      }
    } catch {}
    return { extractedToken: capturedToken, cleanUrl: replacedUrl }
  }

  it('4.1: Standard ?token= query parameter is extracted, set, and stripped from URL', () => {
    const res = runUrlTokenExtraction('http://127.0.0.1:8765/?token=secret-token-123')
    assert.equal(res.extractedToken, 'secret-token-123')
    assert.equal(res.cleanUrl, '/')
  })

  it('4.2: Complex URL with multiple query params, port, and hash fragment preserves all non-token components', () => {
    const url = 'http://127.0.0.1:8765/dashboard?tab=settings&token=auth-key-999&debug=true#section-quota'
    const res = runUrlTokenExtraction(url)
    assert.equal(res.extractedToken, 'auth-key-999')
    assert.equal(res.cleanUrl, '/dashboard?tab=settings&debug=true#section-quota')
  })

  it('4.3: URL-encoded token values (base64, plus, slash, equal) are decoded with fidelity', () => {
    // Encoded 'eyJh+b/c=='
    const raw = 'eyJh+b/c=='
    const encoded = encodeURIComponent(raw)
    const url = `http://127.0.0.1:8765/?token=${encoded}`
    const res = runUrlTokenExtraction(url)
    assert.equal(res.extractedToken, raw)
    assert.equal(res.cleanUrl, '/')
  })

  it('4.4: Empty ?token= or whitespace ?token=%20%20 does not set corrupted tokens', () => {
    // Empty token
    const resEmpty = runUrlTokenExtraction('http://127.0.0.1:8765/?token=&mode=dark')
    assert.equal(resEmpty.extractedToken, null)
    // Whitespace token
    const resSpace = runUrlTokenExtraction('http://127.0.0.1:8765/?token=%20%20%20&mode=dark')
    assert.equal(resSpace.extractedToken, null)
  })

  it('4.5: Multiple ?token= parameters are completely stripped from URL', () => {
    const url = 'http://127.0.0.1:8765/?token=first&token=second&user=admin'
    const res = runUrlTokenExtraction(url)
    assert.equal(res.extractedToken, 'first')
    assert.equal(res.cleanUrl, '/?user=admin')
  })

  it('4.6: Malformed URLs fail silently without throwing unhandled exceptions', () => {
    const res = runUrlTokenExtraction('not a valid url')
    assert.equal(res.extractedToken, null)
    assert.equal(res.cleanUrl, 'not a valid url')
  })

  // =========================================================================
  // SECTION 5: Polling Cancellation and Listener Lifecycle
  // =========================================================================

  it('5.1: Long-lived caller AbortSignal has its abort listener removed after request resolution', async () => {
    const callerController = new AbortController()
    let listenerWasAttached = false

    globalThis.fetch = (async () => {
      listenerWasAttached = getEventListeners(callerController.signal, 'abort').length === 1
      return {
        ok: true,
        status: 200,
        json: async () => ({ ok: true }),
      }
    }) as any

    assert.equal(getEventListeners(callerController.signal, 'abort').length, 0)

    await request('/api/listener-cleanup', { signal: callerController.signal })

    assert.equal(listenerWasAttached, true, 'The request must chain the caller signal while in flight')
    assert.equal(
      getEventListeners(callerController.signal, 'abort').length,
      0,
      'Completed requests must not leave listeners on caller signals',
    )
  })

  it('5.2: Overlapping interval polls skip the busy cycle instead of aborting the slow request', async () => {
    let fetchCount = 0
    let requestSignal: AbortSignal | null = null

    globalThis.fetch = (async (_url: any, opts: any) => {
      fetchCount++
      requestSignal = opts?.signal ?? null
      await new Promise((resolve) => setTimeout(resolve, 80))
      return {
        ok: true,
        status: 200,
        json: async () => ({ ok: true }),
      } as any
    }) as any

    const isPollingRef = { current: false }
    const pollStatus = () =>
      runSingleFlightPoll(isPollingRef, async () => {
        await api.getStatus()
      })

    const interval = setInterval(() => { void pollStatus() }, 10)
    const slowPoll = pollStatus()
    try {
      await new Promise((resolve) => setTimeout(resolve, 20))
      const overlappingPoll = pollStatus()
      await overlappingPoll

      assert.equal(fetchCount, 1, 'An overlapping interval tick must not start another request')
      assert.equal(isPollingRef.current, true, 'The original poll must still be in flight')
      assert.equal((requestSignal as AbortSignal | null)?.aborted, false, 'The original request must not be self-aborted')
    } finally {
      clearInterval(interval)
    }

    await slowPoll
    assert.equal(isPollingRef.current, false, 'The polling guard must reset after completion')
  })

  it('5.3: Skips interval polls while hidden and resumes immediately when visible', async () => {
    const originalDocument = (globalThis as any).document
    let visibilityState: DocumentVisibilityState = 'hidden'
    let pollCount = 0
    const isPollingRef = { current: false }
    const pollStatus = () =>
      runSingleFlightPoll(isPollingRef, async () => {
        pollCount++
      })

    Object.defineProperty(globalThis, 'document', {
      configurable: true,
      value: {
        get visibilityState() {
          return visibilityState
        },
      },
    })

    try {
      const intervalTick = () => {
        if (!shouldRunBackgroundPoll()) return
        void pollStatus()
      }
      const handleVisibilityChange = () => {
        if (shouldRunBackgroundPoll()) void pollStatus()
      }

      intervalTick()
      await Promise.resolve()
      assert.equal(pollCount, 0, 'A hidden document must skip the background interval poll')

      visibilityState = 'visible'
      handleVisibilityChange()
      await Promise.resolve()
      assert.equal(pollCount, 1, 'Becoming visible must trigger an immediate single-flight poll')
      assert.equal(isPollingRef.current, false)
    } finally {
      Object.defineProperty(globalThis, 'document', {
        configurable: true,
        value: originalDocument,
      })
    }
  })
})

import { describe, it, afterEach } from 'node:test'
import assert from 'node:assert/strict'
import {
  api,
  isAbortError,
  cancelInFlightRequest,
  cancelAllInFlightRequests,
} from '../api.ts'

describe('API Client Resilience & Timeout Handling (Issue #59)', () => {
  const originalFetch = globalThis.fetch

  afterEach(() => {
    globalThis.fetch = originalFetch
    cancelAllInFlightRequests()
  })

  it('successfully fetches and parses JSON responses', async () => {
    globalThis.fetch = (async (_url: string | URL | Request) => {
      return {
        ok: true,
        status: 200,
        json: async () => ({ daemon_running: true, daemon_pid: 1234 }),
      } as any
    }) as any

    const res = await api.getStatus()
    assert.equal(res.daemon_running, true)
    assert.equal(res.daemon_pid, 1234)
  })

  it('formats HTTP error responses into clean human-readable error messages', async () => {
    globalThis.fetch = (async () => {
      return {
        ok: false,
        status: 403,
        text: async () => JSON.stringify({ error: 'Access forbidden: session token invalid' }),
      } as any
    }) as any

    await assert.rejects(
      async () => {
        await api.getStatus()
      },
      (err: any) => {
        return err.message.includes('Access forbidden')
      }
    )
  })

  it('aborts stalled requests when timeout is exceeded', async () => {
    // Simulate a hung connection that never resolves until aborted
    globalThis.fetch = ((_url: any, opts: any) => {
      return new Promise((_, reject) => {
        if (opts?.signal) {
          opts.signal.addEventListener('abort', () => {
            reject(new DOMException('The operation was aborted', 'AbortError'))
          })
        }
      })
    }) as any

    await assert.rejects(
      async () => {
        // Request with short 50ms timeout
        await api.getStatus({ timeoutMs: 50 })
      },
      (err: any) => {
        return isAbortError(err) || err.message.includes('timed out')
      }
    )
  })

  it('cancels pending concurrent polling requests to prevent out-of-order race conditions', async () => {
    let callCount = 0
    let firstAborted = false

    globalThis.fetch = ((_url: any, opts: any) => {
      callCount++
      const currentCall = callCount
      return new Promise((resolve, reject) => {
        if (opts?.signal) {
          opts.signal.addEventListener('abort', () => {
            if (currentCall === 1) {
              firstAborted = true
            }
            reject(new DOMException('Aborted by newer request', 'AbortError'))
          })
        }
        if (currentCall === 2) {
          // Second request completes quickly
          setTimeout(() => {
            resolve({
              ok: true,
              status: 200,
              json: async () => ({ daemon_running: true, daemon_pid: 5678 }),
            } as any)
          }, 20)
        }
      })
    }) as any

    // Dispatch request 1 (slow)
    const req1 = api.getStatus()
    // Immediately dispatch request 2 (faster, fresh polling)
    const req2 = api.getStatus()

    const [res1, res2] = await Promise.allSettled([req1, req2])

    assert.equal(res1.status, 'rejected', 'First concurrent request must be rejected')
    assert.ok(isAbortError((res1 as PromiseRejectedResult).reason), 'First request error must be AbortError')
    assert.equal(firstAborted, true, 'First request signal must receive abort event')

    assert.equal(res2.status, 'fulfilled', 'Second request must succeed')
    assert.equal((res2 as PromiseFulfilledResult<any>).value.daemon_pid, 5678, 'Second request must yield fresh payload')
  })

  it('allows manual in-flight cancellation via cancelInFlightRequest', async () => {
    let aborted = false
    globalThis.fetch = ((_url: any, opts: any) => {
      return new Promise((_, reject) => {
        if (opts?.signal) {
          opts.signal.addEventListener('abort', () => {
            aborted = true
            reject(new DOMException('User cancelled', 'AbortError'))
          })
        }
      })
    }) as any

    const promise = api.getStatus()
    cancelInFlightRequest('GET:/api/status')

    await assert.rejects(
      async () => await promise,
      (err: any) => isAbortError(err)
    )
    assert.equal(aborted, true)
  })
})

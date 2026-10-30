import test from 'node:test'
import assert from 'node:assert/strict'

test('frontend daemon requests use the Electron broker without calling fetch', async () => {
  const calls: Array<{ method: string; path: string; body?: string; timeoutMs?: number }> = []
  const storage = new Map<string, string>()
  let fetchCalls = 0
  const originalFetch = globalThis.fetch
  const originalWindow = (globalThis as any).window
  const originalLocalStorage = (globalThis as any).localStorage

  ;(globalThis as any).window = {
    location: { href: 'http://127.0.0.1:8765/', origin: 'http://127.0.0.1:8765' },
    history: { replaceState() {} },
    swiss: {
      request: async (request: { method: string; path: string; body?: string; timeoutMs?: number }) => {
        calls.push(request)
        return {
          ok: true,
          status: 200,
          statusText: 'OK',
          headers: { 'content-type': 'application/json' },
          body: JSON.stringify({ status: 'ok' }),
        }
      },
    },
  }
  ;(globalThis as any).localStorage = {
    getItem: (key: string) => storage.get(key) ?? null,
    setItem: (key: string, value: string) => storage.set(key, value),
    removeItem: (key: string) => storage.delete(key),
  }
  globalThis.fetch = (async () => {
    fetchCalls += 1
    throw new Error('direct fetch must not run')
  }) as typeof fetch

  try {
    const api = await import('../api')
    const result = await api.request<{ status: string }>('/api/status', { timeoutMs: 2500 })
    assert.deepEqual(result, { status: 'ok' })
    assert.equal(fetchCalls, 0)
    assert.deepEqual(calls, [{ method: 'GET', path: '/api/status', body: undefined, timeoutMs: 2500 }])
  } finally {
    globalThis.fetch = originalFetch
    ;(globalThis as any).window = originalWindow
    ;(globalThis as any).localStorage = originalLocalStorage
  }
})

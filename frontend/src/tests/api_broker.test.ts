import test from 'node:test'
import assert from 'node:assert/strict'

test('frontend module init unconditionally purges the legacy token from both storages', async () => {
  const local = new Map<string, string>([['swiss_session_token', 'existing-local-token']])
  const session = new Map<string, string>([['swiss_session_token', 'existing-session-token']])
  const originalWindow = (globalThis as any).window
  const originalLocalStorage = (globalThis as any).localStorage
  const originalSessionStorage = (globalThis as any).sessionStorage

  ;(globalThis as any).window = {
    swiss: {
      request: async () => ({
        ok: true,
        status: 200,
        statusText: 'OK',
        headers: {},
        body: '{}',
      }),
    },
  }
  ;(globalThis as any).localStorage = {
    getItem: (key: string) => local.get(key) ?? null,
    setItem: (key: string, value: string) => local.set(key, value),
    removeItem: (key: string) => local.delete(key),
  }
  ;(globalThis as any).sessionStorage = {
    getItem: (key: string) => session.get(key) ?? null,
    setItem: (key: string, value: string) => session.set(key, value),
    removeItem: (key: string) => session.delete(key),
  }

  try {
    const api = await import('../api')
    assert.equal(local.has('swiss_session_token'), false)
    assert.equal(session.has('swiss_session_token'), false)
    api.setSessionToken('must-not-be-persisted')
    assert.equal(local.has('swiss_session_token'), false)
    assert.equal(session.has('swiss_session_token'), false)
  } finally {
    ;(globalThis as any).window = originalWindow
    ;(globalThis as any).localStorage = originalLocalStorage
    ;(globalThis as any).sessionStorage = originalSessionStorage
  }
})

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


test('non-Electron privileged requests fail before unauthenticated daemon access', async () => {
  const api = await import('../api')
  const originalFetch = globalThis.fetch
  const originalWindow = (globalThis as any).window
  const originalLocalStorage = (globalThis as any).localStorage
  const storage = new Map<string, string>()
  const requests: string[] = []

  ;(globalThis as any).window = {
    location: { href: 'http://127.0.0.1:8765/', origin: 'http://127.0.0.1:8765' },
    history: { replaceState() {} },
  }
  ;(globalThis as any).localStorage = {
    getItem: (key: string) => storage.get(key) ?? null,
    setItem: (key: string, value: string) => storage.set(key, value),
    removeItem: (key: string) => storage.delete(key),
  }
  api.setSessionToken(null)
  globalThis.fetch = (async (url: string | URL | Request) => {
    requests.push(String(url))
    return new Response(JSON.stringify({ password_required: true }), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    })
  }) as typeof fetch

  try {
    await assert.rejects(
      () => api.request('/api/files/write', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ path: '/tmp/blocked', content: 'blocked' }),
      }),
      (error: any) => error?.code === 'DAEMON_AUTH_REQUIRED' && /authentication is required/.test(error.message),
    )
    assert.equal(requests.some((url) => url.includes('/api/files/write')), false)
  } finally {
    globalThis.fetch = originalFetch
    ;(globalThis as any).window = originalWindow
    ;(globalThis as any).localStorage = originalLocalStorage
  }
})

test('non-Electron privileged requests use an already scoped browser token', async () => {
  const api = await import('../api')
  const originalFetch = globalThis.fetch
  const originalWindow = (globalThis as any).window
  const originalLocalStorage = (globalThis as any).localStorage
  const storage = new Map<string, string>()
  let requestHeaders: Headers | null = null

  ;(globalThis as any).window = {
    location: { href: 'http://127.0.0.1:8765/', origin: 'http://127.0.0.1:8765' },
    history: { replaceState() {} },
  }
  ;(globalThis as any).localStorage = {
    getItem: (key: string) => storage.get(key) ?? null,
    setItem: (key: string, value: string) => storage.set(key, value),
    removeItem: (key: string) => storage.delete(key),
  }
  api.setSessionToken('scoped-web-token')
  globalThis.fetch = (async (_url: string | URL | Request, init?: RequestInit) => {
    requestHeaders = new Headers(init?.headers || {})
    return new Response(JSON.stringify({ success: true }), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    })
  }) as typeof fetch

  try {
    const result = await api.request<{ success: boolean }>('/api/files/write', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ path: '/tmp/allowed', content: 'allowed' }),
    })
    assert.deepEqual(result, { success: true })
    assert.equal(requestHeaders?.get('X-Session-Token'), 'scoped-web-token')
  } finally {
    api.setSessionToken(null)
    globalThis.fetch = originalFetch
    ;(globalThis as any).window = originalWindow
    ;(globalThis as any).localStorage = originalLocalStorage
  }
})

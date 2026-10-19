/**
 * memo_settings.test.ts — Quick Memos extraction mode + language selectors
 *
 * Covers the settings UI added for #143 (manual vs. automatic text extraction)
 * and #144 (voice / OCR language selection):
 *   - defaults render as Manual + "Auto detect" when the daemon reports no value;
 *   - switching to Automatic POSTs `auto_extract: true` through the existing
 *     memo-config save path (no dedicated save button);
 *   - changing a language POSTs the corresponding language code.
 */

import { describe, it, beforeEach, afterEach } from 'node:test'
import assert from 'node:assert/strict'
import React, { act } from 'react'
import { initDOM, mount, type RenderResult } from '../test-dom'
import { ExtensionsPage } from '../pages/ExtensionsPage'
import { cancelAllInFlightRequests } from '../api'

const { window: testWindow } = initDOM()
Object.assign(globalThis, { localStorage: testWindow.localStorage })

interface RecordedCall {
  url: string
  method: string
  body?: any
}

const MEMO_CONFIG_DEFAULTS = {
  storage_location: 'global',
  view_scope: 'all',
  search_scope: 'text',
}

/** Installs a fetch stub that answers the three endpoints ExtensionsPage hits on mount. */
function installFetch(memoConfig: Record<string, unknown>): RecordedCall[] {
  const calls: RecordedCall[] = []
  globalThis.fetch = (async (url: any, opts: any) => {
    const target = String(url)
    const method = (opts?.method || 'GET').toUpperCase()
    const body = opts?.body ? JSON.parse(String(opts.body)) : undefined
    calls.push({ url: target, method, body })

    let payload: any = { success: true }
    if (target.startsWith('/api/enhancements')) {
      payload = { extensions: {}, left_panel_extensions_mode: 'single' }
    } else if (target.startsWith('/api/files/ide/config')) {
      payload = { success: true, preferred_ide: 'code' }
    } else if (target.startsWith('/api/memos/config')) {
      payload = { success: true, config: method === 'POST' ? body : memoConfig }
    }
    return { ok: true, status: 200, json: async () => payload } as any
  }) as any
  return calls
}

const flush = async () => {
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 0))
  })
}

const memoPosts = (calls: RecordedCall[]) =>
  calls.filter((c) => c.url.startsWith('/api/memos/config') && c.method === 'POST')

const findButton = (view: RenderResult, label: string): HTMLButtonElement | undefined =>
  view.querySelectorAll<HTMLButtonElement>('button').find((b) => (b.textContent || '').trim() === label)

describe('Quick Memos settings — extraction mode and languages', () => {
  const originalFetch = globalThis.fetch
  let view: RenderResult | null = null

  beforeEach(() => {
    testWindow.localStorage.clear()
  })

  afterEach(() => {
    view?.unmount()
    view = null
    globalThis.fetch = originalFetch
    cancelAllInFlightRequests()
  })

  it('renders Manual and "Auto detect" defaults when the daemon reports no new fields', async () => {
    installFetch(MEMO_CONFIG_DEFAULTS)
    view = await mount(React.createElement(ExtensionsPage))
    await flush()

    const manual = findButton(view, 'Manual')
    const automatic = findButton(view, 'Automatic')
    assert.ok(manual, 'Manual segment must render')
    assert.ok(automatic, 'Automatic segment must render')
    assert.equal(manual!.getAttribute('aria-pressed'), 'true')
    assert.equal(automatic!.getAttribute('aria-pressed'), 'false')

    const voice = view.querySelector<HTMLSelectElement>('select[aria-label="Voice language"]')
    const ocr = view.querySelector<HTMLSelectElement>('select[aria-label="OCR language"]')
    assert.ok(voice, 'voice language select must render')
    assert.ok(ocr, 'OCR language select must render')
    assert.equal(voice!.value, 'auto')
    assert.equal(ocr!.value, 'auto')

    const text = view.container.textContent || ''
    assert.ok(text.includes('Extract text per memo from the card buttons.'), 'manual description must render')
  })

  it('persists auto_extract: true when Automatic is chosen', async () => {
    const calls = installFetch(MEMO_CONFIG_DEFAULTS)
    view = await mount(React.createElement(ExtensionsPage))
    await flush()

    await view.click(findButton(view, 'Automatic'))
    await flush()

    const posts = memoPosts(calls)
    assert.equal(posts.length, 1)
    assert.equal(posts[0].body.auto_extract, true)
    assert.ok(
      (view.container.textContent || '').includes('Extract text as soon as a voice or image memo is saved.'),
      'automatic description must render'
    )
  })

  it('persists the selected voice language code', async () => {
    const calls = installFetch(MEMO_CONFIG_DEFAULTS)
    view = await mount(React.createElement(ExtensionsPage))
    await flush()

    const voice = view.querySelector<HTMLSelectElement>('select[aria-label="Voice language"]')!
    await act(async () => {
      voice.value = 'ja'
      voice.dispatchEvent(new (globalThis as any).Event('change', { bubbles: true }))
    })
    await flush()

    const posts = memoPosts(calls)
    assert.equal(posts.length, 1)
    assert.equal(posts[0].body.voice_language, 'ja')
    assert.equal(posts[0].body.auto_extract, false)
  })

  it('persists the selected OCR language code', async () => {
    const calls = installFetch(MEMO_CONFIG_DEFAULTS)
    view = await mount(React.createElement(ExtensionsPage))
    await flush()

    const ocr = view.querySelector<HTMLSelectElement>('select[aria-label="OCR language"]')!
    await act(async () => {
      ocr.value = 'zh-Hans'
      ocr.dispatchEvent(new (globalThis as any).Event('change', { bubbles: true }))
    })
    await flush()

    const posts = memoPosts(calls)
    assert.equal(posts.length, 1)
    assert.equal(posts[0].body.ocr_language, 'zh-Hans')
  })
})

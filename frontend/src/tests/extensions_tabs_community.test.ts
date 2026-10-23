/**
 * extensions_tabs_community.test.ts — Unit tests for Extensions top tab switcher & Community Extensions UI
 *
 * Validates Ticket #203:
 * 1. App.tsx renders top header segmented switcher with 'Built-in' and 'Community' tabs when currentTool === 7.
 * 2. ExtensionsPage defaults to 'Built-in' (activeTab === 0) and renders all built-in gadgets.
 * 3. ExtensionsPage with activeTab={1} renders Community tab, empty state, and supports create,
 *    visibility toggle, JSON import, and deletion.
 */

import { describe, it, beforeEach, afterEach } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import React, { act } from 'react'
import { initDOM, mount, type RenderResult } from '../test-dom'
import { ExtensionsPage } from '../pages/ExtensionsPage'
import { cancelAllInFlightRequests } from '../api'
import type { CommunityExtension } from '../types'

const { window: testWindow } = initDOM()
Object.assign(globalThis, { localStorage: testWindow.localStorage })

interface RecordedCall {
  url: string
  method: string
  body?: any
}

function installFetch(initialExtensions: CommunityExtension[] = []): {
  calls: RecordedCall[]
  getExtensions: () => CommunityExtension[]
} {
  const calls: RecordedCall[] = []
  let extensions: CommunityExtension[] = [...initialExtensions]

  globalThis.fetch = (async (url: any, opts: any) => {
    const target = String(url)
    const method = (opts?.method || 'GET').toUpperCase()
    const body = opts?.body ? JSON.parse(String(opts.body)) : undefined
    calls.push({ url: target, method, body })

    let payload: any = { success: true }
    if (target.startsWith('/api/enhancements/apply')) {
      payload = { success: true, message: 'Applied' }
    } else if (target.startsWith('/api/enhancements')) {
      payload = {
        extensions: {},
        left_panel_extensions_mode: 'single',
        community_extensions: extensions,
      }
    } else if (target.startsWith('/api/files/ide/config')) {
      payload = { success: true, preferred_ide: 'code' }
    } else if (target.startsWith('/api/memos/config')) {
      payload = { success: true, config: {} }
    } else if (target.startsWith('/api/extensions/community')) {
      if (method === 'GET') {
        payload = { success: true, extensions }
      } else if (method === 'POST') {
        if (body?.extensions && Array.isArray(body.extensions)) {
          for (const item of body.extensions) {
            const idx = extensions.findIndex((e) => e.id === item.id)
            if (idx >= 0) extensions[idx] = { ...extensions[idx], ...item }
            else extensions.push(item)
          }
          payload = { success: true, extensions }
        } else {
          const item = body as CommunityExtension
          const idx = extensions.findIndex((e) => e.id === item.id)
          if (idx >= 0) extensions[idx] = { ...extensions[idx], ...item }
          else extensions.push(item)
          payload = { success: true, extension: item, extensions }
        }
      } else if (method === 'DELETE') {
        const urlObj = new URL(target, 'http://localhost:8765')
        const id = urlObj.searchParams.get('id')
        extensions = extensions.filter((e) => e.id !== id)
        payload = { success: true, deleted: id, extensions }
      }
    }
    return { ok: true, status: 200, json: async () => payload } as any
  }) as any

  return {
    calls,
    getExtensions: () => extensions,
  }
}

const flush = async () => {
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 0))
  })
}

const findButton = (view: RenderResult, label: string): HTMLButtonElement | undefined =>
  view
    .querySelectorAll<HTMLButtonElement>('button')
    .find((b) => (b.textContent || '').trim() === label) ||
  view
    .querySelectorAll<HTMLButtonElement>('button')
    .find((b) => (b.textContent || '').trim().includes(label))

function simulateInput(element: HTMLElement | null | undefined, value: string) {
  if (!element) return
  const propsKey = Object.keys(element).find((k) => k.startsWith('__reactProps'))
  if (propsKey && (element as any)[propsKey]?.onChange) {
    ;(element as any)[propsKey].onChange({ target: { value } })
  } else {
    ;(element as any).value = value
    element.dispatchEvent(new (globalThis as any).Event('input', { bubbles: true }))
    element.dispatchEvent(new (globalThis as any).Event('change', { bubbles: true }))
  }
}

describe('Extensions top tab switcher & Community Extensions UI (#203)', () => {
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

  it('1. App.tsx renders the top header segmented switcher with Built-in and Community tabs when currentTool === 7', () => {
    const __dirname = path.dirname(fileURLToPath(import.meta.url))
    const appSrc = fs.readFileSync(path.resolve(__dirname, '../App.tsx'), 'utf8')

    // Header condition allows currentTool === 7 to render top header
    assert.ok(
      appSrc.includes('currentTool !== 6'),
      'header condition must permit currentTool === 7'
    )
    assert.ok(
      !appSrc.includes('currentTool !== 7'),
      'old header exclusion for tool 7 must be removed'
    )

    // State definition for extensionsTab
    assert.ok(
      appSrc.includes('const [extensionsTab, setExtensionsTab] = useState<number>(0)'),
      'App must declare extensionsTab state defaulting to 0'
    )

    // Segmented tab switcher for tool 7
    assert.ok(
      appSrc.includes("['Built-in', 'Community'].map("),
      'App must map over Built-in and Community tabs'
    )
    assert.ok(
      appSrc.includes('currentTool === 7 &&'),
      'App must conditionally render segmented switcher for currentTool === 7'
    )

    // ExtensionsPage receives activeTab and onTabChange
    assert.ok(
      appSrc.includes('activeTab={extensionsTab}'),
      'ExtensionsPage must receive activeTab prop'
    )
    assert.ok(
      appSrc.includes('onTabChange={setExtensionsTab}'),
      'ExtensionsPage must receive onTabChange prop'
    )
  })

  it('2. ExtensionsPage defaults to Built-in (activeTab === 0) and renders all built-in gadgets', async () => {
    installFetch()
    view = await mount(React.createElement(ExtensionsPage))
    await flush()

    const content = view.container.textContent || ''
    assert.ok(
      content.includes('Left Panel Extensions Access Mode'),
      'Left Panel Access Mode card must render in Built-in tab'
    )
    assert.ok(
      content.includes('GitHub Workspace'),
      'GitHub Workspace card must render in Built-in tab'
    )
    assert.ok(
      content.includes('Preview Browser'),
      'Preview Browser card must render in Built-in tab'
    )
    assert.ok(
      content.includes('Auxiliary File Explorer'),
      'Auxiliary File Explorer card must render in Built-in tab'
    )
    assert.ok(
      content.includes('Quick Memos'),
      'Quick Memos card must render in Built-in tab'
    )

    // Community tab content must NOT be displayed in Built-in tab
    assert.ok(
      !content.includes('No community extensions installed.'),
      'Community empty state must not appear on Built-in tab'
    )
  })

  it('3. ExtensionsPage with activeTab={1} renders Community tab, empty state, and handles create, vis toggle, import, and delete', async () => {
    const { calls } = installFetch([])
    view = await mount(React.createElement(ExtensionsPage, { activeTab: 1 }))
    await flush()

    const content = view.container.textContent || ''
    assert.ok(
      content.includes('Community Extensions'),
      'Community Extensions header card must render'
    )
    assert.ok(
      content.includes('Create, import, and manage custom web or HTML/JS extensions'),
      'Community subtitle must render'
    )
    assert.ok(
      content.includes('No community extensions installed. Click Create Extension or Import JSON to add your first custom extension.'),
      'Empty state card must render when communityExtensions is empty'
    )

    // --- Step A: Create Extension ---
    const createBtn = findButton(view, 'Create Extension')
    assert.ok(createBtn, 'Create Extension button must exist')
    await view.click(createBtn)
    await flush()

    // Editor card should now be open
    const nameInput = view.querySelector<HTMLInputElement>('input[aria-label="Extension Name"]')
    const idInput = view.querySelector<HTMLInputElement>('input[aria-label="Extension ID"]')
    const descInput = view.querySelector<HTMLTextAreaElement>('textarea[aria-label="Extension Description"]')
    const urlInput = view.querySelector<HTMLInputElement>('input[aria-label="Extension Target URL"]')
    assert.ok(nameInput, 'Extension Name input must render in editor')
    assert.ok(idInput, 'Extension ID input must render in editor')
    assert.ok(descInput, 'Extension Description textarea must render in editor')
    assert.ok(urlInput, 'Target URL input must render for url source_type')

    await act(async () => {
      simulateInput(nameInput, 'SQLite Viewer')
    })

    await act(async () => {
      simulateInput(descInput, 'Database inspection tool')
    })

    await act(async () => {
      simulateInput(urlInput, 'http://localhost:4000')
    })

    // Select Database icon
    const dbIconBtn = view.querySelector<HTMLButtonElement>('button[aria-label="Select icon Database"]')
    assert.ok(dbIconBtn, 'Database icon option button must exist')
    await view.click(dbIconBtn)

    // Save extension
    const saveBtn = findButton(view, 'Save Extension')
    assert.ok(saveBtn, 'Save Extension button must exist')
    await view.click(saveBtn)
    await flush()

    // Verify save POST call
    const saveCall = calls.find(
      (c) => c.url.startsWith('/api/extensions/community') && c.method === 'POST'
    )
    assert.ok(saveCall, 'saveCommunityExtension POST must be invoked')
    assert.equal(saveCall.body.name, 'SQLite Viewer')
    assert.equal(saveCall.body.id, 'sqlite-viewer')
    assert.equal(saveCall.body.icon, 'database')
    assert.equal(saveCall.body.url, 'http://localhost:4000')
    assert.equal(saveCall.body.source_type, 'url')

    // Card should now be rendered in the list
    assert.ok(
      (view.container.textContent || '').includes('SQLite Viewer'),
      'Saved extension card must appear in the list'
    )

    // --- Step B: Toggle Visibility Switch ---
    const mainPageToggle = view.querySelector<HTMLInputElement>(
      'input[aria-label="SQLite Viewer available in main page"]'
    )
    assert.ok(mainPageToggle, 'Main page ToggleSwitch for SQLite Viewer must exist')
    calls.length = 0 // clear recorded calls
    await view.click(mainPageToggle.parentElement)
    await flush()

    const toggleCall = calls.find(
      (c) => c.url.startsWith('/api/extensions/community') && c.method === 'POST'
    )
    assert.ok(toggleCall, 'Toggling switch must trigger saveCommunityExtension')
    assert.equal(toggleCall.body.main_page, true)

    // --- Step C: Import JSON ---
    const importBtn = findButton(view, 'Import JSON')
    assert.ok(importBtn, 'Import JSON button must exist')
    await view.click(importBtn)
    await flush()

    const importTextarea = view.querySelector<HTMLTextAreaElement>(
      'textarea[aria-label="Import JSON Content"]'
    )
    assert.ok(importTextarea, 'Import JSON textarea must render')

    const testImportPayload = [
      {
        id: 'graph-viewer',
        name: 'Graph Viewer',
        source_type: 'url',
        url: 'http://localhost:5000',
        enabled: true,
        aux_panel: true,
        main_page: false,
      },
    ]

    await act(async () => {
      simulateInput(importTextarea, JSON.stringify(testImportPayload))
    })

    const submitImportBtn = findButton(view, 'Import')
    assert.ok(submitImportBtn, 'Import submit button must exist in dialog')
    calls.length = 0
    await view.click(submitImportBtn)
    await flush()

    const importCall = calls.find(
      (c) => c.url.startsWith('/api/extensions/community') && c.method === 'POST'
    )
    assert.ok(importCall, 'importCommunityExtensions POST must be invoked')
    assert.ok(Array.isArray(importCall.body.extensions))
    assert.equal(importCall.body.extensions[0].id, 'graph-viewer')

    assert.ok(
      (view.container.textContent || '').includes('Graph Viewer'),
      'Imported extension card must now be rendered'
    )

    // --- Step D: Delete Extension ---
    const deleteBtn = view
      .querySelectorAll<HTMLButtonElement>('button')
      .find((b) => (b.textContent || '').trim().includes('Delete'))
    assert.ok(deleteBtn, 'Delete button must exist on extension card')
    calls.length = 0
    await view.click(deleteBtn)
    await flush()

    const deleteCall = calls.find(
      (c) => c.url.startsWith('/api/extensions/community') && c.method === 'DELETE'
    )
    assert.ok(deleteCall, 'deleteCommunityExtension DELETE must be invoked')
    assert.ok(deleteCall.url.includes('id='), 'DELETE call must include id query param')
  })
})

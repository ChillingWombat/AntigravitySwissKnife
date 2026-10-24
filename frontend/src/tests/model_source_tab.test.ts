import { afterEach, describe, it } from 'node:test'
import assert from 'node:assert/strict'
import React, { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { cancelAllInFlightRequests } from '../api'
import { CustomModelsPage } from '../pages/CustomModelsPage'
import { ModelSourceTab } from '../components/ModelSourceTab'
import { initDOM, mount, type RenderResult } from '../test-dom'

const integration = {
  mode: 'profile_isolation',
  apps: {
    antigravity: { source: 'native', subagents: 'app' },
    vscode: { source: 'native', subagents: 'app' },
  },
  proxy: { scope: 'antigravity', pin: '' },
}

const jsonResponse = (payload: unknown) =>
  ({
    ok: true,
    status: 200,
    json: async () => payload,
  }) as Response

const flush = async () => {
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 20))
  })
}

const compactText = (value: string | null | undefined) => (value || '').replace(/\s+/g, ' ').trim()

describe('Custom model navigation and integration mode layout', () => {
  const originalFetch = globalThis.fetch
  let root: Root | null = null
  let view: RenderResult | null = null

  afterEach(() => {
    if (root) {
      act(() => root?.unmount())
      root = null
    }
    view?.unmount()
    view = null
    globalThis.fetch = originalFetch
    cancelAllInFlightRequests()
  })

  const mockFetch = (url: unknown) => {
    const target = String(url)
    if (target === '/api/custom_models') {
      return jsonResponse({ version: '1.0.0', inference_supported: true, models: [] })
    }
    if (target === '/api/rules') {
      return jsonResponse({ default_custom_model: '' })
    }
    return jsonResponse({ integration, endpoint: 'http://127.0.0.1:8765/v1', gemini_base: 'http://127.0.0.1:8765/gemini' })
  }

  for (const portalId of ['custom-models-top-bar-portal', 'top-bar-right']) {
    it(`renders the tab switcher in #${portalId} and keeps model actions in the Models body`, async () => {
      const { document } = initDOM()
      const topBar = document.createElement('div')
      topBar.id = portalId
      document.body.appendChild(topBar)
      const container = document.createElement('div')
      document.body.appendChild(container)

      globalThis.fetch = (async (url: unknown) => mockFetch(url)) as typeof fetch
      root = createRoot(container as unknown as Element)
      await act(async () => {
        root?.render(React.createElement(CustomModelsPage))
      })
      await flush()

      assert.match(topBar.textContent || '', /Models/)
      assert.match(topBar.textContent || '', /Integration Mode/)
      assert.doesNotMatch(document.body.textContent || '', /Model Source & Mode Settings/)

      const gadget = (container as any).querySelector('.google-card.custom-models-toolbar-gadget')
      assert.ok(gadget, 'Models body must render the dedicated toolbar gadget')
      assert.ok(gadget?.classList.contains('google-card'))
      const labels = Array.from(gadget?.querySelectorAll('button') || []).map((button: any) => compactText(button.textContent))
      assert.ok(labels.some((label) => label.startsWith('All Models')), 'All Models filter must be in the Models gadget')
      assert.ok(labels.some((label) => label.startsWith('Enabled')), 'Enabled filter must be in the Models gadget')
      assert.ok(labels.some((label) => label.startsWith('Disabled')), 'Disabled filter must be in the Models gadget')
      assert.ok(labels.some((label) => label === 'Add Custom Model'), 'Add Custom Model must be in the Models gadget')
      assert.ok(labels.some((label) => label === 'Add GCP Models'), 'Add GCP Models must be in the Models gadget')
      assert.ok(gadget?.querySelector('button[title="Refresh Quotas"]'), 'Refresh must be in the Models gadget')
      assert.equal((topBar as any).querySelector('button[title="Refresh Quotas"]'), null, 'Model actions must not remain in the header portal')
    })
  }

  it('wraps mode cards and explanation copy inside their gadget boundaries', async () => {
    initDOM()
    globalThis.fetch = (async () => mockFetch('/api/custom_models/integration')) as typeof fetch
    view = await mount(React.createElement(ModelSourceTab))
    await flush()

    const modeCards = view.querySelectorAll<HTMLButtonElement>('button').filter((button) => {
      const text = compactText(button.textContent)
      return text.includes('Profile Isolation') || text.includes('API Proxy')
    })
    assert.equal(modeCards.length, 2, 'Both integration mode cards must render')

    for (const card of modeCards) {
      assert.equal(card.style.whiteSpace, 'normal')
      assert.equal(card.style.overflowWrap, 'break-word')
      assert.equal(card.style.wordBreak, 'break-word')
      assert.ok(card.style.minWidth === '0' || card.style.minWidth === '0px')
      assert.equal(card.style.overflow, 'hidden')

      const description = card.querySelector<HTMLParagraphElement>('p')
      assert.ok(description, 'Mode card explanation must render')
      assert.equal(description?.style.whiteSpace, 'normal')
      assert.equal(description?.style.overflowWrap, 'break-word')
      assert.equal(description?.style.wordBreak, 'break-word')
      assert.ok(description?.style.minWidth === '0' || description?.style.minWidth === '0px')
    }
  })
})

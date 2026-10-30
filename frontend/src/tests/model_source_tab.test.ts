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

  const mockFetch = (url: unknown, init?: RequestInit) => {
    const target = String(url)
    if (target === '/api/custom_models') {
      return jsonResponse({ version: '1.0.0', inference_supported: true, models: [] })
    }
    if (target === '/api/rules') {
      return jsonResponse({ default_custom_model: '' })
    }
    if (init?.method === 'POST' && target === '/api/custom_models/integration' && init.body) {
      return jsonResponse({ success: true, integration: JSON.parse(String(init.body)) })
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

  it('renders the AGY CLI note card without an icon', async () => {
    initDOM()
    globalThis.fetch = (async () => mockFetch('/api/custom_models/integration')) as typeof fetch
    view = await mount(React.createElement(ModelSourceTab))
    await flush()

    const note = view.querySelector<HTMLDivElement>('div[role="note"]')
    assert.ok(note, 'AGY CLI note must render')
    assert.match(note.textContent || '', /AGY CLI/)
    assert.match(note.textContent || '', /Runs independently/)
    assert.equal(note.querySelector('svg'), null, 'AGY CLI note must not have an icon')
  })

  it('renders proxy scope tabs with "Antigravity Only", "Global Use", and "Increased Ban Risk" icon tooltip', async () => {
    initDOM()
    const proxyIntegration = {
      mode: 'api_proxy',
      apps: {
        antigravity: { source: 'native', subagents: 'app' },
        vscode: { source: 'native', subagents: 'app' },
      },
      proxy: { scope: 'antigravity', pin: '' },
    }
    globalThis.fetch = (async () => jsonResponse({
      integration: proxyIntegration,
      endpoint: 'http://127.0.0.1:8765/v1',
      gemini_base: 'http://127.0.0.1:8765/v1beta',
    })) as typeof fetch

    view = await mount(React.createElement(ModelSourceTab))
    await flush()

    const buttons = view.querySelectorAll<HTMLButtonElement>('button')
    const buttonTexts = buttons.map((b) => compactText(b.textContent))

    assert.ok(buttonTexts.some((text) => text === 'Antigravity Only'), 'Antigravity Only tab must exist')
    assert.ok(buttonTexts.some((text) => text.includes('Global Use')), 'Global Use tab must exist')
    assert.ok(!buttonTexts.some((text) => text.includes('Antigravity apps only')), 'Legacy tab text must be removed')
    assert.ok(!buttonTexts.some((text) => text.includes('Also expose to external agents')), 'Legacy tab text must be removed')

    // Find the Global Use button
    const globalUseButton = buttons.find((b) => compactText(b.textContent).includes('Global Use'))
    assert.ok(globalUseButton, 'Global Use button must exist')

    // Verify "Increased Ban Risk" icon title is on or beside the Global Use button
    const alertIcon = globalUseButton.querySelector('[title="Increased Ban Risk"]')
    assert.ok(alertIcon, '"Increased Ban Risk" icon title must be present on Global Use button')
    assert.equal(alertIcon.getAttribute('aria-label'), 'Increased Ban Risk')

    // Verify standalone warning row and explanatory sentence are removed
    const textContent = view.container.textContent || ''
    assert.doesNotMatch(textContent, /External agents can call native models through this endpoint/)
  })

  it('conditionally hides protocol fields on "Antigravity Only" and displays them on "Global Use"', async () => {
    initDOM()
    let currentIntegration = {
      mode: 'api_proxy',
      apps: {
        antigravity: { source: 'native', subagents: 'app' },
        vscode: { source: 'native', subagents: 'app' },
      },
      proxy: { scope: 'antigravity', pin: '' },
    }

    globalThis.fetch = (async (url: unknown, init?: RequestInit) => {
      const target = String(url)
      if (init?.method === 'POST' && target === '/api/custom_models/integration' && init.body) {
        currentIntegration = JSON.parse(String(init.body))
        return jsonResponse({ success: true, integration: currentIntegration })
      }
      return jsonResponse({
        integration: currentIntegration,
        endpoint: 'http://127.0.0.1:8765/v1',
        gemini_base: 'http://127.0.0.1:8765/v1beta',
      })
    }) as typeof fetch

    view = await mount(React.createElement(ModelSourceTab))
    await flush()

    // 1. Antigravity Only selected: Protocol fields and Private Key must NOT be rendered
    assert.doesNotMatch(view.container.textContent || '', /OpenAI protocol/)
    assert.doesNotMatch(view.container.textContent || '', /Gemini protocol/)
    assert.doesNotMatch(view.container.textContent || '', /Private Key/)
    assert.equal(view.querySelector('input[placeholder="Required"]'), null)

    // 2. Click "Global Use" tab
    const globalUseBtn = view.querySelectorAll<HTMLButtonElement>('button')
      .find((b) => compactText(b.textContent).includes('Global Use'))
    assert.ok(globalUseBtn, 'Global Use button must be found')

    await view.click(globalUseBtn)
    await flush()

    // Now protocol sections and Private Key ARE rendered
    assert.match(view.container.textContent || '', /OpenAI protocol/)
    assert.match(view.container.textContent || '', /Gemini protocol/)
    assert.match(view.container.textContent || '', /Private Key/)

    const privateKeyInput = view.querySelector<HTMLInputElement>('input[placeholder="Required"]')
    assert.ok(privateKeyInput, 'Private key input with placeholder "Required" must be rendered')
    assert.doesNotMatch(view.container.textContent || '', /PIN \/ private key \(optional\)/)
    assert.equal(view.querySelector('input[placeholder="Require a PIN for external calls"]'), null)

    // 3. Switch back to "Antigravity Only"
    const antigravityOnlyBtn = view.querySelectorAll<HTMLButtonElement>('button')
      .find((b) => compactText(b.textContent) === 'Antigravity Only')
    assert.ok(antigravityOnlyBtn, 'Antigravity Only button must be found')

    await view.click(antigravityOnlyBtn)
    await flush()

    // Protocol sections must be hidden again
    assert.doesNotMatch(view.container.textContent || '', /OpenAI protocol/)
    assert.doesNotMatch(view.container.textContent || '', /Gemini protocol/)
    assert.doesNotMatch(view.container.textContent || '', /Private Key/)
  })

  it('renders clean base URLs, icon-only copy buttons, and eliminates explanatory subtitles under protocols', async () => {
    initDOM()
    const externalIntegration = {
      mode: 'api_proxy',
      apps: {
        antigravity: { source: 'native', subagents: 'app' },
        vscode: { source: 'native', subagents: 'app' },
      },
      proxy: { scope: 'external', pin: 'test-secret' },
    }

    globalThis.fetch = (async () => jsonResponse({
      integration: externalIntegration,
      endpoint: 'http://127.0.0.1:8765/v1',
      gemini_base: 'http://127.0.0.1:8765/v1beta',
    })) as typeof fetch

    view = await mount(React.createElement(ModelSourceTab))
    await flush()

    // Clean displayed base URLs (no /models suffix)
    const codeElements = view.querySelectorAll<HTMLElement>('code')
    assert.equal(codeElements.length, 2, 'Two code elements for OpenAI and Gemini endpoints')
    assert.equal(compactText(codeElements[0].textContent), 'http://127.0.0.1:8765/v1')
    assert.equal(compactText(codeElements[1].textContent), 'http://127.0.0.1:8765/v1beta')

    // Explanatory subtitles removed
    const containerText = view.container.textContent || ''
    assert.doesNotMatch(containerText, /Gemini model calls are converted to the Gemini protocol/)
    assert.doesNotMatch(containerText, /Gemini models only, listed and served without conversion/)

    // Icon-only copy buttons
    const copyButtons = view.querySelectorAll<HTMLButtonElement>('button[aria-label="Copy"]')
    assert.equal(copyButtons.length, 2, 'Two icon-only copy buttons must exist')

    for (const copyBtn of copyButtons) {
      assert.equal(compactText(copyBtn.textContent), '', 'Copy button must be icon-only with no text label')
      assert.equal(copyBtn.title, 'Copy')
      assert.ok(copyBtn.querySelector('svg'), 'Copy button must contain an SVG icon')
    }

    // Test clicking OpenAI copy button
    await view.click(copyButtons[0])
    await flush()
    assert.equal(await navigator.clipboard.readText(), 'http://127.0.0.1:8765/v1')
    assert.equal(copyButtons[0].title, 'Copied')

    // Test clicking Gemini copy button
    await view.click(copyButtons[1])
    await flush()
    assert.equal(await navigator.clipboard.readText(), 'http://127.0.0.1:8765/v1beta')
    assert.equal(copyButtons[1].title, 'Copied')
  })

  it('shows a configured PIN without returning the secret to the renderer', async () => {
    initDOM()
    const externalIntegration = {
      mode: 'api_proxy',
      apps: {
        antigravity: { source: 'native', subagents: 'app' },
        vscode: { source: 'native', subagents: 'app' },
      },
      proxy: { scope: 'external', pin: '' },
    }

    globalThis.fetch = (async () => jsonResponse({
      integration: externalIntegration,
      endpoint: 'http://127.0.0.1:8765/v1',
      gemini_base: 'http://127.0.0.1:8765/v1beta',
      pin_set: true,
    })) as typeof fetch

    view = await mount(React.createElement(ModelSourceTab))
    await flush()

    const privateKeyInput = view.querySelector<HTMLInputElement>('input[placeholder="Configured"]')
    assert.ok(privateKeyInput, 'Configured PIN placeholder must render')
    assert.equal(privateKeyInput.value, '', 'Stored PIN must not be sent to the renderer')
  })
})

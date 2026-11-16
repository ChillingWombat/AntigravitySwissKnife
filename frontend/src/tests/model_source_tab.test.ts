import { afterEach, describe, it } from 'node:test'
import assert from 'node:assert/strict'
import React, { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { cancelAllInFlightRequests } from '../api'
import { CustomModelsPage } from '../pages/CustomModelsPage'
import { ModelSourceTab } from '../components/ModelSourceTab'
import {
  SwitcherSettingsPage,
  THINKING_LEVEL_ORDER,
  THINKING_SUFFIX_RE,
  MODEL_LEVEL_SUFFIX_RE,
  buildGeminiBaseModels,
  formatThinkingLevel,
} from '../pages/SwitcherSettingsPage'
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

  it('renders 429 Anti-Cascade Guard status in API Proxy mode and resets a tripped circuit breaker', async () => {
    const proxyIntegration = {
      mode: 'api_proxy',
      apps: {
        antigravity: { source: 'native', subagents: 'app' },
        vscode: { source: 'native', subagents: 'app' },
      },
      proxy: { scope: 'antigravity', pin: '' },
    }

    initDOM()
    globalThis.fetch = (async () => jsonResponse({
      success: true,
      integration: proxyIntegration,
      endpoint: 'http://127.0.0.1:8765/v1',
      gemini_base: 'http://127.0.0.1:8765/v1beta',
      pin_set: false,
      protection: {
        circuit_open: false,
        anomalous_429_blocked: 1,
        correlated_429_aborts: 0,
        sanitized_requests: 2,
        blocked_external_clients: 0,
      },
    })) as typeof fetch

    view = await mount(React.createElement(ModelSourceTab))
    await flush()

    const closedText = compactText(view.container.textContent)
    assert.match(closedText, /429 Anti-Cascade Guard/)
    assert.match(closedText, /Protected/)
    assert.match(closedText, /1 blocked · 2 sanitized/)
    assert.doesNotMatch(closedText, /Circuit Tripped/)
    assert.equal(
      view.querySelectorAll<HTMLButtonElement>('button').some((b) => compactText(b.textContent) === 'Reset'),
      false,
      'Reset button must not appear when circuit is closed'
    )

    view.unmount()
    view = null

    initDOM()
    const postedBodies: Array<Record<string, unknown>> = []
    let protectionState = {
      circuit_open: true,
      circuit_reason: 'anomalous_trap_429: missing Retry-After',
      circuit_open_until: '2026-11-17T01:45:00Z',
      anomalous_429_blocked: 1,
      correlated_429_aborts: 0,
      sanitized_requests: 0,
      blocked_external_clients: 0,
    }

    globalThis.fetch = (async (url: unknown, init?: RequestInit) => {
      const target = String(url)
      if (init?.method === 'POST' && target === '/api/custom_models/integration' && init.body) {
        const parsed = JSON.parse(String(init.body)) as Record<string, unknown>
        postedBodies.push(parsed)
        if (parsed.reset_circuit_breaker === true) {
          protectionState = {
            ...protectionState,
            circuit_open: false,
            circuit_reason: '',
            circuit_open_until: '',
          }
        }
        return jsonResponse({
          success: true,
          integration: proxyIntegration,
          pin_set: false,
          protection: protectionState,
        })
      }
      return jsonResponse({
        success: true,
        integration: proxyIntegration,
        endpoint: 'http://127.0.0.1:8765/v1',
        gemini_base: 'http://127.0.0.1:8765/v1beta',
        pin_set: false,
        protection: protectionState,
      })
    }) as typeof fetch

    view = await mount(React.createElement(ModelSourceTab))
    await flush()

    const openText = compactText(view.container.textContent)
    assert.match(openText, /429 Anti-Cascade Guard/)
    assert.match(openText, /Circuit Tripped/)
    assert.match(openText, /anomalous_trap_429: missing Retry-After/)

    const resetBtn = view.querySelectorAll<HTMLButtonElement>('button')
      .find((b) => compactText(b.textContent) === 'Reset')
    assert.ok(resetBtn, 'Reset button must render when circuit breaker is open')
    assert.equal(resetBtn.style.whiteSpace, 'nowrap')
    assert.ok(resetBtn.querySelector('svg'), 'Reset button must render a Lucide SVG icon')

    await view.click(resetBtn)
    await flush()

    assert.equal(postedBodies.length, 1, 'Clicking Reset must send one POST to /api/custom_models/integration')
    assert.equal(postedBodies[0].reset_circuit_breaker, true)
    assert.equal(postedBodies[0].mode, 'api_proxy')

    const afterResetText = compactText(view.container.textContent)
    assert.match(afterResetText, /Protected/)
    assert.doesNotMatch(afterResetText, /Circuit Tripped/)
  })
})
describe('SwitcherSettingsPage Gemini 4, Orchestrator (Argon), and Claude 5.5 support', () => {
  const originalFetch = globalThis.fetch
  let view: RenderResult | null = null

  afterEach(() => {
    view?.unmount()
    view = null
    globalThis.fetch = originalFetch
    cancelAllInFlightRequests()
  })

  it('includes xhigh in THINKING_LEVEL_ORDER and suffix regexes while preserving (Argon)', () => {
    assert.deepEqual(THINKING_LEVEL_ORDER, ['off', 'low', 'medium', 'high', 'xhigh'])
    assert.equal(THINKING_SUFFIX_RE.test('Gemini 4 Hydrogen (XHigh)'), true)
    assert.equal(THINKING_SUFFIX_RE.test('Gemini 4 Argon (High)'), true)
    assert.equal(THINKING_SUFFIX_RE.test('Orchestrator (Argon)'), false)
    assert.equal(MODEL_LEVEL_SUFFIX_RE.test('gemini-4-hydrogen-xhigh'), true)
    assert.equal(MODEL_LEVEL_SUFFIX_RE.test('gemini-pro-orchestrator-g1-pro'), false)
    assert.equal(formatThinkingLevel('xhigh'), 'XHigh')
    assert.equal(formatThinkingLevel('high'), 'High')
    assert.equal(formatThinkingLevel('off'), 'Off')
  })

  it('groups Gemini 4 Hydrogen (XHigh), Gemini 4 Argon (High), and preserves Orchestrator (Argon)', () => {
    const baseModels = buildGeminiBaseModels([
      {
        id: 'gemini-4-argon-high',
        display_name: 'Gemini 4 Argon (High)',
        supports_thinking: true,
        thinking_levels: ['low', 'medium', 'high'],
        provider: 'Google',
      },
      {
        id: 'gemini-4-argon-medium',
        display_name: 'Gemini 4 Argon (Medium)',
        supports_thinking: true,
        thinking_levels: ['low', 'medium', 'high'],
        provider: 'Google',
      },
      {
        id: 'gemini-4-argon-low',
        display_name: 'Gemini 4 Argon (Low)',
        supports_thinking: true,
        thinking_levels: ['low', 'medium', 'high'],
        provider: 'Google',
      },
      {
        id: 'gemini-4-hydrogen-xhigh',
        display_name: 'Gemini 4 Hydrogen (XHigh)',
        supports_thinking: true,
        thinking_levels: ['low', 'medium', 'high', 'xhigh'],
        provider: 'Google',
      },
      {
        id: 'gemini-4-hydrogen-high',
        display_name: 'Gemini 4 Hydrogen (High)',
        supports_thinking: true,
        thinking_levels: ['low', 'medium', 'high', 'xhigh'],
        recommended: true,
        provider: 'Google',
      },
      {
        id: 'gemini-4-hydrogen-medium',
        display_name: 'Gemini 4 Hydrogen (Medium)',
        supports_thinking: true,
        thinking_levels: ['low', 'medium', 'high', 'xhigh'],
        provider: 'Google',
      },
      {
        id: 'gemini-4-hydrogen-low',
        display_name: 'Gemini 4 Hydrogen (Low)',
        supports_thinking: true,
        thinking_levels: ['low', 'medium', 'high', 'xhigh'],
        provider: 'Google',
      },
      {
        id: 'gemini-pro-orchestrator-g1-pro',
        display_name: 'Orchestrator (Argon)',
        supports_thinking: true,
        thinking_levels: ['high'],
        provider: 'Google',
      },
      {
        id: 'gemini-3.8-flash-high',
        display_name: 'Gemini 3.8 Flash (High)',
        supports_thinking: true,
        thinking_levels: ['off', 'low', 'medium', 'high'],
        provider: 'Google',
      },
      {
        id: 'gemini-pro-agent',
        display_name: 'Gemini 3.1 Pro (High)',
        supports_thinking: true,
        thinking_levels: ['off', 'low', 'medium', 'high'],
        provider: 'Google',
      },
      {
        id: 'gemini-3.1-pro-low',
        display_name: 'Gemini 3.1 Pro (Low)',
        supports_thinking: true,
        thinking_levels: ['off', 'low', 'medium', 'high'],
        provider: 'Google',
      },
    ])

    const byId = new Map(baseModels.map((m) => [m.id, m]))

    const argon = byId.get('gemini-4-argon')
    assert.ok(argon, 'Gemini 4 Argon base model must exist')
    assert.equal(argon.displayName, 'Gemini 4 Argon')
    assert.equal(argon.supportsThinking, true)
    assert.deepEqual(argon.thinkingLevels, ['low', 'medium', 'high'])
    assert.deepEqual(argon.modelIds, ['gemini-4-argon-high', 'gemini-4-argon-medium', 'gemini-4-argon-low'])

    const hydrogen = byId.get('gemini-4-hydrogen')
    assert.ok(hydrogen, 'Gemini 4 Hydrogen base model must exist')
    assert.equal(hydrogen.displayName, 'Gemini 4 Hydrogen')
    assert.equal(hydrogen.supportsThinking, true)
    assert.deepEqual(hydrogen.thinkingLevels, ['low', 'medium', 'high', 'xhigh'])
    assert.deepEqual(hydrogen.modelIds, [
      'gemini-4-hydrogen-xhigh',
      'gemini-4-hydrogen-high',
      'gemini-4-hydrogen-medium',
      'gemini-4-hydrogen-low',
    ])

    const orchestrator = byId.get('gemini-pro-orchestrator-g1-pro')
    assert.ok(orchestrator, 'Orchestrator (Argon) must preserve canonical ID gemini-pro-orchestrator-g1-pro')
    assert.equal(orchestrator.displayName, 'Orchestrator (Argon)')
    assert.equal(orchestrator.supportsThinking, true)
    assert.deepEqual(orchestrator.thinkingLevels, ['high'])
    assert.deepEqual(orchestrator.modelIds, ['gemini-pro-orchestrator-g1-pro'])

    const flash38 = byId.get('gemini-3.8-flash')
    assert.ok(flash38, 'Gemini 3.8 Flash base model must exist')
    assert.deepEqual(flash38.thinkingLevels, ['off', 'low', 'medium', 'high'])

    const pro31 = byId.get('gemini-3.1-pro')
    assert.ok(pro31, 'Gemini 3.1 Pro variants must still group cleanly')
    assert.deepEqual(pro31.modelIds, ['gemini-pro-agent', 'gemini-3.1-pro-low'])
  })

  it('infers xhigh thinking level when thinking_levels is omitted on an (XHigh) variant', () => {
    const [hydrogen] = buildGeminiBaseModels([
      {
        id: 'gemini-4-hydrogen-xhigh',
        display_name: 'Gemini 4 Hydrogen (XHigh)',
        supports_thinking: true,
      },
    ])
    assert.equal(hydrogen.id, 'gemini-4-hydrogen')
    assert.equal(hydrogen.displayName, 'Gemini 4 Hydrogen')
    assert.equal(hydrogen.supportsThinking, true)
    assert.deepEqual(hydrogen.thinkingLevels, ['off', 'low', 'medium', 'high', 'xhigh'])
  })

  it('renders Gemini 4 Hydrogen (XHigh), Gemini 4 Argon, Orchestrator (Argon), and Claude 5.5 selectors in SwitcherSettingsPage', async () => {
    initDOM()
    globalThis.fetch = (async (url: unknown) => {
      const target = String(url)
      if (target.includes('/api/models/available')) {
        return jsonResponse({
          success: true,
          default_gemini: 'gemini-3.8-flash-high',
          default_non_gemini: 'claude-opus-5-5-high',
          gemini_models: [
            {
              id: 'gemini-4-argon-high',
              display_name: 'Gemini 4 Argon (High)',
              supports_thinking: true,
              thinking_levels: ['low', 'medium', 'high'],
              provider: 'Google',
            },
            {
              id: 'gemini-4-hydrogen-xhigh',
              display_name: 'Gemini 4 Hydrogen (XHigh)',
              supports_thinking: true,
              thinking_levels: ['low', 'medium', 'high', 'xhigh'],
              provider: 'Google',
            },
            {
              id: 'gemini-4-hydrogen-high',
              display_name: 'Gemini 4 Hydrogen (High)',
              supports_thinking: true,
              thinking_levels: ['low', 'medium', 'high', 'xhigh'],
              recommended: true,
              provider: 'Google',
            },
            {
              id: 'gemini-pro-orchestrator-g1-pro',
              display_name: 'Orchestrator (Argon)',
              supports_thinking: true,
              thinking_levels: ['high'],
              provider: 'Google',
            },
            {
              id: 'gemini-3.8-flash-high',
              display_name: 'Gemini 3.8 Flash (High)',
              supports_thinking: true,
              thinking_levels: ['off', 'low', 'medium', 'high'],
              provider: 'Google',
            },
          ],
          non_gemini_models: [
            {
              id: 'claude-opus-5-5-high',
              display_name: 'Claude Opus 5.5 (High)',
              supports_thinking: true,
              thinking_levels: ['low', 'medium', 'high'],
              recommended: true,
              provider: 'Anthropic',
            },
            {
              id: 'claude-sonnet-5-5-high',
              display_name: 'Claude Sonnet 5.5 (High)',
              supports_thinking: true,
              thinking_levels: ['low', 'medium', 'high'],
              recommended: true,
              provider: 'Anthropic',
            },
          ],
        })
      }
      if (target.includes('/api/custom_models')) {
        return jsonResponse({ models: [] })
      }
      return jsonResponse({
        desktop_app: { installed: true },
        agy_cli: { installed: false },
        vscode_extension: { installed: false },
      })
    }) as typeof fetch

    view = await mount(React.createElement(SwitcherSettingsPage, { initialRules: null, onSaved: () => {} }))
    await flush()

    const geminiSelect = view.querySelector<HTMLSelectElement>('select[aria-label="Default Gemini Model"]')
    const thinkingSelect = view.querySelector<HTMLSelectElement>('select[aria-label="Default Gemini Thinking Level"]')
    const nonGeminiSelect = view.querySelector<HTMLSelectElement>('select[aria-label="Default Non-Gemini Native Model"]')

    assert.ok(geminiSelect, 'Default Gemini Model select must render')
    assert.ok(thinkingSelect, 'Default Gemini Thinking Level select must render')
    assert.ok(nonGeminiSelect, 'Default Non-Gemini Native Model select must render')

    const geminiOptions = Array.from(geminiSelect.querySelectorAll('option')).map((opt) => ({
      value: opt.value,
      label: compactText(opt.textContent),
    }))
    assert.deepEqual(geminiOptions, [
      { value: 'gemini-4-argon', label: 'Gemini 4 Argon' },
      { value: 'gemini-4-hydrogen', label: 'Gemini 4 Hydrogen' },
      { value: 'gemini-pro-orchestrator-g1-pro', label: 'Orchestrator (Argon)' },
      { value: 'gemini-3.8-flash', label: 'Gemini 3.8 Flash' },
    ])

    const nonGeminiOptions = Array.from(nonGeminiSelect.querySelectorAll('option')).map((opt) => ({
      value: opt.value,
      label: compactText(opt.textContent),
    }))
    assert.deepEqual(nonGeminiOptions, [
      { value: 'claude-opus-5.5', label: 'Claude Opus 5.5' },
      { value: 'claude-sonnet-5.5', label: 'Claude Sonnet 5.5' },
    ])

    const selectOption = async (el: HTMLSelectElement, nextValue: string) => {
      await act(async () => {
        const setter = Object.getOwnPropertyDescriptor(window.HTMLSelectElement.prototype, 'value')?.set
        setter?.call(el, nextValue)
        el.dispatchEvent(new window.Event('change', { bubbles: true }))
      })
      await flush()
    }

    await selectOption(geminiSelect, 'gemini-4-hydrogen')
    assert.equal(thinkingSelect.disabled, false)
    const hydrogenLevels = Array.from(thinkingSelect.querySelectorAll('option')).map((opt) => ({
      value: opt.value,
      label: compactText(opt.textContent),
    }))
    assert.deepEqual(hydrogenLevels, [
      { value: 'low', label: 'Low' },
      { value: 'medium', label: 'Medium' },
      { value: 'high', label: 'High' },
      { value: 'xhigh', label: 'XHigh' },
    ])

    await selectOption(geminiSelect, 'gemini-pro-orchestrator-g1-pro')
    assert.equal(thinkingSelect.disabled, false, 'Orchestrator (Argon) thinking level selector must remain enabled')
    const orchestratorLevels = Array.from(thinkingSelect.querySelectorAll('option')).map((opt) => ({
      value: opt.value,
      label: compactText(opt.textContent),
    }))
    assert.deepEqual(orchestratorLevels, [{ value: 'high', label: 'High' }])
    assert.equal(thinkingSelect.value, 'high')
  })
})

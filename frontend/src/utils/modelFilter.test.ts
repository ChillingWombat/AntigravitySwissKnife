import { afterEach, beforeEach, describe, it } from 'node:test'
import assert from 'node:assert/strict'
import React, { act } from 'react'
import { fireEvent } from '@testing-library/react'
import {
  filterModels,
  filterFetchedModels,
  computeModelFilterStats,
  resolveEffectiveCustomModel,
} from './modelFilter.ts'
import { setSessionToken } from '../api'
import { CustomModelsPage } from '../pages/CustomModelsPage'
import { initDOM, mount, type RenderResult } from '../test-dom'
import type { CustomModel, ModelInfo } from '../types.ts'

const mockModels: CustomModel[] = [
  {
    id: 'm1',
    name: 'deepseek-v4-flash',
    display_name: 'DeepSeek V4.1 Flash',
    provider_type: 'custom',
    base_url: 'https://opencode.ai/zen/go',
      enabled: true,
    is_default: true,
    quota_type: 'quota',
    prepaid_balance: 0,
    total_budget: 0,
    quota_fraction: 1.0,
    created_at: '2026-10-01T00:00:00Z',
    updated_at: '2026-10-01T00:00:00Z',
  },
  {
    id: 'm2',
    name: 'claude-3-7-sonnet',
    display_name: 'Claude 3.7 Sonnet',
    provider_type: 'anthropic',
    base_url: 'https://api.anthropic.com',
      enabled: false,
    is_default: false,
    quota_type: 'balance',
    prepaid_balance: 10,
    total_budget: 50,
    quota_fraction: 0.2,
    created_at: '2026-10-02T00:00:00Z',
    updated_at: '2026-10-02T00:00:00Z',
  },
  {
    id: 'm3',
    name: 'gemini-2-5-pro',
    display_name: 'Gemini 2.5 Pro',
    provider_type: 'gemini',
    base_url: 'https://generativelanguage.googleapis.com',
      enabled: true,
    is_default: false,
    quota_type: 'quota',
    prepaid_balance: 0,
    total_budget: 0,
    quota_fraction: 0.85,
    created_at: '2026-10-03T00:00:00Z',
    updated_at: '2026-10-03T00:00:00Z',
  },
]

describe('modelFilter utility', () => {
  describe('filterModels', () => {
    it('returns all models when filter is "all"', () => {
      const result = filterModels(mockModels, 'all')
      assert.equal(result.length, 3)
      assert.deepEqual(
        result.map((m) => m.id),
        ['m1', 'm2', 'm3']
      )
    })

    it('returns only enabled models when filter is "enabled"', () => {
      const result = filterModels(mockModels, 'enabled')
      assert.equal(result.length, 2)
      assert.deepEqual(
        result.map((m) => m.id),
        ['m1', 'm3']
      )
      assert.ok(result.every((m) => m.enabled === true))
    })

    it('returns only disabled models when filter is "disabled"', () => {
      const result = filterModels(mockModels, 'disabled')
      assert.equal(result.length, 1)
      assert.deepEqual(
        result.map((m) => m.id),
        ['m2']
      )
      assert.ok(result.every((m) => m.enabled === false))
    })

    it('handles empty model array safely', () => {
      assert.deepEqual(filterModels([], 'all'), [])
      assert.deepEqual(filterModels([], 'enabled'), [])
      assert.deepEqual(filterModels([], 'disabled'), [])
    })

    it('defaults to "all" when filter parameter is omitted', () => {
      const result = filterModels(mockModels)
      assert.equal(result.length, 3)
    })
  })

  describe('computeModelFilterStats', () => {
    it('correctly tallies total, enabled, and disabled counts', () => {
      const stats = computeModelFilterStats(mockModels)
      assert.deepEqual(stats, {
        total: 3,
        enabled: 2,
        disabled: 1,
      })
    })

    it('returns zeroes for empty models list', () => {
      const stats = computeModelFilterStats([])
      assert.deepEqual(stats, {
        total: 0,
        enabled: 0,
        disabled: 0,
      })
    })
  })

  describe('resolveEffectiveCustomModel', () => {
    it('returns blank string when available custom models list is empty', () => {
      assert.equal(resolveEffectiveCustomModel([]), '')
      assert.equal(resolveEffectiveCustomModel([], 'm1'), '')
      assert.equal(resolveEffectiveCustomModel(undefined as any), '')
    })

    it('defaults to the first model in the list when no default is set or invalid', () => {
      const models = [{ id: 'custom-gpt4' }, { id: 'custom-claude' }]
      assert.equal(resolveEffectiveCustomModel(models), 'custom-gpt4')
      assert.equal(resolveEffectiveCustomModel(models, ''), 'custom-gpt4')
      assert.equal(resolveEffectiveCustomModel(models, 'non-existent-id'), 'custom-gpt4')
    })

    it('preserves the manually chosen model if it exists in the available list', () => {
      const models = [{ id: 'custom-gpt4' }, { id: 'custom-claude' }, { id: 'custom-gemini' }]
      assert.equal(resolveEffectiveCustomModel(models, 'custom-claude'), 'custom-claude')
      assert.equal(resolveEffectiveCustomModel(models, 'custom-gemini'), 'custom-gemini')
    })

    it('matches by model_name or display name when default is set by model identifier', () => {
      const models = [
        { id: 'openai-deepseek', name: 'DeepSeek V4.1', model_name: 'deepseek-v4-flash' },
        { id: 'gcp-claude-opus-5-5-1791904890297', name: 'Gemini 4 Argon (GCP)', model_name: 'gemini-4-argon' },
      ]
      assert.equal(resolveEffectiveCustomModel(models, 'gemini-4-argon'), 'gcp-claude-opus-5-5-1791904890297')
      assert.equal(resolveEffectiveCustomModel(models, 'Gemini 4 Argon (GCP)'), 'gcp-claude-opus-5-5-1791904890297')
      assert.equal(resolveEffectiveCustomModel(models, 'deepseek-v4-flash'), 'openai-deepseek')
    })

    it('handles whitespace trimming and invalid/null array elements safely', () => {
      const models = [
        null as any,
        { id: '' },
        { id: '   ' },
        { id: 'custom-claude' },
        { id: 'custom-gemini' },
      ]
      assert.equal(resolveEffectiveCustomModel(models), 'custom-claude')
      assert.equal(resolveEffectiveCustomModel(models, '  custom-gemini  '), 'custom-gemini')
      assert.equal(resolveEffectiveCustomModel([null as any, { id: ' ' }]), '')
    })
  })

  describe('filterFetchedModels', () => {
    const fetched: ModelInfo[] = [
      {
        id: 'models/gemini-2.5-pro',
        display_name: 'Gemini 2.5 Pro',
        description: 'Advanced reasoning model with thinking capabilities',
      },
      {
        id: 'models/claude-3-7-sonnet@20250219',
        display_name: 'Claude 3.7 Sonnet',
        description: 'Hybrid reasoning and standard generation model',
      },
      {
        id: 'deepseek-chat',
        display_name: 'DeepSeek V3',
        description: 'High efficiency open weights model',
      },
    ]

    it('returns empty array when input models is empty or null', () => {
      assert.deepEqual(filterFetchedModels([], 'pro'), [])
      assert.deepEqual(filterFetchedModels(null as any, 'pro'), [])
    })

    it('returns all models when search query is empty or whitespace', () => {
      assert.equal(filterFetchedModels(fetched, '').length, 3)
      assert.equal(filterFetchedModels(fetched, '   \t  ').length, 3)
    })

    it('matches by cleaned id or raw id', () => {
      const byClean = filterFetchedModels(fetched, 'claude-3-7-sonnet')
      assert.equal(byClean.length, 1)
      assert.equal(byClean[0].display_name, 'Claude 3.7 Sonnet')
    })

    it('matches by display name or description case-insensitively', () => {
      const byDesc = filterFetchedModels(fetched, 'hybrid reasoning')
      assert.equal(byDesc.length, 1)
      assert.equal(byDesc[0].id, 'models/claude-3-7-sonnet@20250219')
    })

    it('supports multi-token queries matching across fields', () => {
      const multi = filterFetchedModels(fetched, 'gemini thinking')
      assert.equal(multi.length, 1)
      assert.equal(multi[0].id, 'models/gemini-2.5-pro')
    })

    it('returns empty array when query matches no model', () => {
      const results = filterFetchedModels(fetched, 'non-existent-model-xyz')
      assert.deepEqual(results, [])
    })

    it('filters models with parenthesized context annotations', () => {
      const annotatedList: ModelInfo[] = [
        { id: 'deepseek-v4-flash (1,048,576 ctx)', display_name: 'DeepSeek Flash' },
      ]
      const results = filterFetchedModels(annotatedList, 'deepseek-v4-flash')
      assert.equal(results.length, 1)
    })

    it('matches models with complex tags, ports, and version punctuation', () => {
      const taggedList: ModelInfo[] = [
        { id: 'deepseek-r1:14b', display_name: 'DeepSeek R1 14B' },
        { id: 'meta-llama/llama-3.1-70b-instruct', display_name: 'Llama 3.1 70B' },
        { id: 'claude-3-7-sonnet@20250219', display_name: 'Claude 3.7 Sonnet' },
        { id: 'qwen3-embedding:0.6b', display_name: 'Qwen3 Embedding:0.6b' },
        { id: 'all-minilm:latest', display_name: 'All Minilm:latest' },
      ]
      assert.equal(filterFetchedModels(taggedList, '14b').length, 1)
      assert.equal(filterFetchedModels(taggedList, 'meta-llama 70b').length, 1)
      assert.equal(filterFetchedModels(taggedList, '@20250219').length, 1)
      assert.equal(filterFetchedModels(taggedList, 'CLAUDE SONNET').length, 1)
      assert.equal(filterFetchedModels(taggedList, 'qwen 0.6b').length, 1)
      assert.equal(filterFetchedModels(taggedList, 'minilm').length, 1)
    })

    it('handles malformed or sparse model objects without throwing', () => {
      const sparse: ModelInfo[] = [
        null as any,
        { id: '' },
        { id: 'gpt-4o', display_name: undefined },
      ]
      assert.equal(filterFetchedModels(sparse, 'gpt').length, 1)
    })
  })
})

describe('CustomModelsPage fetched-model dropdown behavior', () => {
  const MODEL_ENDPOINT = 'https://models.example.test/v1'
  const MODEL_RESPONSE = [
    {
      id: 'models/gemini-4-argon',
      display_name: 'Gemini 4 Argon',
      description: 'Frontier reasoning model',
    },
    {
      id: 'models/claude-3-7-sonnet',
      display_name: 'Claude 3.7 Sonnet',
      description: 'Hybrid reasoning model',
    },
    {
      id: 'deepseek-chat',
      display_name: 'DeepSeek V3',
      description: 'Open weights model',
    },
  ]

  const originalFetch = globalThis.fetch
  let originalWindowFetch: typeof fetch
  let originalElectronAPI: unknown
  let requestController: AbortController
  let view: RenderResult | null = null
  let requests: Array<{ path: string; body: string }> = []

  const jsonResponse = (body: unknown): Response => ({
    ok: true,
    status: 200,
    headers: {
      get: (name: string) => name.toLowerCase() === 'content-type' ? 'application/json' : null,
    },
    json: async () => body,
    text: async () => JSON.stringify(body),
  }) as unknown as Response

  const installFetchMock = () => {
    requestController = new AbortController()
    const fetchMock = async (
      input: RequestInfo | URL,
      init?: RequestInit,
    ): Promise<Response> => {
      const raw = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url
      const path = new URL(raw, 'http://localhost').pathname
      const body = typeof init?.body === 'string' ? init.body : ''
      requests.push({ path, body })

      if (requestController.signal.aborted) {
        throw new DOMException('Request aborted', 'AbortError')
      }
      if (body.includes(MODEL_ENDPOINT)) {
        return jsonResponse({ success: true, models: MODEL_RESPONSE })
      }
      if (path.endsWith('/api/auth/status')) return jsonResponse({ password_required: false })
      if (path.includes('/api/custom_models')) {
        return jsonResponse({ success: true, models: [], providers: [], count: 0 })
      }
      return jsonResponse({ success: true })
    }

    globalThis.fetch = fetchMock as typeof fetch
    window.fetch = fetchMock as typeof window.fetch
  }

  const flush = async () => {
    for (let i = 0; i < 4; i += 1) {
      await act(async () => {
        await new Promise((resolve) => setTimeout(resolve, 0))
      })
    }
  }

  const setInputValue = async (input: HTMLInputElement, value: string) => {
    const propsKey = Object.keys(input).find((key) => key.startsWith('__reactProps'))
    const onChange = propsKey ? (input as any)[propsKey]?.onChange : undefined
    assert.equal(typeof onChange, 'function', 'input must expose a React onChange handler')
    await act(async () => {
      onChange({ target: { value } })
    })
    await flush()
  }

  const pressEscape = async (input: HTMLInputElement) => {
    const propsKey = Object.keys(input).find((key) => key.startsWith('__reactProps'))
    const onKeyDown = propsKey ? (input as any)[propsKey]?.onKeyDown : undefined
    assert.equal(typeof onKeyDown, 'function', 'input must expose a React onKeyDown handler')
    await act(async () => {
      fireEvent.keyDown(input, { key: 'Escape', code: 'Escape' })
      // happy-dom does not bridge synthetic key events into React's delegated listener.
      onKeyDown({ key: 'Escape', code: 'Escape', stopPropagation() {} })
    })
    await flush()
  }

  const findButton = (label: string): HTMLButtonElement | undefined => (
    Array.from(document.querySelectorAll<HTMLButtonElement>('button'))
      .find((button) => button.textContent?.includes(label))
  )

  beforeEach(() => {
    initDOM()
    requests = []
    originalWindowFetch = window.fetch
    originalElectronAPI = (window as any).electronAPI
    window.localStorage.clear()
    ;(window as any).happyDOM.setURL('http://localhost/')
    window.history.replaceState(null, '', '#/custom-models')
    setSessionToken('test-session')
    ;(window as any).electronAPI = { isElectron: false }
    installFetchMock()
  })

  afterEach(() => {
    view?.unmount()
    view = null
    requestController?.abort()
    setSessionToken(null)
    globalThis.fetch = originalFetch
    window.fetch = originalWindowFetch
    ;(window as any).electronAPI = originalElectronAPI
  })

  it('stays open while filtering and closes on Escape after a provider request', async () => {
    view = await mount(React.createElement(CustomModelsPage))
    await flush()

    const addButton = findButton('Add Custom Model')
    assert.ok(addButton, 'Add Custom Model button must render')
    await view.click(addButton)
    await flush()

    const endpointInput = document.querySelector<HTMLInputElement>(
      'input[placeholder^="e.g. https://generativelanguage.googleapis.com"]',
    )
    const apiKeyInput = document.querySelector<HTMLInputElement>('input[aria-label="API key"]')
    assert.ok(endpointInput, 'Endpoint input must render')
    assert.ok(apiKeyInput, 'API key input must render')

    await setInputValue(endpointInput, MODEL_ENDPOINT)
    await setInputValue(apiKeyInput, 'sk-test')

    const fetchButton = findButton('Fetch Models')
    assert.ok(fetchButton, 'Fetch Models button must render')
    await view.click(fetchButton)
    await flush()

    let text = document.body.textContent || ''
    assert.match(text, /Available Models \(3\)/)
    assert.match(text, /Gemini 4 Argon/)
    assert.match(text, /Claude 3.7 Sonnet/)
    assert.match(text, /DeepSeek V3/)

    const modelInput = document.querySelector<HTMLInputElement>(
      'input[placeholder="e.g. gemini-4-argon"]',
    )
    assert.ok(modelInput, 'Model identifier input must render')
    await setInputValue(modelInput, 'claude')

    text = document.body.textContent || ''
    assert.match(text, /Available Models \(1 of 3\)/)
    assert.match(text, /Claude 3.7 Sonnet/)
    assert.doesNotMatch(text, /Gemini 4 Argon/)
    assert.doesNotMatch(text, /DeepSeek V3/)

    await pressEscape(modelInput)
    assert.doesNotMatch(document.body.textContent || '', /Available Models \(/)

    const providerRequest = requests.find((request) => request.body.includes(MODEL_ENDPOINT))
    assert.ok(providerRequest, 'provider model request must be sent')
    assert.match(providerRequest.body, /models\.example\.test/)
    assert.match(providerRequest.body, /sk-test/)
  })
})

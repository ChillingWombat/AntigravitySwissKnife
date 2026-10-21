import { afterEach, describe, it } from 'node:test'
import assert from 'node:assert/strict'
import React, { act } from 'react'
import { cancelAllInFlightRequests } from '../api'
import { CustomModelsPage } from '../pages/CustomModelsPage'
import { initDOM, mount, type RenderResult } from '../test-dom'

const makeModel = (id: string, displayName: string) => ({
  id,
  name: id,
  display_name: displayName,
  provider_type: 'openai',
  base_url: 'https://opencode.ai/zen/go',
  api_key: '[REDACTED]',
  project_mappings: ['*'],
  quota_type: 'na',
  prepaid_balance: 0,
  total_budget: 0,
  quota_fraction: null,
  is_default: false,
  enabled: true,
  context_window: 1000000,
})

const makeConfig = (...models: ReturnType<typeof makeModel>[]) => ({
  version: '1.0.0',
  inference_supported: true,
  models,
  project_binds: {},
})

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

const findEditButton = (view: RenderResult, modelName: string) => {
  const card = view
    .querySelectorAll<HTMLElement>('.google-card')
    .find((candidate) => candidate.textContent?.includes(modelName))
  return Array.from(card?.querySelectorAll('button') || []).find(
    (button) => button.textContent?.trim() === 'Edit'
  )
}

const apiKeyInput = (view: RenderResult) =>
  view.querySelector<HTMLInputElement>('input[placeholder="sk-..."]')

describe('Custom model edit API key hydration', () => {
  const originalFetch = globalThis.fetch
  let view: RenderResult | null = null

  afterEach(() => {
    view?.unmount()
    view = null
    globalThis.fetch = originalFetch
    cancelAllInFlightRequests()
  })

  it('loads the saved unredacted key into the edit modal and preserves the reveal control', async () => {
    initDOM()
    globalThis.fetch = (async (url: any) => {
      const target = String(url)
      if (target.startsWith('/api/custom_models/key?')) {
        return jsonResponse({ success: true, id: 'deepseek-model', api_key: 'sk-live-saved-key' })
      }
      if (target === '/api/custom_models') {
        return jsonResponse(makeConfig(makeModel('deepseek-model', 'DeepSeek V4.1 Flash')))
      }
      if (target === '/api/rules') {
        return jsonResponse({ default_custom_model: '' })
      }
      return jsonResponse({})
    }) as typeof fetch

    view = await mount(React.createElement(CustomModelsPage))
    await flush()

    const editButton = findEditButton(view, 'DeepSeek V4.1 Flash')
    assert.ok(editButton, 'DeepSeek model Edit button must render')
    await view.click(editButton)
    await flush()

    const input = apiKeyInput(view)
    assert.ok(input, 'API Key input must render in the edit modal')
    assert.equal(input?.value, 'sk-live-saved-key', 'edit modal must receive the saved key instead of [REDACTED]')
    assert.equal(input?.type, 'password', 'saved key must remain masked until explicitly revealed')

    const revealButton = input?.parentElement?.querySelector('button')
    assert.ok(revealButton, 'API key reveal button must render')
    await view.click(revealButton)
    await flush()

    assert.equal(apiKeyInput(view)?.type, 'text', 'reveal control must still expose the hydrated key')
  })

  it('ignores an older key response after the user switches models', async () => {
    initDOM()
    let resolveFirst!: (response: Response) => void
    let resolveSecond!: (response: Response) => void
    const firstKey = new Promise<Response>((resolve) => {
      resolveFirst = resolve
    })
    const secondKey = new Promise<Response>((resolve) => {
      resolveSecond = resolve
    })

    globalThis.fetch = (async (url: any) => {
      const target = String(url)
      if (target.includes('/api/custom_models/key?id=first-model')) return firstKey
      if (target.includes('/api/custom_models/key?id=second-model')) return secondKey
      if (target === '/api/custom_models') {
        return jsonResponse(
          makeConfig(
            makeModel('first-model', 'First Model'),
            makeModel('second-model', 'Second Model')
          )
        )
      }
      if (target === '/api/rules') {
        return jsonResponse({ default_custom_model: '' })
      }
      return jsonResponse({})
    }) as typeof fetch

    view = await mount(React.createElement(CustomModelsPage))
    await flush()

    const firstEdit = findEditButton(view, 'First Model')
    assert.ok(firstEdit, 'First model Edit button must render')
    await view.click(firstEdit)
    await flush()

    const secondEdit = findEditButton(view, 'Second Model')
    assert.ok(secondEdit, 'Second model Edit button must render')
    await view.click(secondEdit)
    await flush()

    await act(async () => {
      resolveSecond(jsonResponse({ success: true, id: 'second-model', api_key: 'sk-second-model' }))
      await secondKey
    })
    await flush()
    assert.equal(apiKeyInput(view)?.value, 'sk-second-model')

    await act(async () => {
      resolveFirst(jsonResponse({ success: true, id: 'first-model', api_key: 'sk-first-model' }))
      await firstKey
    })
    await flush()

    assert.equal(
      apiKeyInput(view)?.value,
      'sk-second-model',
      'an older response must not overwrite the currently edited model'
    )
  })
})

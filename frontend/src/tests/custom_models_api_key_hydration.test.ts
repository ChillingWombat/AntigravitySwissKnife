import { afterEach, describe, it } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
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
  view.querySelector<HTMLInputElement>('input[placeholder="sk-..."]') ??
  document.querySelector<HTMLInputElement>('input[placeholder="sk-..."]')

describe('Custom model edit key handling', () => {
  const originalFetch = globalThis.fetch
  let view: RenderResult | null = null

  afterEach(() => {
    view?.unmount()
    view = null
    globalThis.fetch = originalFetch
    cancelAllInFlightRequests()
  })

  it('opens edit mode without requesting the saved API key', async () => {
    initDOM()
    const requested: string[] = []
    globalThis.fetch = (async (url: any) => {
      const target = String(url)
      requested.push(target)
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

    assert.equal(apiKeyInput(view)?.value, '', 'saved keys must stay out of the renderer')
    assert.equal(
      requested.some((target) => target.startsWith('/api/custom_models/key')),
      false,
      'edit mode must not request the sensitive key endpoint'
    )
  })

  it('passes the edited model id to fetchModels so the daemon can hydrate the key', () => {
    const pageSource = fs.readFileSync(
      path.resolve(import.meta.dirname, '..', 'pages', 'CustomModelsPage.tsx'),
      'utf8'
    )
    assert.ok(
      pageSource.includes('api.fetchModels(effectiveProvider, baseUrl.trim(), apiKey.trim(), editingModel?.id)'),
      'model discovery must pass the saved model id for server-side key hydration'
    )
    assert.ok(
      pageSource.includes('resolveFormProviderType(baseUrl, editingModel)'),
      'edit mode must preserve the stored provider when the endpoint is unchanged'
    )
    assert.ok(
      !pageSource.includes('const inferred = inferProviderType(baseUrl.trim())'),
      'edit mode must not overwrite the stored provider from URL heuristics'
    )
    assert.ok(
      !pageSource.includes('api.getCustomModelKey('),
      'custom model edit flows must not fetch unredacted credentials'
    )
  })
})

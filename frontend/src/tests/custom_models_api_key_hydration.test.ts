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

function createDeferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((resolvePromise) => {
    resolve = resolvePromise
  })
  return { promise, resolve }
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

  it('hydrates the saved API key only for the selected editor', async () => {
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
      if (target.startsWith('/api/custom_models/key?')) {
        const parsed = new URL(target, 'http://localhost')
        assert.equal(parsed.searchParams.get('id'), 'deepseek-model')
        assert.equal(parsed.searchParams.get('field'), 'api_key')
        return jsonResponse({ id: 'deepseek-model', field: 'api_key', value: 'test-saved-api-key' })
      }
      return jsonResponse({})
    }) as typeof fetch

    view = await mount(React.createElement(CustomModelsPage))
    await flush()
    assert.equal(
      requested.some((target) => target.startsWith('/api/custom_models/key')),
      false,
      'loading the model list must not fetch credentials'
    )

    const editButton = findEditButton(view, 'DeepSeek V4.1 Flash')
    assert.ok(editButton, 'DeepSeek model Edit button must render')
    await view.click(editButton)
    await flush()

    assert.equal(apiKeyInput(view)?.value, 'test-saved-api-key', 'selected API-key editor must show the saved key')
    assert.equal(
      requested.filter((target) => target.startsWith('/api/custom_models/key')).length,
      1,
      'opening one editor must make one selected-model credential request'
    )
  })

  it('keeps model B credentials when model A hydration resolves late', async () => {
    initDOM()
    const firstHydration = createDeferred<Response>()
    const secondHydration = createDeferred<Response>()
    const requestedModels: string[] = []
    globalThis.fetch = (async (url: any) => {
      const target = String(url)
      if (target === '/api/custom_models') {
        return jsonResponse(
          makeConfig(
            makeModel('first-api-model', 'First API Model'),
            makeModel('second-api-model', 'Second API Model')
          )
        )
      }
      if (target === '/api/rules') {
        return jsonResponse({ default_custom_model: '' })
      }
      if (target.startsWith('/api/custom_models/key?')) {
        const parsed = new URL(target, 'http://localhost')
        const id = parsed.searchParams.get('id')
        assert.equal(parsed.searchParams.get('field'), 'api_key')
        assert.ok(id, 'credential request must identify its model')
        requestedModels.push(id)
        if (id === 'first-api-model') return firstHydration.promise
        assert.equal(id, 'second-api-model')
        return secondHydration.promise
      }
      return jsonResponse({})
    }) as typeof fetch

    view = await mount(React.createElement(CustomModelsPage))
    await flush()
    const firstEditButton = findEditButton(view, 'First API Model')
    assert.ok(firstEditButton, 'first model Edit button must render')
    await view.click(firstEditButton)
    await flush()
    assert.deepEqual(requestedModels, ['first-api-model'], 'opening model A must start one pending hydration')

    const cancelButton = Array.from(document.querySelectorAll<HTMLButtonElement>('button')).find(
      (button) => button.textContent?.trim() === 'Cancel'
    )
    assert.ok(cancelButton, 'cancel button must render')
    await view.click(cancelButton)
    await flush()
    assert.equal(apiKeyInput(view), null, 'closing model A must clear its pending editor')

    const secondEditButton = findEditButton(view, 'Second API Model')
    assert.ok(secondEditButton, 'second model Edit button must render while model A hydration is pending')
    await view.click(secondEditButton)
    await flush()
    assert.deepEqual(
      requestedModels,
      ['first-api-model', 'second-api-model'],
      'switching to model B must start its own hydration before model A resolves'
    )
    assert.equal(apiKeyInput(view)?.value, '', 'model B must not display model A credentials')

    secondHydration.resolve(
      jsonResponse({ id: 'second-api-model', field: 'api_key', value: 'second-model-saved-key' })
    )
    await flush()
    assert.equal(apiKeyInput(view)?.value, 'second-model-saved-key', 'model B must display its own key')

    firstHydration.resolve(jsonResponse({ id: 'first-api-model', field: 'api_key', value: 'first-model-saved-key' }))
    await flush()
    assert.equal(
      apiKeyInput(view)?.value,
      'second-model-saved-key',
      'a late response from model A must not replace model B credentials'
    )
  })

  it('discards a superseded hydration after closing and reopening the editor', async () => {
    initDOM()
    const supersededHydration = createDeferred<Response>()
    const reopenedHydration = createDeferred<Response>()
    let hydrationRequests = 0
    globalThis.fetch = (async (url: any) => {
      const target = String(url)
      if (target === '/api/custom_models') {
        return jsonResponse(makeConfig(makeModel('first-api-model', 'First API Model')))
      }
      if (target === '/api/rules') {
        return jsonResponse({ default_custom_model: '' })
      }
      if (target.startsWith('/api/custom_models/key?')) {
        const parsed = new URL(target, 'http://localhost')
        assert.equal(parsed.searchParams.get('id'), 'first-api-model')
        assert.equal(parsed.searchParams.get('field'), 'api_key')
        const hydration = hydrationRequests === 0 ? supersededHydration : reopenedHydration
        if (hydrationRequests > 1) throw new Error('unexpected extra hydration request')
        hydrationRequests += 1
        return hydration.promise
      }
      return jsonResponse({})
    }) as typeof fetch

    view = await mount(React.createElement(CustomModelsPage))
    await flush()
    const firstEditButton = findEditButton(view, 'First API Model')
    assert.ok(firstEditButton, 'model Edit button must render')
    await view.click(firstEditButton)
    await flush()
    assert.equal(hydrationRequests, 1, 'opening model A must leave its first hydration pending')

    const cancelButton = Array.from(document.querySelectorAll<HTMLButtonElement>('button')).find(
      (button) => button.textContent?.trim() === 'Cancel'
    )
    assert.ok(cancelButton, 'cancel button must render')
    await view.click(cancelButton)
    await flush()
    assert.equal(apiKeyInput(view), null, 'closing model A must clear the editor')

    const reopenedEditButton = findEditButton(view, 'First API Model')
    assert.ok(reopenedEditButton, 'model Edit button must render after close')
    await view.click(reopenedEditButton)
    await flush()
    assert.equal(hydrationRequests, 2, 'reopening model A must start a fresh hydration')
    assert.equal(apiKeyInput(view)?.value, '', 'reopened model A must start with an empty field')

    supersededHydration.resolve(
      jsonResponse({ id: 'first-api-model', field: 'api_key', value: 'superseded-model-key' })
    )
    await flush()
    assert.equal(
      apiKeyInput(view)?.value,
      '',
      'a cancelled hydration must not populate the reopened editor'
    )

    reopenedHydration.resolve(
      jsonResponse({ id: 'first-api-model', field: 'api_key', value: 'reopened-model-key' })
    )
    await flush()
    assert.equal(apiKeyInput(view)?.value, 'reopened-model-key', 'the reopened editor must use its own hydration')
  })

  it('clears stale API credentials when switching editors before hydration settles', async () => {
    initDOM()
    let resolveSecondHydration!: (response: Response) => void
    const secondHydration = new Promise<Response>((resolve) => {
      resolveSecondHydration = resolve
    })
    globalThis.fetch = (async (url: any) => {
      const target = String(url)
      if (target === '/api/custom_models') {
        return jsonResponse(
          makeConfig(
            makeModel('first-api-model', 'First API Model'),
            makeModel('second-api-model', 'Second API Model')
          )
        )
      }
      if (target === '/api/rules') {
        return jsonResponse({ default_custom_model: '' })
      }
      if (target.startsWith('/api/custom_models/key?')) {
        const parsed = new URL(target, 'http://localhost')
        const id = parsed.searchParams.get('id')
        assert.equal(parsed.searchParams.get('field'), 'api_key')
        if (id === 'first-api-model') {
          return jsonResponse({ id, field: 'api_key', value: 'first-model-saved-key' })
        }
        assert.equal(id, 'second-api-model')
        return secondHydration
      }
      return jsonResponse({})
    }) as typeof fetch

    view = await mount(React.createElement(CustomModelsPage))
    await flush()
    const firstEditButton = findEditButton(view, 'First API Model')
    assert.ok(firstEditButton, 'first model Edit button must render')
    await view.click(firstEditButton)
    await flush()
    assert.equal(apiKeyInput(view)?.value, 'first-model-saved-key')

    const cancelButton = Array.from(document.querySelectorAll<HTMLButtonElement>('button')).find(
      (button) => button.textContent?.trim() === 'Cancel'
    )
    assert.ok(cancelButton, 'cancel button must render')
    await view.click(cancelButton)
    await flush()

    const secondEditButton = findEditButton(view, 'Second API Model')
    assert.ok(secondEditButton, 'second model Edit button must render')
    await view.click(secondEditButton)
    await flush()
    assert.equal(
      apiKeyInput(view)?.value,
      '',
      'the previous model key must be cleared while the next hydration is still pending'
    )

    resolveSecondHydration(jsonResponse({ id: 'second-api-model', field: 'api_key', value: 'second-model-saved-key' }))
    await flush()
    assert.equal(apiKeyInput(view)?.value, 'second-model-saved-key')
  })

  it('clears credential state on close and reports hydration failures', async () => {
    initDOM()
    let keyRequests = 0
    globalThis.fetch = (async (url: any) => {
      const target = String(url)
      if (target === '/api/custom_models') {
        return jsonResponse(makeConfig(makeModel('deepseek-model', 'DeepSeek V4.1 Flash')))
      }
      if (target === '/api/rules') {
        return jsonResponse({ default_custom_model: '' })
      }
      if (target.startsWith('/api/custom_models/key?')) {
        keyRequests += 1
        if (keyRequests === 1) {
          return jsonResponse({ id: 'deepseek-model', field: 'api_key', value: 'test-saved-api-key' })
        }
        return {
          ok: false,
          status: 503,
          statusText: 'Service Unavailable',
          json: async () => ({ error: 'key store unavailable' }),
          text: async () => '{"error":"key store unavailable"}',
        } as Response
      }
      return jsonResponse({})
    }) as typeof fetch

    view = await mount(React.createElement(CustomModelsPage))
    await flush()
    const editButton = findEditButton(view, 'DeepSeek V4.1 Flash')
    assert.ok(editButton, 'DeepSeek model Edit button must render')
    await view.click(editButton)
    await flush()
    assert.equal(apiKeyInput(view)?.value, 'test-saved-api-key')

    const cancelButton = Array.from(document.querySelectorAll<HTMLButtonElement>('button')).find(
      (button) => button.textContent?.trim() === 'Cancel'
    )
    assert.ok(cancelButton, 'cancel button must render')
    await view.click(cancelButton)
    await flush()
    assert.equal(apiKeyInput(view), null, 'closing the dialog must discard the credential')

    const reopenedEditButton = findEditButton(view, 'DeepSeek V4.1 Flash')
    assert.ok(reopenedEditButton, 'DeepSeek model Edit button must render after close')
    await view.click(reopenedEditButton)
    await flush()
    assert.equal(keyRequests, 2, 'reopening the editor must hydrate again')
    assert.equal(apiKeyInput(view)?.value, '', 'failed hydration must not leave a stale or placeholder key')
    assert.match(document.body.textContent || '', /Saved API key could not be loaded/)
  })

  it('pretty-prints hydrated GCP service-account JSON and keeps malformed credentials visible', async () => {
    initDOM()
    const compact = '{"type":"service_account","project_id":"test-proj","private_key":"test-key","client_email":"test@example.com"}'
    let returnMalformed = false
    globalThis.fetch = (async (url: any) => {
      const target = String(url)
      if (target === '/api/custom_models') {
        return jsonResponse(
          makeConfig({
            ...makeModel('gcp-model', 'GCP Test Model'),
            provider_type: 'gcp',
            base_url: 'https://aiplatform.googleapis.com/v1/projects/test-proj/locations/global/publishers/google/models/gemini-4-argon:generateContent',
          })
        )
      }
      if (target === '/api/rules') {
        return jsonResponse({ default_custom_model: '' })
      }
      if (target.startsWith('/api/custom_models/key?')) {
        const parsed = new URL(target, 'http://localhost')
        assert.equal(parsed.searchParams.get('id'), 'gcp-model')
        assert.equal(parsed.searchParams.get('field'), 'gcp_service_account_key')
        return jsonResponse({
          id: 'gcp-model',
          field: 'gcp_service_account_key',
          value: returnMalformed ? '{"type":"service_account"' : compact,
        })
      }
      return jsonResponse({})
    }) as typeof fetch

    view = await mount(React.createElement(CustomModelsPage))
    await flush()
    const editButton = findEditButton(view, 'GCP Test Model')
    assert.ok(editButton, 'GCP model Edit button must render')
    await view.click(editButton)
    await flush()
    let textarea = document.querySelector<HTMLTextAreaElement>('#gcp-service-account-json')
    const expectedPretty = [
      '{',
      '  "type": "service_account",',
      '  "project_id": "test-proj",',
      '  "private_key": "test-key",',
      '  "client_email": "test@example.com"',
      '}',
    ].join('\n')
    assert.equal(textarea?.value, expectedPretty, 'saved GCP JSON must be multiline pretty-printed JSON')
    assert.match(textarea?.value || '', /\n  "project_id": "test-proj"/)
    assert.notEqual(textarea?.value, compact, 'the hydrated editor must not retain the compact raw JSON')

    const cancelButton = Array.from(document.querySelectorAll<HTMLButtonElement>('button')).find(
      (button) => button.textContent?.trim() === 'Cancel'
    )
    assert.ok(cancelButton, 'cancel button must render')
    await view.click(cancelButton)
    await flush()
    returnMalformed = true
    const reopenedEditButton = findEditButton(view, 'GCP Test Model')
    assert.ok(reopenedEditButton, 'GCP model Edit button must render after close')
    await view.click(reopenedEditButton)
    await flush()
    textarea = document.querySelector<HTMLTextAreaElement>('#gcp-service-account-json')
    assert.equal(textarea?.value, '{"type":"service_account"', 'malformed saved JSON must remain visible')
    assert.match(document.body.textContent || '', /Saved service-account key is not valid JSON/)
  })

  it('passes the edited model id to fetchModels while hydrating the selected credential', () => {
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
      pageSource.includes("api.getCustomModelKey(editingModel.id, 'api_key')"),
      'selected API-key edit flow must fetch only its credential field'
    )
  })
})

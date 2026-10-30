import { afterEach, describe, it } from 'node:test'
import assert from 'node:assert/strict'
import React, { act } from 'react'
import { cancelAllInFlightRequests } from '../../api'
import { CustomModelsPage } from '../../pages/CustomModelsPage'
import { mount, type RenderResult } from '../../test-dom'

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

const findButton = (scope: ParentNode, label: string) =>
  Array.from(scope.querySelectorAll<HTMLButtonElement>('button')).find(
    (button) => button.textContent?.trim() === label
  )

const findEditButton = (view: RenderResult, modelName: string) => {
  const card = view
    .querySelectorAll<HTMLElement>('.google-card')
    .find((candidate) => candidate.textContent?.includes(modelName))
  return Array.from(card?.querySelectorAll('button') || []).find(
    (button) => button.textContent?.trim() === 'Edit'
  )
}

const dialogName = (dialog: Element) => {
  const titleId = dialog.getAttribute('aria-labelledby')
  return titleId ? document.getElementById(titleId)?.textContent?.trim() : undefined
}

const controlName = (control: HTMLElement) => {
  const ariaLabel = control.getAttribute('aria-label')?.trim()
  if (ariaLabel) return ariaLabel

  const labelledBy = control.getAttribute('aria-labelledby')
  if (labelledBy) {
    return labelledBy
      .split(/\s+/)
      .map((id) => document.getElementById(id)?.textContent?.trim())
      .filter(Boolean)
      .join(' ')
  }

  if (!control.id) return undefined
  return document.querySelector(`label[for="${control.id}"]`)?.textContent?.replace(/\s+/g, ' ').trim()
}

const findDialog = (name: string) =>
  Array.from(document.querySelectorAll('dialog')).find((dialog) => dialogName(dialog) === name)

const pressEscape = async (dialog: Element) => {
  await act(async () => {
    dialog.dispatchEvent(
      new window.KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true })
    )
  })
  await flush()
}

describe('Custom model dialog accessibility', () => {
  const originalFetch = globalThis.fetch
  let view: RenderResult | null = null

  afterEach(() => {
    view?.unmount()
    view = null
    globalThis.fetch = originalFetch
    document.body.style.overflow = ''
    cancelAllInFlightRequests()
  })

  const mountPage = async (...models: ReturnType<typeof makeModel>[]) => {
    globalThis.fetch = (async (url: any) => {
      const target = String(url)
      if (target === '/api/custom_models') return jsonResponse(makeConfig(...models))
      if (target === '/api/rules') return jsonResponse({ default_custom_model: '' })
      return jsonResponse({})
    }) as typeof fetch

    view = await mount(React.createElement(CustomModelsPage))
    await flush()
    return view
  }

  it('names the add dialog and its icon-only controls, then restores on Escape', async () => {
    const page: RenderResult = await mountPage()
    const opener = findButton(page.container, 'Add Custom Model')
    assert.ok(opener)
    document.body.style.overflow = 'clip'
    opener.focus()
    await page.click(opener)
    await flush()

    const dialog = findDialog('Add Custom Model Provider')
    assert.ok(dialog, 'add dialog must have an accessible name')
    assert.equal(dialog.getAttribute('role'), 'dialog')
    assert.equal(dialog.getAttribute('aria-modal'), 'true')
    assert.equal(document.body.style.overflow, 'hidden')

    assert.ok(dialog.querySelector('button[aria-label="Close custom model dialog"]'))
    const apiKeyInput = dialog.querySelector<HTMLInputElement>('input[aria-label="API key"]')
    assert.ok(apiKeyInput)
    const reveal = dialog.querySelector<HTMLButtonElement>('button[aria-label="Show API key"]')
    assert.ok(reveal)
    await page.click(reveal)

    assert.equal(apiKeyInput.type, 'text')
    assert.ok(dialog.querySelector('button[aria-label="Hide API key"]'))

    await pressEscape(dialog)
    assert.equal(findDialog('Add Custom Model Provider'), undefined)
    assert.equal(document.body.style.overflow, 'clip')
    assert.equal(document.activeElement, opener)
  })

  it('names the GCP dialog and its icon-only close control', async () => {
    const page: RenderResult = await mountPage()
    const opener = findButton(page.container, 'Add GCP Models')
    assert.ok(opener)
    opener.focus()
    await page.click(opener)
    await flush()

    const dialog = findDialog('Add GCP Vertex AI Model')
    assert.ok(dialog, 'GCP dialog must have an accessible name')
    assert.equal(dialog.getAttribute('role'), 'dialog')
    assert.equal(dialog.getAttribute('aria-modal'), 'true')
    assert.ok(dialog.querySelector('button[aria-label="Close GCP model dialog"]'))

    const serviceAccountJson = dialog.querySelector<HTMLTextAreaElement>('#gcp-service-account-json')
    assert.ok(serviceAccountJson, 'GCP service account JSON textarea must have a stable id')
    assert.ok(
      dialog.querySelector('label[for="gcp-service-account-json"]'),
      'GCP service account JSON label must reference the textarea'
    )
    assert.equal(controlName(serviceAccountJson), 'Service Account JSON Key:')

    await pressEscape(dialog)
    assert.equal(findDialog('Add GCP Vertex AI Model'), undefined)
    assert.equal(document.activeElement, opener)
  })

  it('wraps the delete confirmation with the same modal semantics', async () => {
    const page: RenderResult = await mountPage(makeModel('delete-model', 'Delete Me'))
    const edit = findEditButton(page, 'Delete Me')
    assert.ok(edit)
    await page.click(edit)
    await flush()

    const editDialog = findDialog('Edit Custom Model')
    assert.ok(editDialog)
    const deleteButton = findButton(editDialog, 'Delete Model')
    assert.ok(deleteButton)
    await page.click(deleteButton)
    await flush()

    const deleteDialog = findDialog('Delete Custom Model')
    assert.ok(deleteDialog, 'delete dialog must have an accessible name')
    assert.equal(deleteDialog.getAttribute('role'), 'dialog')
    assert.equal(deleteDialog.getAttribute('aria-modal'), 'true')

    const cancel = findButton(deleteDialog, 'Cancel')
    assert.ok(cancel)
    await page.click(cancel)
    await flush()
    assert.equal(findDialog('Delete Custom Model'), undefined)
    assert.ok(findDialog('Edit Custom Model'), 'closing the delete dialog must leave the edit dialog open')
  })
})

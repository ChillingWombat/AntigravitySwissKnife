import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import React, { act } from 'react'
import { cancelAllInFlightRequests } from '../../api'
import { CustomModelsPage } from '../CustomModelsPage'
import { mount, type RenderResult } from '../../test-dom'
import type { CustomModel } from '../../types'
import {
  dialogStateReducer,
  initialDialogState,
  type ModelFormDialogState,
} from './useDialogState'

const model = {
  id: 'delete-model',
  name: 'delete-model',
  display_name: 'Delete Me',
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
} as CustomModel

const editorState: ModelFormDialogState = {
  type: 'modelForm',
  mode: 'edit',
  initialModel: model,
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

const findButton = (scope: ParentNode, label: string) =>
  Array.from(scope.querySelectorAll<HTMLButtonElement>('button')).find(
    (button) => button.textContent?.trim() === label
  )

const dialogName = (dialog: Element) => {
  const titleId = dialog.getAttribute('aria-labelledby')
  return titleId ? document.getElementById(titleId)?.textContent?.trim() : undefined
}

const findDialog = (name: string) =>
  Array.from(document.querySelectorAll('dialog')).find((dialog) => dialogName(dialog) === name)

describe('custom model dialog state', () => {
  it('opens model, GCP, and report dialogs and closes to a single neutral state', () => {
    const modelForm = dialogStateReducer(initialDialogState, {
      type: 'openModelForm',
      mode: 'add',
      initialModel: null,
    })
    assert.deepEqual(modelForm, { type: 'modelForm', mode: 'add', initialModel: null })

    const gcpForm = dialogStateReducer(modelForm, {
      type: 'openGcp',
      mode: 'edit',
      initialModel: model,
    })
    assert.deepEqual(gcpForm, { type: 'gcpForm', mode: 'edit', initialModel: model })

    const report = dialogStateReducer(gcpForm, { type: 'openReport' })
    assert.deepEqual(report, { type: 'report' })
    assert.deepEqual(dialogStateReducer(report, { type: 'close' }), { type: 'closed' })
  })

  it('delete from editor returns to the exact editor state', () => {
    const deleting = dialogStateReducer(editorState, {
      type: 'openDelete',
      model: { id: model.id, name: model.display_name },
    })
    assert.equal(deleting.type, 'delete')
    if (deleting.type !== 'delete') throw new Error('expected delete state')
    assert.equal(deleting.returnTo, editorState)

    assert.equal(dialogStateReducer(deleting, { type: 'backToModelForm' }), editorState)
    assert.deepEqual(dialogStateReducer(deleting, { type: 'close' }), { type: 'closed' })
  })

  it('characterizes delete-from-editor canceling back to the editor', async () => {
    const originalFetch = globalThis.fetch
    let view: RenderResult | null = null
    globalThis.fetch = (async (url: any) => {
      const target = String(url)
      if (target === '/api/custom_models') {
        return jsonResponse({
          version: '1.0.0',
          inference_supported: true,
          models: [model],
        })
      }
      if (target === '/api/rules') return jsonResponse({ default_custom_model: '' })
      return jsonResponse({})
    }) as typeof fetch

    try {
      view = await mount(React.createElement(CustomModelsPage))
      await flush()

      const card = view
        .querySelectorAll<HTMLElement>('.google-card')
        .find((candidate) => candidate.textContent?.includes(model.display_name))
      const edit = Array.from(card?.querySelectorAll('button') || []).find(
        (button) => button.textContent?.trim() === 'Edit'
      )
      assert.ok(edit)
      await view.click(edit)
      await flush()

      const editDialog = findDialog('Edit Custom Model')
      assert.ok(editDialog)
      const deleteButton = findButton(editDialog, 'Delete Model')
      assert.ok(deleteButton)
      await view.click(deleteButton)
      await flush()
      assert.ok(findDialog('Delete Custom Model'))

      const deleteDialog = findDialog('Delete Custom Model')
      assert.ok(deleteDialog)
      await view.click(findButton(deleteDialog, 'Cancel'))
      await flush()

      assert.equal(findDialog('Delete Custom Model'), undefined)
      assert.ok(findDialog('Edit Custom Model'))
    } finally {
      view?.unmount()
      globalThis.fetch = originalFetch
      document.body.style.overflow = ''
      cancelAllInFlightRequests()
    }
  })
})

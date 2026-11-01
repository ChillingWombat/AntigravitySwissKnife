import { afterEach, describe, it, mock } from 'node:test'
import assert from 'node:assert/strict'
import React, { act } from 'react'
import { api } from '../api'
import { AccountDetailModal } from '../components/AccountDetailModal'
import type { AccountState } from '../types'
import { initDOM, mount, type RenderResult } from '../test-dom'
import { ACCOUNT_SETUP_TEXTS } from '../utils/accountPresentation'

const flush = async () => {
  for (let index = 0; index < 4; index += 1) {
    await act(async () => {
      await Promise.resolve()
    })
  }
}

function createDeferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((resolvePromise) => {
    resolve = resolvePromise
  })
  return { promise, resolve }
}

const credentialInput = (view: RenderResult, labelText: string) => {
  const label = Array.from(view.querySelectorAll<HTMLLabelElement>('label')).find((candidate) =>
    candidate.textContent?.includes(labelText)
  )
  const input = label?.parentElement?.querySelector<HTMLInputElement>('input')
  assert.ok(input, `${labelText} input must render`)
  return input
}

describe('AccountDetailModal selected credential detail refresh', () => {
  const originalFetch = globalThis.fetch
  let view: RenderResult | null = null

  afterEach(() => {
    view?.unmount()
    view = null
    globalThis.fetch = originalFetch
    mock.restoreAll()
  })

  it('hydrates password, TOTP, and refresh token without unredacting the account list row', async () => {
    initDOM()
    const listAccount = {
      email: 'selected@example.com',
      label: 'Selected Account',
      priority: 'High',
      status: 'ACTIVE',
      is_active: true,
      has_mfa: true,
      password: '[REDACTED]',
      totp_secret: '[REDACTED]',
      refresh_token: '[REDACTED]',
      access_token: '[REDACTED]',
    } as AccountState
    const detail = createDeferred<any[]>()
    const exportAccounts = mock.method(api, 'exportAccounts', () => detail.promise)

    view = await mount(
      React.createElement(AccountDetailModal, {
        account: listAccount,
        onClose: () => {},
        onSaved: () => {},
      })
    )
    await flush()

    assert.equal(exportAccounts.mock.callCount(), 1)
    assert.deepEqual(exportAccounts.mock.calls[0].arguments, ['selected@example.com'])
    assert.equal(credentialInput(view, ACCOUNT_SETUP_TEXTS.PASSWORD_LABEL).value, '[REDACTED]')
    assert.equal(credentialInput(view, ACCOUNT_SETUP_TEXTS.OAUTH_REFRESH_LABEL).value, '[REDACTED]')
    assert.equal(credentialInput(view, 'MFA Secret Key:').value, '[REDACTED]')

    detail.resolve([
      {
        email: 'other@example.com',
        password: 'other-password',
        totp_secret: 'JBSWY3DPEHPK3PXQ',
        refresh_token: '1//other-refresh',
      },
      {
        email: 'selected@example.com',
        password: 'selected-password',
        totp_secret: 'JBSWY3DPEHPK3PXP',
        refresh_token: '1//selected-refresh',
      },
    ])
    await flush()

    assert.equal(credentialInput(view, ACCOUNT_SETUP_TEXTS.PASSWORD_LABEL).value, 'selected-password')
    assert.equal(credentialInput(view, 'MFA Secret Key:').value, 'JBSWY3DPEHPK3PXP')
    assert.equal(
      credentialInput(view, ACCOUNT_SETUP_TEXTS.OAUTH_REFRESH_LABEL).value,
      '1//selected-refresh'
    )
    assert.deepEqual(
      {
        password: listAccount.password,
        totp_secret: listAccount.totp_secret,
        refresh_token: listAccount.refresh_token,
      },
      {
        password: '[REDACTED]',
        totp_secret: '[REDACTED]',
        refresh_token: '[REDACTED]',
      },
      'the general account list row must remain redacted'
    )
  })
})

import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import React, { act, useState } from 'react'
import { ModalDialog } from '../ModalDialog'
import { mount, type RenderResult } from '../../../test-dom'

const flush = async () => {
  await act(async () => {
    await Promise.resolve()
  })
}

const pressKey = async (target: Element, init: KeyboardEventInit) => {
  const event = new window.KeyboardEvent('keydown', { bubbles: true, cancelable: true, ...init })
  await act(async () => {
    target.dispatchEvent(event)
  })
  return event
}

const accessibleName = (dialog: Element) => {
  const titleId = dialog.getAttribute('aria-labelledby')
  return titleId ? document.getElementById(titleId)?.textContent?.trim() : undefined
}

describe('ModalDialog', () => {
  it('wires the accessible name, description, initial focus, and native open state', async () => {
    const initialFocusRef = React.createRef<HTMLButtonElement>()
    let view: RenderResult | null = null

    try {
      view = await mount(
        React.createElement(
          ModalDialog,
          {
            open: true,
            title: 'Account settings',
            description: 'Update account preferences.',
            onClose: () => {},
            initialFocusRef,
            children: React.createElement(
              React.Fragment,
              null,
              React.createElement('button', { type: 'button' }, 'Cancel'),
              React.createElement('button', { type: 'button', ref: initialFocusRef }, 'Save')
            ),
          }
        )
      )

      const dialog = document.querySelector('dialog')
      assert.ok(dialog, 'dialog must render in the document portal')
      assert.equal(dialog.getAttribute('role'), 'dialog')
      assert.equal(dialog.getAttribute('aria-modal'), 'true')
      assert.equal(accessibleName(dialog), 'Account settings')

      const descriptionId = dialog.getAttribute('aria-describedby')
      assert.equal(
        descriptionId ? document.getElementById(descriptionId)?.textContent?.trim() : undefined,
        'Update account preferences.'
      )
      assert.equal(document.activeElement, initialFocusRef.current)
      assert.equal(dialog.hasAttribute('open'), true, 'fallback must expose the dialog when showModal is absent')
    } finally {
      view?.unmount()
    }
  })

  it('cycles Tab forward and backward through the dialog controls', async () => {
    let view: RenderResult | null = null

    try {
      view = await mount(
        React.createElement(
          ModalDialog,
          {
            open: true,
            title: 'Focus loop',
            onClose: () => {},
            children: React.createElement(
              React.Fragment,
              null,
              React.createElement('button', { type: 'button' }, 'First'),
              React.createElement('button', { type: 'button' }, 'Middle'),
              React.createElement('button', { type: 'button' }, 'Last')
            ),
          }
        )
      )

      const dialog = document.querySelector('dialog')
      assert.ok(dialog)
      const buttons = Array.from(dialog.querySelectorAll('button'))
      assert.equal(buttons.length, 3)

      buttons[2].focus()
      const forward = await pressKey(dialog, { key: 'Tab' })
      assert.equal(forward.defaultPrevented, true)
      assert.equal(document.activeElement, buttons[0])

      buttons[0].focus()
      const backward = await pressKey(dialog, { key: 'Tab', shiftKey: true })
      assert.equal(backward.defaultPrevented, true)
      assert.equal(document.activeElement, buttons[2])
    } finally {
      view?.unmount()
    }
  })

  it('closes once on Escape or cancel and restores focus and body overflow', async () => {
    let closeCalls = 0
    let view: RenderResult | null = null

    const Harness = () => {
      const [open, setOpen] = useState(false)
      return React.createElement(
        React.Fragment,
        null,
        React.createElement(
          'button',
          {
            id: 'modal-opener',
            type: 'button',
            onClick: () => setOpen(true),
          },
          'Open dialog'
        ),
        React.createElement(
          ModalDialog,
          {
            open,
            title: 'Guarded close',
            onClose: () => {
              closeCalls += 1
              setOpen(false)
            },
            children: React.createElement('button', { type: 'button' }, 'Inside'),
          }
        )
      )
    }

    try {
      view = await mount(React.createElement(Harness))
      const opener = document.querySelector<HTMLButtonElement>('#modal-opener')
      assert.ok(opener)
      document.body.style.overflow = 'clip'
      opener.focus()
      await view.click(opener)
      await flush()

      const dialog = document.querySelector('dialog')
      assert.ok(dialog)
      assert.equal(document.body.style.overflow, 'hidden')

      await act(async () => {
        dialog.dispatchEvent(new window.KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true }))
        dialog.dispatchEvent(new window.Event('cancel', { bubbles: true, cancelable: true }))
        dialog.dispatchEvent(new window.KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true }))
      })
      await flush()

      assert.equal(closeCalls, 1, 'Escape and cancel must share one guarded close')
      assert.equal(document.querySelector('dialog'), null)
      assert.equal(document.body.style.overflow, 'clip')
      assert.equal(document.activeElement, opener)
    } finally {
      view?.unmount()
      document.body.style.overflow = ''
    }
  })
})

import { afterEach, beforeEach, describe, it } from 'node:test'
import assert from 'node:assert/strict'
import React from 'react'
import { Window } from 'happy-dom'
import { useContextMenuPlacement } from './useContextMenuPlacement'

describe('useContextMenuPlacement', () => {
  let dom: Window

  beforeEach(() => {
    dom = new Window({ url: 'http://localhost' })
    Object.assign(globalThis, {
      window: dom,
      document: dom.document,
      HTMLElement: dom.HTMLElement,
      Node: dom.Node,
      getComputedStyle: dom.getComputedStyle.bind(dom),
    })
    Object.defineProperty(globalThis, 'navigator', {
      value: dom.navigator,
      configurable: true,
    })
  })

  afterEach(async () => {
    await dom.happyDOM.close()
  })

  it('repositions on capture-phase scroll and removes the listener on unmount', async () => {
    const { render } = await import('@testing-library/react')
    const originalAddEventListener = dom.addEventListener.bind(dom)
    const originalRemoveEventListener = dom.removeEventListener.bind(dom)
    let scrollListener: EventListenerOrEventListenerObject | null = null
    let removedListener: EventListenerOrEventListenerObject | null = null
    let capture: boolean | undefined
    let removedCapture: boolean | undefined

    dom.addEventListener = ((type: string, listener: any, options?: any) => {
      if (type === 'scroll') {
        scrollListener = listener
        capture = options
      }
      originalAddEventListener(type, listener, options)
    }) as typeof dom.addEventListener
    dom.removeEventListener = ((type: string, listener: any, options?: any) => {
      if (type === 'scroll') {
        removedListener = listener
        removedCapture = options
      }
      Reflect.apply(originalRemoveEventListener, dom, [type, listener, options])
    }) as typeof dom.removeEventListener

    function Menu() {
      const { menuRef, menuPosition } = useContextMenuPlacement(true, { x: 20, y: 30 })
      return React.createElement(
        'div',
        { ref: menuRef, 'data-testid': 'menu' },
        menuPosition ? `${menuPosition.top},${menuPosition.left}` : ''
      )
    }

    const view = render(React.createElement(Menu))
    try {
      assert.notStrictEqual(scrollListener, null)
      assert.strictEqual(capture, true)
    } finally {
      view.unmount()
    }

    assert.strictEqual(removedListener, scrollListener)
    assert.strictEqual(removedCapture, true)
  })
})

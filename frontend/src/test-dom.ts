/**
 * test-dom.ts — Virtual DOM Test Harness for React Component Verification
 *
 * This module sets up a lightweight browser-like DOM environment in Node.js
 * using `happy-dom`, enabling genuine React component mounting, state updates,
 * DOM inspection, and user interaction simulation without browser overhead.
 */

import { GlobalWindow } from 'happy-dom'
import React, { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'

let currentWindow: GlobalWindow | null = null

/**
 * Initializes the global window and document objects for DOM simulation.
 * Must be invoked prior to mounting any React components in a test suite.
 *
 * @returns An object containing the simulated window and document.
 */
export function initDOM() {
  if (!currentWindow) {
    currentWindow = new GlobalWindow()
    const win = currentWindow

    // Attach DOM globals to Node's globalThis context
    Object.assign(globalThis, {
      window: win,
      document: win.document,
      HTMLElement: win.HTMLElement,
      HTMLInputElement: win.HTMLInputElement,
      HTMLButtonElement: win.HTMLButtonElement,
      HTMLDivElement: win.HTMLDivElement,
      HTMLSpanElement: win.HTMLSpanElement,
      Node: win.Node,
      Event: win.Event,
      MouseEvent: win.MouseEvent,
      CustomEvent: win.CustomEvent,
      MutationObserver: win.MutationObserver,
      requestAnimationFrame: (cb: FrameRequestCallback) => setTimeout(cb, 0),
      cancelAnimationFrame: (id: number) => clearTimeout(id),
    })

    // Navigator is a getter in Node 24+, configure explicitly
    Object.defineProperty(globalThis, 'navigator', {
      value: win.navigator,
      configurable: true,
      writable: true,
    })

    // Configure React's act() environment flag
    ;(globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true
  }

  // Clear previous body elements between test suites
  currentWindow.document.body.innerHTML = ''
  return { window: currentWindow, document: currentWindow.document }
}

export interface RenderResult {
  /** The container div where the React component was mounted */
  container: HTMLElement
  /** Cleanly unmounts the component and detaches the container from document.body */
  unmount: () => void
  /** Finds the first element matching a standard CSS selector */
  querySelector: <T = Element>(selector: string) => T | null
  /** Finds all elements matching a standard CSS selector */
  querySelectorAll: <T = Element>(selector: string) => T[]
  /** Dispatches a bubbling click MouseEvent to the target element wrapped in act() */
  click: (element: Element | null | undefined) => Promise<void>
}

/**
 * Mounts a React component into a fresh container element in the virtual DOM.
 * All mounting and lifecycle operations are wrapped inside React's act() function.
 *
 * @param element - The React element to mount (e.g. `<ToggleSwitch ... />` or `React.createElement(...)`)
 * @returns A RenderResult object providing querying, interaction, and teardown methods.
 */
export async function mount(element: React.ReactElement): Promise<RenderResult> {
  const { document } = initDOM()
  const container = document.createElement('div')
  document.body.appendChild(container)

  let root: Root
  await act(async () => {
    root = createRoot(container as unknown as Element)
    root.render(element)
  })

  return {
    container: container as unknown as HTMLElement,
    unmount: () => {
      act(() => {
        root.unmount()
      })
      container.remove()
    },
    querySelector: <T = Element>(selector: string): T | null => {
      return (container as unknown as Element).querySelector(selector) as T | null
    },
    querySelectorAll: <T = Element>(selector: string): T[] => {
      return Array.from((container as unknown as Element).querySelectorAll(selector)) as T[]
    },
    click: async (element: Element | null | undefined) => {
      if (!element) return
      await act(async () => {
        const event = new (window.MouseEvent)('click', {
          bubbles: true,
          cancelable: true,
        })
        element.dispatchEvent(event)
      })
    },
  }
}

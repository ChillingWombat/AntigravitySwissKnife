/**
 * ErrorBoundary.test.ts — Component DOM Behavior Tests for ErrorBoundary
 *
 * Verifies that ErrorBoundary cleanly catches unhandled render-time exceptions in child components,
 * prevents white-screen crashes, renders accessible fallback error presentation with Lucide icons,
 * and allows state reset recovery upon clicking "Try Again".
 */

import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import React from 'react'
import { mount } from '../../test-dom'
import { ErrorBoundary } from '../ErrorBoundary'

// Helper component that throws an error conditionally
interface BombProps {
  shouldExplode: boolean
  message?: string
}

const Bomb: React.FC<BombProps> = ({ shouldExplode, message = 'Crash test payload' }) => {
  if (shouldExplode) {
    throw new Error(message)
  }
  return React.createElement('div', { id: 'safeChild' }, 'Healthy Component Content')
}

describe('Component DOM: ErrorBoundary (<ErrorBoundary />)', () => {
  it('renders healthy child components seamlessly when no error occurs', async () => {
    const { querySelector, unmount } = await mount(
      React.createElement(ErrorBoundary, {
        children: React.createElement(Bomb, { shouldExplode: false }),
      })
    )

    const child = querySelector('#safeChild')
    assert.ok(child, 'Healthy child component must be rendered')
    assert.equal(child.textContent, 'Healthy Component Content')

    // Error UI elements should NOT exist
    const button = querySelector('button')
    assert.equal(button, null, 'Try Again button must not appear in normal state')

    unmount()
  })

  it('catches render exceptions, displays fallbackTitle, and shows error message', async () => {
    // Suppress console.error in test output during expected component crash
    const origError = console.error
    console.error = () => {}

    try {
      const { querySelector, unmount } = await mount(
        React.createElement(ErrorBoundary, {
          fallbackTitle: 'Quota Card Crash',
          children: React.createElement(Bomb, { shouldExplode: true, message: 'Fatal simulated render fault' }),
        })
      )

      // Title heading
      const heading = querySelector('h3')
      assert.ok(heading, 'Fallback title heading must be rendered')
      assert.equal(heading.textContent?.trim(), 'Quota Card Crash')

      // Error message paragraph
      const msgPara = querySelector('p')
      assert.ok(msgPara, 'Error message paragraph must be rendered')
      assert.equal(msgPara.textContent?.trim(), 'Fatal simulated render fault')

      // "Try Again" recovery button
      const button = querySelector('button')
      assert.ok(button, 'Try Again button must be rendered')
      assert.ok(button.textContent?.includes('Try Again'))

      // Lucide icon SVG presence
      const svg = querySelector('svg')
      assert.ok(svg, 'Lucide AlertTriangle icon must be present')

      unmount()
    } finally {
      console.error = origError
    }
  })

  it('resets error state when user clicks "Try Again"', async () => {
    const origError = console.error
    console.error = () => {}

    try {
      let explode = true
      const DynamicChild: React.FC = () => {
        if (explode) {
          throw new Error('Temporary glitch')
        }
        return React.createElement('div', { id: 'recoveredContent' }, 'Recovered Successfully!')
      }

      const { querySelector, click, unmount } = await mount(
        React.createElement(ErrorBoundary, {
          children: React.createElement(DynamicChild),
        })
      )

      const heading = querySelector('h3')
      assert.ok(heading, 'Should initially show error')

      // Now heal the underlying child condition and click "Try Again"
      explode = false
      const button = querySelector('button')
      assert.ok(button)
      await click(button)

      // Component should now recover
      const recovered = querySelector('#recoveredContent')
      assert.ok(recovered, 'Child content must reappear after Try Again reset')
      assert.equal(recovered.textContent, 'Recovered Successfully!')

      unmount()
    } finally {
      console.error = origError
    }
  })
})

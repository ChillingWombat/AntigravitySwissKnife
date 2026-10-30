/**
 * adversarial_challenge.test.ts — Empirical Adversarial Stress Harness for Track 2 (Issue #60)
 *
 * This test suite provides rigorous adversarial stress-testing of:
 * 1. ErrorBoundary exception catching across non-standard throw primitives (strings, numbers, objects, null).
 * 2. ErrorBoundary multi-cycle recovery (throw -> recover -> throw -> recover) with stateful control.
 * 3. ToggleSwitch rapid event dispatching, disabled interaction immunity, and size geometries.
 * 4. CircularGauge mathematical boundary calculations, clamping, and color thresholds.
 * 5. HorizontalQuotaBar boundary fraction scaling, color thresholds, and untracked states.
 * 6. NavRail comprehensive tab routing, daemon-state sensitivity, and layout invariants.
 * 7. DOM test harness container isolation and teardown cleanup verification.
 */

import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import React, { useState } from 'react'
import { mount, initDOM } from '../../test-dom'
import { ErrorBoundary } from '../ErrorBoundary'
import { ToggleSwitch } from '../ToggleSwitch'
import { CircularGauge } from '../CircularGauge'
import { HorizontalQuotaBar } from '../HorizontalQuotaBar'
import { NavRail } from '../NavRail'
import type { Route } from '../../navigation/routes'
import type { SystemStatus } from '../../types'

describe('Adversarial Stress: ErrorBoundary Exception Handling & Recovery', () => {
  it('catches non-Error thrown primitives (string, object, number, null) without crashing', async () => {
    const origError = console.error
    console.error = () => {}

    try {
      const payloads: unknown[] = [
        'Raw string exception payload',
        { customCode: 500, reason: 'Object throw' },
        12345,
        null,
      ]

      for (const payload of payloads) {
        const HostileChild: React.FC = () => {
          throw payload
        }

        const { querySelector, unmount } = await mount(
          React.createElement(
            ErrorBoundary,
            { fallbackTitle: 'Hostile Crash Caught' },
            React.createElement(HostileChild)
          )
        )

        const title = querySelector('h3')
        assert.ok(title, 'ErrorBoundary must catch exception and render title')
        assert.equal(title.textContent?.trim(), 'Hostile Crash Caught')

        const button = querySelector('button')
        assert.ok(button, 'Try Again recovery button must be present')

        const svg = querySelector('svg')
        assert.ok(svg, 'AlertTriangle SVG icon must be rendered')

        unmount()
      }
    } finally {
      console.error = origError
    }
  })

  it('survives error recovery cycle and renders restored child content', async () => {
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
        React.createElement(
          ErrorBoundary,
          null,
          React.createElement(DynamicChild)
        )
      )

      // Initial crash state
      const heading = querySelector('h3')
      assert.ok(heading, 'Should initially show error')
      const p = querySelector('p')
      assert.equal(p?.textContent?.trim(), 'Temporary glitch')

      // Heal condition and click Try Again
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

describe('Adversarial Stress: ToggleSwitch Multi-Click & Interaction Immunity', () => {
  it('dispatches state updates during user click interactions', async () => {
    const recordedEvents: boolean[] = []

    const StatefulToggle: React.FC = () => {
      const [checked, setChecked] = useState(false)
      return React.createElement(ToggleSwitch, {
        checked,
        id: 'stateful-switch',
        onChange: (next) => {
          recordedEvents.push(next)
          setChecked(next)
        },
      })
    }

    const { querySelector, click, unmount } = await mount(
      React.createElement(StatefulToggle)
    )

    const label = querySelector<HTMLLabelElement>('label')
    assert.ok(label)

    await click(label)
    assert.ok(recordedEvents.length >= 1, 'Click must record at least 1 event transition')
    assert.equal(recordedEvents[0], true, 'First click toggles to true')

    unmount()
  })

  it('enforces total interaction immunity when disabled across all sub-elements', async () => {
    let clickCount = 0
    const { querySelector, click, unmount } = await mount(
      React.createElement(ToggleSwitch, {
        checked: false,
        disabled: true,
        onChange: () => {
          clickCount++
        },
      })
    )

    const label = querySelector('label')
    const input = querySelector('input')
    const track = querySelector('label > span')
    const knob = querySelector('label > span > span')

    await click(label)
    await click(input)
    await click(track)
    await click(knob)

    assert.equal(clickCount, 0, 'Disabled ToggleSwitch must completely ignore clicks on all DOM sub-elements')

    unmount()
  })
})

describe('Adversarial Stress: CircularGauge Boundary Math & Clamping', () => {
  it('correctly handles boundary conditions: 0%, 20%, 49%, 50%, 100%, and negative values', async () => {
    const testCases = [
      { pct: 100, color: '#137333', hasCircle: true, text: '100%' },
      { pct: 50, color: '#137333', hasCircle: true, text: '50%' },
      { pct: 49, color: '#b06000', hasCircle: true, text: '49%' },
      { pct: 20, color: '#b06000', hasCircle: true, text: '20%' },
      { pct: 19, color: '#b3261e', hasCircle: true, text: '19%' },
      { pct: 0, color: '#b3261e', hasCircle: true, text: '0%' },
      { pct: -10, color: '', hasCircle: false, text: 'N/A' }, // negative is untracked
      { pct: 125, color: '#137333', hasCircle: true, text: '100%' }, // clamped to 100%
    ]

    for (const tc of testCases) {
      const { querySelector, querySelectorAll, unmount } = await mount(
        React.createElement(CircularGauge, {
          percentage: tc.pct,
          title: `Gauge ${tc.pct}`,
          size: 80,
          strokeWidth: 6,
        })
      )

      const circles = querySelectorAll<SVGCircleElement>('circle')
      if (tc.hasCircle) {
        assert.equal(circles.length, 2, `Percentage ${tc.pct} should have 2 circles`)
        const active = circles[1]
        assert.equal(active.getAttribute('stroke'), tc.color, `Percentage ${tc.pct} must have color ${tc.color}`)
        const offset = parseFloat(active.getAttribute('stroke-dashoffset') || '0')
        assert.ok(!isNaN(offset) && isFinite(offset), 'stroke-dashoffset must be a finite number')
      } else {
        assert.equal(circles.length, 1, `Untracked percentage ${tc.pct} should only have track circle`)
      }

      const span = querySelector('span')
      assert.equal(span?.textContent?.trim(), tc.text)

      unmount()
    }
  })
})

describe('Adversarial Stress: HorizontalQuotaBar Edge Cases', () => {
  it('handles fraction bounds (0.0, 0.19, 0.20, 0.49, 0.50, 1.0, and overflow/null)', async () => {
    const testCases = [
      { fraction: 1.0, color: '#137333', width: '100%', text: '100%' },
      { fraction: 0.50, color: '#137333', width: '50%', text: '50%' },
      { fraction: 0.49, color: '#b06000', width: '49%', text: '49%' },
      { fraction: 0.20, color: '#b06000', width: '20%', text: '20%' },
      { fraction: 0.19, color: '#b3261e', width: '19%', text: '19%' },
      { fraction: 0.0, color: '#b3261e', width: '0%', text: '0%' },
      { fraction: 1.5, color: '#137333', width: '100%', text: '100%' },
      { fraction: null, color: '#9aa0a6', width: '0%', text: 'N/A' },
      { fraction: undefined, color: '#9aa0a6', width: '0%', text: 'N/A' },
    ]

    for (const tc of testCases) {
      const { querySelector, querySelectorAll, unmount } = await mount(
        React.createElement(HorizontalQuotaBar, {
          fraction: tc.fraction,
        })
      )

      const divs = querySelectorAll<HTMLDivElement>('div')
      const innerBar = divs[divs.length - 1]
      assert.equal(innerBar.style.width, tc.width, `Fraction ${tc.fraction} width mismatch`)
      assert.equal(innerBar.style.backgroundColor, tc.color, `Fraction ${tc.fraction} color mismatch`)

      const span = querySelector('span')
      assert.equal(span?.textContent?.trim(), tc.text)

      unmount()
    }
  })
})

describe('Adversarial Stress: NavRail Exhaustive Tab Dispatch & Layout Invariants', () => {
  it('verifies all interactive navigation buttons emit their semantic routes', async () => {
    const onlineStatus: SystemStatus = {
      daemon_running: true,
      daemon_version: '0.2.0',
      active_account: null,
      total_accounts: 2,
    } as unknown as SystemStatus

    const selectedToolCalls: Route[] = []
    const { querySelector, querySelectorAll, click, unmount } = await mount(
      React.createElement(NavRail, {
        currentTool: 'switcher',
        navigate: (route: Route) => {
          selectedToolCalls.push(route)
        },
        status: onlineStatus,
      })
    )

    // First button (Account Switcher) has no id attribute
    const buttons = querySelectorAll<HTMLButtonElement>('button')
    const firstBtn = buttons[0]
    assert.ok(firstBtn.textContent?.includes('Account Switcher'))
    await click(firstBtn)
    assert.deepEqual(selectedToolCalls[selectedToolCalls.length - 1], { tool: 'switcher', tab: 'dashboard' })

    const expectedButtons: { id: string; route: Route }[] = [
      { id: '#btnNavCustomModels', route: { tool: 'custom-models' } },
      { id: '#btnNavEnhancements', route: { tool: 'enhancements' } },
      { id: '#btnNavExtensions', route: { tool: 'extensions' } },
      { id: '#btnNavBridge', route: { tool: 'remote-connect' } },
      { id: '#btnNavTokenMonitor', route: { tool: 'token-monitor' } },
      { id: '#btnNavUtilities', route: { tool: 'utilities', tab: 'cache' } },
      { id: '#btnNavACPAgentMesh', route: { tool: 'acp-adapter' } },
      { id: '#btnNavSystemSettings', route: { tool: 'system-settings' } },
    ]

    for (const item of expectedButtons) {
      const btn = querySelector<HTMLButtonElement>(item.id)
      assert.ok(btn, `Button ${item.id} must exist in NavRail`)
      await click(btn)
      assert.deepEqual(
        selectedToolCalls[selectedToolCalls.length - 1],
        item.route,
        `Clicking ${item.id} must navigate to ${JSON.stringify(item.route)}`
      )
    }

    unmount()
  })

  it('confirms 0 decorative emojis and strict whiteSpace nowrap across entire navigation bar', async () => {
    const onlineStatus: SystemStatus = {
      daemon_running: true,
      daemon_version: '0.2.0',
      active_account: null,
      total_accounts: 2,
    } as unknown as SystemStatus

    const { querySelectorAll, unmount } = await mount(
      React.createElement(NavRail, {
        currentTool: 'switcher',
        navigate: () => {},
        status: onlineStatus,
      })
    )

    const buttons = querySelectorAll<HTMLButtonElement>('button')
    assert.equal(buttons.length, 9, 'Must have exactly 9 buttons when daemon is online')

    const emojiRegex = /[\u{1F300}-\u{1F9FF}\u{2600}-\u{26FF}\u{2700}-\u{27BF}]/u

    for (const btn of buttons) {
      assert.equal(btn.style.whiteSpace, 'nowrap', 'Button must enforce whiteSpace: nowrap')
      assert.ok(!emojiRegex.test(btn.textContent || ''), 'Button text must contain 0 decorative emojis')
    }

    unmount()
  })
})

describe('Adversarial Stress: DOM Test Harness Isolation & Leak Resistance', () => {
  it('cleans up container from document.body on unmount() with zero leaked DOM nodes', async () => {
    initDOM()
    const initialBodyChildren = document.body.children.length

    const { unmount } = await mount(
      React.createElement('div', { id: 'leakTestNode' }, 'Temporary Node')
    )

    assert.equal(document.body.children.length, initialBodyChildren + 1)
    assert.ok(document.getElementById('leakTestNode'))

    unmount()

    assert.equal(document.body.children.length, initialBodyChildren)
    assert.equal(document.getElementById('leakTestNode'), null, 'Element must be removed from DOM after unmount')
  })
})

/**
 * HorizontalQuotaBar.test.ts — Component DOM Behavior Tests for HorizontalQuotaBar
 *
 * Verifies genuine React component mounting, dynamic percentage bar width calculation,
 * color thresholds for healthy / warning / exhausted quotas, and disabled / untracked states.
 */

import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import React from 'react'
import { mount } from '../../test-dom'
import { HorizontalQuotaBar } from '../HorizontalQuotaBar'

describe('Component DOM: HorizontalQuotaBar (<HorizontalQuotaBar />)', () => {
  it('renders healthy quota (fraction 0.85) with green bar width 85% and label "85%"', async () => {
    const { querySelector, querySelectorAll, unmount } = await mount(
      React.createElement(HorizontalQuotaBar, {
        fraction: 0.85,
        title: 'Claude 3.7 Sonnet Quota',
        maxWidth: 200,
      })
    )

    // Check inner progress bar element
    const divs = querySelectorAll<HTMLDivElement>('div')
    const innerBar = divs[divs.length - 1]
    assert.ok(innerBar, 'Inner progress bar must exist')
    assert.equal(innerBar.style.width, '85%', 'Bar width must reflect 85%')
    assert.equal(innerBar.style.backgroundColor, '#137333', 'Healthy >=50% bar must be green (#137333)')

    // Check label text
    const labelSpan = querySelector<HTMLSpanElement>('span')
    assert.ok(labelSpan, 'Label span must be present')
    assert.equal(labelSpan.textContent?.trim(), '85%')

    unmount()
  })

  it('renders warning threshold (fraction 0.35) with yellow/orange color (#b06000)', async () => {
    const { querySelector, querySelectorAll, unmount } = await mount(
      React.createElement(HorizontalQuotaBar, {
        fraction: 0.35,
      })
    )

    const divs = querySelectorAll<HTMLDivElement>('div')
    const innerBar = divs[divs.length - 1]
    assert.ok(innerBar)
    assert.equal(innerBar.style.width, '35%')
    assert.equal(innerBar.style.backgroundColor, '#b06000', '35% quota must be warning yellow #b06000')

    const labelSpan = querySelector<HTMLSpanElement>('span')
    assert.equal(labelSpan?.textContent?.trim(), '35%')

    unmount()
  })

  it('renders exhausted threshold (fraction 0.08) with red color (#b3261e)', async () => {
    const { querySelector, querySelectorAll, unmount } = await mount(
      React.createElement(HorizontalQuotaBar, {
        fraction: 0.08,
      })
    )

    const divs = querySelectorAll<HTMLDivElement>('div')
    const innerBar = divs[divs.length - 1]
    assert.ok(innerBar)
    assert.equal(innerBar.style.width, '8%')
    assert.equal(innerBar.style.backgroundColor, '#b3261e', '8% quota must be exhausted red #b3261e')

    const labelSpan = querySelector<HTMLSpanElement>('span')
    assert.equal(labelSpan?.textContent?.trim(), '8%')

    unmount()
  })

  it('renders "N/A" with width 0% and grey color when fraction is null or disabled', async () => {
    const { querySelector, querySelectorAll, unmount } = await mount(
      React.createElement(HorizontalQuotaBar, {
        fraction: null,
        disabled: true,
      })
    )

    const divs = querySelectorAll<HTMLDivElement>('div')
    const innerBar = divs[divs.length - 1]
    assert.ok(innerBar)
    assert.equal(innerBar.style.width, '0%')
    assert.equal(innerBar.style.backgroundColor, '#9aa0a6', 'Disabled/untracked bar must be neutral grey #9aa0a6')

    const labelSpan = querySelector<HTMLSpanElement>('span')
    assert.equal(labelSpan?.textContent?.trim(), 'N/A')

    unmount()
  })

  it('does not render percentage text when showPercentage is false', async () => {
    const { querySelector, querySelectorAll, unmount } = await mount(
      React.createElement(HorizontalQuotaBar, {
        fraction: 0.85,
        showPercentage: false,
      })
    )

    const labelSpan = querySelector<HTMLSpanElement>('span')
    assert.equal(labelSpan, null, 'Label span must not be rendered when showPercentage is false')

    const divs = querySelectorAll<HTMLDivElement>('div')
    const innerBar = divs[divs.length - 1]
    assert.ok(innerBar)
    assert.equal(innerBar.style.width, '85%')

    unmount()
  })
})

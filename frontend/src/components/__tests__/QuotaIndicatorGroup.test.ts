/**
 * QuotaIndicatorGroup.test.ts — #127 multi-quota indicator DOM tests
 *
 * A single reported quota keeps the circular ring; two or more reported windows
 * (5H + 7D/monthly ...) switch to one labelled progress bar per window so no
 * window is hidden behind another.
 */

import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import React from 'react'
import { mount } from '../../test-dom'
import { QuotaIndicatorGroup } from '../QuotaIndicatorGroup'

describe('Component DOM: QuotaIndicatorGroup (<QuotaIndicatorGroup />)', () => {
  it('renders a single ring for a single reported quota', async () => {
    const { querySelectorAll, unmount } = await mount(
      React.createElement(QuotaIndicatorGroup, {
        buckets: [{ label: 'Quota', fraction: 0.64, value: '64%' }],
        singleTitle: 'Quota: 64%',
      })
    )

    const svgs = querySelectorAll('svg')
    assert.equal(svgs.length, 1, 'a single quota must stay a circular ring')

    unmount()
  })

  it('renders one labelled bar per window when several quotas are reported', async () => {
    const { querySelectorAll, container, unmount } = await mount(
      React.createElement(QuotaIndicatorGroup, {
        buckets: [
          { label: '5H', fraction: 0.7, value: '70%' },
          { label: '7D', fraction: 0.05, value: '5%' },
          { label: 'Monthly', fraction: null },
        ],
      })
    )

    const text = container.textContent || ''
    assert.ok(text.includes('5H'), '5H window label must be visible')
    assert.ok(text.includes('7D'), '7D window label must be visible')
    assert.ok(text.includes('Monthly'), 'Monthly window label must be visible')
    assert.ok(text.includes('70%') && text.includes('5%'), 'each window shows percentage without trailing "left"')
    assert.ok(!text.includes('left'), 'must not render "left" suffix')

    const labels = querySelectorAll('span')
    assert.ok(
      labels.some((s) => s.textContent?.trim() === '5H'),
      'each window must render its own label span'
    )

    unmount()
  })

  it('normalizes percentage text without "left" and eliminates duplicate percentage beside the bar', async () => {
    const { querySelectorAll, container, unmount } = await mount(
      React.createElement(QuotaIndicatorGroup, {
        buckets: [
          { label: '5H', fraction: 0.73, value: '73%' },
          { label: '7D', fraction: 0.4, value: '40% left' },
          { label: '30D', fraction: 0.15 },
          { label: 'Monthly', fraction: null, value: 'Unlimited' },
        ],
      })
    )

    const spans = querySelectorAll<HTMLSpanElement>('span')
    // Each of the 4 buckets must have exactly 2 spans (label + valueText) and 0 spans inside HorizontalQuotaBar
    assert.equal(spans.length, 8, 'must render exactly 2 spans per bucket without trailing bar percentages')

    const spanTexts = spans.map((s) => s.textContent?.trim())
    assert.deepEqual(spanTexts, [
      '5H',
      '73%',
      '7D',
      '40%',
      '30D',
      '15%',
      'Monthly',
      'Unlimited',
    ])

    const text = container.textContent || ''
    assert.ok(!text.includes('left'), 'must not contain "left" suffix')

    unmount()
  })

  it('falls back to the legacy ring for untracked quotas', async () => {
    const { querySelectorAll, container, unmount } = await mount(
      React.createElement(QuotaIndicatorGroup, {
        buckets: [{ label: 'N/A', fraction: null }],
        singleTitle: 'N/A',
      })
    )

    assert.equal(querySelectorAll('svg').length, 1, 'untracked quota must still render the single ring')
    assert.ok((container.textContent || '').includes('N/A'), 'untracked ring keeps the N/A label')

    unmount()
  })
})

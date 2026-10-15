/**
 * CircularGauge.test.ts — Component DOM Behavior Tests for CircularGauge
 *
 * Verifies genuine React component mounting, SVG path calculations,
 * stroke color thresholds (green, orange/yellow, red), and fallback "N/A" states.
 */

import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import React from 'react'
import { mount } from '../../test-dom'
import { CircularGauge } from '../CircularGauge'

describe('Component DOM: CircularGauge (<CircularGauge />)', () => {
  it('renders 75% quota with green SVG stroke and formatted percentage text', async () => {
    const size = 100
    const strokeWidth = 8
    const { querySelector, querySelectorAll, unmount } = await mount(
      React.createElement(CircularGauge, {
        percentage: 75,
        title: 'Gemini 2.5 Quota',
        size,
        strokeWidth,
      })
    )

    // Check SVG container dimensions
    const svg = querySelector<SVGSVGElement>('svg')
    assert.ok(svg, 'SVG element must be rendered')
    assert.equal(svg.getAttribute('width'), '100')
    assert.equal(svg.getAttribute('height'), '100')

    // Two circles: background track and active progress circle
    const circles = querySelectorAll<SVGCircleElement>('circle')
    assert.equal(circles.length, 2, 'Must render background track and active progress circle')

    const trackCircle = circles[0]
    assert.equal(trackCircle.getAttribute('stroke'), '#e5e9f0', 'Background track stroke must be #e5e9f0')

    const activeCircle = circles[1]
    assert.equal(activeCircle.getAttribute('stroke'), '#137333', 'Healthy >=50% stroke must be Google green (#137333)')

    // Stroke dash calculations: radius = (100 - 8)/2 = 46. Circumference = 2 * PI * 46 ~= 289.026
    const radius = (size - strokeWidth) / 2
    const circumference = 2 * Math.PI * radius
    const expectedOffset = circumference - (75 / 100) * circumference
    const actualDashArray = parseFloat(activeCircle.getAttribute('stroke-dasharray') || '0')
    const actualDashOffset = parseFloat(activeCircle.getAttribute('stroke-dashoffset') || '0')

    assert.ok(Math.abs(actualDashArray - circumference) < 0.1, 'Circumference must match 2*PI*radius')
    assert.ok(Math.abs(actualDashOffset - expectedOffset) < 0.1, 'Stroke dash offset must match 75% progress')

    // Percentage value text
    const textSpan = querySelector<HTMLSpanElement>('span')
    assert.ok(textSpan)
    assert.equal(textSpan.textContent?.trim(), '75%')

    // Title label
    assert.ok(document.body.innerHTML.includes('Gemini 2.5 Quota'), 'Title must be rendered')

    unmount()
  })

  it('renders warning color (#b06000) when percentage is between 20% and 49%', async () => {
    const { querySelectorAll, unmount } = await mount(
      React.createElement(CircularGauge, {
        percentage: 35,
        title: 'Claude Quota',
      })
    )

    const circles = querySelectorAll<SVGCircleElement>('circle')
    assert.equal(circles.length, 2)
    const activeCircle = circles[1]
    assert.equal(activeCircle.getAttribute('stroke'), '#b06000', '35% quota must render warning color #b06000')

    unmount()
  })

  it('renders critical exhausted color (#b3261e) when percentage is under 20%', async () => {
    const { querySelectorAll, querySelector, unmount } = await mount(
      React.createElement(CircularGauge, {
        percentage: 12,
        title: 'Depleted Quota',
      })
    )

    const circles = querySelectorAll<SVGCircleElement>('circle')
    assert.equal(circles.length, 2)
    const activeCircle = circles[1]
    assert.equal(activeCircle.getAttribute('stroke'), '#b3261e', '12% quota must render red #b3261e')

    const textSpan = querySelector<HTMLSpanElement>('span')
    assert.ok(textSpan)
    assert.equal(textSpan.textContent?.trim(), '12%')

    unmount()
  })

  it('renders "N/A" and omits active stroke when percentage is null or untracked', async () => {
    const { querySelectorAll, querySelector, unmount } = await mount(
      React.createElement(CircularGauge, {
        percentage: null,
        title: 'Untracked Quota',
      })
    )

    // Only background track circle should be rendered
    const circles = querySelectorAll<SVGCircleElement>('circle')
    assert.equal(circles.length, 1, 'Only track circle rendered when quota is null')

    const textSpan = querySelector<HTMLSpanElement>('span')
    assert.ok(textSpan)
    assert.equal(textSpan.textContent?.trim(), 'N/A', 'Must display N/A for null quota')

    unmount()
  })
})

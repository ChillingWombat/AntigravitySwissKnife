import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import React from 'react'
import fs from 'node:fs'
import path from 'node:path'
import { QuotaIndicatorGroup } from '../components/QuotaIndicatorGroup'
import { mount } from '../test-dom'

const renderedBarWidths = (container: Element): string[] => (
  Array.from(container.querySelectorAll<HTMLElement>('div'))
    .map((element) => element.style.width)
    .filter((width) => width.endsWith('%') && width !== '100%')
)

describe('Quota progress rendered behavior', () => {
  it('renders Gemini and Argon 7D bars with remaining-percentage widths', async () => {
    const rendered = await mount(
      React.createElement(QuotaIndicatorGroup, {
        buckets: [
          { label: 'Gemini 7D', fraction: 0.42, value: '42% remaining' },
          { label: 'Argon 7D', fraction: 0.67, value: '67% remaining' },
        ],
      }),
    )

    const text = rendered.container.textContent || ''
    assert.match(text, /Gemini 7D/)
    assert.match(text, /Argon 7D/)
    assert.match(text, /42% remaining/)
    assert.match(text, /67% remaining/)
    assert.deepEqual(renderedBarWidths(rendered.container), ['42%', '67%'])
    assert.equal(rendered.querySelectorAll('svg').length, 0)

    rendered.unmount()
  })

  it('omits an absent Argon window while keeping both reported quota bars', async () => {
    const rendered = await mount(
      React.createElement(QuotaIndicatorGroup, {
        buckets: [
          { label: 'Gemini 5H', fraction: 0.55 },
          { label: 'Gemini 7D', fraction: 0.80 },
        ],
      }),
    )

    const text = rendered.container.textContent || ''
    assert.match(text, /Gemini 5H/)
    assert.match(text, /Gemini 7D/)
    assert.doesNotMatch(text, /Argon/)
    assert.deepEqual(renderedBarWidths(rendered.container), ['55%', '80%'])
    assert.equal(rendered.querySelectorAll('svg').length, 0)

    rendered.unmount()
  })

  it('centers single 7D and 5H quota bars vertically before multi-window quotas load', () => {
    const pageSrc = fs.readFileSync(
      path.resolve(import.meta.dirname, '..', 'pages', 'QuotaDashboardPage.tsx'),
      'utf8',
    )
    assert.ok(
      pageSrc.includes("verticalAlign: hasMultipleWeeklyBars ? 'top' : 'middle'"),
      '7D Quota cell must use middle vertical alignment when only a single bar is present',
    )
    assert.ok(
      pageSrc.includes("verticalAlign: hasMultiple5hBars ? 'top' : 'middle'"),
      '5H Quota cell must use middle vertical alignment when only a single bar is present',
    )
  })
})

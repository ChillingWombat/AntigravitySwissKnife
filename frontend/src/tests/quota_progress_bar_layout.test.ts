import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'

describe('QuotaDashboardPage model progress bar layout — 2x1 vertical stack', () => {
  const pageSrc = fs.readFileSync(
    path.resolve(import.meta.dirname, '..', 'pages', 'QuotaDashboardPage.tsx'),
    'utf8',
  )

  it('renders 7D multi-model quota cell with Gemini and Argon in a 2x1 vertical stack', () => {
    const weeklySection = pageSrc.indexOf('{hasArgonWeeklyQuota ? (')
    assert.ok(weeklySection > 0, '{hasArgonWeeklyQuota ? ( block not found')
    const blockEnd = pageSrc.indexOf(') : (', weeklySection)
    const block = pageSrc.slice(weeklySection, blockEnd)

    assert.ok(
      block.includes("flexDirection: 'column'"),
      '7D multi-model cell must use flexDirection: "column" (2x1 vertical stack)',
    )
    assert.ok(
      block.includes("gap: '6px'"),
      '7D multi-model cell must use 6px gap between Gemini and Argon rows',
    )
    assert.ok(
      block.indexOf('<span>Gemini</span>') < block.indexOf('<span>Argon</span>'),
      'Gemini row must be positioned above Argon row in the 2x1 stack',
    )
  })

  it('renders 5H multi-model quota cell with Gemini and Argon in a 2x1 vertical stack', () => {
    const section5h = pageSrc.indexOf('if (hasArgon5hQuota) {')
    assert.ok(section5h > 0, 'if (hasArgon5hQuota) block not found')
    const blockEnd = pageSrc.indexOf('if (hasArgonWeeklyQuota)', section5h)
    const block = pageSrc.slice(section5h, blockEnd)

    assert.ok(
      block.includes("flexDirection: 'column'"),
      '5H multi-model cell must use flexDirection: "column" (2x1 vertical stack)',
    )
    assert.ok(
      block.includes("gap: '6px'"),
      '5H multi-model cell must use 6px gap between Gemini and Argon rows',
    )
    assert.ok(
      block.indexOf('<span>Gemini</span>') < block.indexOf('<span>Argon</span>'),
      'Gemini row must be positioned above Argon row in the 2x1 stack',
    )
  })

  it('uses balanced 180px column widths on 5H Quota and 7D Quota headers', () => {
    const header5h = pageSrc.indexOf("setSortMode('quota_5h')")
    assert.ok(header5h > 0, '5H header not found')
    const header5hSlice = pageSrc.slice(header5h, header5h + 200)
    assert.ok(
      header5hSlice.includes("width: '180px'"),
      '5H Quota column width must be 180px',
    )

    const headerWeekly = pageSrc.indexOf("setSortMode('quota_weekly')")
    assert.ok(headerWeekly > 0, '7D header not found')
    const headerWeeklySlice = pageSrc.slice(headerWeekly, headerWeekly + 200)
    assert.ok(
      headerWeeklySlice.includes("width: '180px'"),
      '7D Quota column width must be balanced at 180px',
    )
  })
})

describe('QuotaDashboardPage Argon quota ring conditional rendering', () => {
  const pageSrc = fs.readFileSync(
    path.resolve(import.meta.dirname, '..', 'pages', 'QuotaDashboardPage.tsx'),
    'utf8',
  )

  it('omits Argon progress rings when no Argon quota exists', () => {
    assert.ok(
      pageSrc.includes('{hasArgon7d && (\n                <CircularGauge percentage={gaugeWeeklyArgon} title="Argon 7D"'),
      'Argon 7D ring in standard overview must be conditionally rendered with hasArgon7d',
    )
    assert.ok(
      pageSrc.includes('{(hasArgon5h || hasArgon7d) && ('),
      'Argon row in multi-model overview must be conditionally rendered when hasArgon5h or hasArgon7d is true',
    )
  })
})

import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'

const pageSrc = fs.readFileSync(
  path.resolve(import.meta.dirname, '..', 'pages', 'QuotaDashboardPage.tsx'),
  'utf8',
).replace(/\r\n/g, '\n')

const headerBlock = pageSrc.slice(
  pageSrc.indexOf('<thead>'),
  pageSrc.indexOf('</thead>'),
)

const headers = headerBlock.match(/<th\b[\s\S]*?<\/th>/g) ?? []

describe('QuotaDashboardPage account fleet table headers', () => {
  it('keeps Account left-aligned and centers every other header', () => {
    assert.equal(headers.length, 7)

    const labels = ['Account', 'Plan', '5H Quota', '7D Quota', 'Credit', 'Priority', 'Action']
    for (const [index, label] of labels.entries()) {
      assert.ok(headers[index].includes(label), `${label} header is missing`)
      assert.ok(
        headers[index].includes(`textAlign: '${label === 'Account' ? 'left' : 'center'}'`),
        `${label} header alignment is incorrect`,
      )
    }
  })

  it('centers sortable 5H and 7D header labels with their sort icons', () => {
    for (const label of ['5H Quota', '7D Quota']) {
      const header = headers.find((candidate) => candidate.includes(label))
      assert.ok(header, `${label} header not found`)
      assert.ok(header.includes("justifyContent: 'center'"), `${label} header content must be centered`)
    }
  })

  it('uses a slightly narrower Priority column', () => {
    const priorityHeader = headers.find((header) => header.includes('<span>Priority</span>'))
    assert.ok(priorityHeader, 'Priority header not found')
    assert.ok(priorityHeader.includes("width: '72px'"))
    assert.ok(priorityHeader.includes("minWidth: '72px'"))
  })
})

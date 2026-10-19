import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'

// The GitHub Workspace card labels every other control with a ToggleSwitch
// (Auxiliary Panel, Main Page); "Pin focused chat to top" used to render a raw
// checkbox and looked out of place next to them.
describe('GitHub Workspace settings — pin focused chat control', () => {
  const pageSrc = fs.readFileSync(
    path.resolve(import.meta.dirname, '..', 'pages', 'ExtensionsPage.tsx'),
    'utf8',
  )

  it('renders "Pin focused chat to top" as a ToggleSwitch', () => {
    assert.ok(pageSrc.includes('Pin focused chat to top'), 'label should still exist')

    const labelIdx = pageSrc.indexOf('Pin focused chat to top')
    assert.ok(labelIdx > 0, 'label not found')

    // The control inside the same <label> block must be the shared ToggleSwitch.
    const blockStart = pageSrc.lastIndexOf('<label', labelIdx)
    const blockEnd = pageSrc.indexOf('</label>', labelIdx)
    const block = pageSrc.slice(blockStart, blockEnd)
    assert.ok(block.includes('<ToggleSwitch'), 'expected a ToggleSwitch inside the label')
    assert.ok(!block.includes('type="checkbox"'), 'checkbox input must be gone')
  })

  it('keeps the persisted key and toggling semantics', () => {
    assert.ok(
      pageSrc.includes("localStorage.setItem('antigravity_github_aux_pin_focused'"),
      'persistence key must not change',
    )
    assert.ok(
      pageSrc.includes('ariaLabel="Pin focused chat to top"'),
      'toggle needs an accessible name',
    )
  })
})

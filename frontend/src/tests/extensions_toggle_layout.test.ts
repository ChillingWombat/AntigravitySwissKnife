import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'

describe('ExtensionsPage toggle button layout — 1x2 horizontal row', () => {
  const pageSrc = fs.readFileSync(
    path.resolve(import.meta.dirname, '..', 'pages', 'ExtensionsPage.tsx'),
    'utf8',
  )

  it('renders built-in extension switches in a 1x2 horizontal flex row with 16px gap and center alignment', () => {
    const fnStart = pageSrc.indexOf('const renderExtSwitches =')
    assert.ok(fnStart > 0, 'renderExtSwitches definition not found')
    const fnEnd = pageSrc.indexOf('const githubVis =', fnStart)
    const fnBody = pageSrc.slice(fnStart, fnEnd)

    assert.ok(
      fnBody.includes("flexDirection: 'row'"),
      'renderExtSwitches must use flexDirection: "row" (1x2 horizontal layout)',
    )
    assert.ok(
      fnBody.includes("alignItems: 'center'"),
      'renderExtSwitches must center align toggle items',
    )
    assert.ok(
      fnBody.includes("gap: '16px'"),
      'renderExtSwitches must use 16px gap between toggles',
    )
  })

  it('renders community extension card switches in a 1x2 horizontal flex row with 16px gap and center alignment', () => {
    const listSection = pageSrc.indexOf('{/* Right Side: Visibility Toggles */}')
    assert.ok(listSection > 0, 'Community extensions right side visibility toggles not found')
    const blockEnd = pageSrc.indexOf('</div>', listSection + 50)
    const block = pageSrc.slice(listSection, blockEnd + 6)

    assert.ok(
      block.includes("flexDirection: 'row'"),
      'Community extension toggles must use flexDirection: "row"',
    )
    assert.ok(
      block.includes("alignItems: 'center'"),
      'Community extension toggles must use alignItems: "center"',
    )
    assert.ok(
      block.includes("gap: '16px'"),
      'Community extension toggles must use 16px gap',
    )
  })

  it('preserves label-left and switch-right with whiteSpace: nowrap for all switch labels', () => {
    const labelStyleStart = pageSrc.indexOf('const extSwitchLabelStyle:')
    assert.ok(labelStyleStart > 0, 'extSwitchLabelStyle definition not found')
    const labelStyleEnd = pageSrc.indexOf('}', labelStyleStart)
    const styleBody = pageSrc.slice(labelStyleStart, labelStyleEnd + 1)

    assert.ok(styleBody.includes("display: 'inline-flex'"), 'label must be inline-flex')
    assert.ok(styleBody.includes("alignItems: 'center'"), 'label must align items center')
    assert.ok(styleBody.includes("whiteSpace: 'nowrap'"), 'label must enforce whiteSpace nowrap')
  })

  it('renders Left Panel Extensions Access Mode segmented control with Single button on left and Individual button on right', () => {
    const gadgetStart = pageSrc.indexOf('Left Panel Extensions Access Mode')
    assert.ok(gadgetStart > 0, 'Left Panel Extensions Access Mode section not found')
    const singleIdx = pageSrc.indexOf("handleLeftPanelModeChange('single')", gadgetStart)
    const individualIdx = pageSrc.indexOf("handleLeftPanelModeChange('individual')", gadgetStart)
    assert.ok(singleIdx > 0, 'Single mode button not found')
    assert.ok(individualIdx > 0, 'Individual mode button not found')
    assert.ok(
      singleIdx < individualIdx,
      'Single Swiss Knife Button must appear before (to the left of) Individual Buttons',
    )
  })
})

import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'

describe('Default Models Configuration Model and Thinking Dropdown Pairs', () => {
  const pagePath = path.resolve(import.meta.dirname, '..', 'pages', 'SwitcherSettingsPage.tsx')
  const content = fs.readFileSync(pagePath, 'utf8')

  it('renders a Model and Thinking dropdown pair for Default Gemini Model', () => {
    assert.ok(content.includes('aria-label="Default Gemini Model"'))
    assert.ok(content.includes('aria-label="Default Gemini Thinking Level"'))
  })

  it('renders a Model and Thinking dropdown pair for Default Custom Model', () => {
    assert.ok(content.includes('aria-label="Default Custom Model"'))
    assert.ok(content.includes('aria-label="Default Custom Thinking Level"'))
  })

  it('renders a Model and Thinking dropdown pair for Default Non-Gemini Native Model', () => {
    assert.ok(content.includes('aria-label="Default Non-Gemini Native Model"'))
    assert.ok(content.includes('aria-label="Default Non-Gemini Thinking Level"'))
  })

  it('ensures Gemini Subagent Custom Models setting gadget is completely removed', () => {
    assert.equal(content.includes('Gemini Subagent Custom Models'), false)
    assert.equal(content.includes('Enable Gemini subagent custom models'), false)
  })
})

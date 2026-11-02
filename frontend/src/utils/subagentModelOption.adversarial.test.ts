import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

describe('Adversarial & Empirical Verification: Milestone 13 Subagent Custom Model Options & David-Design', () => {
  const pagePath = path.resolve(__dirname, '../pages/SwitcherSettingsPage.tsx')
  const pageSource = fs.readFileSync(pagePath, 'utf8')

  it('verifies Section 4b Gemini Subagent Custom Models gadget has been cleanly removed', () => {
    assert.equal(pageSource.includes('Section 4b: Gemini Subagent Custom Models'), false, 'Section 4b must not exist')
    assert.equal(pageSource.includes('Enable Gemini subagent custom models'), false, 'Subagent custom models toggle must not exist')
  })
})

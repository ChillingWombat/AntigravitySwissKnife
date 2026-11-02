import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'

describe('AppLockScreen Branding and Logo', () => {
  it('renders the application logo rather than a generic tick or lock icon', () => {
    const compPath = path.resolve(import.meta.dirname, '..', 'components', 'AppLockScreen.tsx')
    const content = fs.readFileSync(compPath, 'utf8')

    assert.ok(content.includes('swissLogoUrl'), 'Must use swissLogoUrl from browserAssets')
    assert.ok(content.includes('<img'), 'Must render img tag for logo')
    assert.ok(content.includes('src={swissLogoUrl}'), 'Must bind src to swissLogoUrl')
    assert.ok(content.includes('alt="Antigravity Swiss Knife Logo"'), 'Must have descriptive alt text')
    assert.ok(!content.includes('<Check'), 'Must not render generic check or tick icon in lock screen')
  })
})

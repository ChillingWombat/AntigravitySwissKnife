import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'

describe('UI Design System Audit & Voice Invariants (Milestone 16)', () => {
  const rootSrcDir = path.resolve(import.meta.dirname, '..')
  const pagesDir = path.join(rootSrcDir, 'pages')
  const componentsDir = path.join(rootSrcDir, 'components')
  const cssPath = path.join(rootSrcDir, 'index.css')

  const ownedPages = [
    'AppEnhancementsPage.tsx',
    'QuotaDashboardPage.tsx',
    'SwitcherSettingsPage.tsx',
    'BrainCachePage.tsx',
    'ArchivedProjectsPage.tsx',
    'SystemSettingsPage.tsx',
    'TokenMonitorPage.tsx',
    'ExtensionsPage.tsx',
  ]

  describe('1. Iconography & Zero-Decorative-Emoji Invariant', () => {
    it('verifies 0 decorative unicode emojis across all production pages and components', () => {
      const emojiRegex = /[\u{1F300}-\u{1F9FF}\u{2600}-\u{26FF}\u{2700}-\u{27BF}]/u

      const checkDir = (dir: string) => {
        const entries = fs.readdirSync(dir, { withFileTypes: true })
        for (const entry of entries) {
          const fullPath = path.join(dir, entry.name)
          if (entry.isDirectory()) {
            checkDir(fullPath)
          } else if (entry.name.endsWith('.tsx') && !entry.name.includes('.test.')) {
            const content = fs.readFileSync(fullPath, 'utf8')
            // Strip comments
            const stripped = content
              .replace(/\/\*[\s\S]*?\*\//g, '')
              .replace(/\/\/.*$/gm, '')

            const match = stripped.match(emojiRegex)
            assert.equal(
              match,
              null,
              `Offending emoji "${match?.[0]}" found in production file ${entry.name}`
            )
          }
        }
      }

      checkDir(pagesDir)
      checkDir(componentsDir)
    })

    it('verifies complete elimination of rogue unicode symbols (▾, ▴, ▼, ›) in AppEnhancementsPage.tsx', () => {
      const appEnhPath = path.join(pagesDir, 'AppEnhancementsPage.tsx')
      const content = fs.readFileSync(appEnhPath, 'utf8')
      const stripped = content
        .replace(/\/\*[\s\S]*?\*\//g, '')
        .replace(/\/\/.*$/gm, '')

      const rogueGlyphs = ['▾', '▴', '▼', '›']
      for (const glyph of rogueGlyphs) {
        assert.equal(
          stripped.includes(glyph),
          false,
          `Rogue unicode character "${glyph}" must not be present in AppEnhancementsPage.tsx`
        )
      }

      // Verify Lucide chevron imports
      assert.ok(
        content.includes('ChevronDown') && content.includes('ChevronUp') && content.includes('ChevronRight'),
        'AppEnhancementsPage must import ChevronDown, ChevronUp, and ChevronRight from lucide-react'
      )
    })
  })

  describe('2. Button & Badge Layout Invariant (white-space: nowrap)', () => {
    it('enforces white-space: nowrap on button and pill classes in index.css', () => {
      const cssContent = fs.readFileSync(cssPath, 'utf8')

      // Check button element selector
      const buttonMatch = cssContent.match(/button\s*\{([^}]+)\}/)
      assert.ok(buttonMatch, 'button rule must exist in index.css')
      assert.ok(
        buttonMatch[1].includes('white-space: nowrap'),
        'button base rule must include white-space: nowrap'
      )

      // Check pill and chip classes
      const requiredClasses = [
        '.btn-pill-primary',
        '.btn-pill-tonal',
        '.btn-pill-outlined',
        '.btn-pill-danger',
        '.badge-chip',
      ]

      for (const cls of requiredClasses) {
        const regex = new RegExp(`${cls.replace('.', '\\.')}\\s*\\{([^}]+)\\}`, 'm')
        const match = cssContent.match(regex)
        assert.ok(match, `${cls} class definition must exist in index.css`)
        assert.ok(
          match[1].includes('white-space: nowrap'),
          `${cls} must explicitly contain white-space: nowrap`
        )
      }
    })
  })

  describe('3. Spacing & Fractional Font Size Elimination', () => {
    it('verifies 0 fractional pixel font sizes across all owned pages', () => {
      const fractionalFontRegex = /fontSize:\s*['"]\d+\.5px['"]/g

      for (const pageName of ownedPages) {
        const filePath = path.join(pagesDir, pageName)
        const content = fs.readFileSync(filePath, 'utf8')
        const matches = content.match(fractionalFontRegex)
        assert.equal(
          matches,
          null,
          `Found fractional font size(s) in ${pageName}: ${matches?.join(', ')}`
        )
      }
    })

    it('verifies absence of non-standard 1.5px solid borders in QuotaDashboard and SwitcherSettings', () => {
      const quotaPath = path.join(pagesDir, 'QuotaDashboardPage.tsx')
      const switcherPath = path.join(pagesDir, 'SwitcherSettingsPage.tsx')

      const quotaContent = fs.readFileSync(quotaPath, 'utf8')
      const switcherContent = fs.readFileSync(switcherPath, 'utf8')

      assert.equal(
        quotaContent.includes('1.5px solid'),
        false,
        'QuotaDashboardPage must not contain 1.5px solid borders'
      )
      assert.equal(
        switcherContent.includes('1.5px solid'),
        false,
        'SwitcherSettingsPage must not contain 1.5px solid borders'
      )
    })
  })

  describe('4. Text Refinements & Voice Invariants (David-Humanizer)', () => {
    it('purges banned AI buzzwords (utilize, supercharge, paradigm shift, delve) from owned pages', () => {
      const bannedWords = ['utilize', 'utilizes', 'supercharge', 'paradigm shift', 'delve']

      for (const pageName of ownedPages) {
        const filePath = path.join(pagesDir, pageName)
        const content = fs.readFileSync(filePath, 'utf8')
        // Strip comments
        const stripped = content
          .replace(/\/\*[\s\S]*?\*\//g, '')
          .replace(/\/\/.*$/gm, '')

        for (const word of bannedWords) {
          const regex = new RegExp(`\\b${word}\\b`, 'i')
          assert.equal(
            regex.test(stripped),
            false,
            `Banned word "${word}" found in ${pageName}`
          )
        }
      }
    })

    it('verifies humanized copy in SwitcherSettingsPage', () => {
      const content = fs.readFileSync(path.join(pagesDir, 'SwitcherSettingsPage.tsx'), 'utf8')
      assert.ok(
        content.includes('Configure rotation triggers and candidate account priority.'),
        'SwitcherSettingsPage strategy subtitle must be humanized'
      )
      assert.ok(
        content.includes('Rotates when active quota hits the threshold. Ranks standby accounts by combined 5-hour and weekly quota.'),
        'SwitcherSettingsPage balanced mode copy must be humanized'
      )
      assert.ok(
        content.includes('Sync active accounts across all Antigravity apps or manage each app independently.'),
        'SwitcherSettingsPage sync subtitle must be humanized'
      )
      assert.ok(
        content.includes('Fallback third-party model used when non-Gemini routing is enabled.'),
        'SwitcherSettingsPage fallback model copy must be direct'
      )
      assert.ok(
        content.includes('Configure how Gemini subagents use custom models.'),
        'SwitcherSettingsPage subagent model copy must be concise'
      )
    })

    it('verifies humanized copy in SystemSettingsPage', () => {
      const content = fs.readFileSync(path.join(pagesDir, 'SystemSettingsPage.tsx'), 'utf8')
      assert.ok(
        content.includes('Lifecycle Mode'),
        'SystemSettingsPage header must be concise "Lifecycle Mode"'
      )
      assert.ok(
        content.includes('Daemon runs while the desktop app is open. Closing the app stops background services.'),
        'SystemSettingsPage app mode copy must be humanized'
      )
      assert.ok(
        content.includes('Restore all Antigravity apps to default state and disable all enhancements.'),
        'SystemSettingsPage clean restore copy must be humanized'
      )
    })

    it('verifies humanized copy in BrainCachePage', () => {
      const content = fs.readFileSync(path.join(pagesDir, 'BrainCachePage.tsx'), 'utf8')
      assert.ok(
        content.includes('Protects conversations beyond Antigravity\'s 500-session limit using filesystem hardlinks.'),
        'BrainCachePage vault copy must be humanized'
      )
      assert.ok(
        content.includes('Reclaims disk space from stale scratch files, step logs, and task outputs without touching active sessions.'),
        'BrainCachePage pruner copy must be humanized'
      )
    })

    it('verifies humanized tooltips and empty state in QuotaDashboardPage', () => {
      const content = fs.readFileSync(path.join(pagesDir, 'QuotaDashboardPage.tsx'), 'utf8')
      assert.ok(
        content.includes('Scan system for local Antigravity and Google accounts.'),
        'QuotaDashboardPage scan tooltip must be direct and humanized'
      )
      assert.ok(
        content.includes('Add and configure an account manually.'),
        'QuotaDashboardPage add account tooltip must be direct and humanized'
      )
      assert.ok(
        content.includes('No accounts configured. Scan local accounts or click Add Account to begin.'),
        'QuotaDashboardPage empty state copy must be humanized'
      )
    })
  })
})
